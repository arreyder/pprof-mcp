package pprof

import (
	"os"
	"strings"

	"github.com/google/pprof/profile"
)

// GenerateProfileHints generates contextual hints based on profile type and analysis.
func GenerateProfileHints(profilePath string, usedSampleIndex string) []string {
	hints := []string{}

	file, err := os.Open(profilePath)
	if err != nil {
		return hints
	}
	defer file.Close()

	prof, err := profile.Parse(file)
	if err != nil {
		return hints
	}

	kind := detectProfileKind(prof)

	// Heap profile hints
	if kind == "heap" {
		hasAllocSpace := false
		hasInuseSpace := false
		for _, st := range prof.SampleType {
			if st.Type == "alloc_space" {
				hasAllocSpace = true
			}
			if st.Type == "inuse_space" {
				hasInuseSpace = true
			}
		}

		if hasAllocSpace && hasInuseSpace {
			if usedSampleIndex == "" || usedSampleIndex == "inuse_space" {
				hints = append(hints,
					"This is a heap profile. Use sample_index='alloc_space' with peek/storylines to see allocation hot spots (where memory is being allocated), not just in-use memory.")
			}
			if usedSampleIndex == "alloc_space" {
				hints = append(hints,
					"Showing allocation hot spots. Use pprof.alloc_paths for detailed allocation path analysis with filtering.")
			}
		}

		// Suggest memory_sanity for heap profiles
		hints = append(hints,
			"Use pprof.memory_sanity to detect RSS vs heap mismatches (SQLite temp_store, CGO, high goroutine stacks).")

		// Suggest storylines for comprehensive view
		hints = append(hints,
			"Use pprof.storylines for a high-level view of where allocations happen in your code.")
	}

	// CPU profile hints
	if kind == "cpu" {
		hints = append(hints,
			"Use cum=true to sort by cumulative time (time in function + callees) to find top-level hot paths.")
		hints = append(hints,
			"Use pprof.storylines for a high-level view of hot code paths in your code.")
		hints = append(hints,
			"Use pprof.overhead_report to identify infrastructure/observability overhead.")
	}

	// Mutex/block profile hints
	if kind == "mutex" || kind == "block" {
		hints = append(hints,
			"Mutex/block profiles show contention. Look for sync.Mutex, sync.RWMutex, and channel operations.")
	}

	// Goroutine profile hints
	if kind == "goroutine" {
		hints = append(hints,
			"Goroutine profile shows stack traces. Look for goroutine leaks (growing count) or blocked goroutines.")
	}

	return hints
}

// GenerateOverheadHints generates hints based on detected overhead.
func GenerateOverheadHints(report OverheadReport) []string {
	hints := []string{}

	if report.TotalOverhead > 20 {
		hints = append(hints,
			"High observability overhead detected. Use pprof.overhead_report for detailed analysis and recommendations.")
	}

	for _, d := range report.Detections {
		if d.Severity == "high" && d.Suggestion != "" {
			hints = append(hints, d.Suggestion)
		}
	}

	return hints
}

// AddTopHints adds contextual hints to a TopResult based on profile analysis.
func AddTopHints(result *TopResult, profilePath string, usedSampleIndex string) {
	hints := GenerateProfileHints(profilePath, usedSampleIndex)

	// Check for common patterns in the results
	hasOtelAllocs := false
	hasLoggingAllocs := false

	for _, row := range result.Rows {
		if strings.Contains(row.Name, "opentelemetry") || strings.Contains(row.Name, "otel") {
			hasOtelAllocs = true
		}
		if strings.Contains(row.Name, "zap") || strings.Contains(row.Name, "logrus") {
			hasLoggingAllocs = true
		}
	}

	if hasOtelAllocs || hasLoggingAllocs {
		hints = append(hints,
			"Significant observability overhead detected. Use pprof.overhead_report for detailed analysis.")
	}

	result.Hints = hints
}
