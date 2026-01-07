package datadog

import (
	"context"
	"fmt"
	"math"
	"time"
)

type PickStrategy string

const (
	PickLatest       PickStrategy = "latest"
	PickOldest       PickStrategy = "oldest"
	PickClosestToTS  PickStrategy = "closest_to_ts"
	PickMostSamples  PickStrategy = "most_samples"
	PickManualIndex  PickStrategy = "manual_index"
	PickAnomalous    PickStrategy = "anomaly"
)

type PickProfilesParams struct {
	Service   string
	Env       string
	From      string
	To        string
	Hours     int
	Limit     int
	Site      string
	Host      string // Optional host filter (supports wildcards)
	Strategy  PickStrategy
	TargetTS  string
	Index     int
}

type PickResult struct {
	Candidate ProfileCandidate `json:"candidate"`
	Reason    string           `json:"reason"`
	Warnings  []string         `json:"warnings,omitempty"`
}

func PickProfile(ctx context.Context, params PickProfilesParams) (PickResult, error) {
	listResult, err := ListProfiles(ctx, ListProfilesParams{
		Service: params.Service,
		Env:     params.Env,
		From:    params.From,
		To:      params.To,
		Hours:   params.Hours,
		Limit:   params.Limit,
		Site:    params.Site,
		Host:    params.Host,
	})
	if err != nil {
		return PickResult{}, err
	}

	candidates := listResult.Candidates
	warnings := append([]string{}, listResult.Warnings...)
	if len(candidates) == 0 {
		return PickResult{}, fmt.Errorf("no profiles found")
	}

	if params.Index >= 0 {
		if params.Index >= len(candidates) {
			return PickResult{}, fmt.Errorf("manual index %d out of range", params.Index)
		}
		return PickResult{Candidate: candidates[params.Index], Reason: fmt.Sprintf("manual_index=%d", params.Index)}, nil
	}

	switch params.Strategy {
	case PickLatest, "":
		return PickResult{Candidate: candidates[0], Reason: "latest"}, nil
	case PickOldest:
		// Candidates are sorted newest first, so oldest is last
		return PickResult{Candidate: candidates[len(candidates)-1], Reason: "oldest"}, nil
	case PickClosestToTS:
		target, err := parseTimestamp(params.TargetTS)
		if err != nil {
			return PickResult{}, fmt.Errorf("invalid target timestamp: %w", err)
		}
		candidate := closestToTimestamp(candidates, target)
		return PickResult{Candidate: candidate, Reason: fmt.Sprintf("closest_to_ts=%s", params.TargetTS)}, nil
	case PickMostSamples:
		candidate, ok := pickMostSamples(candidates)
		if !ok {
			warnings = append(warnings, "most_samples unavailable; falling back to latest")
			return PickResult{Candidate: candidates[0], Reason: "latest", Warnings: warnings}, nil
		}
		return PickResult{Candidate: candidate, Reason: "most_samples"}, nil
	case PickAnomalous:
		candidate, score, field, ok := pickAnomalous(candidates)
		if !ok {
			warnings = append(warnings, "anomaly detection unavailable (no numeric fields); falling back to latest")
			return PickResult{Candidate: candidates[0], Reason: "latest", Warnings: warnings}, nil
		}
		return PickResult{Candidate: candidate, Reason: fmt.Sprintf("anomaly: %.1f stddev on %s", score, field), Warnings: warnings}, nil
	case PickManualIndex:
		return PickResult{}, fmt.Errorf("manual_index strategy requires --index")
	default:
		return PickResult{}, fmt.Errorf("unknown strategy: %s", params.Strategy)
	}
}

func closestToTimestamp(candidates []ProfileCandidate, target time.Time) ProfileCandidate {
	best := candidates[0]
	bestDelta := time.Duration(1<<63 - 1)
	for _, candidate := range candidates {
		parsed, err := parseTimestamp(candidate.Timestamp)
		if err != nil {
			continue
		}
		delta := parsed.Sub(target)
		if delta < 0 {
			delta = -delta
		}
		if delta < bestDelta {
			bestDelta = delta
			best = candidate
		}
	}
	return best
}

// pickAnomalous finds the profile with the highest z-score across numeric fields.
// Returns the candidate, z-score, field name, and whether detection succeeded.
func pickAnomalous(candidates []ProfileCandidate) (ProfileCandidate, float64, string, bool) {
	if len(candidates) < 3 {
		// Need at least 3 samples for meaningful statistics
		return ProfileCandidate{}, 0, "", false
	}

	// Collect all numeric field names across candidates
	fieldSet := make(map[string]bool)
	for _, c := range candidates {
		for k := range c.NumericFields {
			fieldSet[k] = true
		}
	}
	if len(fieldSet) == 0 {
		return ProfileCandidate{}, 0, "", false
	}

	// Priority fields for anomaly detection (most indicative of problems)
	priorityFields := []string{
		"cpu-samples", "cpu_samples",
		"alloc-samples", "alloc_samples", "alloc_space",
		"inuse_space", "inuse-space",
		"goroutines", "goroutine",
		"heap", "heap_inuse",
	}

	var bestCandidate ProfileCandidate
	var bestZScore float64
	var bestField string

	// Check priority fields first, then all others
	fieldsToCheck := append([]string{}, priorityFields...)
	for field := range fieldSet {
		found := false
		for _, p := range priorityFields {
			if field == p {
				found = true
				break
			}
		}
		if !found {
			fieldsToCheck = append(fieldsToCheck, field)
		}
	}

	for _, field := range fieldsToCheck {
		if !fieldSet[field] {
			continue
		}

		candidate, zScore, ok := findAnomalyForField(candidates, field)
		if !ok {
			continue
		}

		// Use absolute z-score but prefer high values (potential issues)
		if zScore < 0 {
			zScore = -zScore
		}

		if zScore > bestZScore {
			bestZScore = zScore
			bestCandidate = candidate
			bestField = field
		}
	}

	if bestZScore < 2.0 {
		// No significant anomaly found (threshold: 2 standard deviations)
		return ProfileCandidate{}, 0, "", false
	}

	return bestCandidate, bestZScore, bestField, true
}

// findAnomalyForField calculates z-scores for a specific field and returns the most anomalous candidate.
func findAnomalyForField(candidates []ProfileCandidate, field string) (ProfileCandidate, float64, bool) {
	var values []float64
	var indices []int

	for i, c := range candidates {
		if v, ok := c.NumericFields[field]; ok {
			values = append(values, v)
			indices = append(indices, i)
		}
	}

	if len(values) < 3 {
		return ProfileCandidate{}, 0, false
	}

	mean, stddev := meanStddev(values)
	if stddev == 0 {
		return ProfileCandidate{}, 0, false
	}

	var bestIdx int
	var bestZScore float64

	for i, v := range values {
		zScore := (v - mean) / stddev
		absZ := zScore
		if absZ < 0 {
			absZ = -absZ
		}
		if absZ > bestZScore {
			bestZScore = absZ
			bestIdx = indices[i]
		}
	}

	return candidates[bestIdx], bestZScore, true
}

func meanStddev(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	var sumSquares float64
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(values))

	return mean, math.Sqrt(variance)
}
