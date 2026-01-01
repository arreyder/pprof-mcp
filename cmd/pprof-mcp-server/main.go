package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/pprof/profile"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/arreyder/pprof-mcp/internal/d2"
	"github.com/arreyder/pprof-mcp/internal/datadog"
	"github.com/arreyder/pprof-mcp/internal/pprof"
	"github.com/arreyder/pprof-mcp/internal/profiles"
	"github.com/arreyder/pprof-mcp/internal/services"
)

func main() {
	nameModeFlag := flag.String("tool-name-mode", "", "Tool name mode: default or codex")
	flag.Parse()

	s := mcp.NewServer(&mcp.Implementation{
		Name:    "pprof-mcp",
		Title:   "pprof MCP",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Instructions: "Profiling tools for Datadog profile download and deterministic pprof analysis.",
	})

	nameMode := toolNameModeFromEnv()
	if strings.TrimSpace(*nameModeFlag) != "" {
		nameMode = toolNameModeFromString(strings.ToLower(strings.TrimSpace(*nameModeFlag)))
	}
	registry := NewToolRegistry()
	if err := registry.AddAll(ToolSchemas()); err != nil {
		log.Fatalf("Tool registry error: %v", err)
	}
	for _, def := range registry.List() {
		def := def
		tool := *def.Tool
		canonicalName := def.Tool.Name
		tool.Name = toolNameForMode(canonicalName, nameMode)
		if nameMode == toolNameModeCodex {
			tool.Description = fmt.Sprintf("Codex tool name: %s\n\n%s", tool.Name, tool.Description)
		}
		mcp.AddTool(s, &tool, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			return invokeTool(ctx, &tool, canonicalName, def.Handler, args)
		})
	}

	log.Println("Starting pprof MCP server over stdio")
	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Error serving MCP: %v", err)
		os.Exit(1)
	}
}

func invokeTool(ctx context.Context, tool *mcp.Tool, canonicalName string, handler ToolHandler, args map[string]any) (*mcp.CallToolResult, any, error) {
	if err := ValidateArgsWithName(tool, canonicalName, args); err != nil {
		return ErrorResult(err, ""), nil, nil
	}

	cleanedArgs, err := sanitizeArgs(args)
	if err != nil {
		return ErrorResult(err, "Provide paths within PPROF_MCP_BASEDIR if it is set."), nil, nil
	}

	result, err := handler(ctx, cleanedArgs)
	if err != nil {
		if errors.Is(err, pprof.ErrNoMatches) {
			return noMatchesResult(tool.Name, cleanedArgs, err), nil, nil
		}
		return ErrorResult(err, ""), nil, nil
	}

	switch v := result.(type) {
	case ToolOutput:
		res := TextResult(v.Text)
		if v.Structured != nil {
			return res, v.Structured, nil
		}
		return res, nil, nil
	case *ToolOutput:
		res := TextResult(v.Text)
		if v.Structured != nil {
			return res, v.Structured, nil
		}
		return res, nil, nil
	case string:
		return TextResult(v), nil, nil
	case []mcp.Content:
		return &mcp.CallToolResult{Content: v}, nil, nil
	case mcp.Content:
		return &mcp.CallToolResult{Content: []mcp.Content{v}}, nil, nil
	default:
		return formatUnexpectedResult(v), nil, nil
	}
}

func noMatchesResult(toolName string, args map[string]any, err error) *mcp.CallToolResult {
	hint := "Try a broader regex (e.g., (?i)GetLimits), or use pprof.top with focus to find the exact function name."
	pattern := firstNonEmpty(
		getString(args, "regex"),
		getString(args, "function"),
		getString(args, "focus"),
		getString(args, "tag_focus"),
		getString(args, "tag_show"),
	)
	msg := "No matching symbols found."
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		msg = strings.TrimSpace(err.Error())
	}

	payload := map[string]any{
		"matched": false,
		"reason":  "no_matches",
		"tool":    toolName,
		"hint":    hint,
	}
	if pattern != "" {
		payload["pattern"] = pattern
	}

	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: msg + "\nHint: " + hint}},
		StructuredContent: payload,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func profilesDownloadAutoTool(ctx context.Context, args map[string]any) (interface{}, error) {
	// Detect environment
	isD2 := d2.IsD2Environment()

	if isD2 {
		// D2 mode - use local kubectl download
		service := getString(args, "service")
		outDir := getString(args, "out_dir")
		seconds := getInt(args, "seconds", 30)

		result, err := d2.DownloadProfiles(ctx, d2.DownloadParams{
			Service: service,
			OutDir:  outDir,
			Seconds: seconds,
		})
		if err != nil {
			return nil, fmt.Errorf("d2 download failed: %w", err)
		}

		// Register profile handles
		timestamp := time.Now().UTC().Format(time.RFC3339)
		handles := []map[string]any{}
		for _, file := range result.Files {
			handle, err := profileRegistry.Register(profiles.Metadata{
				Service:   result.Service,
				Env:       "d2",
				Type:      file.Type,
				Timestamp: timestamp,
				Path:      file.Path,
				Bytes:     file.Bytes,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to register profile handle: %w", err)
			}
			handles = append(handles, map[string]any{
				"type":   file.Type,
				"handle": handle,
				"bytes":  file.Bytes,
			})
		}

		resultPayload := map[string]any{
			"service":   result.Service,
			"namespace": result.Namespace,
			"pod_name":  result.PodName,
			"pod_ip":    result.PodIP,
			"files":     handles,
		}
		if len(result.Warnings) > 0 {
			resultPayload["warnings"] = result.Warnings
		}

		payload := map[string]any{
			"command": fmt.Sprintf("kubectl port-forward -n %s %s 1337:1337 (d2 mode)", result.Namespace, result.PodName),
			"mode":    "d2",
			"result":  resultPayload,
		}
		return marshalJSON(payload)
	}

	// Datadog mode - use Datadog API
	service := getString(args, "service")
	env := getString(args, "env")
	if env == "" {
		return nil, fmt.Errorf("env parameter required for Datadog mode (not in d2 environment)")
	}
	outDir := getString(args, "out_dir")
	hours := getInt(args, "hours", 72)
	site := getString(args, "dd_site")
	if site == "" {
		site = getString(args, "site")
	}
	profileID := getString(args, "profile_id")
	eventID := getString(args, "event_id")

	result, err := datadog.DownloadLatestBundle(ctx, datadog.DownloadParams{
		Service:   service,
		Env:       env,
		OutDir:    outDir,
		Site:      site,
		Hours:     hours,
		ProfileID: profileID,
		EventID:   eventID,
	})
	if err != nil {
		return nil, fmt.Errorf("datadog download failed: %w", err)
	}

	bundle, err := registerBundleHandles(result)
	if err != nil {
		return nil, err
	}

	resultPayload := map[string]any{
		"service":    result.Service,
		"env":        result.Env,
		"dd_site":    result.DDSite,
		"from_ts":    result.FromTS,
		"to_ts":      result.ToTS,
		"profile_id": result.ProfileID,
		"event_id":   result.EventID,
		"timestamp":  result.Timestamp,
		"files":      bundle.Handles,
	}
	if result.MetricsPath != "" {
		resultPayload["metrics_path"] = result.MetricsPath
	}
	if len(result.Warnings) > 0 {
		resultPayload["warnings"] = result.Warnings
	}

	payload := map[string]any{
		"command": fmt.Sprintf("%s (datadog mode)", buildDownloadCommand(service, env, outDir, hours, site, profileID, eventID)),
		"mode":    "datadog",
		"result":  resultPayload,
	}
	return marshalJSON(payload)
}

func downloadTool(ctx context.Context, args map[string]any) (interface{}, error) {
	service := getString(args, "service")
	env := getString(args, "env")
	outDir := getString(args, "out_dir")
	hours := getInt(args, "hours", 72)
	site := getString(args, "dd_site")
	if site == "" {
		site = getString(args, "site")
	}
	profileID := getString(args, "profile_id")
	eventID := getString(args, "event_id")

	result, err := datadog.DownloadLatestBundle(ctx, datadog.DownloadParams{
		Service:   service,
		Env:       env,
		OutDir:    outDir,
		Site:      site,
		Hours:     hours,
		ProfileID: profileID,
		EventID:   eventID,
	})
	if err != nil {
		return nil, err
	}

	bundle, err := registerBundleHandles(result)
	if err != nil {
		return nil, err
	}

	resultPayload := map[string]any{
		"service":    result.Service,
		"env":        result.Env,
		"dd_site":    result.DDSite,
		"from_ts":    result.FromTS,
		"to_ts":      result.ToTS,
		"profile_id": result.ProfileID,
		"event_id":   result.EventID,
		"timestamp":  result.Timestamp,
		"files":      bundle.Handles,
	}
	if result.MetricsPath != "" {
		resultPayload["metrics_path"] = result.MetricsPath
	}
	if len(result.Warnings) > 0 {
		resultPayload["warnings"] = result.Warnings
	}

	payload := map[string]any{
		"command": buildDownloadCommand(service, env, outDir, hours, site, profileID, eventID),
		"result":  resultPayload,
	}
	return marshalJSON(payload)
}

func d2DownloadTool(ctx context.Context, args map[string]any) (interface{}, error) {
	service := getString(args, "service")
	outDir := getString(args, "out_dir")
	seconds := getInt(args, "seconds", 30)

	result, err := d2.DownloadProfiles(ctx, d2.DownloadParams{
		Service: service,
		OutDir:  outDir,
		Seconds: seconds,
	})
	if err != nil {
		return nil, err
	}

	// Register profile handles similar to datadog download
	timestamp := time.Now().UTC().Format(time.RFC3339)
	handles := []map[string]any{}
	for _, file := range result.Files {
		handle, err := profileRegistry.Register(profiles.Metadata{
			Service:   result.Service,
			Env:       "d2",
			Type:      file.Type,
			Timestamp: timestamp,
			Path:      file.Path,
			Bytes:     file.Bytes,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to register profile handle: %w", err)
		}
		handles = append(handles, map[string]any{
			"type":   file.Type,
			"handle": handle,
			"bytes":  file.Bytes,
		})
	}

	resultPayload := map[string]any{
		"service":   result.Service,
		"namespace": result.Namespace,
		"pod_name":  result.PodName,
		"pod_ip":    result.PodIP,
		"files":     handles,
	}
	if len(result.Warnings) > 0 {
		resultPayload["warnings"] = result.Warnings
	}

	payload := map[string]any{
		"command": fmt.Sprintf("kubectl port-forward -n %s %s 4421:4421", result.Namespace, result.PodName),
		"result":  resultPayload,
	}
	return marshalJSON(payload)
}

func pprofTopTool(ctx context.Context, args map[string]any) (interface{}, error) {
	profilePath := getString(args, "profile")
	sampleIndex := getString(args, "sample_index")

	result, err := pprof.RunTop(ctx, pprof.TopParams{
		Profile:     profilePath,
		Binary:      getString(args, "binary"),
		Cum:         getBool(args, "cum"),
		NodeCount:   getInt(args, "nodecount", 0),
		Focus:       getString(args, "focus"),
		Ignore:      getString(args, "ignore"),
		SampleIndex: sampleIndex,
	})
	if err != nil {
		return nil, err
	}

	// Add contextual hints based on profile type
	pprof.AddTopHints(&result, profilePath, sampleIndex)

	payload := map[string]any{
		"command": result.Command,
		"raw":     result.Raw,
		"rows":    result.Rows,
		"summary": result.Summary,
	}
	if len(result.Hints) > 0 {
		payload["hints"] = result.Hints
	}
	if getBool(args, "compare_baseline") {
		baselinePath := getString(args, "baseline_path")
		if baselinePath == "" {
			var err error
			baselinePath, err = defaultBaselinePath()
			if err != nil {
				return nil, err
			}
		}
		meta, err := pprof.RunMeta(profilePath)
		if err != nil {
			return nil, err
		}
		sampleKey := sampleIndex
		if sampleKey == "" {
			sampleKey = "default"
		}
		baselineKey := buildBaselineKey(
			getString(args, "service"),
			getString(args, "env"),
			getString(args, "baseline_key"),
			meta.DetectedKind,
			sampleKey,
		)
		baseline, err := compareAndUpdateBaseline(baselinePath, baselineKey, meta.DetectedKind, sampleKey, result.Rows)
		if err != nil {
			return nil, err
		}
		payload["baseline"] = baseline
	}
	return marshalJSON(payload)
}

func pprofPeekTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunPeek(ctx, pprof.PeekParams{
		Profile:     getString(args, "profile"),
		Binary:      getString(args, "binary"),
		Regex:       getString(args, "regex"),
		SampleIndex: getString(args, "sample_index"),
	})
	if err != nil {
		return nil, err
	}

	raw := result.Raw
	if maxLines := getInt(args, "max_lines", 0); maxLines > 0 {
		trimmed, total, truncated := truncateLines(raw, maxLines)
		raw = trimmed
		payload := map[string]any{
			"command":     result.Command,
			"raw":         raw,
			"total_lines": total,
			"truncated":   truncated,
		}
		return marshalJSON(payload)
	}

	payload := map[string]any{
		"command": result.Command,
		"raw":     raw,
	}
	return marshalJSON(payload)
}

func pprofListTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunList(ctx, pprof.ListParams{
		Profile:     getString(args, "profile"),
		Binary:      getString(args, "binary"),
		Function:    getString(args, "function"),
		RepoRoot:    getString(args, "repo_root"),
		TrimPath:    getString(args, "trim_path"),
		SourcePaths: parseStringList(args, "source_paths"),
	})
	if err != nil {
		return nil, err
	}

	raw := result.Raw
	if maxLines := getInt(args, "max_lines", 0); maxLines > 0 {
		trimmed, total, truncated := truncateLines(raw, maxLines)
		raw = trimmed
		payload := map[string]any{
			"command":     result.Command,
			"raw":         raw,
			"total_lines": total,
			"truncated":   truncated,
		}
		return marshalJSON(payload)
	}

	payload := map[string]any{
		"command": result.Command,
		"raw":     raw,
	}
	return marshalJSON(payload)
}

func pprofTracesTool(ctx context.Context, args map[string]any) (interface{}, error) {
	lines := getInt(args, "lines", 0)
	if lines == 0 {
		lines = getInt(args, "max_lines", defaultTracesLines)
	}
	if lines > maxTracesLines {
		lines = maxTracesLines
	}

	result, err := pprof.RunTracesHead(ctx, pprof.TracesParams{
		Profile: getString(args, "profile"),
		Binary:  getString(args, "binary"),
		Lines:   lines,
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command":     result.Command,
		"raw":         result.Raw,
		"total_lines": result.TotalLines,
		"truncated":   result.Truncated,
	}
	return marshalJSON(payload)
}

func pprofDiffTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunDiffTop(ctx, pprof.DiffTopParams{
		Before:      getString(args, "before"),
		After:       getString(args, "after"),
		Binary:      getString(args, "binary"),
		Cum:         getBool(args, "cum"),
		NodeCount:   getInt(args, "nodecount", 0),
		Focus:       getString(args, "focus"),
		Ignore:      getString(args, "ignore"),
		SampleIndex: getString(args, "sample_index"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"commands": result.Commands,
		"before":   result.Before,
		"after":    result.After,
		"deltas":   result.Deltas,
	}
	return marshalJSON(payload)
}

func pprofRegressionCheckTool(ctx context.Context, args map[string]any) (interface{}, error) {
	checks, err := parseRegressionChecks(args)
	if err != nil {
		return nil, err
	}

	result, err := pprof.RunRegressionCheck(ctx, pprof.RegressionCheckParams{
		Profile:     getString(args, "profile"),
		SampleIndex: getString(args, "sample_index"),
		Checks:      checks,
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": "pprof regression_check",
		"result":  result,
	}
	summary := "All regression checks passed."
	if !result.Passed {
		summary = "One or more regression checks failed."
	}
	return marshalJSONWithSummary(summary, payload)
}

func pprofMetaTool(ctx context.Context, args map[string]any) (interface{}, error) {
	profilePath := getString(args, "profile")
	meta, err := pprof.RunMeta(profilePath)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": pprof.FormatMetaCommand(profilePath),
		"result":  meta,
	}
	return marshalJSON(payload)
}

func pprofStorylinesTool(ctx context.Context, args map[string]any) (interface{}, error) {
	prefixes := parseStringList(args, "repo_prefix")
	result, err := pprof.RunStorylines(ctx, pprof.StorylinesParams{
		Profile:      getString(args, "profile"),
		N:            getInt(args, "n", 4),
		Focus:        getString(args, "focus"),
		Ignore:       getString(args, "ignore"),
		RepoPrefixes: prefixes,
		RepoRoot:     getString(args, "repo_root"),
		TrimPath:     getString(args, "trim_path"),
		SampleIndex:  getString(args, "sample_index"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": result.Command,
		"result":  result,
	}
	return marshalJSON(payload)
}

func pprofMemorySanityTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunMemorySanity(ctx, pprof.MemorySanityParams{
		HeapProfile:      getString(args, "heap_profile"),
		GoroutineProfile: getString(args, "goroutine_profile"),
		Binary:           getString(args, "binary"),
		ContainerRSSMB:   getInt(args, "container_rss_mb", 0),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": "pprof memory_sanity",
		"result":  result,
	}
	return marshalJSON(payload)
}

func pprofGoroutineAnalysisTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunGoroutineAnalysis(pprof.GoroutineAnalysisParams{
		Profile: getString(args, "profile"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": "pprof goroutine_analysis",
		"result":  result,
	}
	summary := fmt.Sprintf("Found %d goroutines across %d states.", result.TotalGoroutines, len(result.ByState))
	return marshalJSONWithSummary(summary, payload)
}

func pprofContentionAnalysisTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunContentionAnalysis(pprof.ContentionAnalysisParams{
		Profile: getString(args, "profile"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": "pprof contention_analysis",
		"result":  result,
	}
	summary := fmt.Sprintf("Contention summary: %d contentions, %s total delay.", result.TotalContentions, result.TotalDelay)
	return marshalJSONWithSummary(summary, payload)
}

func pprofDiscoverTool(ctx context.Context, args map[string]any) (interface{}, error) {
	service := getString(args, "service")
	env := getString(args, "env")
	outDir := getString(args, "out_dir")
	if outDir == "" {
		var err error
		outDir, err = os.MkdirTemp("", "pprof-discover-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
	}
	hours := getInt(args, "hours", 72)
	site := getString(args, "dd_site")
	if site == "" {
		site = getString(args, "site")
	}
	profileID := getString(args, "profile_id")
	eventID := getString(args, "event_id")

	downloadResult, err := datadog.DownloadLatestBundle(ctx, datadog.DownloadParams{
		Service:   service,
		Env:       env,
		OutDir:    outDir,
		Hours:     hours,
		Site:      site,
		ProfileID: profileID,
		EventID:   eventID,
	})
	if err != nil {
		return nil, err
	}

	bundle, err := registerBundleHandles(downloadResult)
	if err != nil {
		return nil, err
	}

	profileInputs := make([]pprof.DiscoveryProfileInput, 0, len(downloadResult.Files))
	for _, file := range downloadResult.Files {
		profileInputs = append(profileInputs, pprof.DiscoveryProfileInput{
			Type:   file.Type,
			Path:   file.Path,
			Handle: bundle.HandleByType[file.Type],
			Bytes:  file.Bytes,
		})
	}

	report, err := pprof.RunDiscovery(ctx, pprof.DiscoveryParams{
		Service:        service,
		Env:            env,
		Timestamp:      downloadResult.Timestamp,
		Profiles:       profileInputs,
		RepoPrefixes:   parseStringList(args, "repo_prefix"),
		ContainerRSSMB: getInt(args, "container_rss_mb", 0),
	})
	if err != nil {
		return nil, err
	}
	if len(downloadResult.Warnings) > 0 {
		report.Warnings = append(report.Warnings, downloadResult.Warnings...)
	}

	payload := map[string]any{
		"command": "pprof discover",
		"result":  report,
	}
	summary := fmt.Sprintf("Discovery complete for %s/%s with %d recommendations.", service, env, len(report.Recommendations))
	return marshalJSONWithSummary(summary, payload)
}

func pprofCrossCorrelateTool(ctx context.Context, args map[string]any) (interface{}, error) {
	bundlePaths, warnings, err := resolveBundlePaths(args["bundle"])
	if err != nil {
		return nil, err
	}

	result, err := pprof.RunCrossCorrelate(ctx, pprof.CrossCorrelateParams{
		Profiles:  bundlePaths,
		NodeCount: getInt(args, "nodecount", 0),
	})
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
	}

	payload := map[string]any{
		"command": "pprof cross_correlate",
		"result":  result,
	}
	summary := fmt.Sprintf("Found %d correlated hotspots.", len(result.Correlations))
	return marshalJSONWithSummary(summary, payload)
}

func pprofHotspotSummaryTool(ctx context.Context, args map[string]any) (interface{}, error) {
	bundlePaths, warnings, err := resolveBundlePaths(args["bundle"])
	if err != nil {
		return nil, err
	}

	result, err := pprof.RunHotspotSummary(ctx, pprof.HotspotSummaryParams{
		Profiles:  bundlePaths,
		NodeCount: getInt(args, "nodecount", 0),
	})
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
	}

	payload := map[string]any{
		"command": "pprof hotspot_summary",
		"result":  result,
	}
	summary := "Hotspot summary generated."
	if result.GoroutineCount != nil {
		summary = fmt.Sprintf("Hotspot summary generated with %d goroutines.", *result.GoroutineCount)
	}
	return marshalJSONWithSummary(summary, payload)
}

func pprofTraceSourceTool(ctx context.Context, args map[string]any) (interface{}, error) {
	showVendor := true
	if _, ok := args["show_vendor"]; ok {
		showVendor = getBool(args, "show_vendor")
	}
	result, err := pprof.RunTraceSource(pprof.TraceSourceParams{
		Profile:      getString(args, "profile"),
		Function:     getString(args, "function"),
		RepoRoot:     getString(args, "repo_root"),
		MaxDepth:     getInt(args, "max_depth", 0),
		ShowVendor:   showVendor,
		ContextLines: getInt(args, "context_lines", 0),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": "pprof trace_source",
		"result":  result,
	}
	summary := fmt.Sprintf("Traced %d functions.", result.TotalFunctionsTraced)
	return marshalJSONWithSummary(summary, payload)
}

func pprofVendorAnalyzeTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunVendorAnalyze(ctx, pprof.VendorAnalyzeParams{
		Profile:      getString(args, "profile"),
		RepoRoot:     getString(args, "repo_root"),
		MinPct:       getFloat(args, "min_pct", 0),
		CheckUpdates: getBool(args, "check_updates"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": "pprof vendor_analyze",
		"result":  result,
	}
	summary := fmt.Sprintf("Found %d vendor hotspots.", len(result.VendorHotspots))
	return marshalJSONWithSummary(summary, payload)
}

func pprofExplainOverheadTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunExplainOverhead(ctx, pprof.ExplainOverheadParams{
		Profile:     getString(args, "profile"),
		Category:    getString(args, "category"),
		Function:    getString(args, "function"),
		DetailLevel: getString(args, "detail_level"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": "pprof explain_overhead",
		"result":  result,
	}
	summary := fmt.Sprintf("Explanation generated for %s.", result.Category)
	return marshalJSONWithSummary(summary, payload)
}

func pprofSuggestFixTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunSuggestFix(ctx, pprof.SuggestFixParams{
		Profile:        getString(args, "profile"),
		Issue:          getString(args, "issue"),
		RepoRoot:       getString(args, "repo_root"),
		TargetFunction: getString(args, "target_function"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": "pprof suggest_fix",
		"result":  result,
	}
	outputFormat := strings.ToLower(strings.TrimSpace(getString(args, "output_format")))
	if outputFormat == "diff" {
		diff := ""
		if len(result.ApplicableFixes) > 0 {
			diff = result.ApplicableFixes[0].Diff
		}
		return ToolOutput{Text: diff, Structured: payload}, nil
	}
	if outputFormat == "pr_description" {
		desc := ""
		if len(result.ApplicableFixes) > 0 {
			desc = result.ApplicableFixes[0].PRDescription
		}
		return ToolOutput{Text: desc, Structured: payload}, nil
	}

	summary := fmt.Sprintf("Generated %d fix suggestions.", len(result.ApplicableFixes))
	return marshalJSONWithSummary(summary, payload)
}

func datadogProfilesListTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := datadog.ListProfiles(ctx, datadog.ListProfilesParams{
		Service: getString(args, "service"),
		Env:     getString(args, "env"),
		From:    getString(args, "from"),
		To:      getString(args, "to"),
		Hours:   getInt(args, "hours", 72),
		Limit:   getInt(args, "limit", 50),
		Site:    getString(args, "site"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": fmt.Sprintf("profctl datadog profiles list --service %s --env %s", result.Service, result.Env),
		"result":  result,
	}
	summary := fmt.Sprintf("Found %d profiles for %s/%s from %s to %s.", len(result.Candidates), result.Service, result.Env, result.FromTS, result.ToTS)
	return marshalJSONWithSummary(summary, payload)
}

func datadogProfilesPickTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := datadog.PickProfile(ctx, datadog.PickProfilesParams{
		Service:  getString(args, "service"),
		Env:      getString(args, "env"),
		From:     getString(args, "from"),
		To:       getString(args, "to"),
		Hours:    getInt(args, "hours", 72),
		Limit:    getInt(args, "limit", 50),
		Site:     getString(args, "site"),
		Strategy: datadog.PickStrategy(getString(args, "strategy")),
		TargetTS: getString(args, "target_ts"),
		Index:    getInt(args, "index", -1),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": fmt.Sprintf("profctl datadog profiles pick --service %s --env %s", getString(args, "service"), getString(args, "env")),
		"result":  result,
	}
	return marshalJSON(payload)
}

func datadogProfilesAggregateTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := datadog.AggregateProfiles(ctx, datadog.AggregateProfilesParams{
		Service:     getString(args, "service"),
		Env:         getString(args, "env"),
		Window:      getString(args, "window"),
		Limit:       getInt(args, "limit", 10),
		Site:        getString(args, "site"),
		OutDir:      getString(args, "out_dir"),
		ProfileType: getString(args, "profile_type"),
	})
	if err != nil {
		return nil, err
	}

	outputPath := ""
	if len(result.ProfilePaths) == 1 {
		outputPath = result.ProfilePaths[0]
	} else {
		mergePath, err := buildAggregateOutputPath(result.ProfileType, result.ProfilePaths[0])
		if err != nil {
			return nil, err
		}
		mergeResult, err := pprof.RunMerge(ctx, pprof.MergeParams{
			Profiles:   result.ProfilePaths,
			OutputPath: mergePath,
		})
		if err != nil {
			return nil, err
		}
		outputPath = mergeResult.OutputPath
	}

	meta, err := pprof.RunMeta(outputPath)
	if err != nil {
		return nil, err
	}
	totalDuration := formatDurationNanos(meta.DurationNanos)

	handle, err := profileRegistry.Register(profiles.Metadata{
		Service:   result.Service,
		Env:       result.Env,
		Type:      result.ProfileType,
		Timestamp: result.TimeRange.To,
		Path:      outputPath,
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": fmt.Sprintf("profctl datadog profiles aggregate --service %s --env %s --window %s", result.Service, result.Env, getString(args, "window")),
		"result": map[string]any{
			"handle":          handle,
			"profile_type":    result.ProfileType,
			"profiles_merged": len(result.ProfilePaths),
			"time_range": map[string]any{
				"from": result.TimeRange.From,
				"to":   result.TimeRange.To,
			},
			"total_duration": totalDuration,
			"hint":           fmt.Sprintf("Use pprof.top(profile=%q) to analyze aggregated data.", handle),
			"warnings":       result.Warnings,
		},
	}
	summary := fmt.Sprintf("Aggregated %d profiles into %s.", len(result.ProfilePaths), handle)
	return marshalJSONWithSummary(summary, payload)
}

func repoServicesTool(ctx context.Context, args map[string]any) (interface{}, error) {
	repoRoot := getString(args, "repo_root")
	if repoRoot == "" {
		repoRoot = "."
	}
	items, err := services.Discover(repoRoot)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command":  fmt.Sprintf("profctl repo services discover --repo_root %s", repoRoot),
		"services": items,
	}
	return marshalJSON(payload)
}

func datadogMetricsDiscoverTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := datadog.DiscoverMetrics(ctx, datadog.MetricsDiscoverParams{
		Service: getString(args, "service"),
		Env:     getString(args, "env"),
		Site:    getString(args, "site"),
		Query:   getString(args, "query"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": fmt.Sprintf("profctl datadog metrics discover --service %s", getString(args, "service")),
		"result":  result,
		"table":   datadog.FormatMetricsTable(result.Metrics),
	}
	return marshalJSON(payload)
}

func datadogProfilesCompareRangeTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := datadog.CompareRange(ctx, datadog.CompareRangeParams{
		Service:     getString(args, "service"),
		Env:         getString(args, "env"),
		Site:        getString(args, "site"),
		BeforeFrom:  getString(args, "before_from"),
		BeforeTo:    getString(args, "before_to"),
		AfterFrom:   getString(args, "after_from"),
		AfterTo:     getString(args, "after_to"),
		OutDir:      getString(args, "out_dir"),
		ProfileType: getString(args, "profile_type"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command":   "profctl datadog profiles compare_range",
		"result":    result,
		"formatted": datadog.FormatCompareResult(result),
	}
	return marshalJSON(payload)
}

func datadogProfilesNearEventTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := datadog.FindProfilesNearEvent(ctx, datadog.NearEventParams{
		Service:   getString(args, "service"),
		Env:       getString(args, "env"),
		Site:      getString(args, "site"),
		EventTime: getString(args, "event_time"),
		Window:    getString(args, "window"),
		Limit:     getInt(args, "limit", 10),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command":   "profctl datadog profiles near_event",
		"result":    result,
		"formatted": datadog.FormatNearEventResult(result),
	}
	return marshalJSON(payload)
}

func pprofTagsTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunTags(ctx, pprof.TagsParams{
		Profile:     getString(args, "profile"),
		Binary:      getString(args, "binary"),
		TagFocus:    getString(args, "tag_focus"),
		TagIgnore:   getString(args, "tag_ignore"),
		TagShow:     getString(args, "tag_show"),
		Cum:         getBool(args, "cum"),
		NodeCount:   getInt(args, "nodecount", 0),
		SampleIndex: getString(args, "sample_index"),
	})
	if err != nil {
		return nil, err
	}

	raw := result.Raw
	if maxLines := getInt(args, "max_lines", 0); maxLines > 0 {
		trimmed, total, truncated := truncateLines(raw, maxLines)
		raw = trimmed
		payload := map[string]any{
			"command":     result.Command,
			"raw":         raw,
			"total_lines": total,
			"truncated":   truncated,
		}
		if len(result.Tags) > 0 {
			payload["tags"] = result.Tags
		}
		return marshalJSON(payload)
	}

	payload := map[string]any{
		"command": result.Command,
		"raw":     raw,
	}
	if len(result.Tags) > 0 {
		payload["tags"] = result.Tags
	}
	return marshalJSON(payload)
}

func pprofFlamegraphTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunFlamegraph(ctx, pprof.FlamegraphParams{
		Profile:     getString(args, "profile"),
		Binary:      getString(args, "binary"),
		OutputPath:  getString(args, "output_path"),
		Focus:       getString(args, "focus"),
		Ignore:      getString(args, "ignore"),
		TagFocus:    getString(args, "tag_focus"),
		TagIgnore:   getString(args, "tag_ignore"),
		SampleIndex: getString(args, "sample_index"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command":     result.Command,
		"output_path": result.OutputPath,
		"message":     result.Message,
	}
	return marshalJSON(payload)
}

func pprofCallgraphTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunCallgraph(ctx, pprof.CallgraphParams{
		Profile:     getString(args, "profile"),
		Binary:      getString(args, "binary"),
		OutputPath:  getString(args, "output_path"),
		Format:      getString(args, "format"),
		Focus:       getString(args, "focus"),
		Ignore:      getString(args, "ignore"),
		NodeCount:   getInt(args, "nodecount", 0),
		EdgeFrac:    getFloat(args, "edge_frac", 0),
		NodeFrac:    getFloat(args, "node_frac", 0),
		SampleIndex: getString(args, "sample_index"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command":     result.Command,
		"output_path": result.OutputPath,
		"format":      result.Format,
		"message":     result.Message,
	}
	return marshalJSON(payload)
}

func pprofFocusPathsTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunFocusPaths(ctx, pprof.FocusPathsParams{
		Profile:     getString(args, "profile"),
		Binary:      getString(args, "binary"),
		Function:    getString(args, "function"),
		Cum:         getBool(args, "cum"),
		NodeCount:   getInt(args, "nodecount", 0),
		SampleIndex: getString(args, "sample_index"),
	})
	if err != nil {
		return nil, err
	}

	raw := result.Raw
	if maxLines := getInt(args, "max_lines", 0); maxLines > 0 {
		trimmed, total, truncated := truncateLines(raw, maxLines)
		raw = trimmed
		payload := map[string]any{
			"command":     result.Command,
			"raw":         raw,
			"total_lines": total,
			"truncated":   truncated,
		}
		return marshalJSON(payload)
	}

	payload := map[string]any{
		"command": result.Command,
		"raw":     raw,
	}
	return marshalJSON(payload)
}

func pprofMergeTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunMerge(ctx, pprof.MergeParams{
		Profiles:   parseStringList(args, "profiles"),
		Binary:     getString(args, "binary"),
		OutputPath: getString(args, "output_path"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command":     result.Command,
		"output_path": result.OutputPath,
		"input_count": result.InputCount,
		"message":     result.Message,
	}
	return marshalJSON(payload)
}

func functionHistoryTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := datadog.SearchFunctionHistory(ctx, datadog.FunctionHistoryParams{
		Service:  getString(args, "service"),
		Env:      getString(args, "env"),
		Function: getString(args, "function"),
		From:     getString(args, "from"),
		To:       getString(args, "to"),
		Hours:    getInt(args, "hours", 72),
		Limit:    getInt(args, "limit", 10),
		Site:     getString(args, "site"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": fmt.Sprintf("profctl function-history --service %s --env %s --function %s",
			result.Service, result.Env, result.Function),
		"result": result,
		"table":  datadog.FormatFunctionHistoryTable(result),
	}
	summary := fmt.Sprintf("Function %s found in %d/%d profiles.", result.Function, result.Summary.FoundInProfiles, result.Summary.TotalProfiles)
	return marshalJSONWithSummary(summary, payload)
}

func pprofAllocPathsTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunAllocPaths(pprof.AllocPathsParams{
		Profile:       getString(args, "profile"),
		MinPercent:    getFloat(args, "min_percent", 1.0),
		MaxPaths:      getInt(args, "max_paths", 20),
		RepoPrefixes:  parseStringList(args, "repo_prefix"),
		GroupBySource: getBool(args, "group_by_source"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": "pprof alloc_paths",
		"result":  result,
	}
	if len(result.Warnings) > 0 {
		payload["warnings"] = result.Warnings
	}
	summary := fmt.Sprintf("Analyzed %s total allocations, found %d allocation paths above threshold.",
		result.TotalAllocStr, len(result.Paths))
	return marshalJSONWithSummary(summary, payload)
}

func pprofOverheadReportTool(ctx context.Context, args map[string]any) (interface{}, error) {
	profilePath := getString(args, "profile")

	prof, err := loadProfile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load profile: %w", err)
	}

	// Find sample index
	sampleIndex := 0
	if si := getString(args, "sample_index"); si != "" {
		for i, st := range prof.SampleType {
			if st.Type == si {
				sampleIndex = i
				break
			}
		}
	}

	result := pprof.DetectOverhead(prof, sampleIndex)

	payload := map[string]any{
		"command": "pprof overhead_report",
		"result":  result,
	}

	// Generate hints for high-overhead categories
	hints := pprof.GenerateOverheadHints(result)
	if len(hints) > 0 {
		payload["hints"] = hints
	}

	summary := fmt.Sprintf("Total observability overhead: %.1f%% (%d categories detected)",
		result.TotalOverhead, len(result.Detections))
	return marshalJSONWithSummary(summary, payload)
}

func pprofGenerateReportTool(ctx context.Context, args map[string]any) (interface{}, error) {
	inputs, err := parseReportInputs(args)
	if err != nil {
		return nil, err
	}

	result, err := pprof.GenerateReport(pprof.ReportParams{
		Title:  getString(args, "title"),
		Inputs: inputs,
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": "pprof generate_report",
		"result": map[string]any{
			"markdown": result.Markdown,
		},
	}
	summary := fmt.Sprintf("Generated report with %d sections.", result.SectionCount)
	return marshalJSONWithSummary(summary, payload)
}

func pprofDetectRepoTool(ctx context.Context, args map[string]any) (interface{}, error) {
	profilePath := getString(args, "profile")

	prof, err := loadProfile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load profile: %w", err)
	}

	result := pprof.DetectRepoFromProfile(prof)

	payload := map[string]any{
		"command": "pprof detect_repo",
		"result":  result,
	}

	var summary string
	if result.DetectedRoot != "" {
		summary = fmt.Sprintf("Detected local repo at %s (confidence: %s)", result.DetectedRoot, result.Confidence)
	} else {
		summary = fmt.Sprintf("Found %d module paths but no local repo match", len(result.ModulePaths))
	}
	return marshalJSONWithSummary(summary, payload)
}

func loadProfile(path string) (*profile.Profile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return profile.Parse(file)
}

func getString(args map[string]any, key string) string {
	if val, ok := args[key]; ok {
		switch typed := val.(type) {
		case string:
			return typed
		case fmt.Stringer:
			return typed.String()
		}
	}
	return ""
}

func getInt(args map[string]any, key string, fallback int) int {
	if val, ok := args[key]; ok {
		switch typed := val.(type) {
		case int:
			return typed
		case int64:
			return int(typed)
		case float64:
			return int(typed)
		case json.Number:
			parsed, err := typed.Int64()
			if err == nil {
				return int(parsed)
			}
		case string:
			parsed, err := strconv.Atoi(typed)
			if err == nil {
				return parsed
			}
		}
	}
	return fallback
}

func getBool(args map[string]any, key string) bool {
	if val, ok := args[key]; ok {
		switch typed := val.(type) {
		case bool:
			return typed
		case string:
			parsed, err := strconv.ParseBool(typed)
			if err == nil {
				return parsed
			}
		}
	}
	return false
}

func getFloat(args map[string]any, key string, fallback float64) float64 {
	if val, ok := args[key]; ok {
		switch typed := val.(type) {
		case float64:
			return typed
		case float32:
			return float64(typed)
		case int:
			return float64(typed)
		case int64:
			return float64(typed)
		case json.Number:
			parsed, err := typed.Float64()
			if err == nil {
				return parsed
			}
		case string:
			parsed, err := strconv.ParseFloat(typed, 64)
			if err == nil {
				return parsed
			}
		}
	}
	return fallback
}

func parseStringList(args map[string]any, key string) []string {
	raw, ok := args[key]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if str, ok := item.(string); ok {
				values = append(values, str)
			}
		}
		return values
	case []string:
		return typed
	case string:
		return []string{typed}
	default:
		return nil
	}
}

func parseRegressionChecks(args map[string]any) ([]pprof.RegressionCheckSpec, error) {
	raw, ok := args["checks"]
	if !ok {
		return nil, fmt.Errorf("checks are required")
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("checks must be an array")
	}
	checks := make([]pprof.RegressionCheckSpec, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("check entries must be objects")
		}
		function, _ := obj["function"].(string)
		metric, _ := obj["metric"].(string)
		max, ok := floatFromAny(obj["max"])
		if !ok {
			return nil, fmt.Errorf("check max must be a number")
		}
		checks = append(checks, pprof.RegressionCheckSpec{
			Function: function,
			Metric:   metric,
			Max:      max,
		})
	}
	return checks, nil
}

func floatFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		if err == nil {
			return parsed, true
		}
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func formatDurationNanos(nanos int64) string {
	if nanos <= 0 {
		return ""
	}
	seconds := float64(nanos) / 1e9
	return fmt.Sprintf("%.1fs", seconds)
}

func buildAggregateOutputPath(profileType, samplePath string) (string, error) {
	if samplePath == "" {
		return "", fmt.Errorf("sample path required to build output path")
	}
	dir := filepath.Dir(samplePath)
	if dir == "" {
		dir = "."
	}
	name := fmt.Sprintf("merged_%s_%d.pprof", profileType, time.Now().Unix())
	return filepath.Join(dir, name), nil
}

func parseReportInputs(args map[string]any) ([]pprof.ReportInput, error) {
	raw, ok := args["inputs"]
	if !ok {
		return nil, fmt.Errorf("inputs are required")
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("inputs must be an array")
	}
	inputs := make([]pprof.ReportInput, 0, len(items))
	for idx, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("inputs[%d] must be an object", idx)
		}
		kind := getString(entry, "kind")
		if kind == "" {
			return nil, fmt.Errorf("inputs[%d] missing kind", idx)
		}
		dataRaw, ok := entry["data"]
		if !ok {
			return nil, fmt.Errorf("inputs[%d] missing data", idx)
		}
		data, ok := dataRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("inputs[%d].data must be an object", idx)
		}
		inputs = append(inputs, pprof.ReportInput{
			Kind: kind,
			Data: data,
		})
	}
	return inputs, nil
}

func buildDownloadCommand(service, env, outDir string, hours int, site, profileID, eventID string) string {
	base := fmt.Sprintf("profctl download --service %s --env %s --out %s --hours %d", service, env, outDir, hours)
	if profileID != "" {
		base += " --profile_id " + profileID
	}
	if eventID != "" {
		base += " --event_id " + eventID
	}
	if site != "" {
		base += " --dd_site " + site
	}
	return base
}

func marshalJSON(payload any) (ToolOutput, error) {
	return marshalJSONWithSummary("", payload)
}

func marshalJSONWithSummary(summary string, payload any) (ToolOutput, error) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ToolOutput{}, err
	}
	if len(data) == 0 {
		return ToolOutput{}, errors.New("empty JSON response")
	}
	text := string(data)
	if summary != "" {
		text = summary + "\n\n" + text
	}
	return ToolOutput{Text: text, Structured: payload}, nil
}
