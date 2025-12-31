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

func resolveTimeWindow(from, to string, hours int) (string, string, []string) {
	warnings := []string{}
	if from != "" || to != "" {
		fromTS := from
		toTS := to
		if fromTS == "" {
			fromTS = time.Now().Add(-72 * time.Hour).UTC().Format(time.RFC3339)
			warnings = append(warnings, "from not provided; defaulted to last 72h")
		}
		if toTS == "" {
			toTS = time.Now().UTC().Format(time.RFC3339)
			warnings = append(warnings, "to not provided; defaulted to now")
		}
		return fromTS, toTS, warnings
	}

	if hours <= 0 {
		hours = 72
	}
	toTS := time.Now().UTC().Format(time.RFC3339)
	fromTS := time.Now().Add(-time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339)
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
			ProfileID:     getString(entry, "profile-id"),
			EventID:       getString(entry, "id"),
			Timestamp:     getStringNested(entry, "attributes", "timestamp"),
			NumericFields: extractNumericFields(entry),
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
	lines := []string{fmt.Sprintf("%3s  %-24s  %-36s  %-36s", "idx", "timestamp", "profile_id", "event_id")}
	for idx, candidate := range candidates {
		lines = append(lines, fmt.Sprintf("%3d  %-24s  %-36s  %-36s", idx, candidate.Timestamp, candidate.ProfileID, candidate.EventID))
	}
	return strings.Join(lines, "\n")
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
