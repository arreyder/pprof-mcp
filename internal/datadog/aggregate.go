package datadog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type AggregateProfilesParams struct {
	Service     string
	Env         string
	Window      string
	Limit       int
	Site        string
	OutDir      string
	ProfileType string
}

type AggregateProfilesResult struct {
	Service        string             `json:"service"`
	Env            string             `json:"env"`
	DDSite         string             `json:"dd_site"`
	ProfileType    string             `json:"profile_type"`
	TimeRange      AggregateTimeRange `json:"time_range"`
	ProfilesMerged int                `json:"profiles_merged"`
	ProfilePaths   []string           `json:"profile_paths"`
	Warnings       []string           `json:"warnings,omitempty"`
}

type AggregateTimeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func AggregateProfiles(ctx context.Context, params AggregateProfilesParams) (AggregateProfilesResult, error) {
	if params.Service == "" || params.Env == "" {
		return AggregateProfilesResult{}, fmt.Errorf("service and env are required")
	}
	if params.Window == "" {
		return AggregateProfilesResult{}, fmt.Errorf("window is required")
	}

	window, err := time.ParseDuration(params.Window)
	if err != nil {
		return AggregateProfilesResult{}, fmt.Errorf("invalid window %q: %w", params.Window, err)
	}
	if window <= 0 {
		return AggregateProfilesResult{}, fmt.Errorf("window must be positive")
	}

	profileType := params.ProfileType
	if profileType == "" {
		profileType = "cpu"
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}

	to := time.Now().UTC()
	from := to.Add(-window)
	fromTS := from.Format(time.RFC3339)
	toTS := to.Format(time.RFC3339)

	listResult, err := ListProfiles(ctx, ListProfilesParams{
		Service: params.Service,
		Env:     params.Env,
		From:    fromTS,
		To:      toTS,
		Limit:   limit,
		Site:    params.Site,
	})
	if err != nil {
		return AggregateProfilesResult{}, err
	}
	if len(listResult.Candidates) == 0 {
		return AggregateProfilesResult{}, fmt.Errorf("no profiles found in the requested window")
	}

	outDir := params.OutDir
	if outDir == "" {
		outDir, err = os.MkdirTemp("", "pprof-aggregate-*")
		if err != nil {
			return AggregateProfilesResult{}, fmt.Errorf("failed to create temp dir: %w", err)
		}
	}

	paths := []string{}
	warnings := append([]string{}, listResult.Warnings...)
	for idx, candidate := range listResult.Candidates {
		if idx >= limit {
			break
		}
		downloadDir := filepath.Join(outDir, fmt.Sprintf("profile-%d", idx+1))
		download, err := DownloadLatestBundle(ctx, DownloadParams{
			Service:   params.Service,
			Env:       params.Env,
			Site:      params.Site,
			OutDir:    downloadDir,
			ProfileID: candidate.ProfileID,
			EventID:   candidate.EventID,
		})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("download failed for profile %s: %v", candidate.ProfileID, err))
			continue
		}
		path := findProfileByType(download.Files, profileType)
		if path == "" {
			warnings = append(warnings, fmt.Sprintf("profile type %q not found for %s", profileType, candidate.ProfileID))
			continue
		}
		paths = append(paths, path)
	}

	if len(paths) == 0 {
		return AggregateProfilesResult{}, fmt.Errorf("no profiles available to merge")
	}

	return AggregateProfilesResult{
		Service:        params.Service,
		Env:            params.Env,
		DDSite:         listResult.DDSite,
		ProfileType:    profileType,
		TimeRange:      AggregateTimeRange{From: fromTS, To: toTS},
		ProfilesMerged: len(paths),
		ProfilePaths:   paths,
		Warnings:       warnings,
	}, nil
}
