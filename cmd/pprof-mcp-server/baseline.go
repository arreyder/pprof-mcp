package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arreyder/pprof-mcp/internal/pprof"
	"github.com/arreyder/pprof-mcp/internal/pprofparse"
)

const (
	defaultBaselineFile = ".pprof-mcp-baselines.json"
)

type baselineStore struct {
	UpdatedAt string                    `json:"updated_at"`
	Entries   map[string]*baselineEntry `json:"entries"`
}

type baselineEntry struct {
	Key         string                       `json:"key"`
	ProfileKind string                       `json:"profile_kind"`
	SampleIndex string                       `json:"sample_index,omitempty"`
	UpdatedAt   string                       `json:"updated_at"`
	Samples     int                          `json:"samples"`
	Functions   map[string]*baselineFunction `json:"functions"`
}

type baselineFunction struct {
	AvgFlatPct float64 `json:"avg_flat_pct"`
	AvgCumPct  float64 `json:"avg_cum_pct"`
	Count      int     `json:"count"`
}

type baselineDeviation struct {
	Function string  `json:"function"`
	Metric   string  `json:"metric"`
	Current  float64 `json:"current"`
	Baseline float64 `json:"baseline"`
	Delta    float64 `json:"delta"`
	Severity string  `json:"severity"`
}

type baselineComparison struct {
	Key             string              `json:"key"`
	ProfileKind     string              `json:"profile_kind"`
	SampleIndex     string              `json:"sample_index,omitempty"`
	BaselineSamples int                 `json:"baseline_samples"`
	Deviations      []baselineDeviation `json:"deviations"`
	Warnings        []string            `json:"warnings,omitempty"`
}

func defaultBaselinePath() (string, error) {
	baseDir := strings.TrimSpace(os.Getenv("PPROF_MCP_BASEDIR"))
	if baseDir != "" {
		baseDir = filepath.Clean(baseDir)
		path := filepath.Join(baseDir, defaultBaselineFile)
		return sanitizePath(baseDir, path)
	}
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return defaultBaselineFile, nil
	}
	return filepath.Join(wd, defaultBaselineFile), nil
}

func loadBaselineStore(path string) (baselineStore, error) {
	store := baselineStore{
		Entries: map[string]*baselineEntry{},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, err
	}
	if len(data) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return store, err
	}
	if store.Entries == nil {
		store.Entries = map[string]*baselineEntry{}
	}
	return store, nil
}

func saveBaselineStore(path string, store baselineStore) error {
	if store.Entries == nil {
		store.Entries = map[string]*baselineEntry{}
	}
	store.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

func buildBaselineKey(service, env, baselineKey, profileKind, sampleIndex string) string {
	key := strings.TrimSpace(baselineKey)
	if key == "" {
		service = strings.TrimSpace(service)
		env = strings.TrimSpace(env)
		if service != "" || env != "" {
			key = fmt.Sprintf("%s:%s", service, env)
		} else {
			key = "default"
		}
	}
	if profileKind != "" {
		key += "|" + profileKind
	}
	if sampleIndex != "" {
		key += "|" + sampleIndex
	}
	return key
}

func compareAndUpdateBaseline(path, key, profileKind, sampleIndex string, rows []pprofparse.TopRow) (baselineComparison, error) {
	comparison := baselineComparison{
		Key:         key,
		ProfileKind: profileKind,
		SampleIndex: sampleIndex,
		Deviations:  []baselineDeviation{},
		Warnings:    []string{},
	}
	store, err := loadBaselineStore(path)
	if err != nil {
		return comparison, err
	}

	entry, exists := store.Entries[key]
	if !exists {
		entry = &baselineEntry{
			Key:         key,
			ProfileKind: profileKind,
			SampleIndex: sampleIndex,
			UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
			Samples:     0,
			Functions:   map[string]*baselineFunction{},
		}
		store.Entries[key] = entry
		comparison.Warnings = append(comparison.Warnings, "baseline initialized from current profile")
	}

	current := map[string]baselineFunction{}
	for _, row := range rows {
		current[row.Name] = baselineFunction{
			AvgFlatPct: pprof.ParsePercent(row.FlatPct),
			AvgCumPct:  pprof.ParsePercent(row.CumPct),
			Count:      1,
		}
	}

	if exists {
		for name, curr := range current {
			base := entry.Functions[name]
			if base == nil || base.Count == 0 {
				continue
			}
			comparison.Deviations = append(comparison.Deviations, diffMetrics(name, "flat_pct", curr.AvgFlatPct, base.AvgFlatPct)...)
			comparison.Deviations = append(comparison.Deviations, diffMetrics(name, "cum_pct", curr.AvgCumPct, base.AvgCumPct)...)
		}
	}

	for name, curr := range current {
		base := entry.Functions[name]
		if base == nil {
			entry.Functions[name] = &baselineFunction{
				AvgFlatPct: curr.AvgFlatPct,
				AvgCumPct:  curr.AvgCumPct,
				Count:      1,
			}
			continue
		}
		base.Count++
		base.AvgFlatPct = rollingAverage(base.AvgFlatPct, curr.AvgFlatPct, base.Count)
		base.AvgCumPct = rollingAverage(base.AvgCumPct, curr.AvgCumPct, base.Count)
	}
	entry.Samples++
	entry.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	comparison.BaselineSamples = entry.Samples

	if err := saveBaselineStore(path, store); err != nil {
		return comparison, err
	}

	return comparison, nil
}

func diffMetrics(name, metric string, current, baseline float64) []baselineDeviation {
	delta := current - baseline
	absDelta := absFloat(delta)
	if baseline == 0 && current < 2 {
		return nil
	}
	if absDelta < 2 && (baseline == 0 || absDelta/baseline < 0.3) {
		return nil
	}
	severity := "low"
	if absDelta >= 10 {
		severity = "high"
	} else if absDelta >= 5 {
		severity = "medium"
	}
	return []baselineDeviation{{
		Function: name,
		Metric:   metric,
		Current:  current,
		Baseline: baseline,
		Delta:    delta,
		Severity: severity,
	}}
}

func rollingAverage(currentAvg, newValue float64, count int) float64 {
	if count <= 1 {
		return newValue
	}
	return (currentAvg*float64(count-1) + newValue) / float64(count)
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
