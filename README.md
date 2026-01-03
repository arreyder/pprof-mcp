# pprof-mcp

MCP server and CLI tooling for Datadog profile download and deterministic pprof analysis.

## Features

- **Download profiles** from Datadog Continuous Profiler
- **Analyze profiles** with pprof (top, peek, list, storylines, etc.)
- **Compare profiles** before/after to verify performance fixes
- **Track functions over time** across multiple profiles
- **Filter by tags** (tenant_id, connector_id, etc.)
- **Generate visualizations** (flamegraphs, call graphs)

## Requirements

- Go 1.22+
- Datadog API keys:
  - `DD_API_KEY`
  - `DD_APP_KEY`
  - `DD_SITE` (optional, defaults to `us3.datadoghq.com`)

## Quick Start

```bash
make vendor
make test
make build-profctl
make build-server
```

## CLI Usage

### List and pick profiles

```bash
# List available profiles from the last 24 hours
./bin/profctl datadog profiles list --service myservice --env prod --hours 24

# Pick oldest profile (for baseline comparison)
./bin/profctl datadog profiles pick --service myservice --env prod --strategy oldest

# Pick latest profile
./bin/profctl datadog profiles pick --service myservice --env prod --strategy latest
```

### Download profiles

```bash
# Download latest profile bundle
./bin/profctl download --service myservice --env prod --out ./profiles

# Download specific profile by ID
./bin/profctl download --service myservice --env prod --out ./profiles \
  --profile_id <PROFILE_ID> --event_id <EVENT_ID>
```

### Analyze profiles

```bash
# Show top CPU consumers
./bin/profctl pprof top --profile ./profiles/myservice_prod_cpu.pprof --cum --nodecount 20

# Show callers/callees of a function
./bin/profctl pprof peek --profile ./profiles/myservice_prod_cpu.pprof --regex "myFunction"

# Line-level annotation
./bin/profctl pprof list --profile ./profiles/myservice_prod_cpu.pprof --function "myFunction" --repo_root .

# Find hot paths in your code
./bin/profctl pprof storylines --profile ./profiles/myservice_prod_cpu.pprof \
  --n 4 --repo_prefix github.com/myorg/myrepo --repo_root .
```

### Compare profiles

```bash
# Compare before/after profiles
./bin/profctl pprof diff_top --before ./baseline_cpu.pprof --after ./current_cpu.pprof
```

## MCP Server

The MCP server runs over stdio and integrates with Claude Desktop/Claude Code.

### Configuration

Add to your Claude config:

**Linux**: `~/.config/Claude/claude_desktop_config.json`
**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`

Optional safety: set `PPROF_MCP_BASEDIR` to restrict file reads/writes to a base directory (paths are cleaned and must stay within this directory). For Codex clients that require tool names without dots, set `PPROF_MCP_TOOL_NAME_MODE=codex` (or pass `--tool-name-mode=codex`) to expose tool names with underscores instead of dots.

### Codex Compatibility

Codex requires tool names that match `^[a-zA-Z0-9_-]+$`. To support Codex, run the server with `PPROF_MCP_TOOL_NAME_MODE=codex` or `--tool-name-mode=codex`. This replaces dots with underscores (e.g., `pprof.top` becomes `pprof_top`) while keeping the same behavior and schemas.

```json
{
  "mcpServers": {
    "pprof-mcp": {
      "command": "/path/to/pprof-mcp-server",
      "env": {
        "DD_API_KEY": "your-api-key",
        "DD_APP_KEY": "your-app-key",
        "DD_SITE": "us3.datadoghq.com"
      }
    }
  }
}
```

Or run from source:

```json
{
  "mcpServers": {
    "pprof-mcp": {
      "command": "bash",
      "args": ["-lc", "cd /path/to/pprof-mcp && go run ./cmd/pprof-mcp-server"],
      "env": {
        "DD_API_KEY": "your-api-key",
        "DD_APP_KEY": "your-app-key"
      }
    }
  }
}
```

## MCP Tools

### Profile Download (Universal)

| Tool | Description |
|------|-------------|
| `profiles.download` | **Smart wrapper** - Auto-detects environment (d2 vs prod/staging) and uses appropriate download method |

### Datadog Integration

| Tool | Description |
|------|-------------|
| `profiles.download_latest_bundle` | Download profile bundle from Datadog (explicit Datadog mode) |
| `datadog.profiles.list` | List available profiles (supports relative times like `-3h`) |
| `datadog.profiles.pick` | Select profile by strategy (latest, oldest, closest_to_ts, manual_index, most_samples, **anomaly**) |
| `datadog.profiles.aggregate` | Aggregate multiple profiles over a time window into a merged handle |
| `datadog.profiles.compare_range` | Compare profiles from two time ranges (before/after deployment) |
| `datadog.profiles.near_event` | Find profiles around a specific event (OOM, restart, incident) |
| `datadog.metrics.discover` | Discover available metrics for correlation (Go runtime, container) |
| `datadog.metrics_at_timestamp` | Query metrics around a specific timestamp (correlate profiles with operational state) |
| `datadog.function_history` | Track a function's CPU% across profiles over time |

### Local D2 Development

| Tool | Description |
|------|-------------|
| `d2.profiles.download` | Download profiles from local d2 services (uses kubectl + port-forward) |

### Profile Analysis

| Tool | Description |
|------|-------------|
| `pprof.top` | Show top functions by CPU/memory (includes contextual hints) |
| `pprof.peek` | Show callers and callees (use `sample_index=alloc_space` for heap) |
| `pprof.list` | Line-level source annotation |
| `pprof.trace_source` | Trace a hot function with source snippets and call chain context |
| `pprof.discover` | Run end-to-end discovery analysis (downloads + analyzes) |
| `pprof.storylines` | Find hot code paths in your repository (auto-detects heap profiles) |
| `pprof.alloc_paths` | Analyze allocation paths with rates (MB/min) and caller chains |
| `pprof.overhead_report` | Detect observability overhead (OTel, zap, gRPC, protobuf) |
| `pprof.explain_overhead` | Explain why an overhead category/function is expensive |
| `pprof.detect_repo` | Auto-detect local repository from profile function names |
| `pprof.memory_sanity` | Detect RSS/heap mismatch patterns (SQLite, CGO, goroutines) |
| `pprof.goroutine_analysis` | Detect goroutine leaks and blocking patterns |
| `pprof.goroutine_categorize` | Categorize goroutines by framework/subsystem (presets: temporal, grpc, http, database, runtime, sync) |
| `pprof.temporal_analysis` | Analyze Temporal SDK worker settings from goroutine profiles (pollers, cached workflows, activities) |
| `pprof.contention_analysis` | Analyze mutex/block contention by lock site |
| `pprof.cross_correlate` | Correlate hotspots across CPU/heap/mutex profiles |
| `pprof.hotspot_summary` | Top hotspots across profile types in one call |
| `pprof.diff_top` | Compare two profiles |
| `pprof.regression_check` | CI-friendly regression thresholds for function metrics |
| `pprof.suggest_fix` | Suggest concrete fixes and optional diffs for known issues (deprecated) |
| `pprof.generate_report` | Generate a markdown report from structured tool outputs |
| `pprof.vendor_analyze` | Analyze vendored/external dependencies in hot paths |
| `pprof.focus_paths` | Show all call paths to a function |
| `pprof.traces_head` | Show stack traces |
| `pprof.tags` | Filter by tags or list available tags |
| `pprof.merge` | Merge multiple profiles |
| `pprof.meta` | Extract profile metadata |

Notes:
- `pprof.peek`, `pprof.list`, `pprof.tags`, and `pprof.focus_paths` accept an optional `max_lines` argument to cap output size.
- `pprof.traces_head` accepts `max_lines` as an alias for `lines`.
- `profiles.download_latest_bundle` accepts `site` or `dd_site` (alias) for Datadog site selection.
- `pprof.top` can persist baselines with `compare_baseline=true` (defaults to `.pprof-mcp-baselines.json`, override via `baseline_path`).

### Visualization

| Tool | Description |
|------|-------------|
| `pprof.flamegraph` | Generate SVG flamegraph |
| `pprof.callgraph` | Generate call graph (DOT/SVG/PNG) |

### Service Discovery

| Tool | Description |
|------|-------------|
| `repo.services.discover` | Discover services in a repository |

See `docs/TOOLING_PROMPT.md` for detailed usage guidance and workflows.

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make vendor` | Sync vendor/ directory |
| `make test` | Run tests |
| `make build-profctl` | Build CLI to `bin/profctl` |
| `make build-server` | Build MCP server to `bin/pprof-mcp-server` |
| `make install` | Install both binaries |
| `make run-server` | Run MCP server (for development) |
| `make clean` | Remove build artifacts |

## License

This project is licensed under the GNU Affero General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
