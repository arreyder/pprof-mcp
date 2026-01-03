package pprof

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemporalAnalysis(t *testing.T) {
	// Skip if test profile doesn't exist
	profilePath := os.ExpandEnv("$HOME/inc-503/temporal_sync_prod-usw2_goroutines.pprof")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Skip("test profile not found at", profilePath)
	}

	result, err := RunTemporalAnalysis(TemporalAnalysisParams{
		Profile: profilePath,
	})
	require.NoError(t, err)

	t.Logf("Total goroutines: %d", result.TotalGoroutines)
	t.Logf("Inferred settings:")
	t.Logf("  Activity pollers: %d", result.InferredSettings.MaxConcurrentActivityTaskPollers)
	t.Logf("  Workflow pollers: %d", result.InferredSettings.MaxConcurrentWorkflowTaskPollers)
	t.Logf("  Cached workflows: %d", result.InferredSettings.CachedWorkflows)
	t.Logf("  Active activities: %d", result.InferredSettings.ActiveActivities)
	t.Logf("Raw counts:")
	t.Logf("  Activity pollers doPoll: %d", result.Counts.ActivityPollersDoPoll)
	t.Logf("  Activity pollers gRPC: %d", result.Counts.ActivityPollersInGRPC)
	t.Logf("  Workflow pollers doPoll: %d", result.Counts.WorkflowPollersDoPoll)
	t.Logf("  Workflow pollers gRPC: %d", result.Counts.WorkflowPollersInGRPC)
	t.Logf("  Workflows cached: %d", result.Counts.WorkflowsCached)
	t.Logf("  Activities executing: %d", result.Counts.ActivitiesExecuting)
	t.Logf("Workflow breakdown (%d types):", len(result.WorkflowBreakdown))
	for i, wf := range result.WorkflowBreakdown {
		if i < 10 {
			t.Logf("  %s [%s]: %d", wf.Name, wf.State, wf.Count)
		}
	}
	t.Logf("Notes:")
	for _, note := range result.InferredSettings.Notes {
		t.Logf("  - %s", note)
	}

	// Basic assertions
	require.Greater(t, result.TotalGoroutines, 0)
}
