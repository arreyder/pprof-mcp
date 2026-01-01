package pprof

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/pprof/profile"

	"github.com/arreyder/pprof-mcp/internal/pprofparse"
)

type DiscoveryParams struct {
	Service        string
	Env            string
	Timestamp      string
	Profiles       []DiscoveryProfileInput
	RepoPrefixes   []string
	ContainerRSSMB int
}

type DiscoveryProfileInput struct {
	Type   string
	Path   string
	Handle string
	Bytes  int64
}

type DiscoveryReport struct {
	Service         string                    `json:"service"`
	Env             string                    `json:"env"`
	Timestamp       string                    `json:"timestamp"`
	Profiles        []DiscoveryProfile        `json:"profiles,omitempty"`
	CPU             *DiscoveryCPU             `json:"cpu,omitempty"`
	Heap            *DiscoveryHeap            `json:"heap,omitempty"`
	Mutex           *DiscoveryMutex           `json:"mutex,omitempty"`
	Goroutine       *GoroutineAnalysisResult  `json:"goroutine,omitempty"`
	Recommendations []DiscoveryRecommendation `json:"recommendations,omitempty"`
	Warnings        []string                  `json:"warnings,omitempty"`
}

type DiscoveryProfile struct {
	Type   string `json:"type"`
	Handle string `json:"handle"`
	Bytes  int64  `json:"bytes"`
}

type DiscoveryCPU struct {
	UtilizationPct float64             `json:"utilization_pct"`
	TopFunctions   []pprofparse.TopRow `json:"top_functions"`
	Overhead       OverheadReport      `json:"overhead"`
	Hints          []string            `json:"hints,omitempty"`
}

type DiscoveryHeap struct {
	AllocRate    string             `json:"alloc_rate,omitempty"`
	TopPaths     []AllocPath        `json:"top_paths"`
	MemorySanity MemorySanityResult `json:"memory_sanity"`
}

type DiscoveryMutex struct {
	TotalDelay     string              `json:"total_delay"`
	TopContentions []pprofparse.TopRow `json:"top_contentions"`
}

type DiscoveryRecommendation struct {
	Priority   string `json:"priority"`
	Area       string `json:"area"`
	Suggestion string `json:"suggestion"`
}

func RunDiscovery(ctx context.Context, params DiscoveryParams) (DiscoveryReport, error) {
	if params.Service == "" || params.Env == "" {
		return DiscoveryReport{}, fmt.Errorf("service and env are required")
	}

	report := DiscoveryReport{
		Service:         params.Service,
		Env:             params.Env,
		Timestamp:       params.Timestamp,
		Recommendations: []DiscoveryRecommendation{},
		Warnings:        []string{},
	}
	if report.Timestamp == "" {
		report.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	profileMap := map[string]DiscoveryProfileInput{}
	for _, prof := range params.Profiles {
		profileMap[prof.Type] = prof
		if prof.Handle != "" {
			report.Profiles = append(report.Profiles, DiscoveryProfile{
				Type:   prof.Type,
				Handle: prof.Handle,
				Bytes:  prof.Bytes,
			})
		}
	}
	sort.Slice(report.Profiles, func(i, j int) bool {
		return report.Profiles[i].Type < report.Profiles[j].Type
	})

	report.CPU = analyzeCPU(ctx, profileMap["cpu"], &report, params)
	report.Heap = analyzeHeap(ctx, profileMap["heap"], profileMap["goroutines"], &report, params)
	report.Mutex = analyzeMutex(ctx, pickMutexProfile(profileMap), &report)
	report.Goroutine = analyzeGoroutines(profileMap["goroutines"], &report)

	report.Recommendations = dedupeRecommendations(report.Recommendations)
	return report, nil
}

func analyzeCPU(ctx context.Context, prof DiscoveryProfileInput, report *DiscoveryReport, params DiscoveryParams) *DiscoveryCPU {
	if prof.Path == "" {
		report.Warnings = append(report.Warnings, "cpu profile missing from bundle")
		return nil
	}
	parsed, err := parseProfile(prof.Path)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("cpu profile parse failed: %v", err))
		return nil
	}

	utilizationPct := cpuUtilizationPct(parsed)
	top, err := RunTop(ctx, TopParams{
		Profile:   prof.Path,
		NodeCount: 15,
	})
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("cpu top failed: %v", err))
	}

	sampleIndex := findSampleTypeIndex(parsed, []string{"cpu", "samples"})
	overhead := DetectOverhead(parsed, sampleIndex)
	hints := GenerateProfileHints(prof.Path, "")

	cpu := &DiscoveryCPU{
		UtilizationPct: utilizationPct,
		TopFunctions:   top.Rows,
		Overhead:       overhead,
		Hints:          hints,
	}

	if overhead.TotalOverhead >= 20 {
		addRecommendation(report, "high", "Observability overhead",
			fmt.Sprintf("Observability/infrastructure accounts for %.1f%% of CPU. Review instrumentation and sampling.", overhead.TotalOverhead))
	}
	for _, detection := range overhead.Detections {
		if detection.Severity == "high" && detection.Suggestion != "" {
			addRecommendation(report, "high", detection.Category, detection.Suggestion)
		}
	}

	return cpu
}

func analyzeHeap(ctx context.Context, prof DiscoveryProfileInput, goroutines DiscoveryProfileInput, report *DiscoveryReport, params DiscoveryParams) *DiscoveryHeap {
	if prof.Path == "" {
		report.Warnings = append(report.Warnings, "heap profile missing from bundle")
		return nil
	}

	allocPaths, err := RunAllocPaths(AllocPathsParams{
		Profile:      prof.Path,
		MinPercent:   1.0,
		MaxPaths:     15,
		RepoPrefixes: params.RepoPrefixes,
	})
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("alloc_paths failed: %v", err))
	}

	memorySanity, err := RunMemorySanity(ctx, MemorySanityParams{
		HeapProfile:      prof.Path,
		GoroutineProfile: goroutines.Path,
		ContainerRSSMB:   params.ContainerRSSMB,
	})
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("memory_sanity failed: %v", err))
	}

	allocRate := ""
	if allocPaths.TotalAlloc > 0 && allocPaths.DurationSecs > 0 {
		bytesPerMin := float64(allocPaths.TotalAlloc) / allocPaths.DurationSecs * 60
		allocRate = formatValue(int64(bytesPerMin), "bytes") + "/min"
	}

	for _, suspicion := range memorySanity.Suspicions {
		if suggestion := suspicionToRecommendation(suspicion); suggestion != "" {
			priority := severityToPriority(suspicion.Severity)
			addRecommendation(report, priority, suspicion.Category, suggestion)
		}
	}
	for _, rec := range memorySanity.Recommendations {
		addRecommendation(report, "medium", "Memory", rec)
	}

	return &DiscoveryHeap{
		AllocRate:    allocRate,
		TopPaths:     allocPaths.Paths,
		MemorySanity: memorySanity,
	}
}

func analyzeMutex(ctx context.Context, prof DiscoveryProfileInput, report *DiscoveryReport) *DiscoveryMutex {
	if prof.Path == "" {
		report.Warnings = append(report.Warnings, "mutex/block profile missing from bundle")
		return nil
	}
	parsed, err := parseProfile(prof.Path)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("mutex profile parse failed: %v", err))
		return nil
	}

	sampleName := pickSampleName(parsed, []string{"delay", "contentions"})
	top, err := RunTop(ctx, TopParams{
		Profile:     prof.Path,
		NodeCount:   10,
		SampleIndex: sampleName,
	})
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("mutex top failed: %v", err))
	}

	totalDelay := ""
	if sampleName != "" {
		index := findSampleTypeIndex(parsed, []string{sampleName})
		if index >= 0 && index < len(parsed.SampleType) {
			total := sampleTotal(parsed, index)
			totalDelay = formatValue(total, parsed.SampleType[index].Unit)
		}
	}

	return &DiscoveryMutex{
		TotalDelay:     totalDelay,
		TopContentions: top.Rows,
	}
}

func analyzeGoroutines(prof DiscoveryProfileInput, report *DiscoveryReport) *GoroutineAnalysisResult {
	if prof.Path == "" {
		report.Warnings = append(report.Warnings, "goroutine profile missing from bundle")
		return nil
	}
	result, err := RunGoroutineAnalysis(GoroutineAnalysisParams{Profile: prof.Path})
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("goroutine_analysis failed: %v", err))
		return nil
	}

	for _, leak := range result.PotentialLeaks {
		if leak.Severity == "high" {
			addRecommendation(report, "high", "Goroutines",
				fmt.Sprintf("Potential goroutine leak: %d goroutines share stack %q.", leak.Count, leak.StackSignature))
		}
	}

	return &result
}

func pickMutexProfile(profileMap map[string]DiscoveryProfileInput) DiscoveryProfileInput {
	if prof, ok := profileMap["mutex"]; ok {
		return prof
	}
	return profileMap["block"]
}

func cpuUtilizationPct(prof *profile.Profile) float64 {
	if prof == nil || prof.DurationNanos <= 0 {
		return 0
	}

	cpuIndex := findSampleTypeIndex(prof, []string{"cpu"})
	totalCPU := int64(0)
	if cpuIndex >= 0 && cpuIndex < len(prof.SampleType) && prof.SampleType[cpuIndex].Type == "cpu" {
		totalCPU = sampleTotal(prof, cpuIndex)
	} else if prof.Period > 0 && prof.PeriodType != nil && prof.PeriodType.Type == "cpu" {
		sampleIndex := findSampleTypeIndex(prof, []string{"samples"})
		totalCPU = sampleTotal(prof, sampleIndex) * prof.Period
	}
	if totalCPU == 0 {
		return 0
	}
	return float64(totalCPU) / float64(prof.DurationNanos) * 100
}

func sampleTotal(prof *profile.Profile, index int) int64 {
	if prof == nil || index < 0 {
		return 0
	}
	var total int64
	for _, sample := range prof.Sample {
		if index < len(sample.Value) {
			total += sample.Value[index]
		}
	}
	return total
}

func pickSampleName(prof *profile.Profile, candidates []string) string {
	for _, name := range candidates {
		for _, st := range prof.SampleType {
			if st.Type == name {
				return name
			}
		}
	}
	return ""
}

func addRecommendation(report *DiscoveryReport, priority, area, suggestion string) {
	if report == nil || suggestion == "" {
		return
	}
	report.Recommendations = append(report.Recommendations, DiscoveryRecommendation{
		Priority:   priority,
		Area:       area,
		Suggestion: strings.TrimSpace(suggestion),
	})
}

func dedupeRecommendations(items []DiscoveryRecommendation) []DiscoveryRecommendation {
	seen := map[string]struct{}{}
	deduped := make([]DiscoveryRecommendation, 0, len(items))
	for _, rec := range items {
		key := strings.ToLower(rec.Priority + "|" + rec.Area + "|" + rec.Suggestion)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, rec)
	}
	return deduped
}

func severityToPriority(severity string) string {
	switch strings.ToLower(severity) {
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

func suspicionToRecommendation(suspicion Suspicion) string {
	if suspicion.Description != "" && suspicion.Evidence != "" {
		return fmt.Sprintf("%s (%s)", suspicion.Description, suspicion.Evidence)
	}
	if suspicion.Description != "" {
		return suspicion.Description
	}
	return ""
}
