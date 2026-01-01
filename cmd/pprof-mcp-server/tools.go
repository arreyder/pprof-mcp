package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolHandler runs a tool with JSON-like arguments.
type ToolHandler func(context.Context, map[string]any) (any, error)

// ToolDefinition combines a tool's schema with its handler.
type ToolDefinition struct {
	Tool    *mcp.Tool
	Handler ToolHandler
}

// ToolSchemas returns all tool definitions.
func ToolSchemas() []ToolDefinition {
	tools := []ToolDefinition{
		{
			Tool: &mcp.Tool{
				Name: "profiles.download_latest_bundle",
				Description: `Download profiling bundle from Datadog for a service.

**When to use**: Start here to get profiles for analysis. Downloads CPU, heap, mutex, and goroutine profiles.

**Workflow**:
1. Use datadog.profiles.list to see available profiles
2. Use datadog.profiles.pick to select a specific profile (by time, strategy, etc.)
3. Use this tool with the profile_id and event_id to download

**Returns**: Paths to downloaded .pprof files for use with other pprof.* tools.`,
				InputSchema: NewObjectSchema(map[string]any{
					"service":    prop("string", "The service name to download profiles for (required)"),
					"env":        prop("string", "The environment (e.g., prod, staging) (required)"),
					"out_dir":    prop("string", "Output directory for downloaded profiles (required)"),
					"hours":      integerProp("Number of hours to look back for profiles (default: 72)", intPtr(0), nil),
					"dd_site":    prop("string", "Datadog site (e.g., datadoghq.com, datadoghq.eu) (alias: site)"),
					"site":       prop("string", "Datadog site (preferred; alias: dd_site)"),
					"profile_id": prop("string", "Specific profile ID to download (use with event_id)"),
					"event_id":   prop("string", "Specific event ID to download (required if profile_id is set)"),
				}, "service", "env", "out_dir"),
				OutputSchema: downloadLatestBundleOutputSchema(),
			},
			Handler: downloadTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.top",
				Description: `Show top functions by CPU/memory usage from a pprof profile.

**When to use**: First tool to run after downloading profiles. Identifies which functions consume the most resources.

**Key options**:
- cum=true: Sort by cumulative time (time spent in function + all functions it calls)
- cum=false (default): Sort by flat time (time spent only in the function itself)
- sample_index: Use 'alloc_space' for heap profiles, 'delay' for mutex/block profiles
- focus: Filter to functions matching regex (e.g., "mypackage")

**Returns**: Structured data with function names, flat/cumulative values, and percentages.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":      ProfilePath(),
					"binary":       BinaryPathOptional(),
					"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
					"nodecount":    integerProp("Maximum number of nodes to show (default: 10)", intPtr(0), nil),
					"focus":        prop("string", "Regex to focus on specific functions"),
					"ignore":       prop("string", "Regex to ignore specific functions"),
					"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space, inuse_space)"),
				}, "profile"),
				OutputSchema: pprofTopOutputSchema(),
			},
			Handler: pprofTopTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.peek",
				Description: `Show callers and callees of functions matching a pattern.

**When to use**: After identifying a hot function with pprof.top, use this to understand:
- Who calls this function (callers)
- What functions it calls (callees)

**Example**: If pprof.top shows "json.Unmarshal" is hot, use peek to see which of YOUR functions call it.

**Important for heap profiles**: Use sample_index="alloc_space" for allocation analysis, otherwise peek defaults to inuse_space which may not show all functions.

**Optional**: Use max_lines to cap the output size.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":      ProfilePath(),
					"binary":       BinaryPathOptional(),
					"regex":        prop("string", "Regex pattern to match function names (required)"),
					"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space, inuse_space). Required for heap profiles to find allocation hot spots."),
					"max_lines":    integerProp("Maximum number of output lines to return", intPtr(0), nil),
				}, "profile", "regex"),
			},
			Handler: pprofPeekTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.list",
				Description: `Show annotated source code with line-level profiling data.

**When to use**: After identifying a hot function, use this to see exactly which LINES are expensive.

**Requirements**: Source code must be available. Use repo_root to specify where sources are located.

**Example output**: Shows each line with CPU time, helping pinpoint the exact bottleneck.

**Optional**: Use max_lines to cap the output size.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":      ProfilePath(),
					"binary":       BinaryPathOptional(),
					"function":     prop("string", "Function name or regex to list source for (required)"),
					"repo_root":    prop("string", "Repository root path for source file resolution"),
					"trim_path":    prop("string", "Path prefix to trim from source file paths (default: /xsrc)"),
					"source_paths": arrayOrStringPropSchema(prop("string", "Source path"), "Additional source paths for vendored or external dependencies (string or list)"),
					"max_lines":    integerProp("Maximum number of output lines to return", intPtr(0), nil),
				}, "profile", "function"),
			},
			Handler: pprofListTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.traces_head",
				Description: `Show stack traces from a profile.

**When to use**: To see the actual call stacks that were sampled. Useful for understanding the full execution context.

**Note**: Output can be large; use 'lines' (or alias 'max_lines') to limit.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":   ProfilePath(),
					"binary":    BinaryPathOptional(),
					"lines":     integerProp("Maximum number of lines to return (default: 200)", intPtr(0), intPtr(maxTracesLines)),
					"max_lines": integerProp("Alias for lines", intPtr(0), intPtr(maxTracesLines)),
				}, "profile"),
			},
			Handler: pprofTracesTool,
		},
		{
			Tool: &mcp.Tool{
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
				InputSchema: NewObjectSchema(map[string]any{
					"before":       prop("string", "Path to the baseline pprof profile (required)"),
					"after":        prop("string", "Path to the comparison pprof profile (required)"),
					"binary":       BinaryPathOptional(),
					"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
					"nodecount":    integerProp("Maximum number of nodes to show", intPtr(0), nil),
					"focus":        prop("string", "Regex to focus on specific functions"),
					"ignore":       prop("string", "Regex to ignore specific functions"),
					"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space, inuse_space)"),
				}, "before", "after"),
			},
			Handler: pprofDiffTool,
		},
		{
			Tool: &mcp.Tool{
				Name:        "pprof.meta",
				Description: "Extract metadata from a pprof profile including sample types, duration, drop frames, and comments. Useful for understanding what data is available in a profile.",
				InputSchema: NewObjectSchema(map[string]any{
					"profile": ProfilePath(),
				}, "profile"),
			},
			Handler: pprofMetaTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.storylines",
				Description: `Find the top N hot code paths ("storylines") in your repository.

**When to use**: To get a high-level view of where time is spent in YOUR code (not library code).

**Key options**:
- repo_prefix: Identifies your code (e.g., "github.com/myorg/myrepo")
- n: Number of storylines to return (default: 4)

**Auto-detection**: For heap profiles, automatically uses alloc_space to show allocation hot spots instead of just in-use memory.

**Returns**: The most expensive execution paths with source-level detail, filtered to your repository code.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":      ProfilePath(),
					"n":            integerProp("Number of storylines to return (default: 4)", intPtr(0), nil),
					"focus":        prop("string", "Regex to focus on specific functions"),
					"ignore":       prop("string", "Regex to ignore specific functions"),
					"repo_prefix":  arrayOrStringPropSchema(prop("string", "Repository prefix"), "Repository path prefixes to identify your code (e.g., github.com/myorg/myrepo) (string or list)"),
					"repo_root":    prop("string", "Local repository root path for source file resolution"),
					"trim_path":    prop("string", "Path prefix to trim from source file paths"),
					"sample_index": prop("string", "Sample index to use (auto-detected for heap profiles: uses alloc_space)"),
				}, "profile"),
			},
			Handler: pprofStorylinesTool,
		},
		{
			Tool: &mcp.Tool{
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
				InputSchema: NewObjectSchema(map[string]any{
					"heap_profile":      prop("string", "Path to heap profile file (required)"),
					"goroutine_profile": prop("string", "Optional path to goroutine profile for stack analysis"),
					"binary":            BinaryPathOptional(),
					"container_rss_mb":  integerProp("Container RSS in MB for mismatch detection", intPtr(0), nil),
				}, "heap_profile"),
			},
			Handler: pprofMemorySanityTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "datadog.profiles.list",
				Description: `List available profiles from Datadog for a service.

**When to use**: To see what profiles are available before downloading. Shows timestamps and IDs.

**Time parameters**:
- hours: Look back N hours from now (default: 72)
- from/to: Specific time range. Supports:
  - Relative: "-3h", "-24h", "-30m"
  - Absolute: "2025-01-15T10:00:00Z" (RFC3339)

**Returns**: List of profile candidates with timestamps, profile_id, and event_id.`,
				InputSchema: NewObjectSchema(map[string]any{
					"service": prop("string", "The service name to list profiles for (required)"),
					"env":     prop("string", "The environment (e.g., prod, staging) (required)"),
					"from":    prop("string", "Start time (RFC3339 or relative like '-1h', '-24h')"),
					"to":      prop("string", "End time (RFC3339 or relative)"),
					"hours":   integerProp("Number of hours to look back (default: 72, ignored if from/to set)", intPtr(0), nil),
					"limit":   integerProp("Maximum number of profiles to return (default: 50)", intPtr(0), nil),
					"site":    prop("string", "Datadog site (e.g., datadoghq.com)"),
				}, "service", "env"),
				OutputSchema: datadogProfilesListOutputSchema(),
			},
			Handler: datadogProfilesListTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "datadog.profiles.pick",
				Description: `Select a specific profile using a selection strategy.

**Strategies**:
- latest (default): Most recent profile
- oldest: Oldest profile in range (useful for before/after comparisons)
- closest_to_ts: Profile closest to target_ts (requires target_ts parameter)
- manual_index: Specific index from list (requires index parameter, 0-based)
- most_samples: Profile with the highest sample count (falls back to latest if unavailable)
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
				InputSchema: NewObjectSchema(map[string]any{
					"service":   prop("string", "The service name (required)"),
					"env":       prop("string", "The environment (required)"),
					"from":      prop("string", "Start time (RFC3339 or relative like '-3h')"),
					"to":        prop("string", "End time (RFC3339 or relative)"),
					"hours":     integerProp("Number of hours to look back (default: 72)", intPtr(0), nil),
					"limit":     integerProp("Maximum profiles to consider (default: 50)", intPtr(0), nil),
					"site":      prop("string", "Datadog site"),
					"strategy":  enumProp("string", "Selection strategy: latest (default), oldest, closest_to_ts (needs target_ts), manual_index (needs index), most_samples, anomaly (finds outliers)", []string{"latest", "oldest", "closest_to_ts", "manual_index", "most_samples", "anomaly"}),
					"target_ts": prop("string", "Target timestamp for 'closest_to_ts' strategy (RFC3339)"),
					"index":     integerProp("Index for 'manual_index' strategy (0-based from list results)", intPtr(0), nil),
				}, "service", "env"),
				OutputSchema: datadogProfilesPickOutputSchema(),
			},
			Handler: datadogProfilesPickTool,
		},
		{
			Tool: &mcp.Tool{
				Name:        "repo.services.discover",
				Description: "Discover services in a repository by scanning for common patterns like Dockerfiles, go.mod, package.json, etc. Useful for finding service names to use with Datadog profiling.",
				InputSchema: NewObjectSchema(map[string]any{
					"repo_root": prop("string", "Root directory of the repository to scan (default: current directory)"),
				}),
			},
			Handler: repoServicesTool,
		},
		{
			Tool: &mcp.Tool{
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
				InputSchema: NewObjectSchema(map[string]any{
					"service": prop("string", "The service name to search for related metrics (required)"),
					"env":     prop("string", "The environment (optional, for context)"),
					"site":    prop("string", "Datadog site (default: from DD_SITE env or us3.datadoghq.com)"),
					"query":   prop("string", "Additional metric name pattern to search for"),
				}, "service"),
			},
			Handler: datadogMetricsDiscoverTool,
		},
		{
			Tool: &mcp.Tool{
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
				InputSchema: NewObjectSchema(map[string]any{
					"service":      prop("string", "The service name (required)"),
					"env":          prop("string", "The environment (required)"),
					"site":         prop("string", "Datadog site"),
					"before_from":  prop("string", "Start of 'before' range (RFC3339 or relative like '-48h') (required)"),
					"before_to":    prop("string", "End of 'before' range (RFC3339 or relative, default: now)"),
					"after_from":   prop("string", "Start of 'after' range (RFC3339 or relative like '-4h') (required)"),
					"after_to":     prop("string", "End of 'after' range (RFC3339 or relative, default: now)"),
					"out_dir":      prop("string", "Directory to store downloaded profiles (default: temp dir)"),
					"profile_type": enumProp("string", "Profile type to compare: cpu, heap, goroutines, mutex, block (default: cpu)", []string{"cpu", "heap", "goroutines", "mutex", "block"}),
				}, "service", "env", "before_from", "after_from"),
				OutputSchema: compareRangeOutputSchema(),
			},
			Handler: datadogProfilesCompareRangeTool,
		},
		{
			Tool: &mcp.Tool{
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
				InputSchema: NewObjectSchema(map[string]any{
					"service":    prop("string", "The service name (required)"),
					"env":        prop("string", "The environment (required)"),
					"site":       prop("string", "Datadog site"),
					"event_time": prop("string", "Timestamp of the event (RFC3339 format, required)"),
					"window":     prop("string", "Time window to search around event (e.g., '30m', '1h', '2h') (default: 1h)"),
					"limit":      integerProp("Max profiles to return per side (default: 10)", intPtr(0), nil),
				}, "service", "env", "event_time"),
			},
			Handler: datadogProfilesNearEventTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.tags",
				Description: `Filter or group profile data by tags/labels.

**When to use**: Profiles often include labels like tenant_id, connector_id, etc. Use this to:
- See what tags are available (tag_show parameter)
- Filter to specific tag values (tag_focus/tag_ignore)

**Example**: Filter CPU profile to a specific tenant: tag_focus="tenant_id:abc123"

**Optional**: Use max_lines to cap the output size.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":      ProfilePath(),
					"binary":       BinaryPathOptional(),
					"tag_focus":    prop("string", "Regex to focus on samples with matching tag values (e.g., 'tenant_id:abc')"),
					"tag_ignore":   prop("string", "Regex to ignore samples with matching tag values"),
					"tag_show":     prop("string", "Show values for a specific tag key (e.g., 'tenant_id' to list all tenants)"),
					"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
					"nodecount":    integerProp("Maximum number of nodes to show", intPtr(0), nil),
					"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
					"max_lines":    integerProp("Maximum number of output lines to return", intPtr(0), nil),
				}, "profile"),
			},
			Handler: pprofTagsTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.flamegraph",
				Description: `Generate a flamegraph SVG visualization from a profile.

**When to use**: For visual exploration of where time is spent. Flamegraphs show the full call stack with width proportional to time spent.

**Output**: SVG file that can be opened in a browser for interactive exploration.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":      ProfilePath(),
					"output_path":  prop("string", "Path to write the SVG file (required)"),
					"binary":       BinaryPathOptional(),
					"focus":        prop("string", "Regex to focus on specific functions"),
					"ignore":       prop("string", "Regex to ignore specific functions"),
					"tag_focus":    prop("string", "Regex to focus on samples with matching tag values"),
					"tag_ignore":   prop("string", "Regex to ignore samples with matching tag values"),
					"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
				}, "profile", "output_path"),
			},
			Handler: pprofFlamegraphTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.callgraph",
				Description: `Generate a call graph visualization showing function relationships.

**When to use**: To visualize how functions call each other and where time flows.

**Formats**:
- dot: GraphViz DOT format (can be rendered with graphviz)
- svg: Direct SVG visualization
- png: PNG image`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":      ProfilePath(),
					"output_path":  prop("string", "Path to write the output file (required)"),
					"binary":       BinaryPathOptional(),
					"format":       enumProp("string", "Output format: dot, svg, or png (default: dot)", []string{"dot", "svg", "png"}),
					"focus":        prop("string", "Regex to focus on specific functions"),
					"ignore":       prop("string", "Regex to ignore specific functions"),
					"nodecount":    integerProp("Maximum number of nodes to show", intPtr(0), nil),
					"edge_frac":    numberProp("Hide edges below this fraction (0.0-1.0)", floatPtr(0), floatPtr(1)),
					"node_frac":    numberProp("Hide nodes below this fraction (0.0-1.0)", floatPtr(0), floatPtr(1)),
					"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
				}, "profile", "output_path"),
			},
			Handler: pprofCallgraphTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.focus_paths",
				Description: `Show all call paths that lead to a specific function.

**When to use**: When you know a function is hot (from pprof.top) and want to understand ALL the different ways it gets called.

**Difference from peek**: peek shows immediate callers/callees; focus_paths shows complete call stacks.

**Optional**: Use max_lines to cap the output size.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":      ProfilePath(),
					"function":     prop("string", "Target function name or regex to find paths to (required)"),
					"binary":       BinaryPathOptional(),
					"cum":          prop("boolean", "Sort by cumulative value instead of flat (default: false)"),
					"nodecount":    integerProp("Maximum number of paths to show", intPtr(0), nil),
					"sample_index": prop("string", "Sample index to use (e.g., cpu, alloc_space)"),
					"max_lines":    integerProp("Maximum number of output lines to return", intPtr(0), nil),
				}, "profile", "function"),
			},
			Handler: pprofFocusPathsTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.merge",
				Description: `Merge multiple profiles into a single aggregated profile.

**When to use**:
- Combine profiles from different instances/pods
- Aggregate profiles over a longer time period
- Create a representative profile from multiple samples

**Output**: A new .pprof file that can be analyzed with other pprof.* tools.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profiles":    arrayOrStringPropMin(prop("string", "Profile path"), "List of profile paths to merge (required, minimum 2)", 2),
					"output_path": prop("string", "Path to write the merged profile (required)"),
					"binary":      BinaryPathOptional(),
				}, "profiles", "output_path"),
			},
			Handler: pprofMergeTool,
		},
		{
			Tool: &mcp.Tool{
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
				InputSchema: NewObjectSchema(map[string]any{
					"service":  prop("string", "The service name (required)"),
					"env":      prop("string", "The environment (required)"),
					"function": prop("string", "Function name or pattern to search for (required)"),
					"from":     prop("string", "Start time (RFC3339 or relative like '-24h')"),
					"to":       prop("string", "End time (RFC3339 or relative)"),
					"hours":    integerProp("Number of hours to look back (default: 72)", intPtr(0), nil),
					"limit":    integerProp("Maximum number of profiles to check (default: 10)", intPtr(0), nil),
					"site":     prop("string", "Datadog site"),
				}, "service", "env", "function"),
				OutputSchema: functionHistoryOutputSchema(),
			},
			Handler: functionHistoryTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.alloc_paths",
				Description: `Analyze allocation paths in a heap profile with intelligent filtering.

**When to use**: When pprof.top shows high allocations but you need to understand:
- Where allocations originate in YOUR code (not runtime)
- Allocation rates (MB/min) not just totals
- Grouped by source location for cleaner output

**Key options**:
- min_percent: Filter out paths below this threshold (default: 1%)
- max_paths: Limit number of paths returned (default: 20)
- repo_prefix: Focus on your code (auto-detected if not specified)

**Returns**: Allocation paths sorted by size, with caller chains and rates.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":         ProfilePath(),
					"min_percent":     numberProp("Minimum allocation percentage to include (default: 1.0)", floatPtr(0), floatPtr(100)),
					"max_paths":       integerProp("Maximum paths to return (default: 20)", intPtr(1), nil),
					"repo_prefix":     arrayOrStringPropSchema(prop("string", "Repository prefix"), "Filter to paths containing these prefixes (auto-detected if not specified)"),
					"group_by_source": prop("boolean", "Group by first app frame instead of allocation site (default: false)"),
				}, "profile"),
			},
			Handler: pprofAllocPathsTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.overhead_report",
				Description: `Detect observability and infrastructure overhead in a profile.

**When to use**: To understand how much of your CPU/memory is spent on:
- OpenTelemetry tracing
- Logging (zap, logrus)
- Prometheus metrics
- gRPC/protobuf overhead
- JSON serialization
- Runtime/GC

**Returns**:
- Breakdown by category with percentages
- Top functions per category
- Severity ratings (low/medium/high)
- Actionable suggestions for high-overhead categories

**Example insight**: "OpenTelemetry Tracing: 13% - Consider reducing trace sampling rate"`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile":      ProfilePath(),
					"sample_index": prop("string", "Sample index to analyze (auto-detected based on profile type)"),
				}, "profile"),
			},
			Handler: pprofOverheadReportTool,
		},
		{
			Tool: &mcp.Tool{
				Name: "pprof.detect_repo",
				Description: `Auto-detect repository information from a profile.

**When to use**: To find the local repo root for source annotation without manual configuration.

**How it works**:
1. Extracts Go module paths from function names in the profile
2. Searches common locations for matching local repos
3. Validates by checking for go.mod or project structure

**Returns**: Detected module paths, local repo root (if found), and confidence level.`,
				InputSchema: NewObjectSchema(map[string]any{
					"profile": ProfilePath(),
				}, "profile"),
			},
			Handler: pprofDetectRepoTool,
		},
	}
	return tools
}
