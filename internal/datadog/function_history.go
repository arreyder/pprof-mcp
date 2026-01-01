package datadog

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// FunctionHistoryParams configures the function history search.
type FunctionHistoryParams struct {
	Service  string
	Env      string
	Function string // Regex pattern to match function names
	From     string
	To       string
	Hours    int
	Limit    int
	Site     string
}

// FunctionHistoryEntry represents a function's presence in a single profile.
type FunctionHistoryEntry struct {
	Timestamp   string  `json:"timestamp"`
	ProfileID   string  `json:"profile_id"`
	EventID     string  `json:"event_id"`
	FlatPercent float64 `json:"flat_percent"`
	CumPercent  float64 `json:"cum_percent"`
	FlatValue   string  `json:"flat_value"`
	CumValue    string  `json:"cum_value"`
	Found       bool    `json:"found"`
}

// FunctionHistoryResult contains the search results across profiles.
type FunctionHistoryResult struct {
	Service  string                 `json:"service"`
	Env      string                 `json:"env"`
	Function string                 `json:"function"`
	FromTS   string                 `json:"from_ts"`
	ToTS     string                 `json:"to_ts"`
	Entries  []FunctionHistoryEntry `json:"entries"`
	Summary  FunctionHistorySummary `json:"summary"`
	Warnings []string               `json:"warnings,omitempty"`
}

// FunctionHistorySummary provides aggregate stats.
type FunctionHistorySummary struct {
	TotalProfiles   int     `json:"total_profiles"`
	FoundInProfiles int     `json:"found_in_profiles"`
	MaxFlatPercent  float64 `json:"max_flat_percent"`
	MinFlatPercent  float64 `json:"min_flat_percent"`
	AvgFlatPercent  float64 `json:"avg_flat_percent"`
}

const functionHistoryConcurrency = 3

// SearchFunctionHistory searches for a function across multiple profiles over time.
func SearchFunctionHistory(ctx context.Context, params FunctionHistoryParams) (FunctionHistoryResult, error) {
	if params.Function == "" {
		return FunctionHistoryResult{}, fmt.Errorf("function pattern is required")
	}

	// List available profiles
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
		return FunctionHistoryResult{}, fmt.Errorf("failed to list profiles: %w", err)
	}

	if len(listResult.Candidates) == 0 {
		return FunctionHistoryResult{
			Service:  params.Service,
			Env:      params.Env,
			Function: params.Function,
			FromTS:   listResult.FromTS,
			ToTS:     listResult.ToTS,
			Entries:  []FunctionHistoryEntry{},
			Warnings: []string{"no profiles found in time range"},
		}, nil
	}

	// Create temp directory for downloads
	tmpDir, err := os.MkdirTemp("", "pprof-function-history-*")
	if err != nil {
		return FunctionHistoryResult{}, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	entries := make([]FunctionHistoryEntry, len(listResult.Candidates))
	warningsByIndex := make([][]string, len(listResult.Candidates))

	sem := make(chan struct{}, functionHistoryConcurrency)
	var wg sync.WaitGroup

	// Process each profile with a small concurrency limit.
	for i, candidate := range listResult.Candidates {
		if err := ctx.Err(); err != nil {
			return FunctionHistoryResult{}, err
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return FunctionHistoryResult{}, ctx.Err()
		}

		wg.Add(1)
		go func(idx int, c ProfileCandidate) {
			defer wg.Done()
			defer func() { <-sem }()

			entry := FunctionHistoryEntry{
				Timestamp: c.Timestamp,
				ProfileID: c.ProfileID,
				EventID:   c.EventID,
				Found:     false,
			}

			if err := ctx.Err(); err != nil {
				entries[idx] = entry
				return
			}

			profileDir := filepath.Join(tmpDir, sanitizeFilename(c.ProfileID))
			result, err := DownloadLatestBundle(ctx, DownloadParams{
				Service:   params.Service,
				Env:       params.Env,
				OutDir:    profileDir,
				Site:      params.Site,
				Hours:     params.Hours,
				ProfileID: c.ProfileID,
				EventID:   c.EventID,
			})
			if err != nil {
				warningsByIndex[idx] = append(warningsByIndex[idx], fmt.Sprintf("failed to download profile %s: %v", c.ProfileID, err))
				entries[idx] = entry
				return
			}

			cpuProfile := findCPUProfile(result.Files)
			if cpuProfile == "" {
				warningsByIndex[idx] = append(warningsByIndex[idx], fmt.Sprintf("no CPU profile found for %s", c.ProfileID))
				entries[idx] = entry
				return
			}

			funcResult, err := searchFunctionInProfile(ctx, cpuProfile, params.Function)
			if err != nil {
				warningsByIndex[idx] = append(warningsByIndex[idx], fmt.Sprintf("failed to search profile %s: %v", c.ProfileID, err))
				entries[idx] = entry
				return
			}

			entry.Found = funcResult.Found
			entry.FlatPercent = funcResult.FlatPercent
			entry.CumPercent = funcResult.CumPercent
			entry.FlatValue = funcResult.FlatValue
			entry.CumValue = funcResult.CumValue
			entries[idx] = entry
		}(i, candidate)
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return FunctionHistoryResult{}, err
	}

	warnings := make([]string, 0)
	for _, items := range warningsByIndex {
		warnings = append(warnings, items...)
	}

	// Sort by timestamp (newest first)
	sort.Slice(entries, func(i, j int) bool {
		ti, _ := parseTimestamp(entries[i].Timestamp)
		tj, _ := parseTimestamp(entries[j].Timestamp)
		return ti.After(tj)
	})

	// Calculate summary
	summary := calculateSummary(entries)

	return FunctionHistoryResult{
		Service:  params.Service,
		Env:      params.Env,
		Function: params.Function,
		FromTS:   listResult.FromTS,
		ToTS:     listResult.ToTS,
		Entries:  entries,
		Summary:  summary,
		Warnings: warnings,
	}, nil
}

type functionSearchResult struct {
	Found       bool
	FlatPercent float64
	CumPercent  float64
	FlatValue   string
	CumValue    string
}

func searchFunctionInProfile(ctx context.Context, profilePath, functionPattern string) (functionSearchResult, error) {
	// Use go tool pprof -top with focus to find the function
	output, err := runPprofTop(ctx, profilePath, functionPattern)
	if err != nil {
		return functionSearchResult{}, err
	}

	return parseFunctionFromTop(output, functionPattern), nil
}

func runPprofTop(ctx context.Context, profilePath, focus string) (string, error) {
	args := []string{"tool", "pprof", "-top", "-focus", focus, "-nodecount", "50", profilePath}
	cmd := exec.CommandContext(ctx, "go", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pprof top failed: %w\n%s", err, stderr.String())
	}
	return string(stdout), nil
}

func parseFunctionFromTop(output, pattern string) functionSearchResult {
	lines := strings.Split(output, "\n")
	patternLower := strings.ToLower(pattern)

	for _, line := range lines {
		// Skip header lines
		if strings.HasPrefix(strings.TrimSpace(line), "flat") ||
			strings.HasPrefix(strings.TrimSpace(line), "Showing") ||
			strings.TrimSpace(line) == "" {
			continue
		}

		// Check if this line contains our function
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, patternLower) {
			// Parse the line: "flat flat% sum% cum cum% name"
			fields := strings.Fields(line)
			if len(fields) >= 6 {
				flatPercent := parsePercent(fields[1])
				cumPercent := parsePercent(fields[4])
				return functionSearchResult{
					Found:       true,
					FlatPercent: flatPercent,
					CumPercent:  cumPercent,
					FlatValue:   fields[0],
					CumValue:    fields[3],
				}
			}
		}
	}

	return functionSearchResult{Found: false}
}

func parsePercent(s string) float64 {
	s = strings.TrimSuffix(s, "%")
	var val float64
	fmt.Sscanf(s, "%f", &val)
	return val
}

func findCPUProfile(files []ProfileFile) string {
	for _, f := range files {
		if f.Type == "cpu" {
			return f.Path
		}
	}
	// Fallback: any .pprof file with cpu in the name
	for _, f := range files {
		lower := strings.ToLower(f.Path)
		if strings.Contains(lower, "cpu") && strings.HasSuffix(lower, ".pprof") {
			return f.Path
		}
	}
	// Last resort: any .pprof file
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f.Path), ".pprof") {
			return f.Path
		}
	}
	return ""
}

func sanitizeFilename(s string) string {
	return strings.ReplaceAll(s, "/", "_")
}

func calculateSummary(entries []FunctionHistoryEntry) FunctionHistorySummary {
	summary := FunctionHistorySummary{
		TotalProfiles:  len(entries),
		MinFlatPercent: -1,
	}

	var total float64
	for _, e := range entries {
		if e.Found {
			summary.FoundInProfiles++
			total += e.FlatPercent
			if e.FlatPercent > summary.MaxFlatPercent {
				summary.MaxFlatPercent = e.FlatPercent
			}
			if summary.MinFlatPercent < 0 || e.FlatPercent < summary.MinFlatPercent {
				summary.MinFlatPercent = e.FlatPercent
			}
		}
	}

	if summary.FoundInProfiles > 0 {
		summary.AvgFlatPercent = total / float64(summary.FoundInProfiles)
	}
	if summary.MinFlatPercent < 0 {
		summary.MinFlatPercent = 0
	}

	return summary
}

// FormatFunctionHistoryTable formats the results as a table.
func FormatFunctionHistoryTable(result FunctionHistoryResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Function: %s\n", result.Function))
	sb.WriteString(fmt.Sprintf("Service: %s, Env: %s\n", result.Service, result.Env))
	sb.WriteString(fmt.Sprintf("Time range: %s to %s\n\n", result.FromTS, result.ToTS))

	sb.WriteString(fmt.Sprintf("%3s  %-24s  %8s  %8s  %s\n", "idx", "timestamp", "flat%", "cum%", "found"))
	sb.WriteString(strings.Repeat("-", 60) + "\n")

	for idx, entry := range result.Entries {
		ts := entry.Timestamp
		if len(ts) > 24 {
			ts = ts[:24]
		}
		found := "no"
		flatStr := "-"
		cumStr := "-"
		if entry.Found {
			found = "yes"
			flatStr = fmt.Sprintf("%.2f%%", entry.FlatPercent)
			cumStr = fmt.Sprintf("%.2f%%", entry.CumPercent)
		}
		sb.WriteString(fmt.Sprintf("%3d  %-24s  %8s  %8s  %s\n", idx, ts, flatStr, cumStr, found))
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Summary: Found in %d/%d profiles\n", result.Summary.FoundInProfiles, result.Summary.TotalProfiles))
	if result.Summary.FoundInProfiles > 0 {
		sb.WriteString(fmt.Sprintf("  Max: %.2f%%, Min: %.2f%%, Avg: %.2f%%\n",
			result.Summary.MaxFlatPercent, result.Summary.MinFlatPercent, result.Summary.AvgFlatPercent))
	}

	return sb.String()
}
