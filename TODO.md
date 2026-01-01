# pprof-mcp Improvement Backlog

Future improvements identified during discovery analysis sessions.

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

## Medium Value Improvements

### 4. Comparative Baseline Context

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

## Low Value Improvements

### 6. [x] Auto-Suggest Tags/Tenant Analysis

**Effort:** Low
**Value:** Low

Automatically detect if profiles have tenant labels and suggest `pprof.tags` analysis.

**Requirements:**
- Check profile for common label keys (tenant_id, connector_id)
- Add hint to pprof.top output if labels detected
- Example: "Profile contains tenant_id labels. Use pprof.tags to analyze per-tenant."

---

## Completed

- [x] Add file:line source locations to `alloc_paths` output
- [x] Auto-suggest `memory_sanity` for heap profiles
- [x] Add `storylines` hint to discovery flow
- [x] Fix `sample_index` for heap profiles in `pprof.peek`
- [x] Add `overhead_report` tool
- [x] Add `detect_repo` tool
- [x] Add `alloc_paths` tool with allocation rates
