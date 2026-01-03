package pprof

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

// GoroutineCategorizeParams configures goroutine categorization.
type GoroutineCategorizeParams struct {
	Profile    string
	Categories map[string]string // name -> regex pattern
	Presets    []string          // preset category groups to include
}

// GoroutineCategorizeResult contains categorized goroutine counts.
type GoroutineCategorizeResult struct {
	TotalGoroutines int                          `json:"total_goroutines"`
	Categories      []GoroutineCategory          `json:"categories"`
	Uncategorized   int                          `json:"uncategorized"`
	TopUncategorized []GoroutineUncategorized    `json:"top_uncategorized,omitempty"`
	PresetsUsed     []string                     `json:"presets_used,omitempty"`
	Warnings        []string                     `json:"warnings,omitempty"`
}

// GoroutineCategory represents a category with its count.
type GoroutineCategory struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Count       int    `json:"count"`
	Percent     float64 `json:"percent"`
	SampleStack string `json:"sample_stack,omitempty"`
}

// GoroutineUncategorized represents an uncategorized stack signature.
type GoroutineUncategorized struct {
	Signature string `json:"signature"`
	Count     int    `json:"count"`
}

// Preset category groups
var categoryPresets = map[string]map[string]string{
	"temporal": {
		"temporal_activity_poller":   `activityTaskPoller.*(PollTask|doPoll)`,
		"temporal_workflow_poller":   `workflowTaskPoller.*(PollTask|doPoll)`,
		"temporal_activity_exec":     `activityTaskPoller.*ProcessTask`,
		"temporal_workflow_cached":   `coroutineState\.(initialYield|yield)`,
		"temporal_local_activity":    `localActivityTaskPoller`,
		"temporal_heartbeat":         `temporalInvoker.*Heartbeat|internal\.heartbeat`,
		"temporal_task_dispatcher":   `baseWorker.*runTaskDispatcher`,
		"temporal_eager_dispatcher":  `baseWorker.*runEagerTaskDispatcher`,
	},
	"grpc": {
		"grpc_server_handler":     `grpc\..*\.Serve|grpc\.handleStream`,
		"grpc_client_stream":      `grpc\..*clientStream|ClientConn.*Invoke`,
		"grpc_http2_reader":       `http2Client.*reader|http2.*readLoop`,
		"grpc_http2_writer":       `loopyWriter.*run`,
		"grpc_keepalive":          `http2Client.*keepalive`,
		"grpc_callback_serializer": `grpcsync\..*CallbackSerializer`,
	},
	"http": {
		"http_server":        `http\..*Serve|http\.serverHandler`,
		"http_client":        `http\..*RoundTrip|persistConn\.readLoop`,
		"http2_client":       `http2\..*ClientConn|http2\..*readLoop`,
	},
	"database": {
		"sql_connection":     `database/sql\.(.*DB|.*Conn)`,
		"postgres":           `pgx|pq\.|lib/pq`,
		"mongodb":            `mongo-driver`,
		"redis":              `go-redis|redigo`,
	},
	"runtime": {
		"runtime_gc":         `runtime\.gc|runtime\.bgscavenge`,
		"runtime_sysmon":     `runtime\.sysmon`,
		"runtime_netpoll":    `runtime\.netpoll`,
		"runtime_timer":      `runtime\.timerproc|runtime\.runTimer`,
		"signal_handler":     `os/signal\.loop|signal_recv`,
	},
	"sync": {
		"sync_mutex":      `sync\.\(.*Mutex\)`,
		"sync_cond":       `sync\.\(.*Cond\)`,
		"sync_waitgroup":  `sync\.\(.*WaitGroup\)`,
		"sync_pool":       `sync\.Pool`,
		"channel_recv":    `runtime\.chanrecv`,
		"channel_send":    `runtime\.chansend`,
		"select":          `runtime\.selectgo`,
	},
	"observability": {
		"datadog_profiler":   `dd-trace-go.*profiler`,
		"datadog_tracer":     `dd-trace-go.*tracer`,
		"otel_exporter":      `opentelemetry.*exporter`,
		"prometheus":         `prometheus.*`,
	},
}

// RunGoroutineCategorize categorizes goroutines by patterns.
func RunGoroutineCategorize(params GoroutineCategorizeParams) (GoroutineCategorizeResult, error) {
	result := GoroutineCategorizeResult{
		Categories:       []GoroutineCategory{},
		TopUncategorized: []GoroutineUncategorized{},
		PresetsUsed:      []string{},
		Warnings:         []string{},
	}

	if params.Profile == "" {
		return result, fmt.Errorf("profile is required")
	}

	file, err := os.Open(params.Profile)
	if err != nil {
		return result, err
	}
	defer file.Close()

	prof, err := profile.Parse(file)
	if err != nil {
		return result, err
	}

	if detectProfileKind(prof) != "goroutine" {
		result.Warnings = append(result.Warnings, "profile does not appear to be a goroutine profile; results may be inaccurate")
	}

	// Build category patterns from presets and custom categories
	categories := make(map[string]*categoryMatcher)

	// Add presets first
	for _, presetName := range params.Presets {
		if preset, ok := categoryPresets[presetName]; ok {
			result.PresetsUsed = append(result.PresetsUsed, presetName)
			for name, pattern := range preset {
				re, err := regexp.Compile(pattern)
				if err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("invalid pattern for %s: %v", name, err))
					continue
				}
				categories[name] = &categoryMatcher{
					pattern: pattern,
					re:      re,
				}
			}
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("unknown preset: %s", presetName))
		}
	}

	// Add custom categories (override presets if same name)
	for name, pattern := range params.Categories {
		re, err := regexp.Compile(pattern)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("invalid pattern for %s: %v", name, err))
			continue
		}
		categories[name] = &categoryMatcher{
			pattern: pattern,
			re:      re,
		}
	}

	// If no categories specified, use a sensible default
	if len(categories) == 0 {
		// Use all presets
		for presetName, preset := range categoryPresets {
			result.PresetsUsed = append(result.PresetsUsed, presetName)
			for name, pattern := range preset {
				re, err := regexp.Compile(pattern)
				if err != nil {
					continue
				}
				categories[name] = &categoryMatcher{
					pattern: pattern,
					re:      re,
				}
			}
		}
		sort.Strings(result.PresetsUsed)
	}

	sampleIndex := findSampleTypeIndex(prof, []string{"goroutine", "goroutines"})
	uncategorizedStacks := make(map[string]int)

	for _, sample := range prof.Sample {
		count := sampleValue(sample, sampleIndex)
		if count <= 0 {
			count = 1
		}
		result.TotalGoroutines += count

		stack := stackFrames(sample)
		stackStr := strings.Join(stack, " | ")

		matched := false
		for _, matcher := range categories {
			if matcher.re.MatchString(stackStr) {
				matcher.count += count
				if matcher.sampleStack == "" {
					matcher.sampleStack = stackSignature(stack, 6)
				}
				matched = true
				break // Only count in first matching category
			}
		}

		if !matched {
			result.Uncategorized += count
			sig := stackSignature(stack, 4)
			if sig != "" {
				uncategorizedStacks[sig] += count
			}
		}
	}

	// Build category results
	for name, matcher := range categories {
		if matcher.count > 0 {
			result.Categories = append(result.Categories, GoroutineCategory{
				Name:        name,
				Pattern:     matcher.pattern,
				Count:       matcher.count,
				Percent:     float64(matcher.count) / float64(result.TotalGoroutines) * 100,
				SampleStack: matcher.sampleStack,
			})
		}
	}
	sort.Slice(result.Categories, func(i, j int) bool {
		return result.Categories[i].Count > result.Categories[j].Count
	})

	// Build top uncategorized
	for sig, count := range uncategorizedStacks {
		result.TopUncategorized = append(result.TopUncategorized, GoroutineUncategorized{
			Signature: sig,
			Count:     count,
		})
	}
	sort.Slice(result.TopUncategorized, func(i, j int) bool {
		return result.TopUncategorized[i].Count > result.TopUncategorized[j].Count
	})
	if len(result.TopUncategorized) > 10 {
		result.TopUncategorized = result.TopUncategorized[:10]
	}

	return result, nil
}

type categoryMatcher struct {
	pattern     string
	re          *regexp.Regexp
	count       int
	sampleStack string
}

// ListCategoryPresets returns available preset names.
func ListCategoryPresets() []string {
	presets := make([]string, 0, len(categoryPresets))
	for name := range categoryPresets {
		presets = append(presets, name)
	}
	sort.Strings(presets)
	return presets
}

// GetCategoryPreset returns patterns for a preset.
func GetCategoryPreset(name string) (map[string]string, bool) {
	preset, ok := categoryPresets[name]
	return preset, ok
}
