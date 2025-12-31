package datadog

import (
	"context"
	"fmt"
	"time"
)

type PickStrategy string

const (
	PickLatest       PickStrategy = "latest"
	PickOldest       PickStrategy = "oldest"
	PickClosestToTS  PickStrategy = "closest_to_ts"
	PickMostSamples  PickStrategy = "most_samples"
	PickManualIndex  PickStrategy = "manual_index"
)

type PickProfilesParams struct {
	Service   string
	Env       string
	From      string
	To        string
	Hours     int
	Limit     int
	Site      string
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
