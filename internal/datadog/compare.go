package datadog

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type CompareRangeParams struct {
	Service string
	Env     string
	Site    string

	// Before range
	BeforeFrom string
	BeforeTo   string

	// After range
	AfterFrom string
	AfterTo   string

	// Output directory for downloaded profiles
	OutDir string

	// Profile type to compare (cpu, heap, etc.)
	ProfileType string
}

type CompareRangeResult struct {
	Service       string         `json:"service"`
	Env           string         `json:"env"`
	BeforeProfile ProfileSummary `json:"before_profile"`
	AfterProfile  ProfileSummary `json:"after_profile"`
	Diff          string         `json:"diff"`
	TopChanges    []FunctionDiff `json:"top_changes"`
	Warnings      []string       `json:"warnings,omitempty"`
}

type ProfileSummary struct {
	Timestamp string `json:"timestamp"`
	ProfileID string `json:"profile_id"`
	FilePath  string `json:"file_path"`
}

type FunctionDiff struct {
	Function   string  `json:"function"`
	BeforeFlat string  `json:"before_flat"`
	AfterFlat  string  `json:"after_flat"`
	Change     string  `json:"change"`
	Severity   string  `json:"severity"` // increase, decrease, new, removed
}

// CompareRange downloads profiles from two time ranges and compares them.
func CompareRange(ctx context.Context, params CompareRangeParams) (CompareRangeResult, error) {
	if params.Service == "" || params.Env == "" {
		return CompareRangeResult{}, fmt.Errorf("service and env are required")
	}
	if params.BeforeFrom == "" || params.AfterFrom == "" {
		return CompareRangeResult{}, fmt.Errorf("before_from and after_from time ranges are required")
	}

	result := CompareRangeResult{
		Service:  params.Service,
		Env:      params.Env,
		Warnings: []string{},
	}

	outDir := params.OutDir
	if outDir == "" {
		var err error
		outDir, err = os.MkdirTemp("", "pprof-compare-*")
		if err != nil {
			return result, fmt.Errorf("failed to create temp dir: %w", err)
		}
	}

	profileType := params.ProfileType
	if profileType == "" {
		profileType = "cpu"
	}

	// Pick and download before profile (oldest in range gives baseline)
	beforeResult, err := PickProfile(ctx, PickProfilesParams{
		Service:  params.Service,
		Env:      params.Env,
		Site:     params.Site,
		From:     params.BeforeFrom,
		To:       params.BeforeTo,
		Strategy: PickOldest,
		Limit:    10,
	})
	if err != nil {
		return result, fmt.Errorf("failed to pick before profile: %w", err)
	}

	beforeDownload, err := DownloadLatestBundle(ctx, DownloadParams{
		Service:   params.Service,
		Env:       params.Env,
		Site:      params.Site,
		OutDir:    filepath.Join(outDir, "before"),
		ProfileID: beforeResult.Candidate.ProfileID,
		EventID:   beforeResult.Candidate.EventID,
	})
	if err != nil {
		return result, fmt.Errorf("failed to download before profile: %w", err)
	}

	beforeFile := findProfileByType(beforeDownload.Files, profileType)
	if beforeFile == "" {
		return result, fmt.Errorf("before profile type %q not found in bundle", profileType)
	}

	result.BeforeProfile = ProfileSummary{
		Timestamp: beforeResult.Candidate.Timestamp,
		ProfileID: beforeResult.Candidate.ProfileID,
		FilePath:  beforeFile,
	}

	// Pick and download after profile (latest in range gives current state)
	afterResult, err := PickProfile(ctx, PickProfilesParams{
		Service:  params.Service,
		Env:      params.Env,
		Site:     params.Site,
		From:     params.AfterFrom,
		To:       params.AfterTo,
		Strategy: PickLatest,
		Limit:    10,
	})
	if err != nil {
		return result, fmt.Errorf("failed to pick after profile: %w", err)
	}

	afterDownload, err := DownloadLatestBundle(ctx, DownloadParams{
		Service:   params.Service,
		Env:       params.Env,
		Site:      params.Site,
		OutDir:    filepath.Join(outDir, "after"),
		ProfileID: afterResult.Candidate.ProfileID,
		EventID:   afterResult.Candidate.EventID,
	})
	if err != nil {
		return result, fmt.Errorf("failed to download after profile: %w", err)
	}

	afterFile := findProfileByType(afterDownload.Files, profileType)
	if afterFile == "" {
		return result, fmt.Errorf("after profile type %q not found in bundle", profileType)
	}

	result.AfterProfile = ProfileSummary{
		Timestamp: afterResult.Candidate.Timestamp,
		ProfileID: afterResult.Candidate.ProfileID,
		FilePath:  afterFile,
	}

	// Run pprof diff
	diffOutput, err := runPprofDiff(ctx, beforeFile, afterFile)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("diff failed: %v", err))
	} else {
		result.Diff = diffOutput
		result.TopChanges = parseDiffChanges(diffOutput)
	}

	return result, nil
}

func findProfileByType(files []ProfileFile, profileType string) string {
	for _, f := range files {
		if f.Type == profileType {
			return f.Path
		}
	}
	// Fallback: look for filename match
	for _, f := range files {
		lower := strings.ToLower(f.Path)
		if strings.Contains(lower, profileType) {
			return f.Path
		}
	}
	return ""
}

func runPprofDiff(ctx context.Context, before, after string) (string, error) {
	// pprof -top -base=before after shows diff
	args := []string{"tool", "pprof", "-top", "-nodecount=20", fmt.Sprintf("-base=%s", before), after}

	cmd := exec.CommandContext(ctx, "go", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pprof diff failed: %w: %s", err, string(out))
	}
	return string(out), nil
}

func parseDiffChanges(diffOutput string) []FunctionDiff {
	var changes []FunctionDiff

	lines := strings.Split(diffOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Showing") || strings.HasPrefix(line, "File:") {
			continue
		}

		// Parse pprof top output lines
		// Format typically: "flat  flat%  sum%  cum  cum%  function"
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		// Skip header line
		if fields[0] == "flat" {
			continue
		}

		flat := fields[0]
		function := fields[len(fields)-1]

		// Determine severity based on the value
		severity := "neutral"
		if strings.HasPrefix(flat, "-") {
			severity = "decrease"
		} else if flat != "0" && !strings.HasPrefix(flat, "0.") {
			severity = "increase"
		}

		changes = append(changes, FunctionDiff{
			Function:  function,
			AfterFlat: flat,
			Severity:  severity,
		})

		if len(changes) >= 10 {
			break
		}
	}

	return changes
}

// FormatCompareResult formats the comparison result for display
func FormatCompareResult(result CompareRangeResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Profile Comparison: %s (%s)\n", result.Service, result.Env))
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	sb.WriteString("Before:\n")
	sb.WriteString(fmt.Sprintf("  Timestamp: %s\n", result.BeforeProfile.Timestamp))
	sb.WriteString(fmt.Sprintf("  Profile:   %s\n", result.BeforeProfile.ProfileID))

	sb.WriteString("\nAfter:\n")
	sb.WriteString(fmt.Sprintf("  Timestamp: %s\n", result.AfterProfile.Timestamp))
	sb.WriteString(fmt.Sprintf("  Profile:   %s\n", result.AfterProfile.ProfileID))

	if len(result.TopChanges) > 0 {
		sb.WriteString("\nTop Changes:\n")
		for _, change := range result.TopChanges {
			marker := " "
			switch change.Severity {
			case "increase":
				marker = "+"
			case "decrease":
				marker = "-"
			}
			sb.WriteString(fmt.Sprintf("  %s %s: %s\n", marker, change.AfterFlat, change.Function))
		}
	}

	return sb.String()
}
