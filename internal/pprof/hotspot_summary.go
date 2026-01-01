package pprof

import (
	"context"
	"fmt"
)

const defaultHotspotCount = 5

type HotspotSummaryParams struct {
	Profiles  map[string]string
	NodeCount int
}

type HotspotSummaryResult struct {
	CPUTop5        []CPUHotspot   `json:"cpu_top5,omitempty"`
	HeapTop5       []HeapHotspot  `json:"heap_top5,omitempty"`
	MutexTop5      []MutexHotspot `json:"mutex_top5,omitempty"`
	GoroutineCount *int           `json:"goroutine_count,omitempty"`
	Warnings       []string       `json:"warnings,omitempty"`
}

func RunHotspotSummary(ctx context.Context, params HotspotSummaryParams) (HotspotSummaryResult, error) {
	result := HotspotSummaryResult{
		CPUTop5:   []CPUHotspot{},
		HeapTop5:  []HeapHotspot{},
		MutexTop5: []MutexHotspot{},
		Warnings:  []string{},
	}
	if len(params.Profiles) == 0 {
		return result, fmt.Errorf("profiles are required")
	}

	nodeCount := params.NodeCount
	if nodeCount <= 0 {
		nodeCount = defaultHotspotCount
	}

	if cpuPath := params.Profiles["cpu"]; cpuPath != "" {
		cpuTop, warn := runTopMetrics(ctx, cpuPath, nodeCount, "")
		if warn != "" {
			result.Warnings = append(result.Warnings, warn)
		}
		result.CPUTop5 = topCPUHotspots(cpuTop)
	} else {
		result.Warnings = append(result.Warnings, "cpu profile missing from bundle")
	}

	if heapPath := params.Profiles["heap"]; heapPath != "" {
		heapIndex, warn := pickHeapSampleIndex(heapPath)
		if warn != "" {
			result.Warnings = append(result.Warnings, warn)
		}
		heapTop, warn := runTopMetrics(ctx, heapPath, nodeCount, heapIndex)
		if warn != "" {
			result.Warnings = append(result.Warnings, warn)
		}
		result.HeapTop5 = topHeapHotspots(heapTop)
	} else {
		result.Warnings = append(result.Warnings, "heap profile missing from bundle")
	}

	mutexPath := params.Profiles["mutex"]
	if mutexPath == "" {
		mutexPath = params.Profiles["block"]
	}
	if mutexPath != "" {
		mutexIndex, warn := pickMutexSampleIndex(mutexPath)
		if warn != "" {
			result.Warnings = append(result.Warnings, warn)
		}
		mutexTop, warn := runTopMetrics(ctx, mutexPath, nodeCount, mutexIndex)
		if warn != "" {
			result.Warnings = append(result.Warnings, warn)
		}
		result.MutexTop5 = topMutexHotspots(mutexTop)
	} else {
		result.Warnings = append(result.Warnings, "mutex/block profile missing from bundle")
	}

	if goroutinePath := params.Profiles["goroutines"]; goroutinePath != "" {
		analysis, err := RunGoroutineAnalysis(GoroutineAnalysisParams{Profile: goroutinePath})
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("goroutine analysis failed: %v", err))
		} else {
			count := analysis.TotalGoroutines
			result.GoroutineCount = &count
		}
	}

	return result, nil
}

func topCPUHotspots(metrics []topMetric) []CPUHotspot {
	out := []CPUHotspot{}
	for _, item := range metrics {
		out = append(out, CPUHotspot{Function: item.name, FlatPct: item.pct})
	}
	return out
}

func topHeapHotspots(metrics []topMetric) []HeapHotspot {
	out := []HeapHotspot{}
	for _, item := range metrics {
		out = append(out, HeapHotspot{Function: item.name, AllocPct: item.pct})
	}
	return out
}

func topMutexHotspots(metrics []topMetric) []MutexHotspot {
	out := []MutexHotspot{}
	for _, item := range metrics {
		out = append(out, MutexHotspot{Function: item.name, DelayPct: item.pct})
	}
	return out
}
