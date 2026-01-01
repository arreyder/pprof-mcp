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

### 5. [x] Comparative Baseline Context

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

### 7. [x] Add `pprof.cross_correlate` Tool

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

### 8. [x] Add `pprof.regression_check` Tool

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

---

## NEW: Source & Fix Analysis Tools

These tools were identified during a temporal_sync profiling session where tracing the root cause (protojson in vendored baton-sdk) required manual file reading and grep searches.

### 12. [x] Add `pprof.trace_source` Tool

**Effort:** High
**Value:** Very High

Trace hot functions through the call chain, displaying actual source code including vendored dependencies.

**Requirements:**
- Given a hot function, show the complete call chain with source code
- Resolve source files in: app code, vendor directory, Go mod cache
- Annotate source lines with CPU/memory attribution from the profile
- Support context lines around hot spots

**Parameters:**
```json
{
  "profile": "string (required) - Path or handle to pprof profile",
  "function": "string (required) - Function name or regex to trace",
  "repo_root": "string (optional) - Local repository root for source resolution",
  "max_depth": "integer (optional, default: 10) - Maximum call stack depth to trace",
  "show_vendor": "boolean (optional, default: true) - Include vendored dependency source",
  "context_lines": "integer (optional, default: 5) - Lines of context around hot lines"
}
```

**Implementation approach:**
1. Use existing `pprof_focus_paths` logic to get call stack for target function
2. Extract source locations from profile data (file path + line number per frame)
3. Resolve source files:
   - App code: `{repo_root}/{relative_path}`
   - Vendored: `{repo_root}/vendor/{package_path}/{file}`
   - Go mod cache: `{GOPATH}/pkg/mod/{package}@{version}/{file}`
4. Read each source file and highlight hot lines with profile metrics
5. Return structured output with annotated source snippets

**Source resolution logic:**
```go
func resolveSourceFile(frame StackFrame, repoRoot string) (string, error) {
    // 1. Check if it's a local app file
    if strings.HasPrefix(frame.Package, repoPrefix) {
        relativePath := strings.TrimPrefix(frame.File, "/xsrc/")
        return filepath.Join(repoRoot, relativePath), nil
    }

    // 2. Check vendor directory
    vendorPath := filepath.Join(repoRoot, "vendor", frame.Package, filepath.Base(frame.File))
    if fileExists(vendorPath) {
        return vendorPath, nil
    }

    // 3. Try go mod download cache
    goModCache := filepath.Join(os.Getenv("GOPATH"), "pkg/mod", frame.Package+"@"+version)
    if fileExists(goModCache) {
        return goModCache, nil
    }

    return "", ErrSourceNotFound
}
```

**Output structure:**
```json
{
  "call_chain": [
    {
      "function": "gitlab.com/ductone/c1/pkg/ulambda/connectors.(*connectorClient).Invoke",
      "file": "/home/user/repos/c1/pkg/ulambda/connectors/connector_client.go",
      "line": 219,
      "flat_pct": 0.5,
      "cum_pct": 15.2,
      "source_snippet": "217:   token, err := c.getAuthToken(ctx)\n218:   // ...\n>>>219:   return c.cc.Invoke(ctx, method, args, reply, opts...)\n220:   // ...",
      "is_vendor": false
    },
    {
      "function": "google.golang.org/protobuf/encoding/protojson.decoder.unmarshalMessage",
      "file": "/home/user/repos/c1/vendor/google.golang.org/protobuf/encoding/protojson/decode.go",
      "line": 142,
      "flat_pct": 8.2,
      "cum_pct": 12.1,
      "source_snippet": "... annotated source ...",
      "is_vendor": true,
      "vendor_package": "google.golang.org/protobuf",
      "vendor_version": "v1.31.0"
    }
  ],
  "total_functions_traced": 8,
  "app_functions": 3,
  "vendor_functions": 5
}
```

**Validation requirements:**
1. Schema must define `call_chain` as array with all nested fields
2. Test with function in app code
3. Test with function in vendored dependency
4. Test with function not found (should return helpful error)
5. Test `max_depth` limiting
6. Handle case where source file doesn't exist locally

---

### 13. [x] Add `pprof.vendor_analyze` Tool

**Effort:** Medium
**Value:** High

Analyze vendored dependencies appearing in hot paths, providing version info, upstream links, and known performance issues.

**Requirements:**
- Identify external packages in hot paths
- Parse go.mod to get current versions
- Aggregate CPU/memory by top-level package
- Enrich with known performance issues from knowledge base
- Optionally check for newer versions

**Parameters:**
```json
{
  "profile": "string (required) - Path or handle to pprof profile",
  "repo_root": "string (optional) - Repository root to find go.mod/vendor",
  "min_pct": "number (optional, default: 1.0) - Minimum percentage to include",
  "check_updates": "boolean (optional, default: false) - Check for newer versions"
}
```

**Implementation approach:**
1. Run `pprof_top` to get hot functions
2. Parse function names to extract package paths, filter to external packages
3. Parse `go.mod` to get current versions:
   ```go
   func parseGoMod(repoRoot string) (map[string]string, error) {
       content, _ := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
       // Parse require blocks, extract module -> version mapping
   }
   ```
4. Aggregate by top-level package (sum CPU/memory percentages)
5. Match against known performance issues database
6. Derive GitHub/GitLab repo URL from package path

**Knowledge base file (`known_perf_issues.yaml`):**
```yaml
packages:
  "google.golang.org/protobuf":
    repo_url: "https://github.com/protocolbuffers/protobuf-go"
    patterns:
      - match: "protojson"
        severity: high
        issue: "protojson uses reflection, 5-10x slower than binary proto"
        recommendation: "Use proto.Marshal/Unmarshal for performance-critical paths"
      - match: "anypb.UnmarshalTo"
        severity: medium
        issue: "Any type requires type registry lookup for each message"
        recommendation: "Consider using concrete types instead of Any"

  "encoding/json":
    patterns:
      - match: "Unmarshal|Decode"
        severity: medium
        issue: "Standard library json uses reflection"
        recommendation: "Consider json-iterator/go or code generation (easyjson)"

  "github.com/klauspost/compress/zstd":
    patterns:
      - match: "Encode|Decode"
        severity: low
        issue: "Compression is CPU-intensive by nature"
        recommendation: "Consider compression level trade-offs, use sync.Pool for encoders"

  "database/sql":
    patterns:
      - match: "(*DB).Query|(*DB).Exec"
        severity: medium
        issue: "Connection pool contention or query preparation overhead"
        recommendation: "Use prepared statements, tune pool size"
```

**Output structure:**
```json
{
  "vendor_hotspots": [
    {
      "package": "google.golang.org/protobuf",
      "version": "v1.31.0",
      "total_flat_pct": 12.5,
      "total_cum_pct": 45.2,
      "hot_functions": [
        {"name": "protojson.decoder.unmarshalMessage", "flat_pct": 8.2},
        {"name": "protojson.decoder.unmarshalAny", "flat_pct": 4.3}
      ],
      "repo_url": "https://github.com/protocolbuffers/protobuf-go",
      "latest_version": "v1.32.0",
      "known_issues": [
        {
          "pattern": "protojson",
          "severity": "high",
          "issue": "protojson uses reflection and is 5-10x slower than binary proto",
          "recommendation": "Use proto.Marshal/Unmarshal instead of protojson when possible"
        }
      ]
    }
  ],
  "total_vendor_pct": 65.3,
  "total_app_pct": 34.7
}
```

**Validation requirements:**
1. Schema must define `vendor_hotspots` array with nested `hot_functions` and `known_issues` arrays
2. Test with profile that has vendored dependencies in hot path
3. Test `check_updates` flag (may need network access or mock)
4. Test with missing go.mod (should still work, just without versions)
5. Verify known_issues matching works with regex patterns

---

### 14. [x] Add `pprof.explain_overhead` Tool

**Effort:** Medium
**Value:** High

Given an overhead category or hot function, provide detailed explanation of why it's slow and the underlying mechanisms.

**Requirements:**
- Accept category (from overhead_report) or specific function
- Return detailed explanation of WHY it's slow, not just WHAT is slow
- Include optimization strategies with expected impact and effort
- Contextualize with actual profile data if provided

**Parameters:**
```json
{
  "profile": "string (optional) - Path or handle for context",
  "category": "string (optional) - Overhead category from overhead_report (e.g., 'Protobuf Serialization', 'Runtime/GC')",
  "function": "string (optional) - Specific function to explain",
  "detail_level": "string (optional, default: 'standard') - 'brief', 'standard', 'detailed'"
}
```

**Implementation approach:**
1. Build explanation database (`explanations.yaml`) with structured knowledge
2. Match input (category or function) to explanations
3. If profile provided, pull actual metrics to contextualize
4. Format response based on detail_level

**Explanation database (`explanations.yaml`):**
```yaml
categories:
  "Protobuf Serialization":
    brief: "Protocol buffer marshaling/unmarshaling overhead"
    standard: |
      Protobuf serialization appears as overhead due to:

      1. **Reflection**: protojson and some proto operations use Go reflection
         to access message fields dynamically, which is slow compared to
         generated code.

      2. **Memory allocation**: Each unmarshal creates new message objects.
         Without pooling, this creates GC pressure.

      3. **Type resolution**: google.protobuf.Any requires registry lookups
         via protoregistry.GlobalTypes.FindMessageByURL() for every Any field.

      4. **String conversion**: protojson parses/generates UTF-8 strings,
         allocating for each string field.
    detailed: |
      [Even more detail with code examples...]
    common_causes:
      - "Using protojson instead of binary proto"
      - "Excessive use of google.protobuf.Any wrappers"
      - "Not reusing message objects with Reset()"
      - "Large messages with many fields"
    optimization_strategies:
      - strategy: "Switch to binary proto"
        impact: "5-10x faster serialization"
        effort: "medium"
        description: "Replace protojson.Marshal/Unmarshal with proto.Marshal/Unmarshal"
      - strategy: "Use message pools"
        impact: "2-3x faster, reduces GC"
        effort: "low"
        description: "Use sync.Pool to reuse message objects"
      - strategy: "Avoid Any types"
        impact: "Eliminates type registry lookups"
        effort: "high"
        description: "Use concrete types or oneof instead of Any"

  "Runtime/GC":
    brief: "Go garbage collection overhead"
    standard: |
      High GC overhead indicates memory allocation pressure:

      1. **Allocation rate**: Creating many short-lived objects triggers
         frequent GC cycles. Each cycle pauses goroutines for marking.

      2. **Heap size**: Larger heaps take longer to scan. GC work is
         proportional to live heap size.

      3. **Pointer-heavy structures**: Maps, slices of pointers, and
         linked structures require more GC scanning than flat data.
    optimization_strategies:
      - strategy: "Reduce allocations with sync.Pool"
        impact: "20-50% GC reduction"
        effort: "low"
      - strategy: "Pre-allocate slices"
        impact: "Reduces slice growth allocations"
        effort: "low"
      - strategy: "Use GOGC tuning"
        impact: "Trade memory for CPU"
        effort: "low"
        description: "Set GOGC=200 to run GC less frequently"

functions:
  "protojson.decoder.unmarshalAny":
    category: "Protobuf Serialization"
    explanation: |
      This function unmarshals google.protobuf.Any messages from JSON.

      It's slow because it must:
      1. Parse the @type URL string from JSON
      2. Look up the message type in protoregistry.GlobalTypes
      3. Create a new instance of that type via reflection
      4. Recursively unmarshal the nested message

      The type registry lookup alone requires string parsing and map
      lookups for EVERY Any field in the message tree.
    why_expensive: "Type registry lookup + reflection-based instantiation"
    alternatives:
      - "Use concrete types instead of Any"
      - "Use binary proto encoding (avoids JSON parsing entirely)"
      - "If Any is required, consider caching resolved types"
```

**Output structure:**
```json
{
  "category": "Protobuf Serialization",
  "explanation": {
    "summary": "Protocol buffer marshaling/unmarshaling overhead",
    "detailed": "... full explanation from database ...",
    "why_slow": [
      "Reflection-based field access",
      "Type URL resolution for Any types",
      "String parsing allocations"
    ],
    "common_causes": [
      "Using protojson instead of binary proto",
      "Excessive use of google.protobuf.Any wrappers"
    ]
  },
  "in_your_profile": {
    "total_pct": 159.7,
    "top_contributors": [
      {"function": "protojson.decoder.unmarshalMessage", "pct": 45.2},
      {"function": "protojson.decoder.unmarshalAny", "pct": 32.1}
    ]
  },
  "optimization_strategies": [
    {
      "strategy": "Switch to binary proto encoding",
      "expected_impact": "5-10x faster serialization",
      "effort": "medium",
      "description": "Replace protojson.Marshal/Unmarshal with proto.Marshal/Unmarshal",
      "applicable": true,
      "reason": "Your profile shows protojson is dominant; binary proto would eliminate this"
    }
  ]
}
```

**Validation requirements:**
1. Schema must define nested `explanation`, `in_your_profile`, and `optimization_strategies` objects
2. Test with known category (e.g., "Runtime/GC")
3. Test with specific function (e.g., "protojson.decoder.unmarshalAny")
4. Test with profile provided (should include `in_your_profile` section)
5. Test without profile (should omit `in_your_profile` or return null)
6. Test with unknown category/function (should return helpful message)

---

### 15. [x] Add `pprof.suggest_fix` Tool

**Effort:** High
**Value:** Very High

Generate concrete, actionable code changes to fix identified performance issues, including file paths, diffs, and PR descriptions.

**Requirements:**
- Identify applicable fixes based on profile analysis
- Generate concrete code patches (unified diff format)
- Include PR description with context and expected impact
- Handle vendored dependencies (note upstream PR needed)

**Parameters:**
```json
{
  "profile": "string (required) - Path or handle to pprof profile",
  "issue": "string (required) - Issue identifier (e.g., 'protojson_overhead', 'gc_pressure', 'allocation_hot_spot')",
  "repo_root": "string (optional) - Repository root for generating patches",
  "target_function": "string (optional) - Specific function to optimize",
  "output_format": "string (optional, default: 'structured') - 'structured', 'diff', 'pr_description'"
}
```

**Implementation approach:**
1. Define fix templates (`fix_templates.yaml`) for common issues
2. Analyze profile to identify which fixes apply
3. If repo_root provided, find actual files and generate real diffs
4. Generate PR description with context

**Fix templates (`fix_templates.yaml`):**
```yaml
fixes:
  protojson_to_binary:
    issue_id: "protojson_overhead"
    description: "Replace protojson with binary proto encoding"
    applicable_when:
      - "protojson appears in hot path with >10% CPU"
      - "Binary proto is acceptable (not human-readable requirement)"
    detection_patterns:
      - "protojson.Marshal"
      - "protojson.Unmarshal"
      - "protojson.decoder"
    template:
      before: |
        import "google.golang.org/protobuf/encoding/protojson"

        func marshal(msg proto.Message) ([]byte, error) {
            return protojson.Marshal(msg)
        }

        func unmarshal(b []byte, msg proto.Message) error {
            return protojson.Unmarshal(b, msg)
        }
      after: |
        import "google.golang.org/protobuf/proto"

        func marshal(msg proto.Message) ([]byte, error) {
            return proto.Marshal(msg)
        }

        func unmarshal(b []byte, msg proto.Message) error {
            return proto.Unmarshal(b, msg)
        }
    considerations:
      - "Binary proto is not human-readable for debugging"
      - "Requires coordinated update if used in API/storage"
      - "Consider content-type negotiation for backward compatibility"
    expected_impact:
      cpu_reduction: "40-60%"
      allocation_reduction: "50-70%"
    pr_template: |
      ## Summary
      Optimize {service} by switching from protojson to binary proto encoding.

      ## Problem
      Profile analysis shows protojson serialization consuming {overhead_pct}% of CPU time.

      Top functions:
      {top_functions}

      ## Solution
      Replace `protojson.Marshal/Unmarshal` with `proto.Marshal/Unmarshal`.

      ## Expected Impact
      - CPU reduction: 40-60%
      - Allocation rate reduction: 50-70%

      ## Testing
      - [ ] Unit tests pass
      - [ ] Integration tests pass
      - [ ] Load test comparison before/after

  sync_pool_for_allocations:
    issue_id: "allocation_hot_spot"
    description: "Use sync.Pool to reuse frequently allocated objects"
    applicable_when:
      - "Single type accounts for >5% of allocations"
      - "Object is short-lived and frequently created"
    template:
      before: |
        func process(data []byte) *Result {
            result := &Result{}
            // ... populate result
            return result
        }
      after: |
        var resultPool = sync.Pool{
            New: func() interface{} {
                return &Result{}
            },
        }

        func process(data []byte) *Result {
            result := resultPool.Get().(*Result)
            result.Reset() // Clear previous data
            // ... populate result
            return result
        }

        // Caller must return to pool when done:
        // defer resultPool.Put(result)
    considerations:
      - "Caller must return objects to pool"
      - "Objects must be safely resettable (implement Reset method)"
      - "Don't pool objects that escape to other goroutines long-term"
    expected_impact:
      allocation_reduction: "50-80% for pooled type"
      gc_reduction: "Proportional to allocation reduction"
```

**Output structure:**
```json
{
  "issue": "protojson_overhead",
  "analysis": {
    "overhead_pct": 159.7,
    "top_functions": [
      {"function": "protojson.decoder.unmarshalMessage", "pct": 45.2},
      {"function": "protojson.Unmarshal", "pct": 32.1}
    ]
  },
  "applicable_fixes": [
    {
      "fix_id": "protojson_to_binary",
      "description": "Replace protojson with binary proto encoding",
      "expected_impact": {
        "cpu_reduction": "40-60%",
        "allocation_reduction": "50-70%"
      },
      "files_to_modify": [
        {
          "path": "vendor/github.com/conductorone/baton-sdk/pkg/lambda/grpc/transport.go",
          "is_vendor": true,
          "upstream_repo": "https://github.com/conductorone/baton-sdk",
          "changes": [
            {
              "line": 107,
              "before": "return protojson.Marshal(f.msg)",
              "after": "return proto.Marshal(f.msg)"
            },
            {
              "line": 155,
              "before": "return protojson.Unmarshal(b, f.msg)",
              "after": "return proto.Unmarshal(b, f.msg)"
            }
          ]
        }
      ],
      "diff": "--- a/pkg/lambda/grpc/transport.go\n+++ b/pkg/lambda/grpc/transport.go\n@@ -1,7 +1,7 @@\n...",
      "pr_description": "## Summary\nOptimize temporal_sync by switching from protojson to binary proto...",
      "considerations": [
        "Requires coordinated update to Lambda handlers",
        "Consider content-type negotiation for backward compatibility"
      ],
      "is_vendored": true,
      "upstream_pr_needed": true
    }
  ],
  "next_steps": [
    "1. Open PR in upstream baton-sdk repo with the suggested changes",
    "2. Update Lambda connector handlers to accept binary proto",
    "3. Add content-type negotiation for backward compatibility",
    "4. Update vendor in this repo after upstream merge"
  ]
}
```

**Validation requirements:**
1. Schema must define deeply nested structure (`applicable_fixes` > `files_to_modify` > `changes`)
2. Test with known issue type (e.g., "protojson_overhead")
3. Test with profile that doesn't have the issue (should return empty fixes or "not applicable")
4. Test diff generation with real repo_root
5. Test with vendored file (should set `is_vendored: true`, `upstream_pr_needed: true`)
6. Test `output_format` variations
7. Verify PR description template substitution works

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

### 11. [x] Add `pprof.hotspot_summary` Tool

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
