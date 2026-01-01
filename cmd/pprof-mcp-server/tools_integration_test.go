//go:build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestIntegrationAllTools(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 to run")
	}
	if os.Getenv("DD_API_KEY") == "" || os.Getenv("DD_APP_KEY") == "" {
		t.Skip("missing Datadog API keys")
	}

	ctx := context.Background()
	service := envOrDefault("PPROF_MCP_TEST_SERVICE", "temporal_sync")
	env := envOrDefault("PPROF_MCP_TEST_ENV", "sandbox-usw2")

	_ = runTool(t, ctx, "datadog.profiles.list", map[string]any{
		"service": service,
		"env":     env,
		"hours":   24,
		"limit":   10,
	})

	latestPick := runTool(t, ctx, "datadog.profiles.pick", map[string]any{
		"service":  service,
		"env":      env,
		"hours":    24,
		"limit":    10,
		"strategy": "latest",
	})
	oldestPick := runTool(t, ctx, "datadog.profiles.pick", map[string]any{
		"service":  service,
		"env":      env,
		"hours":    24,
		"limit":    10,
		"strategy": "oldest",
	})

	latestProfileID, latestEventID := extractPickIDs(t, latestPick)
	oldestProfileID, oldestEventID := extractPickIDs(t, oldestPick)

	latestDir := filepath.Join(t.TempDir(), "latest")
	oldestDir := filepath.Join(t.TempDir(), "oldest")

	latestDownload := runTool(t, ctx, "profiles.download_latest_bundle", map[string]any{
		"service":    service,
		"env":        env,
		"out_dir":    latestDir,
		"profile_id": latestProfileID,
		"event_id":   latestEventID,
	})
	oldestDownload := runTool(t, ctx, "profiles.download_latest_bundle", map[string]any{
		"service":    service,
		"env":        env,
		"out_dir":    oldestDir,
		"profile_id": oldestProfileID,
		"event_id":   oldestEventID,
	})

	latestHandles := extractHandles(t, latestDownload)
	oldestHandles := extractHandles(t, oldestDownload)

	cpuHandle := requireHandle(t, latestHandles, "cpu")
	heapHandle := requireHandle(t, latestHandles, "heap")
	mutexHandle := requireHandle(t, latestHandles, "mutex")
	goroutineHandle := requireHandle(t, latestHandles, "goroutines")
	blockHandle := latestHandles["block"]

	topPayload := runTool(t, ctx, "pprof.top", map[string]any{
		"profile":   cpuHandle,
		"nodecount": 5,
	})
	topFunction := extractTopFunction(t, topPayload)
	functionRegex := regexp.QuoteMeta(topFunction)

	_ = runTool(t, ctx, "pprof.meta", map[string]any{
		"profile": cpuHandle,
	})
	_ = runTool(t, ctx, "pprof.peek", map[string]any{
		"profile":   cpuHandle,
		"regex":     functionRegex,
		"max_lines": 120,
	})
	_ = runTool(t, ctx, "pprof.list", map[string]any{
		"profile":   cpuHandle,
		"function":  topFunction,
		"max_lines": 120,
	})
	_ = runTool(t, ctx, "pprof.traces_head", map[string]any{
		"profile":   goroutineHandle,
		"max_lines": 120,
	})
	_ = runTool(t, ctx, "pprof.diff_top", map[string]any{
		"before":    requireHandle(t, oldestHandles, "cpu"),
		"after":     cpuHandle,
		"nodecount": 5,
	})
	_ = runTool(t, ctx, "pprof.regression_check", map[string]any{
		"profile":      cpuHandle,
		"sample_index": "cpu",
		"checks": []any{
			map[string]any{
				"function": functionRegex,
				"metric":   "flat_pct",
				"max":      100.0,
			},
		},
	})
	_ = runTool(t, ctx, "pprof.storylines", map[string]any{
		"profile": cpuHandle,
		"n":       2,
	})
	_ = runTool(t, ctx, "pprof.memory_sanity", map[string]any{
		"heap_profile":      heapHandle,
		"goroutine_profile": goroutineHandle,
	})
	_ = runTool(t, ctx, "pprof.goroutine_analysis", map[string]any{
		"profile": goroutineHandle,
	})
	_ = runTool(t, ctx, "pprof.contention_analysis", map[string]any{
		"profile": mutexHandle,
	})
	_ = runTool(t, ctx, "pprof.discover", map[string]any{
		"service": service,
		"env":     env,
		"hours":   24,
	})
	_ = runTool(t, ctx, "pprof.cross_correlate", map[string]any{
		"bundle":    cpuHandle,
		"nodecount": 5,
	})
	_ = runTool(t, ctx, "pprof.hotspot_summary", map[string]any{
		"bundle":    cpuHandle,
		"nodecount": 3,
	})
	_ = runTool(t, ctx, "datadog.profiles.aggregate", map[string]any{
		"service":      service,
		"env":          env,
		"window":       "1h",
		"limit":        2,
		"profile_type": "cpu",
		"out_dir":      filepath.Join(t.TempDir(), "aggregate"),
	})
	_ = runTool(t, ctx, "repo.services.discover", map[string]any{
		"repo_root": repoRoot(t),
	})
	_ = runTool(t, ctx, "datadog.metrics.discover", map[string]any{
		"service": service,
		"env":     env,
	})
	_ = runTool(t, ctx, "datadog.profiles.compare_range", map[string]any{
		"service":     service,
		"env":         env,
		"before_from": "-48h",
		"before_to":   "-24h",
		"after_from":  "-4h",
		"after_to":    "now",
	})
	_ = runTool(t, ctx, "datadog.profiles.near_event", map[string]any{
		"service":    service,
		"env":        env,
		"event_time": time.Now().Add(-6 * time.Hour).Format(time.RFC3339),
		"window":     "1h",
		"limit":      5,
	})
	_ = runTool(t, ctx, "pprof.tags", map[string]any{
		"profile":   cpuHandle,
		"max_lines": 100,
	})

	flamegraphPath := filepath.Join(t.TempDir(), "flamegraph.svg")
	_ = runTool(t, ctx, "pprof.flamegraph", map[string]any{
		"profile":     cpuHandle,
		"output_path": flamegraphPath,
	})
	assertFileExists(t, flamegraphPath)

	callgraphPath := filepath.Join(t.TempDir(), "callgraph.dot")
	_ = runTool(t, ctx, "pprof.callgraph", map[string]any{
		"profile":     cpuHandle,
		"output_path": callgraphPath,
		"format":      "dot",
	})
	assertFileExists(t, callgraphPath)

	_ = runTool(t, ctx, "pprof.focus_paths", map[string]any{
		"profile":   cpuHandle,
		"function":  topFunction,
		"max_lines": 120,
	})

	mergePath := filepath.Join(t.TempDir(), "merged_cpu.pprof")
	_ = runTool(t, ctx, "pprof.merge", map[string]any{
		"profiles":    []string{requireHandle(t, oldestHandles, "cpu"), cpuHandle},
		"output_path": mergePath,
	})
	assertFileExists(t, mergePath)

	_ = runTool(t, ctx, "datadog.function_history", map[string]any{
		"service":  service,
		"env":      env,
		"function": topFunction,
		"hours":    6,
		"limit":    3,
	})
	_ = runTool(t, ctx, "pprof.alloc_paths", map[string]any{
		"profile":     heapHandle,
		"min_percent": 1.0,
		"max_paths":   5,
	})
	overheadPayload := runTool(t, ctx, "pprof.overhead_report", map[string]any{
		"profile": cpuHandle,
	})
	_ = runTool(t, ctx, "pprof.generate_report", map[string]any{
		"inputs": []any{
			map[string]any{
				"kind": "overhead_report",
				"data": extractResultObject(t, overheadPayload),
			},
		},
	})
	_ = runTool(t, ctx, "pprof.detect_repo", map[string]any{
		"profile": cpuHandle,
	})

	if blockHandle != "" {
		_ = runTool(t, ctx, "pprof.contention_analysis", map[string]any{
			"profile": blockHandle,
		})
	}
}

func runTool(t *testing.T, ctx context.Context, name string, args map[string]any) map[string]any {
	t.Helper()
	def := findTool(t, name)
	res, structured, err := invokeTool(ctx, def.Tool, def.Tool.Name, def.Handler, args)
	if err != nil {
		t.Fatalf("tool %s failed: %v", name, err)
	}
	if res != nil && res.IsError {
		raw, _ := json.Marshal(res.Content)
		t.Fatalf("tool %s returned error: %s", name, string(raw))
	}
	if structured == nil {
		raw, _ := json.Marshal(res.Content)
		t.Fatalf("tool %s missing structured payload: %s", name, string(raw))
	}
	payload, ok := structured.(map[string]any)
	if !ok {
		t.Fatalf("tool %s structured payload not object", name)
	}
	return payload
}

func extractPickIDs(t *testing.T, payload map[string]any) (string, string) {
	t.Helper()
	result := extractResultObject(t, payload)
	candidate, ok := result["candidate"].(map[string]any)
	if !ok {
		t.Fatalf("pick result missing candidate")
	}
	profileID, _ := candidate["profile_id"].(string)
	eventID, _ := candidate["event_id"].(string)
	if profileID == "" || eventID == "" {
		t.Fatalf("pick result missing profile_id or event_id")
	}
	return profileID, eventID
}

func extractHandles(t *testing.T, payload map[string]any) map[string]string {
	t.Helper()
	result := extractResultObject(t, payload)
	rawFiles, ok := result["files"]
	if !ok {
		t.Fatalf("download result missing files")
	}
	var files []any
	switch v := rawFiles.(type) {
	case []any:
		files = v
	case []ProfileHandle:
		files = make([]any, 0, len(v))
		for _, item := range v {
			files = append(files, map[string]any{
				"type":   item.Type,
				"handle": item.Handle,
				"bytes":  item.Bytes,
			})
		}
	default:
		data, err := json.Marshal(v)
		if err == nil {
			_ = json.Unmarshal(data, &files)
		}
	}
	if len(files) == 0 {
		t.Fatalf("download result missing files")
	}
	handles := map[string]string{}
	for _, item := range files {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := obj["type"].(string)
		handle, _ := obj["handle"].(string)
		if kind != "" && handle != "" {
			handles[kind] = handle
		}
	}
	return handles
}

func extractTopFunction(t *testing.T, payload map[string]any) string {
	t.Helper()
	rowsAny := payload["rows"]
	var rows []map[string]any
	switch v := rowsAny.(type) {
	case []map[string]any:
		rows = v
	case []any:
		for _, item := range v {
			if row, ok := item.(map[string]any); ok {
				rows = append(rows, row)
			}
		}
	default:
		data, err := json.Marshal(v)
		if err == nil {
			_ = json.Unmarshal(data, &rows)
		}
	}
	if len(rows) == 0 {
		t.Fatalf("top result missing rows")
	}
	name, _ := rows[0]["name"].(string)
	if name == "" {
		t.Fatalf("top row missing name")
	}
	return name
}

func extractResultObject(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	rawResult, ok := payload["result"]
	if ok {
		if resultMap, ok := rawResult.(map[string]any); ok {
			return resultMap
		}
		data, err := json.Marshal(rawResult)
		if err == nil {
			var resultMap map[string]any
			if json.Unmarshal(data, &resultMap) == nil {
				return resultMap
			}
		}
	}
	raw, _ := json.Marshal(payload)
	t.Fatalf("payload missing result object: %s", string(raw))
	return nil
}

func requireHandle(t *testing.T, handles map[string]string, key string) string {
	t.Helper()
	handle := handles[key]
	if handle == "" {
		t.Fatalf("missing %s handle", key)
	}
	return handle
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected file %s to be non-empty", path)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	return root
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value != "" {
		return value
	}
	return fallback
}
