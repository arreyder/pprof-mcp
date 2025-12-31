package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/conductorone/mcp-go-sdk/mcp/server"

	"github.com/arreyder/pprof-mcp/internal/datadog"
	"github.com/arreyder/pprof-mcp/internal/pprof"
	"github.com/arreyder/pprof-mcp/internal/services"
)

func main() {
	// Create tools provider with full schemas
	toolsProvider := NewSchemaToolsProvider()
	for _, def := range ToolSchemas() {
		toolsProvider.AddTool(def)
	}

	s := server.NewMCPServer("pprof-mcp",
		server.WithName("pprof MCP"),
		server.WithVersion("0.1.0"),
		server.WithInstructions("Profiling tools for Datadog profile download and deterministic pprof analysis."),
		server.WithToolsProvider(toolsProvider),
	)

	log.Println("Starting pprof MCP server over stdio")
	if err := s.ServeStdio(context.Background()); err != nil {
		log.Fatalf("Error serving MCP: %v", err)
		os.Exit(1)
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
