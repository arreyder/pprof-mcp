package pprof

import (
	"context"
	"fmt"
	"sort"
)

const (
	defaultCorrelationNodeCount = 20
	defaultCorrelationTopOnly   = 5
)

type CrossCorrelateParams struct {
	Profiles  map[string]string
	NodeCount int
}

type CrossCorrelateResult struct {
	Correlations      []CorrelationEntry `json:"correlations"`
	CPUOnlyHotspots   []CPUHotspot       `json:"cpu_only_hotspots"`
	HeapOnlyHotspots  []HeapHotspot      `json:"heap_only_hotspots"`
	MutexOnlyHotspots []MutexHotspot     `json:"mutex_only_hotspots"`
	Warnings          []string           `json:"warnings,omitempty"`
}

type CorrelationEntry struct {
	Function      string            `json:"function"`
	CombinedScore float64           `json:"combined_score"`
	CPU           *CorrelationCPU   `json:"cpu,omitempty"`
	Heap          *CorrelationHeap  `json:"heap,omitempty"`
	Mutex         *CorrelationMutex `json:"mutex,omitempty"`
	Insight       string            `json:"insight"`
}

type CorrelationCPU struct {
	FlatPct float64 `json:"flat_pct"`
	Rank    int     `json:"rank"`
}

type CorrelationHeap struct {
	AllocPct float64 `json:"alloc_pct"`
	Rank     int     `json:"rank"`
}

type CorrelationMutex struct {
	DelayPct float64 `json:"delay_pct"`
	Rank     int     `json:"rank"`
}

type CPUHotspot struct {
	Function string  `json:"function"`
	FlatPct  float64 `json:"flat_pct"`
}

type HeapHotspot struct {
	Function string  `json:"function"`
	AllocPct float64 `json:"alloc_pct"`
}

type MutexHotspot struct {
	Function string  `json:"function"`
	DelayPct float64 `json:"delay_pct"`
}

type topMetric struct {
	name string
	pct  float64
	rank int
}

func RunCrossCorrelate(ctx context.Context, params CrossCorrelateParams) (CrossCorrelateResult, error) {
	result := CrossCorrelateResult{
		Correlations:      []CorrelationEntry{},
		CPUOnlyHotspots:   []CPUHotspot{},
		HeapOnlyHotspots:  []HeapHotspot{},
		MutexOnlyHotspots: []MutexHotspot{},
		Warnings:          []string{},
	}
	if len(params.Profiles) == 0 {
		return result, fmt.Errorf("profiles are required")
	}

	nodeCount := params.NodeCount
	if nodeCount <= 0 {
		nodeCount = defaultCorrelationNodeCount
	}

	cpuTop, cpuWarn := runTopMetrics(ctx, params.Profiles["cpu"], nodeCount, "")
	if cpuWarn != "" {
		result.Warnings = append(result.Warnings, cpuWarn)
	}
	heapPath := params.Profiles["heap"]
	heapSampleIndex, heapWarn := pickHeapSampleIndex(heapPath)
	if heapWarn != "" {
		result.Warnings = append(result.Warnings, heapWarn)
	}
	heapTop, heapTopWarn := runTopMetrics(ctx, heapPath, nodeCount, heapSampleIndex)
	if heapTopWarn != "" {
		result.Warnings = append(result.Warnings, heapTopWarn)
	}
	mutexPath := params.Profiles["mutex"]
	if mutexPath == "" {
		mutexPath = params.Profiles["block"]
	}
	mutexSampleIndex, mutexWarn := pickMutexSampleIndex(mutexPath)
	if mutexWarn != "" {
		result.Warnings = append(result.Warnings, mutexWarn)
	}
	mutexTop, mutexTopWarn := runTopMetrics(ctx, mutexPath, nodeCount, mutexSampleIndex)
	if mutexTopWarn != "" {
		result.Warnings = append(result.Warnings, mutexTopWarn)
	}

	metrics := map[string]*CorrelationEntry{}
	maxRankCPU := maxRank(cpuTop)
	maxRankHeap := maxRank(heapTop)
	maxRankMutex := maxRank(mutexTop)

	for _, item := range cpuTop {
		entry := ensureEntry(metrics, item.name)
		entry.CPU = &CorrelationCPU{FlatPct: item.pct, Rank: item.rank}
	}
	for _, item := range heapTop {
		entry := ensureEntry(metrics, item.name)
		entry.Heap = &CorrelationHeap{AllocPct: item.pct, Rank: item.rank}
	}
	for _, item := range mutexTop {
		entry := ensureEntry(metrics, item.name)
		entry.Mutex = &CorrelationMutex{DelayPct: item.pct, Rank: item.rank}
	}

	for _, entry := range metrics {
		score, count := 0.0, 0
		if entry.CPU != nil {
			score += rankScore(entry.CPU.Rank, maxRankCPU)
			count++
		}
		if entry.Heap != nil {
			score += rankScore(entry.Heap.Rank, maxRankHeap)
			count++
		}
		if entry.Mutex != nil {
			score += rankScore(entry.Mutex.Rank, maxRankMutex)
			count++
		}
		if count < 2 {
			continue
		}
		entry.CombinedScore = score / float64(count)
		entry.Insight = correlationInsight(entry)
		result.Correlations = append(result.Correlations, *entry)
	}

	sort.Slice(result.Correlations, func(i, j int) bool {
		return result.Correlations[i].CombinedScore > result.Correlations[j].CombinedScore
	})

	result.CPUOnlyHotspots = buildCPUOnly(cpuTop, metrics)
	result.HeapOnlyHotspots = buildHeapOnly(heapTop, metrics)
	result.MutexOnlyHotspots = buildMutexOnly(mutexTop, metrics)

	return result, nil
}

func runTopMetrics(ctx context.Context, profilePath string, nodeCount int, sampleIndex string) ([]topMetric, string) {
	if profilePath == "" {
		return nil, "profile missing for correlation"
	}
	topResult, err := RunTop(ctx, TopParams{
		Profile:     profilePath,
		NodeCount:   nodeCount,
		SampleIndex: sampleIndex,
	})
	if err != nil {
		return nil, fmt.Sprintf("pprof top failed for %s: %v", profilePath, err)
	}

	metrics := make([]topMetric, 0, len(topResult.Rows))
	for idx, row := range topResult.Rows {
		pct := parsePercent(row.FlatPct)
		if pct == 0 {
			continue
		}
		metrics = append(metrics, topMetric{
			name: row.Name,
			pct:  pct,
			rank: idx + 1,
		})
	}
	return metrics, ""
}

func ensureEntry(metrics map[string]*CorrelationEntry, name string) *CorrelationEntry {
	if entry, ok := metrics[name]; ok {
		return entry
	}
	entry := &CorrelationEntry{Function: name}
	metrics[name] = entry
	return entry
}

func rankScore(rank int, maxRank int) float64 {
	if maxRank <= 0 || rank <= 0 {
		return 0
	}
	if rank > maxRank {
		rank = maxRank
	}
	return float64(maxRank-rank+1) / float64(maxRank)
}

func correlationInsight(entry *CorrelationEntry) string {
	if entry == nil {
		return ""
	}
	hasCPU := entry.CPU != nil
	hasHeap := entry.Heap != nil
	hasMutex := entry.Mutex != nil
	switch {
	case hasCPU && hasHeap && hasMutex:
		return "High CPU, allocations, and lock contention - prime optimization target."
	case hasCPU && hasHeap:
		return "Hot in CPU and allocations."
	case hasCPU && hasMutex:
		return "Hot in CPU and contention."
	case hasHeap && hasMutex:
		return "Hot in allocations and contention."
	default:
		return "Hotspot appears in multiple profiles."
	}
}

func buildCPUOnly(cpuTop []topMetric, metrics map[string]*CorrelationEntry) []CPUHotspot {
	out := []CPUHotspot{}
	for _, item := range cpuTop {
		entry := metrics[item.name]
		if entry == nil || (entry.Heap == nil && entry.Mutex == nil) {
			out = append(out, CPUHotspot{Function: item.name, FlatPct: item.pct})
		}
		if len(out) >= defaultCorrelationTopOnly {
			break
		}
	}
	return out
}

func buildHeapOnly(heapTop []topMetric, metrics map[string]*CorrelationEntry) []HeapHotspot {
	out := []HeapHotspot{}
	for _, item := range heapTop {
		entry := metrics[item.name]
		if entry == nil || (entry.CPU == nil && entry.Mutex == nil) {
			out = append(out, HeapHotspot{Function: item.name, AllocPct: item.pct})
		}
		if len(out) >= defaultCorrelationTopOnly {
			break
		}
	}
	return out
}

func buildMutexOnly(mutexTop []topMetric, metrics map[string]*CorrelationEntry) []MutexHotspot {
	out := []MutexHotspot{}
	for _, item := range mutexTop {
		entry := metrics[item.name]
		if entry == nil || (entry.CPU == nil && entry.Heap == nil) {
			out = append(out, MutexHotspot{Function: item.name, DelayPct: item.pct})
		}
		if len(out) >= defaultCorrelationTopOnly {
			break
		}
	}
	return out
}

func maxRank(metrics []topMetric) int {
	max := 0
	for _, item := range metrics {
		if item.rank > max {
			max = item.rank
		}
	}
	return max
}

func pickHeapSampleIndex(profilePath string) (string, string) {
	if profilePath == "" {
		return "", "heap profile missing"
	}
	prof, err := parseProfile(profilePath)
	if err != nil {
		return "", fmt.Sprintf("heap profile parse failed: %v", err)
	}
	for _, st := range prof.SampleType {
		if st.Type == "alloc_space" {
			return "alloc_space", ""
		}
	}
	if prof.DefaultSampleType != "" {
		return prof.DefaultSampleType, ""
	}
	return "", ""
}

func pickMutexSampleIndex(profilePath string) (string, string) {
	if profilePath == "" {
		return "", "mutex profile missing"
	}
	prof, err := parseProfile(profilePath)
	if err != nil {
		return "", fmt.Sprintf("mutex profile parse failed: %v", err)
	}
	for _, st := range prof.SampleType {
		if st.Type == "delay" {
			return "delay", ""
		}
	}
	for _, st := range prof.SampleType {
		if st.Type == "contentions" {
			return "contentions", ""
		}
	}
	if prof.DefaultSampleType != "" {
		return prof.DefaultSampleType, ""
	}
	return "", ""
}
