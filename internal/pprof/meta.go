package pprof

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

type SampleTypeInfo struct {
	Type string `json:"type"`
	Unit string `json:"unit"`
}

type SampleTotal struct {
	Type  string `json:"type"`
	Unit  string `json:"unit"`
	Total int64  `json:"total"`
}

type MetaResult struct {
	ProfilePath        string           `json:"profile_path"`
	DetectedKind       string           `json:"detected_profile_kind"`
	SampleTypes        []SampleTypeInfo `json:"sample_types"`
	DefaultSampleIndex int              `json:"default_sample_index"`
	Totals             []SampleTotal    `json:"totals"`
	PeriodType         *SampleTypeInfo  `json:"period_type,omitempty"`
	Period             int64            `json:"period,omitempty"`
	TimeNanos          int64            `json:"time_nanos,omitempty"`
	DurationNanos      int64            `json:"duration_nanos,omitempty"`
	LabelKeys          []string         `json:"label_keys,omitempty"`
	GoVersion          *string          `json:"go_version"`
	BuildID            *string          `json:"build_id"`
}

func RunMeta(profilePath string) (MetaResult, error) {
	file, err := os.Open(profilePath)
	if err != nil {
		return MetaResult{}, err
	}
	defer file.Close()

	prof, err := profile.Parse(file)
	if err != nil {
		return MetaResult{}, err
	}

	sampleTypes := make([]SampleTypeInfo, 0, len(prof.SampleType))
	for _, st := range prof.SampleType {
		sampleTypes = append(sampleTypes, SampleTypeInfo{Type: st.Type, Unit: st.Unit})
	}

	defaultIndex := -1
	if prof.DefaultSampleType != "" {
		for i, st := range prof.SampleType {
			if st.Type == prof.DefaultSampleType {
				defaultIndex = i
				break
			}
		}
	}
	if defaultIndex == -1 && len(prof.SampleType) > 0 {
		defaultIndex = 0
	}

	totals := make([]SampleTotal, len(prof.SampleType))
	for i, st := range prof.SampleType {
		totals[i] = SampleTotal{Type: st.Type, Unit: st.Unit, Total: 0}
	}
	for _, sample := range prof.Sample {
		for i, val := range sample.Value {
			if i < len(totals) {
				totals[i].Total += val
			}
		}
	}

	labelKeys := collectLabelKeys(prof.Sample)

	kind := detectKind(profilePath, prof)

	var periodType *SampleTypeInfo
	if prof.PeriodType != nil {
		periodType = &SampleTypeInfo{Type: prof.PeriodType.Type, Unit: prof.PeriodType.Unit}
	}

	goVersion := extractGoVersion(prof.Comments)
	buildID := extractBuildID(prof.Mapping)

	return MetaResult{
		ProfilePath:        profilePath,
		DetectedKind:       kind,
		SampleTypes:        sampleTypes,
		DefaultSampleIndex: defaultIndex,
		Totals:             totals,
		PeriodType:         periodType,
		Period:             prof.Period,
		TimeNanos:          prof.TimeNanos,
		DurationNanos:      prof.DurationNanos,
		LabelKeys:          labelKeys,
		GoVersion:          goVersion,
		BuildID:            buildID,
	}, nil
}

func detectKind(profilePath string, prof *profile.Profile) string {
	name := strings.ToLower(filepath.Base(profilePath))

	if strings.Contains(name, "cpu") {
		return "cpu"
	}
	if strings.Contains(name, "heap") {
		return "heap"
	}
	if strings.Contains(name, "mutex") {
		return "mutex"
	}
	if strings.Contains(name, "block") {
		return "block"
	}
	if strings.Contains(name, "goroutine") {
		return "goroutine"
	}

	for _, st := range prof.SampleType {
		if strings.Contains(st.Type, "alloc") || strings.Contains(st.Type, "inuse") {
			return "heap"
		}
		if st.Type == "goroutines" || st.Type == "goroutine" {
			return "goroutine"
		}
		if st.Type == "samples" && prof.Period > 0 {
			return "cpu"
		}
		if st.Type == "delay" || st.Type == "contentions" {
			return "mutex"
		}
	}

	return "unknown"
}

func collectLabelKeys(samples []*profile.Sample) []string {
	seen := map[string]struct{}{}
	for _, sample := range samples {
		for key := range sample.Label {
			seen[key] = struct{}{}
		}
		for key := range sample.NumLabel {
			seen[key] = struct{}{}
		}
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func extractGoVersion(comments []string) *string {
	for _, comment := range comments {
		lower := strings.ToLower(comment)
		if strings.Contains(lower, "go version") {
			return stringPtr(strings.TrimSpace(comment))
		}
	}
	return nil
}

func extractBuildID(mappings []*profile.Mapping) *string {
	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		if mapping.BuildID != "" {
			return stringPtr(mapping.BuildID)
		}
	}
	return nil
}

func stringPtr(value string) *string {
	return &value
}

func FormatMetaCommand(profilePath string) string {
	return fmt.Sprintf("profctl pprof meta --profile %s", profilePath)
}
