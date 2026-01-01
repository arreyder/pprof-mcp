package datadog

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type NearEventParams struct {
	Service   string
	Env       string
	Site      string
	EventTime string // RFC3339 timestamp of the event (restart, OOM, etc.)
	Window    string // Time window around event (e.g., "30m", "1h")
	Limit     int    // Max profiles to return
}

type NearEventResult struct {
	Service       string             `json:"service"`
	Env           string             `json:"env"`
	EventTime     string             `json:"event_time"`
	Window        string             `json:"window"`
	BeforeEvent   []ProfileCandidate `json:"before_event"`   // Profiles before the event
	AfterEvent    []ProfileCandidate `json:"after_event"`    // Profiles after the event (if service restarted)
	ClosestBefore *ProfileCandidate  `json:"closest_before"` // Last profile before event
	ClosestAfter  *ProfileCandidate  `json:"closest_after"`  // First profile after event
	GapDuration   string             `json:"gap_duration"`   // Duration between closest profiles
	Warnings      []string           `json:"warnings,omitempty"`
}

// FindProfilesNearEvent finds profiles around a specific event time (restart, OOM, etc.)
func FindProfilesNearEvent(ctx context.Context, params NearEventParams) (NearEventResult, error) {
	if params.Service == "" || params.Env == "" {
		return NearEventResult{}, fmt.Errorf("service and env are required")
	}
	if params.EventTime == "" {
		return NearEventResult{}, fmt.Errorf("event_time is required")
	}

	eventTime, err := parseTimestamp(params.EventTime)
	if err != nil {
		return NearEventResult{}, fmt.Errorf("invalid event_time: %w", err)
	}

	window := params.Window
	if window == "" {
		window = "1h"
	}
	windowDuration, err := time.ParseDuration(window)
	if err != nil {
		return NearEventResult{}, fmt.Errorf("invalid window duration: %w", err)
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	// Search for profiles in a window around the event
	fromTime := eventTime.Add(-windowDuration)
	toTime := eventTime.Add(windowDuration)

	listResult, err := ListProfiles(ctx, ListProfilesParams{
		Service: params.Service,
		Env:     params.Env,
		Site:    params.Site,
		From:    fromTime.Format(time.RFC3339),
		To:      toTime.Format(time.RFC3339),
		Limit:   limit * 2, // Get more to ensure we have profiles on both sides
	})
	if err != nil {
		return NearEventResult{}, fmt.Errorf("failed to list profiles: %w", err)
	}

	result := NearEventResult{
		Service:     params.Service,
		Env:         params.Env,
		EventTime:   params.EventTime,
		Window:      window,
		BeforeEvent: []ProfileCandidate{},
		AfterEvent:  []ProfileCandidate{},
		Warnings:    listResult.Warnings,
	}

	// Sort candidates by timestamp
	candidates := listResult.Candidates
	sortByTimestampDesc(candidates)

	// Split into before and after event
	for _, c := range candidates {
		ts, err := parseTimestamp(c.Timestamp)
		if err != nil {
			continue
		}

		if ts.Before(eventTime) {
			result.BeforeEvent = append(result.BeforeEvent, c)
		} else {
			result.AfterEvent = append(result.AfterEvent, c)
		}
	}

	// Sort before (most recent first) and after (oldest first)
	sort.SliceStable(result.BeforeEvent, func(i, j int) bool {
		ti, _ := parseTimestamp(result.BeforeEvent[i].Timestamp)
		tj, _ := parseTimestamp(result.BeforeEvent[j].Timestamp)
		return ti.After(tj)
	})

	sort.SliceStable(result.AfterEvent, func(i, j int) bool {
		ti, _ := parseTimestamp(result.AfterEvent[i].Timestamp)
		tj, _ := parseTimestamp(result.AfterEvent[j].Timestamp)
		return ti.Before(tj)
	})

	// Limit results
	if len(result.BeforeEvent) > limit {
		result.BeforeEvent = result.BeforeEvent[:limit]
	}
	if len(result.AfterEvent) > limit {
		result.AfterEvent = result.AfterEvent[:limit]
	}

	// Find closest profiles
	if len(result.BeforeEvent) > 0 {
		result.ClosestBefore = &result.BeforeEvent[0]
	}
	if len(result.AfterEvent) > 0 {
		result.ClosestAfter = &result.AfterEvent[0]
	}

	// Calculate gap duration
	if result.ClosestBefore != nil && result.ClosestAfter != nil {
		beforeTS, _ := parseTimestamp(result.ClosestBefore.Timestamp)
		afterTS, _ := parseTimestamp(result.ClosestAfter.Timestamp)
		gap := afterTS.Sub(beforeTS)
		result.GapDuration = gap.String()

		// Warn if gap is unusually large (suggests service was down)
		if gap > 5*time.Minute {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Large gap (%s) between profiles suggests service was down/restarting", gap))
		}
	}

	return result, nil
}

// FormatNearEventResult formats the result for display
func FormatNearEventResult(result NearEventResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Profiles Near Event: %s (%s)\n", result.Service, result.Env))
	sb.WriteString(fmt.Sprintf("Event Time: %s (±%s window)\n", result.EventTime, result.Window))
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	if result.ClosestBefore != nil {
		sb.WriteString(fmt.Sprintf("Last profile BEFORE event:\n"))
		sb.WriteString(fmt.Sprintf("  Timestamp: %s\n", result.ClosestBefore.Timestamp))
		sb.WriteString(fmt.Sprintf("  Profile ID: %s\n", result.ClosestBefore.ProfileID))
	} else {
		sb.WriteString("No profiles found BEFORE event\n")
	}

	sb.WriteString("\n")

	if result.ClosestAfter != nil {
		sb.WriteString(fmt.Sprintf("First profile AFTER event:\n"))
		sb.WriteString(fmt.Sprintf("  Timestamp: %s\n", result.ClosestAfter.Timestamp))
		sb.WriteString(fmt.Sprintf("  Profile ID: %s\n", result.ClosestAfter.ProfileID))
	} else {
		sb.WriteString("No profiles found AFTER event\n")
	}

	if result.GapDuration != "" {
		sb.WriteString(fmt.Sprintf("\nGap between profiles: %s\n", result.GapDuration))
	}

	sb.WriteString(fmt.Sprintf("\nTotal profiles: %d before, %d after\n",
		len(result.BeforeEvent), len(result.AfterEvent)))

	return sb.String()
}
