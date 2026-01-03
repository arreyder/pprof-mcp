package pprof

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

// TemporalAnalysisParams configures Temporal SDK worker analysis.
type TemporalAnalysisParams struct {
	Profile string
}

// TemporalAnalysisResult contains inferred Temporal worker configuration.
type TemporalAnalysisResult struct {
	// Inferred settings
	InferredSettings TemporalInferredSettings `json:"inferred_settings"`

	// Raw counts for verification
	Counts TemporalCounts `json:"counts"`

	// Workflow breakdown by type
	WorkflowBreakdown []TemporalWorkflowType `json:"workflow_breakdown"`

	// Activity breakdown
	ActivityBreakdown []TemporalActivityType `json:"activity_breakdown"`

	// Task queues detected
	TaskQueues []string `json:"task_queues,omitempty"`

	// Total goroutines in profile
	TotalGoroutines int `json:"total_goroutines"`

	// Warnings during analysis
	Warnings []string `json:"warnings,omitempty"`
}

// TemporalInferredSettings contains inferred worker configuration values.
type TemporalInferredSettings struct {
	// Pollers
	MaxConcurrentActivityTaskPollers int `json:"max_concurrent_activity_task_pollers"`
	MaxConcurrentWorkflowTaskPollers int `json:"max_concurrent_workflow_task_pollers"`

	// Execution limits (observed, not necessarily the configured max)
	ActiveActivities        int `json:"active_activities"`
	CachedWorkflows         int `json:"cached_workflows"`
	ActiveLocalActivities   int `json:"active_local_activities"`
	ActiveSessions          int `json:"active_sessions"`

	// Notes about inference confidence
	Notes []string `json:"notes,omitempty"`
}

// TemporalCounts contains raw goroutine counts by category.
type TemporalCounts struct {
	// Pollers
	ActivityPollersDoPoll    int `json:"activity_pollers_do_poll"`
	ActivityPollersInGRPC    int `json:"activity_pollers_in_grpc"`
	WorkflowPollersDoPoll    int `json:"workflow_pollers_do_poll"`
	WorkflowPollersInGRPC    int `json:"workflow_pollers_in_grpc"`
	LocalActivityPollers     int `json:"local_activity_pollers"`

	// Executors
	ActivitiesExecuting      int `json:"activities_executing"`
	WorkflowsCached          int `json:"workflows_cached"`
	LocalActivitiesExecuting int `json:"local_activities_executing"`
	SessionsActive           int `json:"sessions_active"`

	// Infrastructure
	HeartbeatGoroutines int `json:"heartbeat_goroutines"`
	GRPCStreams         int `json:"grpc_streams"`
	TaskDispatchers     int `json:"task_dispatchers"`
	EagerDispatchers    int `json:"eager_dispatchers"`
}

// TemporalWorkflowType represents a workflow type with counts.
type TemporalWorkflowType struct {
	Name        string `json:"name"`
	Count       int    `json:"count"`
	State       string `json:"state"` // "selector", "future", "executing"
	SampleStack string `json:"sample_stack,omitempty"`
}

// TemporalActivityType represents an activity type with counts.
type TemporalActivityType struct {
	Name        string `json:"name"`
	Count       int    `json:"count"`
	SampleStack string `json:"sample_stack,omitempty"`
}

// Patterns for detecting Temporal SDK goroutines
var temporalPatterns = struct {
	// Pollers
	activityPollerDoPoll  *regexp.Regexp
	activityPollerGRPC    *regexp.Regexp
	workflowPollerDoPoll  *regexp.Regexp
	workflowPollerGRPC    *regexp.Regexp
	localActivityPoller   *regexp.Regexp

	// Executors
	activityProcessTask   *regexp.Regexp
	workflowCoroutine     *regexp.Regexp
	localActivityExecute  *regexp.Regexp
	sessionWorker         *regexp.Regexp

	// Infrastructure
	heartbeat             *regexp.Regexp
	grpcReadLoop          *regexp.Regexp
	taskDispatcher        *regexp.Regexp
	eagerDispatcher       *regexp.Regexp

	// Workflow extraction
	workflowFunc          *regexp.Regexp
	activityFunc          *regexp.Regexp
}{
	activityPollerDoPoll:  regexp.MustCompile(`activityTaskPoller.*PollTask|basePoller.*doPoll.*activityTaskPoller`),
	activityPollerGRPC:    regexp.MustCompile(`PollActivityTaskQueue`),
	workflowPollerDoPoll:  regexp.MustCompile(`workflowTaskPoller.*PollTask|basePoller.*doPoll.*workflowTaskPoller`),
	workflowPollerGRPC:    regexp.MustCompile(`PollWorkflowTaskQueue`),
	localActivityPoller:   regexp.MustCompile(`localActivityTaskPoller.*PollTask`),

	activityProcessTask:   regexp.MustCompile(`activityTaskPoller.*ProcessTask`),
	workflowCoroutine:     regexp.MustCompile(`coroutineState.*(?:initialYield|yield)|syncWorkflowDefinition.*Execute`),
	localActivityExecute:  regexp.MustCompile(`localActivityTaskPoller.*ProcessTask`),
	sessionWorker:         regexp.MustCompile(`sessionEnvironmentImpl`),

	heartbeat:             regexp.MustCompile(`temporalInvoker.*Heartbeat|internal\.heartbeat`),
	grpcReadLoop:          regexp.MustCompile(`http2Client.*reader|http2.*readLoop`),
	taskDispatcher:        regexp.MustCompile(`baseWorker.*runTaskDispatcher`),
	eagerDispatcher:       regexp.MustCompile(`baseWorker.*runEagerTaskDispatcher`),

	workflowFunc:          regexp.MustCompile(`([a-zA-Z0-9_/.-]+)\.((?:[A-Z][a-zA-Z0-9]*)?Workflow[A-Za-z0-9]*)`),
	activityFunc:          regexp.MustCompile(`([a-zA-Z0-9_/.-]+)\.([A-Z][a-zA-Z0-9]*Activity[A-Za-z0-9]*|[a-zA-Z0-9]*[Aa]ctivity)`),
}

// RunTemporalAnalysis analyzes a goroutine profile for Temporal SDK patterns.
func RunTemporalAnalysis(params TemporalAnalysisParams) (TemporalAnalysisResult, error) {
	result := TemporalAnalysisResult{
		WorkflowBreakdown:  []TemporalWorkflowType{},
		ActivityBreakdown:  []TemporalActivityType{},
		TaskQueues:         []string{},
		Warnings:           []string{},
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

	sampleIndex := findSampleTypeIndex(prof, []string{"goroutine", "goroutines"})

	// Track workflow types and activity types
	workflowTypes := make(map[string]*workflowInfo)
	activityTypes := make(map[string]*activityInfo)

	for _, sample := range prof.Sample {
		count := sampleValue(sample, sampleIndex)
		if count <= 0 {
			count = 1
		}
		result.TotalGoroutines += count

		stack := stackFrames(sample)
		stackStr := strings.Join(stack, " | ")

		// Check each pattern
		if temporalPatterns.activityPollerDoPoll.MatchString(stackStr) {
			result.Counts.ActivityPollersDoPoll += count
		}
		if temporalPatterns.activityPollerGRPC.MatchString(stackStr) {
			result.Counts.ActivityPollersInGRPC += count
		}
		if temporalPatterns.workflowPollerDoPoll.MatchString(stackStr) {
			result.Counts.WorkflowPollersDoPoll += count
		}
		if temporalPatterns.workflowPollerGRPC.MatchString(stackStr) {
			result.Counts.WorkflowPollersInGRPC += count
		}
		if temporalPatterns.localActivityPoller.MatchString(stackStr) {
			result.Counts.LocalActivityPollers += count
		}

		if temporalPatterns.activityProcessTask.MatchString(stackStr) {
			result.Counts.ActivitiesExecuting += count
			// Try to extract activity name
			if name := extractActivityName(stack); name != "" {
				if info, ok := activityTypes[name]; ok {
					info.count += count
				} else {
					activityTypes[name] = &activityInfo{
						count:       count,
						sampleStack: stackSignature(stack, 6),
					}
				}
			}
		}

		if temporalPatterns.workflowCoroutine.MatchString(stackStr) {
			result.Counts.WorkflowsCached += count
			// Extract workflow name and state
			name, state := extractWorkflowInfo(stack)
			if name != "" {
				key := name + ":" + state
				if info, ok := workflowTypes[key]; ok {
					info.count += count
				} else {
					workflowTypes[key] = &workflowInfo{
						name:        name,
						state:       state,
						count:       count,
						sampleStack: stackSignature(stack, 8),
					}
				}
			}
		}

		if temporalPatterns.localActivityExecute.MatchString(stackStr) {
			result.Counts.LocalActivitiesExecuting += count
		}
		if temporalPatterns.sessionWorker.MatchString(stackStr) {
			result.Counts.SessionsActive += count
		}
		if temporalPatterns.heartbeat.MatchString(stackStr) {
			result.Counts.HeartbeatGoroutines += count
		}
		if temporalPatterns.grpcReadLoop.MatchString(stackStr) {
			result.Counts.GRPCStreams += count
		}
		if temporalPatterns.taskDispatcher.MatchString(stackStr) {
			result.Counts.TaskDispatchers += count
		}
		if temporalPatterns.eagerDispatcher.MatchString(stackStr) {
			result.Counts.EagerDispatchers += count
		}
	}

	// Infer settings
	result.InferredSettings = inferTemporalSettings(result.Counts)

	// Build workflow breakdown
	for _, info := range workflowTypes {
		result.WorkflowBreakdown = append(result.WorkflowBreakdown, TemporalWorkflowType{
			Name:        info.name,
			Count:       info.count,
			State:       info.state,
			SampleStack: info.sampleStack,
		})
	}
	sort.Slice(result.WorkflowBreakdown, func(i, j int) bool {
		return result.WorkflowBreakdown[i].Count > result.WorkflowBreakdown[j].Count
	})

	// Build activity breakdown
	for name, info := range activityTypes {
		result.ActivityBreakdown = append(result.ActivityBreakdown, TemporalActivityType{
			Name:        name,
			Count:       info.count,
			SampleStack: info.sampleStack,
		})
	}
	sort.Slice(result.ActivityBreakdown, func(i, j int) bool {
		return result.ActivityBreakdown[i].Count > result.ActivityBreakdown[j].Count
	})

	return result, nil
}

type workflowInfo struct {
	name        string
	state       string
	count       int
	sampleStack string
}

type activityInfo struct {
	count       int
	sampleStack string
}

func inferTemporalSettings(counts TemporalCounts) TemporalInferredSettings {
	settings := TemporalInferredSettings{
		Notes: []string{},
	}

	// Infer activity pollers
	// The max of doPoll and inGRPC gives us the poller count
	// (some may be in doPoll waiting, others in active gRPC call)
	activityPollers := max(counts.ActivityPollersDoPoll, counts.ActivityPollersInGRPC)
	if activityPollers > 0 {
		settings.MaxConcurrentActivityTaskPollers = activityPollers
		if activityPollers == 2 {
			settings.Notes = append(settings.Notes, "Activity pollers appear to use default (2)")
		} else {
			settings.Notes = append(settings.Notes, fmt.Sprintf("Activity pollers configured to %d (non-default)", activityPollers))
		}
	}

	// Infer workflow pollers
	workflowPollers := max(counts.WorkflowPollersDoPoll, counts.WorkflowPollersInGRPC)
	if workflowPollers > 0 {
		settings.MaxConcurrentWorkflowTaskPollers = workflowPollers
		if workflowPollers == 2 {
			settings.Notes = append(settings.Notes, "Workflow pollers appear to use default (2)")
		} else {
			settings.Notes = append(settings.Notes, fmt.Sprintf("Workflow pollers configured to %d (non-default)", workflowPollers))
		}
	}

	// Active counts (observed, not max configured)
	settings.ActiveActivities = counts.ActivitiesExecuting
	settings.CachedWorkflows = counts.WorkflowsCached
	settings.ActiveLocalActivities = counts.LocalActivitiesExecuting
	settings.ActiveSessions = counts.SessionsActive

	// Add notes about utilization
	if counts.ActivitiesExecuting > 0 {
		settings.Notes = append(settings.Notes, fmt.Sprintf("%d activities currently executing", counts.ActivitiesExecuting))
	}
	if counts.WorkflowsCached > 0 {
		settings.Notes = append(settings.Notes, fmt.Sprintf("%d workflows cached (sticky cache)", counts.WorkflowsCached))
	}
	if counts.HeartbeatGoroutines > 0 {
		settings.Notes = append(settings.Notes, fmt.Sprintf("%d heartbeat goroutines active", counts.HeartbeatGoroutines))
	}

	return settings
}

func extractWorkflowInfo(stack []string) (name string, state string) {
	state = "unknown"

	// Determine state from stack
	stackStr := strings.Join(stack, " ")
	switch {
	case strings.Contains(stackStr, "selectorImpl.Select"):
		state = "selector"
	case strings.Contains(stackStr, "decodeFutureImpl.Get") || strings.Contains(stackStr, "channelImpl.Receive"):
		state = "awaiting_future"
	case strings.Contains(stackStr, "Execute"):
		state = "executing"
	}

	// Find workflow function name
	for _, frame := range stack {
		// Skip SDK internals
		if strings.Contains(frame, "go.temporal.io/sdk") {
			continue
		}
		// Look for workflow functions
		if strings.Contains(frame, "Workflow") || strings.Contains(frame, "workflow") {
			// Extract just the function name
			parts := strings.Split(frame, ".")
			if len(parts) >= 2 {
				name = parts[len(parts)-1]
				// Clean up generic type parameters
				if idx := strings.Index(name, "["); idx > 0 {
					name = name[:idx]
				}
				return name, state
			}
			return frame, state
		}
	}

	return "", state
}

func extractActivityName(stack []string) string {
	for _, frame := range stack {
		// Skip SDK internals
		if strings.Contains(frame, "go.temporal.io/sdk") {
			continue
		}
		// Look for activity functions
		if strings.Contains(frame, "Activity") || strings.Contains(frame, "activity") {
			parts := strings.Split(frame, ".")
			if len(parts) >= 2 {
				name := parts[len(parts)-1]
				// Clean up generic type parameters
				if idx := strings.Index(name, "["); idx > 0 {
					name = name[:idx]
				}
				return name
			}
			return frame
		}
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
