package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ListProfilesParams struct {
	Service string
	Env     string
	From    string
	To      string
	Hours   int
	Limit   int
	Site    string
}

type ProfileCandidate struct {
	ProfileID     string             `json:"profile_id"`
	EventID       string             `json:"event_id"`
	Timestamp     string             `json:"timestamp"`
	NumericFields map[string]float64 `json:"numeric_fields,omitempty"`
}

type ListProfilesResult struct {
	Service    string             `json:"service"`
	Env        string             `json:"env"`
	DDSite     string             `json:"dd_site"`
	FromTS     string             `json:"from_ts"`
	ToTS       string             `json:"to_ts"`
	Limit      int                `json:"limit"`
	Candidates []ProfileCandidate `json:"candidates"`
	Warnings   []string           `json:"warnings,omitempty"`
}

func ListProfiles(ctx context.Context, params ListProfilesParams) (ListProfilesResult, error) {
	if params.Service == "" || params.Env == "" {
		return ListProfilesResult{}, fmt.Errorf("service and env are required")
	}

	site := params.Site
	if site == "" {
		site = os.Getenv("DD_SITE")
	}
	if site == "" {
		site = defaultSite
	}

	fromTS, toTS, warnings := resolveTimeWindow(params.From, params.To, params.Hours)
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}

	apiKey, appKey, err := loadKeys()
	if err != nil {
		return ListProfilesResult{}, err
	}

	payload := map[string]any{
		"filter": map[string]any{
			"from":  fromTS,
			"to":    toTS,
			"query": fmt.Sprintf("service:%s env:%s", params.Service, params.Env),
		},
		"sort": map[string]any{
			"field": "timestamp",
			"order": "desc",
		},
		"limit": limit,
	}

	listResp, err := doRequest(ctx, "POST", fmt.Sprintf("https://%s/api/unstable/profiles/list", site), apiKey, appKey, payload)
	if err != nil {
		return ListProfilesResult{}, err
	}

	candidates, err := parseCandidates(listResp)
	if err != nil {
		return ListProfilesResult{}, err
	}

	return ListProfilesResult{
		Service:    params.Service,
		Env:        params.Env,
		DDSite:     site,
		FromTS:     fromTS,
		ToTS:       toTS,
		Limit:      limit,
		Candidates: candidates,
		Warnings:   warnings,
	}, nil
}

// parseRelativeOrAbsoluteTime parses a time string that can be:
// - now: current time
// - relative: "-1h", "-30m", "-2h30m" (negative duration from now)
// - absolute: RFC3339 format like "2025-12-31T19:00:00Z"
// Returns the parsed time formatted as RFC3339.
func parseRelativeOrAbsoluteTime(value string, defaultTime time.Time) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultTime.UTC().Format(time.RFC3339), nil
	}
	if strings.EqualFold(value, "now") {
		return time.Now().UTC().Format(time.RFC3339), nil
	}

	// Check if it's a relative time (starts with - or +)
	if strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return "", fmt.Errorf("invalid relative time %q: %w", value, err)
		}
		return time.Now().Add(duration).UTC().Format(time.RFC3339), nil
	}

	// Try parsing as RFC3339
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		// Try RFC3339Nano
		parsed, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return "", fmt.Errorf("invalid time format %q (expected RFC3339 or relative like -1h): %w", value, err)
		}
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func resolveTimeWindow(from, to string, hours int) (string, string, []string) {
	warnings := []string{}
	now := time.Now()

	if from != "" || to != "" {
		var fromTS, toTS string
		var err error

		if from == "" {
			fromTS = now.Add(-72 * time.Hour).UTC().Format(time.RFC3339)
			warnings = append(warnings, "from not provided; defaulted to last 72h")
		} else {
			fromTS, err = parseRelativeOrAbsoluteTime(from, now.Add(-72*time.Hour))
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("invalid from time: %v; defaulted to last 72h", err))
				fromTS = now.Add(-72 * time.Hour).UTC().Format(time.RFC3339)
			}
		}

		if to == "" {
			toTS = now.UTC().Format(time.RFC3339)
			warnings = append(warnings, "to not provided; defaulted to now")
		} else {
			toTS, err = parseRelativeOrAbsoluteTime(to, now)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("invalid to time: %v; defaulted to now", err))
				toTS = now.UTC().Format(time.RFC3339)
			}
		}

		return fromTS, toTS, warnings
	}

	if hours <= 0 {
		hours = 72
	}
	toTS := now.UTC().Format(time.RFC3339)
	fromTS := now.Add(-time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339)
	return fromTS, toTS, warnings
}

func parseCandidates(resp map[string]any) ([]ProfileCandidate, error) {
	data, ok := resp["data"].([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected datadog response format")
	}
	candidates := make([]ProfileCandidate, 0, len(data))
	for _, item := range data {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		candidate := ProfileCandidate{
			ProfileID:     getStringNested(entry, "attributes", "profile-id"),
			EventID:       getString(entry, "id"),
			Timestamp:     getStringNested(entry, "attributes", "timestamp"),
			NumericFields: extractNumericFields(entry),
		}
		// Fallback: check top-level for backwards compatibility
		if candidate.ProfileID == "" {
			candidate.ProfileID = getString(entry, "profile-id")
		}
		if candidate.ProfileID == "" {
			candidate.ProfileID = getString(entry, "profile_id")
		}
		if candidate.Timestamp == "" {
			candidate.Timestamp = getString(entry, "timestamp")
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

// usefulNumericFields defines the whitelist of fields worth extracting.
// These are the only fields used by formatSampleInfo, sampleScore, and findAnomaly.
var usefulNumericFields = map[string]bool{
	// CPU samples
	"cpu-samples": true, "cpu_samples": true,
	// Allocation metrics
	"alloc-samples": true, "alloc_samples": true, "alloc_space": true,
	// In-use memory
	"inuse_space": true, "inuse-space": true,
	// Goroutines
	"goroutines": true, "goroutine": true,
	// Heap
	"heap": true, "heap_inuse": true,
	// Duration
	"duration": true, "duration_ns": true, "profile_duration": true,
	// Total/generic samples
	"total-samples": true, "total_samples": true, "samples": true,
}

func extractNumericFields(entry map[string]any) map[string]float64 {
	fields := map[string]float64{}
	collectNumbers(fields, entry)
	if attrRaw, ok := entry["attributes"]; ok {
		if attrs, ok := attrRaw.(map[string]any); ok {
			collectNumbers(fields, attrs)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func collectNumbers(out map[string]float64, input map[string]any) {
	for key, val := range input {
		// Only collect fields we actually use
		if !usefulNumericFields[key] {
			continue
		}
		switch typed := val.(type) {
		case float64:
			out[key] = typed
		case int64:
			out[key] = float64(typed)
		case int:
			out[key] = float64(typed)
		case jsonNumber:
			if parsed, err := typed.Float64(); err == nil {
				out[key] = parsed
			}
		case string:
			if parsed, err := parseNumericString(typed); err == nil {
				out[key] = parsed
			}
		}
	}
}

func parseNumericString(value string) (float64, error) {
	clean := strings.TrimSpace(value)
	return strconvParseFloat(clean)
}

type jsonNumber = json.Number

func strconvParseFloat(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}

func pickMostSamples(candidates []ProfileCandidate) (ProfileCandidate, bool) {
	if len(candidates) == 0 {
		return ProfileCandidate{}, false
	}
	best := candidates[0]
	bestScore := -1.0
	for _, candidate := range candidates {
		score := sampleScore(candidate.NumericFields)
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	return best, bestScore >= 0
}

func sampleScore(fields map[string]float64) float64 {
	if len(fields) == 0 {
		return -1
	}
	best := -1.0
	for key, value := range fields {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "sample") || strings.Contains(lower, "value") || strings.Contains(lower, "total") || strings.Contains(lower, "duration") {
			if value > best {
				best = value
			}
		}
	}
	if best < 0 {
		for _, value := range fields {
			if value > best {
				best = value
			}
		}
	}
	return best
}

func FormatCandidatesTable(candidates []ProfileCandidate) string {
	lines := []string{fmt.Sprintf("%3s  %-24s  %-36s  %s", "idx", "timestamp", "profile_id", "samples")}
	for idx, candidate := range candidates {
		sampleInfo := formatSampleInfo(candidate.NumericFields)
		lines = append(lines, fmt.Sprintf("%3d  %-24s  %-36s  %s", idx, candidate.Timestamp, candidate.ProfileID, sampleInfo))
	}
	return strings.Join(lines, "\n")
}

// formatSampleInfo extracts and formats sample-related metrics from numeric fields.
func formatSampleInfo(fields map[string]float64) string {
	if len(fields) == 0 {
		return "-"
	}

	var parts []string

	// Look for common sample-related fields
	sampleKeys := []string{
		"cpu-samples", "cpu_samples",
		"alloc-samples", "alloc_samples",
		"total-samples", "total_samples",
		"samples",
	}

	for _, key := range sampleKeys {
		if val, ok := fields[key]; ok {
			parts = append(parts, formatMetricValue(key, val))
		}
	}

	// Also look for duration
	durationKeys := []string{"duration", "duration_ns", "profile_duration"}
	for _, key := range durationKeys {
		if val, ok := fields[key]; ok {
			// Convert nanoseconds to seconds if it looks like ns
			if val > 1e9 {
				parts = append(parts, fmt.Sprintf("dur=%.1fs", val/1e9))
			} else if val > 1000 {
				parts = append(parts, fmt.Sprintf("dur=%.1fms", val/1e6))
			} else {
				parts = append(parts, fmt.Sprintf("dur=%.1f", val))
			}
			break
		}
	}

	if len(parts) == 0 {
		// Fall back to showing the first numeric field
		for key, val := range fields {
			return formatMetricValue(key, val)
		}
		return "-"
	}

	return strings.Join(parts, " ")
}

func formatMetricValue(key string, val float64) string {
	// Shorten common key names
	shortKey := key
	shortKey = strings.ReplaceAll(shortKey, "-samples", "")
	shortKey = strings.ReplaceAll(shortKey, "_samples", "")
	shortKey = strings.ReplaceAll(shortKey, "samples", "smp")

	if val >= 1e6 {
		return fmt.Sprintf("%s=%.1fM", shortKey, val/1e6)
	} else if val >= 1e3 {
		return fmt.Sprintf("%s=%.1fK", shortKey, val/1e3)
	}
	return fmt.Sprintf("%s=%.0f", shortKey, val)
}

func loadKeys() (string, string, error) {
	apiKey := os.Getenv("DD_API_KEY")
	appKey := os.Getenv("DD_APP_KEY")
	if apiKey == "" || appKey == "" {
		return "", "", fmt.Errorf("missing DD_API_KEY or DD_APP_KEY")
	}
	return apiKey, appKey, nil
}

func parseTimestamp(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}

func sortByTimestampDesc(candidates []ProfileCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		a, errA := parseTimestamp(candidates[i].Timestamp)
		b, errB := parseTimestamp(candidates[j].Timestamp)
		if errA != nil || errB != nil {
			return candidates[i].Timestamp > candidates[j].Timestamp
		}
		return a.After(b)
	})
}
