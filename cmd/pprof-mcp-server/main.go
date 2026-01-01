package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/arreyder/pprof-mcp/internal/datadog"
	"github.com/arreyder/pprof-mcp/internal/pprof"
	"github.com/arreyder/pprof-mcp/internal/services"
)

func main() {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "pprof-mcp",
		Title:   "pprof MCP",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Instructions: "Profiling tools for Datadog profile download and deterministic pprof analysis.",
	})

	for _, def := range ToolSchemas() {
		def := def
		mcp.AddTool(s, def.Tool, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			return invokeTool(ctx, def.Handler, args)
		})
	}

	log.Println("Starting pprof MCP server over stdio")
	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Error serving MCP: %v", err)
		os.Exit(1)
	}
}

func invokeTool(ctx context.Context, handler ToolHandler, args map[string]any) (*mcp.CallToolResult, any, error) {
	result, err := handler(ctx, args)
	if err != nil {
		return nil, nil, err
	}

	switch v := result.(type) {
	case string:
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: v}},
		}, nil, nil
	case []mcp.Content:
		return &mcp.CallToolResult{Content: v}, nil, nil
	case mcp.Content:
		return &mcp.CallToolResult{Content: []mcp.Content{v}}, nil, nil
	default:
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%v", v)}},
		}, nil, nil
	}
}

func downloadTool(ctx context.Context, args map[string]any) (interface{}, error) {
	service := getString(args, "service")
	env := getString(args, "env")
	outDir := getString(args, "out_dir")
	hours := getInt(args, "hours", 72)
	site := getString(args, "dd_site")
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

	payload := map[string]any{
		"command": buildDownloadCommand(service, env, outDir, hours, site, profileID, eventID),
		"result":  result,
	}
	return marshalJSON(payload)
}

func pprofTopTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunTop(ctx, pprof.TopParams{
		Profile:     getString(args, "profile"),
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
		"command": result.Command,
		"raw":     result.Raw,
		"rows":    result.Rows,
		"summary": result.Summary,
	}
	return marshalJSON(payload)
}

func pprofPeekTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunPeek(ctx, pprof.PeekParams{
		Profile: getString(args, "profile"),
		Binary:  getString(args, "binary"),
		Regex:   getString(args, "regex"),
	})
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"command": result.Command,
		"raw":     result.Raw,
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

	payload := map[string]any{
		"command": result.Command,
		"raw":     result.Raw,
	}
	return marshalJSON(payload)
}

func pprofTracesTool(ctx context.Context, args map[string]any) (interface{}, error) {
	result, err := pprof.RunTracesHead(ctx, pprof.TracesParams{
		Profile: getString(args, "profile"),
		Binary:  getString(args, "binary"),
		Lines:   getInt(args, "lines", 200),
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
	return marshalJSON(payload)
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

	payload := map[string]any{
		"command": result.Command,
		"raw":     result.Raw,
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

	payload := map[string]any{
		"command": result.Command,
		"raw":     result.Raw,
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
	return marshalJSON(payload)
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

func marshalJSON(payload any) (interface{}, error) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("empty JSON response")
	}
	return string(data), nil
}
