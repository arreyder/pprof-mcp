# pprof-mcp Improvement Backlog

Future improvements identified during discovery analysis sessions.

---

## Development Guidelines

**CRITICAL: Follow these to avoid breaking the MCP server.**

### Output Schema Requirements

Every tool MUST have a matching output schema in `cmd/pprof-mcp-server/output_schemas.go`:

1. **Schema must match actual output exactly** - If your tool returns a field, the schema must define it
2. **Use `additionalProperties: true`** for result objects that may have dynamic fields
3. **Arrays need explicit schema** - Don't rely on additionalProperties for array fields:
   ```go
   // WRONG - arrays will fail validation
   "result": NewObjectSchemaWithAdditional(map[string]any{...}, false)

   // RIGHT - explicitly define array fields
   "recommendations": arrayPropSchema(NewObjectSchema(map[string]any{
       "priority": prop("string", "..."),
   }, "priority"), "List of recommendations"),
   ```
4. **Test schema validation** - Run the tool via MCP after implementation to catch schema mismatches

### Testing Checklist

Before considering a tool complete:

- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] Tool registered in `cmd/pprof-mcp-server/tools.go`
- [ ] Output schema added to `output_schemas.go`
- [ ] **Restart MCP server** after rebuilding binary
- [ ] Test tool via MCP client (not just unit tests)
- [ ] Test with handle inputs (`handle:abc123`) if tool accepts profiles
- [ ] Verify JSON output parses without schema errors

### Handle Resolution

Tools that accept profile paths MUST support handle resolution:

```go
// Use resolveProfilePath helper
resolvedPath, err := resolveProfilePath(profileParam)
if err != nil {
    return nil, err
}
```

### Common Pitfalls

1. **Forgetting to restart MCP server** - Binary changes require server restart
2. **Schema/output mismatch** - Causes silent failures or cryptic errors
3. **Missing tool registration** - Tool won't appear in MCP
4. **Hardcoded paths** - Use handle abstraction, not temp file paths in examples
5. **Untested edge cases** - Empty profiles, missing sample types, no matches

---

## High Value Improvements

### 1. [x] Add `pprof.discover` Meta-Tool

**Effort:** High
**Value:** Very High

Create a single tool that runs a comprehensive discovery analysis on a service, returning a structured report.

**Requirements:**
- Accept service/env parameters
- Download profiles automatically (CPU, heap, mutex, goroutine)
- Run analysis suite: `top`, `overhead_report`, `alloc_paths`, `memory_sanity`
- Generate structured report with:
  - CPU breakdown and utilization percentage
  - Infrastructure overhead categories with percentages
  - Top allocation sources with rates (MB/min)
  - Mutex/contention issues
  - Prioritized optimization opportunities

**Implementation approach:**
1. Create `internal/pprof/discover.go` with a `RunDiscovery` function
2. Orchestrate profile download via existing datadog functions
3. Run each analysis tool and aggregate results
4. Return a `DiscoveryReport` struct with sections for each category
5. Add tool registration in `tools.go`

**Example usage:**
```
pprof.discover(service="temporal_sync", env="prod-usw2")
```

**Output structure:**
```json
{
  "service": "temporal_sync",
  "env": "prod-usw2",
  "timestamp": "...",
  "cpu": {
    "utilization_pct": 206.0,
    "top_functions": [...],
    "overhead": {...}
  },
  "heap": {
    "alloc_rate": "55.62 GB/min",
    "top_paths": [...],
    "memory_sanity": {...}
  },
  "mutex": {
    "total_delay": "20.6s",
    "top_contentions": [...]
  },
  "recommendations": [
    {"priority": "high", "area": "SQLite", "suggestion": "..."},
    ...
  ]
}
```

---

### 2. [x] Add Goroutine Analysis Tool

**Effort:** Medium
**Value:** High

Create `pprof.goroutine_analysis` to detect goroutine leaks and blocking patterns.

**Requirements:**
- Analyze goroutine profiles for:
  - Total goroutine count
  - Goroutines grouped by state (running, waiting, syscall)
  - Blocked goroutines by wait reason (channel, mutex, IO)
  - Potential leak patterns (large counts of similar stacks)
- Flag anomalies (e.g., >1000 goroutines waiting on same channel)

**Implementation approach:**
1. Create `internal/pprof/goroutine_analysis.go`
2. Parse goroutine profile and group by stack signature
3. Identify common blocking patterns (channel ops, mutex, select)
4. Calculate distribution statistics
5. Return structured analysis with warnings for anomalies

**Example output:**
```json
{
  "total_goroutines": 1234,
  "by_state": {
    "running": 8,
    "syscall": 45,
    "waiting": 1181
  },
  "top_wait_reasons": [
    {"reason": "chan receive", "count": 500, "sample_stack": "..."},
    {"reason": "select", "count": 300, "sample_stack": "..."}
  ],
  "potential_leaks": [
    {"stack_signature": "...", "count": 200, "severity": "high"}
  ]
}
```

---

### 3. [x] Profile Handle Abstraction

**Effort:** High
**Value:** Medium

Simplify profile path management by returning handles instead of file paths.

**Requirements:**
- `profiles.download_latest_bundle` returns handle IDs instead of paths
- Subsequent tools accept handle IDs: `pprof.top(profile="handle:abc123")`
- Handle registry tracks downloaded profiles with metadata
- Handles auto-expire after session ends

**Implementation approach:**
1. Create `internal/profiles/registry.go` with handle management
2. Modify download functions to register handles
3. Add handle resolution to all pprof.* tools
4. Store handle metadata (service, env, type, timestamp, path)

**Benefits:**
- Cleaner tool invocations
- No manual path management
- Enables profile comparison by handle

---

### 4. Add `pprof.contention_analysis` Tool

**Effort:** Medium
**Value:** High

Create dedicated mutex/block profile analysis (parallel to goroutine_analysis).

**Requirements:**
- Analyze mutex and block profiles for lock contention patterns
- Group contentions by lock site (source location)
- Calculate wait time distribution and statistics
- Identify top contenders (functions waiting most)
- Flag potential issues (lock ordering, hot locks)

**Implementation approach:**
1. Create `internal/pprof/contention_analysis.go`
2. Parse mutex profile (contentions, delay sample types)
3. Parse block profile (blocking events)
4. Group by lock acquisition site
5. Calculate per-lock stats: total delay, acquisition count, avg wait
6. Identify patterns: single hot lock vs distributed contention

**Example usage:**
```
pprof.contention_analysis(profile="handle:abc123")
```

**Output structure:**
```json
{
  "profile_type": "mutex",
  "total_contentions": 45000,
  "total_delay": "23.5s",
  "by_lock_site": [
    {
      "lock_site": "sync.(*RWMutex).Lock",
      "source_location": "pkg/cache/cache.go:142",
      "contentions": 12000,
      "total_delay": "8.2s",
      "avg_delay": "683µs",
      "top_waiters": [
        {"function": "pkg/api.GetUser", "delay": "3.1s"},
        {"function": "pkg/api.ListUsers", "delay": "2.4s"}
      ]
    }
  ],
  "patterns": [
    {"type": "hot_lock", "severity": "high", "description": "Single lock accounts for 35% of contention"},
    {"type": "lock_convoy", "severity": "medium", "description": "Multiple goroutines queueing on cache lock"}
  ],
  "recommendations": [
    "Consider sharding cache by key prefix to reduce lock contention",
    "RWMutex at cache.go:142 has high write contention - consider sync.Map for read-heavy workload"
  ]
}
```

**Validation requirements:**
1. Add output schema to `output_schemas.go` with ALL fields (especially arrays like `by_lock_site`, `patterns`, `recommendations`)
2. Test via MCP with real mutex profile: `profiles.download_latest_bundle` then `pprof.contention_analysis(profile="handle:xxx_mutex")`
3. Test with block profile as well
4. Verify handle resolution works

---

## Medium Value Improvements

### 5. Comparative Baseline Context

**Effort:** Medium
**Value:** Medium

Add historical context to analysis results.

**Requirements:**
- Store/retrieve baseline metrics for services
- Compare current profile to historical average
- Surface significant deviations: "Allocations 40% higher than last week"

**Implementation approach:**
1. Optional baseline file or database
2. `pprof.top` with `compare_baseline=true` flag
3. Calculate deltas and surface anomalies

---

### 5. [x] Summary/Report Generation

**Effort:** Medium
**Value:** Medium

Add `pprof.generate_report` that formats analysis results as markdown.

**Requirements:**
- Accept multiple analysis results (from discover or individual tools)
- Generate formatted markdown report with tables
- Include executive summary and recommendations

**Implementation approach:**
1. Create `internal/pprof/report.go` with markdown templating
2. Accept structured input from other tools
3. Generate tables, bullet points, priority lists

---

### 7. Add `pprof.cross_correlate` Tool

**Effort:** Medium
**Value:** Medium

Link hotspots across profile types (CPU, heap, mutex) from the same bundle.

**Requirements:**
- Accept a profile bundle (from download_latest_bundle)
- Find functions that appear in multiple profile types
- Rank by combined impact (e.g., high CPU + high allocations)
- Surface functions that are "hot" across dimensions

**Implementation approach:**
1. Create `internal/pprof/cross_correlate.go`
2. Load CPU, heap, mutex profiles from bundle
3. Build function index with metrics from each profile type
4. Score functions by combined impact
5. Return ranked list with per-profile-type breakdown

**Example usage:**
```
pprof.cross_correlate(bundle="handle:bundle123")
```

**Output structure:**
```json
{
  "correlations": [
    {
      "function": "github.com/org/repo/pkg/sync.ProcessBatch",
      "combined_score": 0.85,
      "cpu": {"flat_pct": 12.5, "rank": 3},
      "heap": {"alloc_pct": 8.2, "rank": 5},
      "mutex": {"delay_pct": 15.0, "rank": 1},
      "insight": "High CPU, allocations, AND lock contention - prime optimization target"
    }
  ],
  "cpu_only_hotspots": [...],
  "heap_only_hotspots": [...],
  "mutex_only_hotspots": [...]
}
```

**Validation requirements:**
1. Schema must define nested objects for `cpu`, `heap`, `mutex` within correlations
2. Test with bundle that has all profile types (CPU + heap + mutex)
3. Test with bundle missing some profile types (should handle gracefully)
4. Verify bundle handle resolution (different from single profile handles)

---

### 8. Add `pprof.regression_check` Tool

**Effort:** Low
**Value:** Medium

CI-friendly tool to check if a function exceeds a threshold.

**Requirements:**
- Accept function pattern and threshold (e.g., `flat_pct > 5%`)
- Return pass/fail with details
- Support multiple checks in one call
- Exit code compatible for CI pipelines

**Implementation approach:**
1. Create `internal/pprof/regression_check.go`
2. Run pprof.top with focus on specified functions
3. Compare against thresholds
4. Return structured pass/fail result

**Example usage:**
```
pprof.regression_check(
  profile="handle:abc123",
  checks=[
    {"function": "runtime.mallocgc", "metric": "flat_pct", "max": 5.0},
    {"function": "encoding/json.*", "metric": "cum_pct", "max": 10.0}
  ]
)
```

**Output structure:**
```json
{
  "passed": false,
  "checks": [
    {
      "function": "runtime.mallocgc",
      "metric": "flat_pct",
      "threshold": 5.0,
      "actual": 3.2,
      "passed": true
    },
    {
      "function": "encoding/json.Unmarshal",
      "metric": "cum_pct",
      "threshold": 10.0,
      "actual": 14.5,
      "passed": false,
      "message": "encoding/json.Unmarshal cum% (14.5%) exceeds threshold (10.0%)"
    }
  ]
}
```

**Validation requirements:**
1. Schema must define `checks` as array with all fields including optional `message`
2. Test passing case (all checks pass)
3. Test failing case (some checks fail)
4. Test with function pattern that matches nothing (should pass with actual=0 or skip)
5. Test with handle input

---

### 9. Add `datadog.profiles.aggregate` Tool

**Effort:** Medium
**Value:** Medium

Merge profiles over a time window for statistically significant analysis.

**Requirements:**
- Accept service, env, and time window
- Download N profiles from window
- Merge into single aggregated profile
- Return handle to merged profile

**Implementation approach:**
1. Create `internal/datadog/aggregate.go`
2. Use profiles.list to find profiles in window
3. Download profiles (limit to reasonable count, e.g., 10)
4. Use pprof.merge to combine
5. Register merged profile as handle

**Example usage:**
```
datadog.profiles.aggregate(
  service="temporal_sync",
  env="prod-usw2",
  window="1h",
  limit=10
)
```

**Output structure:**
```json
{
  "handle": "handle:merged_abc123",
  "profiles_merged": 10,
  "time_range": {
    "from": "2026-01-01T14:00:00Z",
    "to": "2026-01-01T15:00:00Z"
  },
  "total_duration": "610.5s",
  "hint": "Use pprof.top(profile='handle:merged_abc123') to analyze aggregated data"
}
```

**Validation requirements:**
1. Schema must define `time_range` as nested object with `from`/`to` strings
2. Test returned handle works with `pprof.top`
3. Test with window that has < limit profiles (should merge what's available)
4. Test with window that has 0 profiles (should return error, not empty)
5. Verify merged profile registered in handle registry

---

## Low Value Improvements

### 10. [x] Auto-Suggest Tags/Tenant Analysis

**Effort:** Low
**Value:** Low

Automatically detect if profiles have tenant labels and suggest `pprof.tags` analysis.

**Requirements:**
- Check profile for common label keys (tenant_id, connector_id)
- Add hint to pprof.top output if labels detected
- Example: "Profile contains tenant_id labels. Use pprof.tags to analyze per-tenant."

---

### 11. Add `pprof.hotspot_summary` Tool

**Effort:** Low
**Value:** Low

Quick unified view of top hotspots across all profile types in one call.

**Requirements:**
- Accept profile bundle
- Return top 3-5 from each profile type in one response
- Simpler/faster than cross_correlate (no scoring, just aggregation)

**Implementation approach:**
1. Create `internal/pprof/hotspot_summary.go`
2. Run pprof.top on each profile type with nodecount=5
3. Aggregate into single response

**Example usage:**
```
pprof.hotspot_summary(bundle="handle:bundle123")
```

**Output structure:**
```json
{
  "cpu_top5": [{"function": "...", "flat_pct": 12.5}, ...],
  "heap_top5": [{"function": "...", "alloc_pct": 8.2}, ...],
  "mutex_top5": [{"function": "...", "delay_pct": 5.0}, ...],
  "goroutine_count": 1234
}
```

**Validation requirements:**
1. Schema must define each `*_top5` as array of objects
2. Test with full bundle (all profile types present)
3. Test with partial bundle (e.g., no mutex profile - should return null/empty for that field)
4. Verify bundle handle resolution works

---

## Completed

- [x] Add file:line source locations to `alloc_paths` output
- [x] Auto-suggest `memory_sanity` for heap profiles
- [x] Add `storylines` hint to discovery flow
- [x] Fix `sample_index` for heap profiles in `pprof.peek`
- [x] Add `overhead_report` tool
- [x] Add `detect_repo` tool
- [x] Add `alloc_paths` tool with allocation rates
