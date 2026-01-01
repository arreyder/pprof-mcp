package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/conductorone/mcp-go-sdk/mcp/schema"
	"github.com/conductorone/mcp-go-sdk/mcp/server"
)

// ToolDefinition combines a tool's schema with its handler.
type ToolDefinition struct {
	Schema  schema.Tool
	Handler server.ToolHandlerFunc
}

// SchemaToolsProvider implements ToolsProvider with full schema support.
type SchemaToolsProvider struct {
	tools map[string]ToolDefinition
	mu    sync.RWMutex
}

// NewSchemaToolsProvider creates a new provider with schema support.
func NewSchemaToolsProvider() *SchemaToolsProvider {
	return &SchemaToolsProvider{
		tools: make(map[string]ToolDefinition),
	}
}

// AddTool registers a tool with its full schema.
func (p *SchemaToolsProvider) AddTool(def ToolDefinition) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tools[def.Schema.Name] = def
}

// List returns all registered tools with their full schemas.
func (p *SchemaToolsProvider) List(ctx context.Context) ([]schema.Tool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tools := make([]schema.Tool, 0, len(p.tools))
	for _, def := range p.tools {
		tools = append(tools, def.Schema)
	}
	return tools, nil
}

// Call executes a registered tool.
func (p *SchemaToolsProvider) Call(ctx context.Context, name string, args map[string]any) (*schema.CallToolResult, error) {
	p.mu.RLock()
	def, exists := p.tools[name]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool %q not found", name)
	}

	result, err := def.Handler(ctx, args)
	if err != nil {
		return &schema.CallToolResult{
			IsError: true,
			Content: []schema.PromptContentUnion{
				schema.NewPromptContentFromText(fmt.Sprintf("Error: %v", err)),
			},
		}, nil
	}

	var content []schema.PromptContentUnion
	switch v := result.(type) {
	case string:
		content = []schema.PromptContentUnion{
			schema.NewPromptContentFromText(v),
		}
	case []schema.PromptContentUnion:
		content = v
	case schema.PromptContentUnion:
		content = []schema.PromptContentUnion{v}
	default:
		content = []schema.PromptContentUnion{
			schema.NewPromptContentFromText(fmt.Sprintf("%v", v)),
		}
	}

	return &schema.CallToolResult{
		Content: content,
	}, nil
}

// Helper to create JSON schema property
func prop(typ, desc string) json.RawMessage {
	p := map[string]string{"type": typ, "description": desc}
	data, _ := json.Marshal(p)
	return data
}

// Helper to create enum property
func enumProp(typ, desc string, values []string) json.RawMessage {
	p := map[string]any{"type": typ, "description": desc, "enum": values}
	data, _ := json.Marshal(p)
	return data
}

// Helper to create array property
func arrayProp(itemType, desc string) json.RawMessage {
	p := map[string]any{
		"type":        "array",
		"description": desc,
		"items":       map[string]string{"type": itemType},
	}
	data, _ := json.Marshal(p)
	return data
}

// ToolSchemas returns all tool definitions.
func ToolSchemas() []ToolDefinition {
	return []ToolDefinition{
		{
			Schema: schema.Tool{
				Name: "profiles.download_latest_bundle",
				Description: `Download profiling bundle from Datadog for a service.

**When to use**: Start here to get profiles for analysis. Downloads CPU, heap, mutex, and goroutine profiles.

**Workflow**:
1. Use datadog.profiles.list to see available profiles
2. Use datadog.profiles.pick to select a specific profile (by time, strategy, etc.)
3. Use this tool with the profile_id and event_id to download

**Returns**: Paths to downloaded .pprof files for use with other pprof.* tools.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"service":    prop("string", "The service name to download profiles for (required)"),
						"env":        prop("string", "The environment (e.g., prod, staging) (required)"),
						"out_dir":    prop("string", "Output directory for downloaded profiles (required)"),
						"hours":      prop("integer", "Number of hours to look back for profiles (default: 72)"),
						"dd_site":    prop("string", "Datadog site (e.g., datadoghq.com, datadoghq.eu)"),
						"profile_id": prop("string", "Specific profile ID to download (use with event_id)"),
						"event_id":   prop("string", "Specific event ID to download (required if profile_id is set)"),
					},
					Required: []string{"service", "env", "out_dir"},
				},
			},
			Handler: downloadTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.top",
				Description: `Show top functions by CPU/memory usage from a pprof profile.

**When to use**: First tool to run after downloading profiles. Identifies which functions consume the most resources.

**Key options**:
- cum=true: Sort by cumulative time (time spent in function + all functions it calls)
- cum=false (default): Sort by flat time (time spent only in the function itself)
- sample_index: Use 'alloc_space' for heap profiles, 'delay' for mutex/block profiles
- focus: Filter to functions matching regex (e.g., "mypackage")

**Returns**: Structured data with function names, flat/cumulative values, and percentages.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
						"nodecount":    prop("integer", "Maximum number of nodes to show (default: 10)"),
						"focus":        prop("string", "Regex to focus on specific functions"),
						"ignore":       prop("string", "Regex to ignore specific functions"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space, inuse_space)"),
					},
					Required: []string{"profile"},
				},
			},
			Handler: pprofTopTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.peek",
				Description: `Show callers and callees of functions matching a pattern.

**When to use**: After identifying a hot function with pprof.top, use this to understand:
- Who calls this function (callers)
- What functions it calls (callees)

**Example**: If pprof.top shows "json.Unmarshal" is hot, use peek to see which of YOUR functions call it.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile": prop("string", "Path to the pprof profile file (required)"),
						"binary":  prop("string", "Path to the binary for symbol resolution"),
						"regex":   prop("string", "Regex pattern to match function names (required)"),
					},
					Required: []string{"profile", "regex"},
				},
			},
			Handler: pprofPeekTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.list",
				Description: `Show annotated source code with line-level profiling data.

**When to use**: After identifying a hot function, use this to see exactly which LINES are expensive.

**Requirements**: Source code must be available. Use repo_root to specify where sources are located.

**Example output**: Shows each line with CPU time, helping pinpoint the exact bottleneck.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"function":     prop("string", "Function name or regex to list source for (required)"),
						"repo_root":    prop("string", "Repository root path for source file resolution"),
						"trim_path":    prop("string", "Path prefix to trim from source file paths (default: /xsrc)"),
						"source_paths": arrayProp("string", "Additional source paths for vendored or external dependencies"),
					},
					Required: []string{"profile", "function"},
				},
			},
			Handler: pprofListTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.traces_head",
				Description: `Show stack traces from a profile.

**When to use**: To see the actual call stacks that were sampled. Useful for understanding the full execution context.

**Note**: Output can be large; use 'lines' parameter to limit.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile": prop("string", "Path to the pprof profile file (required)"),
						"binary":  prop("string", "Path to the binary for symbol resolution"),
						"lines":   prop("integer", "Maximum number of lines to return (default: 200)"),
					},
					Required: []string{"profile"},
				},
			},
			Handler: pprofTracesTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.diff_top",
				Description: `Compare two profiles to identify performance changes.

**When to use**:
- Before/after optimization comparisons
- Identifying regressions between releases
- Comparing different time periods

**Workflow**:
1. Download baseline profile (e.g., before fix) with profiles.download_latest_bundle
2. Download comparison profile (e.g., after fix)
3. Use this tool with 'before' and 'after' paths

**Returns**: Delta showing which functions improved/regressed and by how much.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"before":       prop("string", "Path to the baseline pprof profile (required)"),
						"after":        prop("string", "Path to the comparison pprof profile (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
						"nodecount":    prop("integer", "Maximum number of nodes to show"),
						"focus":        prop("string", "Regex to focus on specific functions"),
						"ignore":       prop("string", "Regex to ignore specific functions"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space, inuse_space)"),
					},
					Required: []string{"before", "after"},
				},
			},
			Handler: pprofDiffTool,
		},
		{
			Schema: schema.Tool{
				Name:        "pprof.meta",
				Description: "Extract metadata from a pprof profile including sample types, duration, drop frames, and comments. Useful for understanding what data is available in a profile.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile": prop("string", "Path to the pprof profile file (required)"),
					},
					Required: []string{"profile"},
				},
			},
			Handler: pprofMetaTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.storylines",
				Description: `Find the top N hot code paths ("storylines") in your repository.

**When to use**: To get a high-level view of where time is spent in YOUR code (not library code).

**Key options**:
- repo_prefix: Identifies your code (e.g., "github.com/myorg/myrepo")
- n: Number of storylines to return (default: 4)

**Returns**: The most expensive execution paths with source-level detail, filtered to your repository code.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":     prop("string", "Path to the pprof profile file (required)"),
						"n":           prop("integer", "Number of storylines to return (default: 4)"),
						"focus":       prop("string", "Regex to focus on specific functions"),
						"ignore":      prop("string", "Regex to ignore specific functions"),
						"repo_prefix": arrayProp("string", "Repository path prefixes to identify your code (e.g., github.com/myorg/myrepo)"),
						"repo_root":   prop("string", "Local repository root path for source file resolution"),
						"trim_path":   prop("string", "Path prefix to trim from source file paths"),
					},
					Required: []string{"profile"},
				},
			},
			Handler: pprofStorylinesTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.memory_sanity",
				Description: `Analyze a heap profile for patterns that cause RSS growth beyond Go heap.

**When to use**: When container RSS is high but Go heap profile shows low memory usage.

**Detects**:
- SQLite temp_store=MEMORY patterns (RSS grows outside Go heap)
- High goroutine counts (stack memory not in heap)
- CGO allocations (memory outside Go control)
- Memory fragmentation patterns
- RSS/heap mismatch when container_rss_mb is provided

**Provides**:
- Suspicions with severity levels (low/medium/high)
- Actionable recommendations (GODEBUG settings, pragma changes)

**Example use case**: Container OOM but heap profile shows only 124MB. This tool identifies likely causes like temp_store=MEMORY.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"heap_profile":      prop("string", "Path to heap profile file (required)"),
						"goroutine_profile": prop("string", "Optional path to goroutine profile for stack analysis"),
						"binary":            prop("string", "Path to binary for symbol resolution"),
						"container_rss_mb":  prop("integer", "Container RSS in MB for mismatch detection"),
					},
					Required: []string{"heap_profile"},
				},
			},
			Handler: pprofMemorySanityTool,
		},
		{
			Schema: schema.Tool{
				Name: "datadog.profiles.list",
				Description: `List available profiles from Datadog for a service.

**When to use**: To see what profiles are available before downloading. Shows timestamps and IDs.

**Time parameters**:
- hours: Look back N hours from now (default: 72)
- from/to: Specific time range. Supports:
  - Relative: "-3h", "-24h", "-30m"
  - Absolute: "2025-01-15T10:00:00Z" (RFC3339)

**Returns**: List of profile candidates with timestamps, profile_id, and event_id.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"service": prop("string", "The service name to list profiles for (required)"),
						"env":     prop("string", "The environment (e.g., prod, staging) (required)"),
						"from":    prop("string", "Start time (RFC3339 or relative like '-1h', '-24h')"),
						"to":      prop("string", "End time (RFC3339 or relative)"),
						"hours":   prop("integer", "Number of hours to look back (default: 72, ignored if from/to set)"),
						"limit":   prop("integer", "Maximum number of profiles to return (default: 50)"),
						"site":    prop("string", "Datadog site (e.g., datadoghq.com)"),
					},
					Required: []string{"service", "env"},
				},
			},
			Handler: datadogProfilesListTool,
		},
		{
			Schema: schema.Tool{
				Name: "datadog.profiles.pick",
				Description: `Select a specific profile using a selection strategy.

**Strategies**:
- latest (default): Most recent profile
- oldest: Oldest profile in range (useful for before/after comparisons)
- closest: Profile closest to target_ts (requires target_ts parameter)
- index: Specific index from list (requires index parameter, 0-based)
- anomaly: Profile with highest statistical deviation (z-score > 2σ on CPU/memory/goroutine metrics)

**Workflow for before/after comparison**:
1. Pick oldest profile: strategy="oldest" for the baseline
2. Pick latest profile: strategy="latest" for current state
3. Download both with profiles.download_latest_bundle
4. Compare with pprof.diff_top

**Workflow for finding problematic profiles**:
1. Pick anomalous profile: strategy="anomaly" to find outliers
2. Download with profiles.download_latest_bundle using the profile_id
3. Analyze with pprof.top or pprof.storylines`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"service":   prop("string", "The service name (required)"),
						"env":       prop("string", "The environment (required)"),
						"from":      prop("string", "Start time (RFC3339 or relative like '-3h')"),
						"to":        prop("string", "End time (RFC3339 or relative)"),
						"hours":     prop("integer", "Number of hours to look back (default: 72)"),
						"limit":     prop("integer", "Maximum profiles to consider (default: 50)"),
						"site":      prop("string", "Datadog site"),
						"strategy":  enumProp("string", "Selection strategy: latest (default), oldest, closest (needs target_ts), index (needs index), anomaly (finds outliers)", []string{"latest", "oldest", "closest", "index", "anomaly"}),
						"target_ts": prop("string", "Target timestamp for 'closest' strategy (RFC3339)"),
						"index":     prop("integer", "Index for 'index' strategy (0-based from list results)"),
					},
					Required: []string{"service", "env"},
				},
			},
			Handler: datadogProfilesPickTool,
		},
		{
			Schema: schema.Tool{
				Name:        "repo.services.discover",
				Description: "Discover services in a repository by scanning for common patterns like Dockerfiles, go.mod, package.json, etc. Useful for finding service names to use with Datadog profiling.",
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"repo_root": prop("string", "Root directory of the repository to scan (default: current directory)"),
					},
					Required: []string{},
				},
			},
			Handler: repoServicesTool,
		},
		{
			Schema: schema.Tool{
				Name: "datadog.metrics.discover",
				Description: `Discover available Datadog metrics that match a service filter.

**Use cases**:
- Find Go runtime metrics (go.memstats, go.goroutines, go.gc) for correlation with profiles
- Find container/k8s metrics (container.memory, kubernetes.cpu) for RSS investigation
- Find service-specific metrics for application-level correlation

**Priority order**: Go runtime metrics are shown first, followed by container metrics, then others.

**Example workflow**:
1. Discover metrics for your service
2. Use metric names with Datadog dashboards/queries to correlate with profile timestamps`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"service": prop("string", "The service name to search for related metrics (required)"),
						"env":     prop("string", "The environment (optional, for context)"),
						"site":    prop("string", "Datadog site (default: from DD_SITE env or us3.datadoghq.com)"),
						"query":   prop("string", "Additional metric name pattern to search for"),
					},
					Required: []string{"service"},
				},
			},
			Handler: datadogMetricsDiscoverTool,
		},
		{
			Schema: schema.Tool{
				Name: "datadog.profiles.compare_range",
				Description: `Compare profiles from two time ranges to identify performance changes.

**When to use**:
- After a deployment to see what changed
- To compare "before incident" vs "during incident" profiles
- To identify performance regressions between releases

**How it works**:
1. Downloads oldest profile from "before" range (baseline)
2. Downloads latest profile from "after" range (current state)
3. Runs pprof diff to show what changed
4. Returns top function changes with increase/decrease indicators

**Example**: Compare profiles before and after a deploy:
- before_from="-48h", before_to="-24h" (yesterday's baseline)
- after_from="-4h", after_to="now" (recent profiles)`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"service":      prop("string", "The service name (required)"),
						"env":          prop("string", "The environment (required)"),
						"site":         prop("string", "Datadog site"),
						"before_from":  prop("string", "Start of 'before' range (RFC3339 or relative like '-48h') (required)"),
						"before_to":    prop("string", "End of 'before' range (RFC3339 or relative, default: before_from + 12h)"),
						"after_from":   prop("string", "Start of 'after' range (RFC3339 or relative like '-4h') (required)"),
						"after_to":     prop("string", "End of 'after' range (RFC3339 or relative, default: now)"),
						"out_dir":      prop("string", "Directory to store downloaded profiles (default: temp dir)"),
						"profile_type": prop("string", "Profile type to compare: cpu, heap, goroutines, mutex, block (default: cpu)"),
					},
					Required: []string{"service", "env", "before_from", "after_from"},
				},
			},
			Handler: datadogProfilesCompareRangeTool,
		},
		{
			Schema: schema.Tool{
				Name: "datadog.profiles.near_event",
				Description: `Find profiles around a specific event time (restart, OOM, incident, etc.).

**When to use**:
- Investigating an OOM kill - find the last profile before the kill
- Analyzing a restart - compare profiles before vs after
- Debugging an incident - find profiles at a specific timestamp

**Returns**:
- Profiles BEFORE the event (sorted by timestamp, most recent first)
- Profiles AFTER the event (sorted by timestamp, oldest first)
- The closest profile on each side
- Gap duration (helps identify if service was down)

**Example**: Find profiles around an OOM at 2025-01-15T10:30:00Z:
- event_time="2025-01-15T10:30:00Z"
- window="1h" (search 1 hour before and after)`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"service":    prop("string", "The service name (required)"),
						"env":        prop("string", "The environment (required)"),
						"site":       prop("string", "Datadog site"),
						"event_time": prop("string", "Timestamp of the event (RFC3339 format, required)"),
						"window":     prop("string", "Time window to search around event (e.g., '30m', '1h', '2h') (default: 1h)"),
						"limit":      prop("integer", "Max profiles to return per side (default: 10)"),
					},
					Required: []string{"service", "env", "event_time"},
				},
			},
			Handler: datadogProfilesNearEventTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.tags",
				Description: `Filter or group profile data by tags/labels.

**When to use**: Profiles often include labels like tenant_id, connector_id, etc. Use this to:
- See what tags are available (tag_show parameter)
- Filter to specific tag values (tag_focus/tag_ignore)

**Example**: Filter CPU profile to a specific tenant: tag_focus="tenant_id:abc123"`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"tag_focus":    prop("string", "Regex to focus on samples with matching tag values (e.g., 'tenant_id:abc')"),
						"tag_ignore":   prop("string", "Regex to ignore samples with matching tag values"),
						"tag_show":     prop("string", "Show values for a specific tag key (e.g., 'tenant_id' to list all tenants)"),
						"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
						"nodecount":    prop("integer", "Maximum number of nodes to show"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
					},
					Required: []string{"profile"},
				},
			},
			Handler: pprofTagsTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.flamegraph",
				Description: `Generate a flamegraph SVG visualization from a profile.

**When to use**: For visual exploration of where time is spent. Flamegraphs show the full call stack with width proportional to time spent.

**Output**: SVG file that can be opened in a browser for interactive exploration.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"output_path":  prop("string", "Path to write the SVG file (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"focus":        prop("string", "Regex to focus on specific functions"),
						"ignore":       prop("string", "Regex to ignore specific functions"),
						"tag_focus":    prop("string", "Regex to focus on samples with matching tag values"),
						"tag_ignore":   prop("string", "Regex to ignore samples with matching tag values"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
					},
					Required: []string{"profile", "output_path"},
				},
			},
			Handler: pprofFlamegraphTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.callgraph",
				Description: `Generate a call graph visualization showing function relationships.

**When to use**: To visualize how functions call each other and where time flows.

**Formats**:
- dot: GraphViz DOT format (can be rendered with graphviz)
- svg: Direct SVG visualization
- png: PNG image`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"output_path":  prop("string", "Path to write the output file (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"format":       enumProp("string", "Output format: dot, svg, or png (default: dot)", []string{"dot", "svg", "png"}),
						"focus":        prop("string", "Regex to focus on specific functions"),
						"ignore":       prop("string", "Regex to ignore specific functions"),
						"nodecount":    prop("integer", "Maximum number of nodes to show"),
						"edge_frac":    prop("number", "Hide edges below this fraction (0.0-1.0)"),
						"node_frac":    prop("number", "Hide nodes below this fraction (0.0-1.0)"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
					},
					Required: []string{"profile", "output_path"},
				},
			},
			Handler: pprofCallgraphTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.focus_paths",
				Description: `Show all call paths that lead to a specific function.

**When to use**: When you know a function is hot (from pprof.top) and want to understand ALL the different ways it gets called.

**Difference from peek**: peek shows immediate callers/callees; focus_paths shows complete call stacks.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profile":      prop("string", "Path to the pprof profile file (required)"),
						"function":     prop("string", "Target function name or regex to find paths to (required)"),
						"binary":       prop("string", "Path to the binary for symbol resolution"),
						"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
						"nodecount":    prop("integer", "Maximum number of paths to show"),
						"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
					},
					Required: []string{"profile", "function"},
				},
			},
			Handler: pprofFocusPathsTool,
		},
		{
			Schema: schema.Tool{
				Name: "pprof.merge",
				Description: `Merge multiple profiles into a single aggregated profile.

**When to use**:
- Combine profiles from different instances/pods
- Aggregate profiles over a longer time period
- Create a representative profile from multiple samples

**Output**: A new .pprof file that can be analyzed with other pprof.* tools.`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"profiles":    arrayProp("string", "List of profile paths to merge (required, minimum 2)"),
						"output_path": prop("string", "Path to write the merged profile (required)"),
						"binary":      prop("string", "Path to the binary for symbol resolution"),
					},
					Required: []string{"profiles", "output_path"},
				},
			},
			Handler: pprofMergeTool,
		},
		{
			Schema: schema.Tool{
				Name: "datadog.function_history",
				Description: `Search for a function across multiple profiles over time.

**When to use**: To track how a function's CPU usage changes over time. Useful for:
- Verifying a performance fix reduced CPU usage
- Finding when a regression was introduced
- Monitoring function performance over deployments

**Workflow**:
1. Specify the function pattern (e.g., "getFinishedSync")
2. Set time range with from/to or hours
3. Tool downloads profiles and searches each one
4. Returns a table showing function's CPU% at each timestamp

**Example**: Track "myFunction" over the last 24 hours:
  function="myFunction", hours=24, limit=10`,
				InputSchema: schema.ToolInputSchema{
					Type: "object",
					Properties: map[string]json.RawMessage{
						"service":  prop("string", "The service name (required)"),
						"env":      prop("string", "The environment (required)"),
						"function": prop("string", "Function name or pattern to search for (required)"),
						"from":     prop("string", "Start time (RFC3339 or relative like '-24h')"),
						"to":       prop("string", "End time (RFC3339 or relative)"),
						"hours":    prop("integer", "Number of hours to look back (default: 72)"),
						"limit":    prop("integer", "Maximum number of profiles to check (default: 10)"),
						"site":     prop("string", "Datadog site"),
					},
					Required: []string{"service", "env", "function"},
				},
			},
			Handler: functionHistoryTool,
		},
	}
}
