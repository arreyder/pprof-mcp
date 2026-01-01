package pprof

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

const (
	defaultTopWaiters = 3
)

type ContentionAnalysisParams struct {
	Profile string
}

type ContentionAnalysisResult struct {
	ProfileType      string               `json:"profile_type"`
	TotalContentions int64                `json:"total_contentions"`
	TotalDelay       string               `json:"total_delay"`
	ByLockSite       []LockContentionSite `json:"by_lock_site"`
	Patterns         []ContentionPattern  `json:"patterns"`
	Recommendations  []string             `json:"recommendations"`
	Warnings         []string             `json:"warnings,omitempty"`
}

type LockContentionSite struct {
	LockSite       string             `json:"lock_site"`
	SourceLocation string             `json:"source_location,omitempty"`
	Contentions    int64              `json:"contentions"`
	TotalDelay     string             `json:"total_delay"`
	AvgDelay       string             `json:"avg_delay"`
	TopWaiters     []ContentionWaiter `json:"top_waiters"`
}

type ContentionWaiter struct {
	Function string `json:"function"`
	Delay    string `json:"delay"`
}

type ContentionPattern struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

type lockStats struct {
	lockSite       string
	sourceLocation string
	contentions    int64
	totalDelay     int64
	waiters        map[string]int64
}

type frameInfo struct {
	function string
	file     string
	line     int64
}

func RunContentionAnalysis(params ContentionAnalysisParams) (ContentionAnalysisResult, error) {
	result := ContentionAnalysisResult{
		ByLockSite:      []LockContentionSite{},
		Patterns:        []ContentionPattern{},
		Recommendations: []string{},
		Warnings:        []string{},
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

	result.ProfileType = detectContentionProfileType(params.Profile, prof)
	if result.ProfileType != "mutex" && result.ProfileType != "block" {
		result.Warnings = append(result.Warnings, "profile does not appear to be a mutex/block profile; results may be inaccurate")
	}

	delayIndex := findSampleIndexExact(prof, "delay")
	contentionsIndex := findSampleIndexExact(prof, "contentions")
	delayUnit := sampleUnit(prof, delayIndex, "nanoseconds")
	if delayIndex == -1 {
		result.Warnings = append(result.Warnings, "delay sample type not found; total_delay may be inaccurate")
	}
	if contentionsIndex == -1 {
		result.Warnings = append(result.Warnings, "contentions sample type not found; counts are approximated")
	}

	lockMap := map[string]*lockStats{}
	var totalDelay int64

	for _, sample := range prof.Sample {
		contentions := sampleValueInt64(sample, contentionsIndex)
		if contentionsIndex == -1 {
			contentions = 1
		}
		delay := sampleValueInt64(sample, delayIndex)

		if contentions == 0 && delay == 0 {
			continue
		}

		result.TotalContentions += contentions
		totalDelay += delay

		frames := sampleFrames(sample)
		lockSite, lockIndex := pickLockSite(frames)
		if lockSite == "" {
			continue
		}
		sourceLocation, waiterFunc := pickSourceAndWaiter(frames, lockIndex)

		key := lockSite
		if sourceLocation != "" {
			key = lockSite + "@" + sourceLocation
		}

		stats, ok := lockMap[key]
		if !ok {
			stats = &lockStats{
				lockSite:       lockSite,
				sourceLocation: sourceLocation,
				waiters:        map[string]int64{},
			}
			lockMap[key] = stats
		}
		stats.contentions += contentions
		stats.totalDelay += delay
		if waiterFunc != "" {
			stats.waiters[waiterFunc] += delay
		}
	}

	result.TotalDelay = formatValue(totalDelay, delayUnit)
	result.ByLockSite = buildLockSites(lockMap, delayUnit)
	result.Patterns = detectContentionPatterns(lockMap, result.TotalContentions)
	result.Recommendations = buildContentionRecommendations(result.Patterns, result.ByLockSite)

	return result, nil
}

func detectContentionProfileType(path string, prof *profile.Profile) string {
	name := strings.ToLower(filepath.Base(path))
	if strings.Contains(name, "block") {
		return "block"
	}
	if strings.Contains(name, "mutex") {
		return "mutex"
	}
	if prof == nil {
		return "unknown"
	}
	for _, st := range prof.SampleType {
		if st.Type == "delay" || st.Type == "contentions" {
			return "mutex"
		}
	}
	return "unknown"
}

func findSampleIndexExact(prof *profile.Profile, name string) int {
	if prof == nil {
		return -1
	}
	for i, st := range prof.SampleType {
		if st.Type == name {
			return i
		}
	}
	return -1
}

func sampleValueInt64(sample *profile.Sample, idx int) int64 {
	if sample == nil || idx < 0 {
		return 0
	}
	if idx < len(sample.Value) {
		return sample.Value[idx]
	}
	if len(sample.Value) > 0 {
		return sample.Value[0]
	}
	return 0
}

func sampleUnit(prof *profile.Profile, idx int, fallback string) string {
	if prof != nil && idx >= 0 && idx < len(prof.SampleType) {
		if unit := prof.SampleType[idx].Unit; unit != "" {
			return unit
		}
	}
	if fallback != "" {
		return fallback
	}
	return "nanoseconds"
}

func sampleFrames(sample *profile.Sample) []frameInfo {
	frames := []frameInfo{}
	if sample == nil {
		return frames
	}
	for _, loc := range sample.Location {
		if loc == nil {
			continue
		}
		for _, line := range loc.Line {
			if line.Function == nil || line.Function.Name == "" {
				continue
			}
			frame := frameInfo{
				function: line.Function.Name,
				file:     line.Function.Filename,
				line:     int64(line.Line),
			}
			frames = append(frames, frame)
			break
		}
	}
	return frames
}

func pickLockSite(frames []frameInfo) (string, int) {
	for i, frame := range frames {
		lower := strings.ToLower(frame.function)
		switch {
		case strings.Contains(lower, "sync.(*mutex).lock"),
			strings.Contains(lower, "sync.(*rwmutex)."),
			strings.Contains(lower, "runtime.semacquire"),
			strings.Contains(lower, "runtime.semacquiremutex"):
			return frame.function, i
		}
	}
	if len(frames) > 0 {
		return frames[0].function, 0
	}
	return "", -1
}

func pickSourceAndWaiter(frames []frameInfo, lockIndex int) (string, string) {
	sourceLocation := ""
	waiterFunc := ""
	start := lockIndex + 1
	if start < 0 {
		start = 0
	}
	for i := start; i < len(frames); i++ {
		if isRuntimeFrame(frames[i].function) {
			continue
		}
		waiterFunc = frames[i].function
		if frames[i].file != "" && frames[i].line > 0 {
			sourceLocation = fmt.Sprintf("%s:%d", frames[i].file, frames[i].line)
		}
		break
	}
	if sourceLocation == "" && lockIndex >= 0 && lockIndex < len(frames) {
		if frames[lockIndex].file != "" && frames[lockIndex].line > 0 {
			sourceLocation = fmt.Sprintf("%s:%d", frames[lockIndex].file, frames[lockIndex].line)
		}
	}
	if waiterFunc == "" && len(frames) > 0 {
		waiterFunc = frames[0].function
	}
	return sourceLocation, waiterFunc
}

func isRuntimeFrame(name string) bool {
	return strings.HasPrefix(name, "runtime.") || strings.HasPrefix(name, "sync.")
}

func buildLockSites(lockMap map[string]*lockStats, delayUnit string) []LockContentionSite {
	ordered := make([]*lockStats, 0, len(lockMap))
	for _, stats := range lockMap {
		ordered = append(ordered, stats)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].totalDelay > ordered[j].totalDelay
	})

	items := make([]LockContentionSite, 0, len(ordered))
	for _, stats := range ordered {
		avgDelay := int64(0)
		if stats.contentions > 0 {
			avgDelay = stats.totalDelay / stats.contentions
		}
		items = append(items, LockContentionSite{
			LockSite:       stats.lockSite,
			SourceLocation: stats.sourceLocation,
			Contentions:    stats.contentions,
			TotalDelay:     formatValue(stats.totalDelay, delayUnit),
			AvgDelay:       formatValue(avgDelay, delayUnit),
			TopWaiters:     buildTopWaiters(stats.waiters, delayUnit, defaultTopWaiters),
		})
	}
	return items
}

func buildTopWaiters(waiters map[string]int64, delayUnit string, limit int) []ContentionWaiter {
	type waiterStat struct {
		function string
		delay    int64
	}
	list := make([]waiterStat, 0, len(waiters))
	for name, delay := range waiters {
		list = append(list, waiterStat{function: name, delay: delay})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].delay > list[j].delay
	})
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	out := make([]ContentionWaiter, 0, len(list))
	for _, w := range list {
		out = append(out, ContentionWaiter{
			Function: w.function,
			Delay:    formatValue(w.delay, delayUnit),
		})
	}
	return out
}

func detectContentionPatterns(lockMap map[string]*lockStats, totalContentions int64) []ContentionPattern {
	patterns := []ContentionPattern{}
	if len(lockMap) == 0 {
		return patterns
	}
	type lockEntry struct {
		key   string
		delay int64
	}
	entries := make([]lockEntry, 0, len(lockMap))
	var totalDelay int64
	for key, stats := range lockMap {
		totalDelay += stats.totalDelay
		entries = append(entries, lockEntry{key: key, delay: stats.totalDelay})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].delay > entries[j].delay
	})

	if totalDelay > 0 {
		top := entries[0]
		topPct := float64(top.delay) / float64(totalDelay) * 100
		if topPct >= 35 {
			patterns = append(patterns, ContentionPattern{
				Type:        "hot_lock",
				Severity:    "high",
				Description: fmt.Sprintf("Single lock accounts for %.1f%% of total delay", topPct),
			})
		}
		if len(entries) >= 3 {
			top3 := entries[0].delay + entries[1].delay + entries[2].delay
			top3Pct := float64(top3) / float64(totalDelay) * 100
			if top3Pct >= 70 {
				patterns = append(patterns, ContentionPattern{
					Type:        "lock_convoy",
					Severity:    "medium",
					Description: fmt.Sprintf("Top 3 locks account for %.1f%% of total delay", top3Pct),
				})
			}
		}
	}
	if totalContentions >= 50000 {
		patterns = append(patterns, ContentionPattern{
			Type:        "high_contention",
			Severity:    "medium",
			Description: fmt.Sprintf("High contention count (%d)", totalContentions),
		})
	}

	return patterns
}

func buildContentionRecommendations(patterns []ContentionPattern, lockSites []LockContentionSite) []string {
	recs := []string{}
	for _, pattern := range patterns {
		switch pattern.Type {
		case "hot_lock":
			if len(lockSites) > 0 {
				recs = append(recs, fmt.Sprintf("Consider sharding or reducing critical sections around %s (%s).", lockSites[0].LockSite, lockSites[0].SourceLocation))
			} else {
				recs = append(recs, "Consider sharding or reducing critical sections around hot locks.")
			}
		case "lock_convoy":
			recs = append(recs, "Multiple locks show high contention; review lock ordering and reduce time spent in critical sections.")
		case "high_contention":
			recs = append(recs, "High contention volume detected; consider batching or lock-free data structures where possible.")
		}
	}
	return recs
}

// formatValue is defined in overhead_detect.go.
