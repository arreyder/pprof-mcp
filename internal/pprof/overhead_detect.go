package pprof

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

// OverheadCategory represents a category of observability/infrastructure overhead.
type OverheadCategory struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Patterns    []string `json:"-"` // Function name patterns to match
}

// OverheadDetection represents detected overhead in a profile.
type OverheadDetection struct {
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Value       int64   `json:"value"`
	ValueStr    string  `json:"value_str"`
	Percentage  float64 `json:"percentage"`
	TopFuncs    []string `json:"top_functions,omitempty"`
	Severity    string  `json:"severity"` // "low", "medium", "high"
	Suggestion  string  `json:"suggestion,omitempty"`
}

// OverheadReport contains the full overhead analysis.
type OverheadReport struct {
	ProfileKind    string              `json:"profile_kind"`
	TotalValue     int64               `json:"total_value"`
	TotalValueStr  string              `json:"total_value_str"`
	Unit           string              `json:"unit"`
	Detections     []OverheadDetection `json:"detections"`
	TotalOverhead  float64             `json:"total_overhead_pct"`
	Warnings       []string            `json:"warnings,omitempty"`
}

var overheadCategories = []OverheadCategory{
	{
		Name:        "OpenTelemetry Tracing",
		Description: "OTel span creation, attributes, and export",
		Patterns: []string{
			"go.opentelemetry.io/otel",
			"opentelemetry",
		},
	},
	{
		Name:        "Logging (zap)",
		Description: "Zap logger allocations and writes",
		Patterns: []string{
			"go.uber.org/zap",
			"zapcore",
		},
	},
	{
		Name:        "Logging (logrus)",
		Description: "Logrus logger allocations",
		Patterns: []string{
			"github.com/sirupsen/logrus",
		},
	},
	{
		Name:        "Prometheus Metrics",
		Description: "Prometheus metric collection and export",
		Patterns: []string{
			"github.com/prometheus/",
			"prometheus/client_golang",
		},
	},
	{
		Name:        "gRPC Framework",
		Description: "gRPC infrastructure (interceptors, encoding, transport)",
		Patterns: []string{
			"google.golang.org/grpc",
			"grpc-ecosystem",
		},
	},
	{
		Name:        "Protobuf Serialization",
		Description: "Protocol buffer marshaling/unmarshaling",
		Patterns: []string{
			"google.golang.org/protobuf",
			"github.com/golang/protobuf",
		},
	},
	{
		Name:        "JSON Serialization",
		Description: "JSON encoding/decoding",
		Patterns: []string{
			"encoding/json",
			"github.com/json-iterator",
			"github.com/goccy/go-json",
		},
	},
	{
		Name:        "HTTP Framework",
		Description: "HTTP server/client infrastructure",
		Patterns: []string{
			"net/http",
			"golang.org/x/net/http2",
		},
	},
	{
		Name:        "Context Operations",
		Description: "Context value storage and propagation",
		Patterns: []string{
			"context.WithValue",
			"context.WithCancel",
			"context.WithDeadline",
		},
	},
	{
		Name:        "Runtime/GC",
		Description: "Go runtime and garbage collection",
		Patterns: []string{
			"runtime.mallocgc",
			"runtime.gcBgMarkWorker",
			"runtime.scanobject",
			"runtime.markroot",
		},
	},
}

// DetectOverhead analyzes a profile for observability and infrastructure overhead.
func DetectOverhead(prof *profile.Profile, sampleIndex int) OverheadReport {
	report := OverheadReport{
		ProfileKind: detectProfileKind(prof),
		Detections:  []OverheadDetection{},
		Warnings:    []string{},
	}

	if sampleIndex >= len(prof.SampleType) {
		sampleIndex = 0
	}

	st := prof.SampleType[sampleIndex]
	report.Unit = st.Unit

	// Calculate total
	var total int64
	for _, sample := range prof.Sample {
		if sampleIndex < len(sample.Value) {
			total += sample.Value[sampleIndex]
		}
	}
	report.TotalValue = total
	report.TotalValueStr = formatValue(total, st.Unit)

	if total == 0 {
		report.Warnings = append(report.Warnings, "profile has no samples")
		return report
	}

	// Aggregate by category
	categoryValues := make(map[string]int64)
	categoryFuncs := make(map[string]map[string]int64)

	for _, sample := range prof.Sample {
		value := int64(0)
		if sampleIndex < len(sample.Value) {
			value = sample.Value[sampleIndex]
		}
		if value == 0 {
			continue
		}

		// Check each location in the stack
		for _, loc := range sample.Location {
			for _, line := range loc.Line {
				if line.Function == nil {
					continue
				}
				funcName := line.Function.Name

				for _, cat := range overheadCategories {
					if matchesCategory(funcName, cat) {
						categoryValues[cat.Name] += value
						if categoryFuncs[cat.Name] == nil {
							categoryFuncs[cat.Name] = make(map[string]int64)
						}
						categoryFuncs[cat.Name][funcName] += value
						break // Only count once per category per sample
					}
				}
			}
		}
	}

	// Build detections
	var totalOverhead float64
	for _, cat := range overheadCategories {
		value := categoryValues[cat.Name]
		if value == 0 {
			continue
		}

		pct := float64(value) / float64(total) * 100
		if pct < 1.0 {
			continue // Skip insignificant overhead
		}

		totalOverhead += pct

		// Get top functions for this category
		funcs := categoryFuncs[cat.Name]
		topFuncs := getTopFuncs(funcs, 3)

		detection := OverheadDetection{
			Category:    cat.Name,
			Description: cat.Description,
			Value:       value,
			ValueStr:    formatValue(value, st.Unit),
			Percentage:  pct,
			TopFuncs:    topFuncs,
			Severity:    getSeverity(pct),
			Suggestion:  getSuggestion(cat.Name, pct),
		}
		report.Detections = append(report.Detections, detection)
	}

	// Sort by percentage descending
	sort.Slice(report.Detections, func(i, j int) bool {
		return report.Detections[i].Percentage > report.Detections[j].Percentage
	})

	report.TotalOverhead = totalOverhead

	// Add warnings for high overhead
	if totalOverhead > 30 {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("High observability overhead: %.1f%% of profile is infrastructure/observability code", totalOverhead))
	}

	return report
}

func matchesCategory(funcName string, cat OverheadCategory) bool {
	for _, pattern := range cat.Patterns {
		if strings.Contains(funcName, pattern) {
			return true
		}
	}
	return false
}

func detectProfileKind(prof *profile.Profile) string {
	for _, st := range prof.SampleType {
		switch st.Type {
		case "alloc_space", "alloc_objects", "inuse_space", "inuse_objects":
			return "heap"
		case "samples", "cpu":
			return "cpu"
		case "goroutines", "goroutine":
			return "goroutine"
		case "delay", "contentions":
			return "mutex"
		}
	}
	return "unknown"
}

func formatValue(value int64, unit string) string {
	switch unit {
	case "bytes":
		if value >= 1<<30 {
			return fmt.Sprintf("%.2fGB", float64(value)/(1<<30))
		} else if value >= 1<<20 {
			return fmt.Sprintf("%.2fMB", float64(value)/(1<<20))
		} else if value >= 1<<10 {
			return fmt.Sprintf("%.2fKB", float64(value)/(1<<10))
		}
		return fmt.Sprintf("%dB", value)
	case "nanoseconds":
		if value >= 1e9 {
			return fmt.Sprintf("%.2fs", float64(value)/1e9)
		} else if value >= 1e6 {
			return fmt.Sprintf("%.2fms", float64(value)/1e6)
		} else if value >= 1e3 {
			return fmt.Sprintf("%.2fus", float64(value)/1e3)
		}
		return fmt.Sprintf("%dns", value)
	default:
		return fmt.Sprintf("%d %s", value, unit)
	}
}

func getTopFuncs(funcs map[string]int64, n int) []string {
	type funcVal struct {
		name  string
		value int64
	}
	var sorted []funcVal
	for name, value := range funcs {
		sorted = append(sorted, funcVal{name, value})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].value > sorted[j].value
	})

	var result []string
	for i := 0; i < n && i < len(sorted); i++ {
		result = append(result, sorted[i].name)
	}
	return result
}

func getSeverity(pct float64) string {
	if pct >= 15 {
		return "high"
	} else if pct >= 5 {
		return "medium"
	}
	return "low"
}

func getSuggestion(category string, pct float64) string {
	if pct < 5 {
		return ""
	}

	suggestions := map[string]string{
		"OpenTelemetry Tracing": "Consider reducing trace sampling rate or limiting span attributes",
		"Logging (zap)":         "Consider adjusting log level or using sampling for high-frequency logs",
		"Logging (logrus)":      "Consider switching to zap for better performance, or reduce log verbosity",
		"Prometheus Metrics":    "Review metric cardinality; high-cardinality labels cause memory growth",
		"gRPC Framework":        "This is typically unavoidable for gRPC services; focus on application code",
		"Protobuf Serialization": "Consider message pooling or lazy unmarshaling for large messages",
		"JSON Serialization":    "Consider using json-iterator or code generation for hot paths",
		"Context Operations":    "Reduce context.WithValue usage; consider alternative patterns for passing data",
		"Runtime/GC":            "High GC overhead suggests allocation pressure; review allocation hot spots",
	}

	if suggestion, ok := suggestions[category]; ok && pct >= 10 {
		return suggestion
	}
	return ""
}
