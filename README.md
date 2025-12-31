# pprof-mcp

MCP server and CLI tooling for Datadog profile download and deterministic pprof analysis.

## Requirements

- Go 1.25.5
- Datadog API keys in env:
  - `DD_API_KEY`
  - `DD_APP_KEY`
  - `DD_SITE` (optional, default `us3.datadoghq.com`)

If `github.com/conductorone/mcp-go-sdk` is private, configure git to use SSH:

```bash
git config --global url."git@github.com:".insteadOf "https://github.com/"
```

## Quick Start

```bash
make vendor
make test
make build-profctl
make build-server
```

### CLI usage

List candidates and pick a representative window:

```bash
./bin/profctl datadog profiles list --service temporal_sync --env prod-usw2 --hours 72 --limit 20 --json
./bin/profctl datadog profiles pick --service temporal_sync --env prod-usw2 --strategy most_samples --hours 72 --limit 20 --json
```

Download a specific profile bundle:

```bash
./bin/profctl download --service temporal_sync --env prod-usw2 --out ./profiles \
  --profile_id <PROFILE_ID> --event_id <EVENT_ID> --hours 72
```

Inspect metadata and storylines:

```bash
./bin/profctl pprof meta --profile ./profiles/temporal_sync_prod-usw2_cpu.pprof --json
./bin/profctl pprof storylines --profile ./profiles/temporal_sync_prod-usw2_cpu.pprof --n 4 \
  --repo_prefix gitlab.com/ductone/c1 --repo_root . --json
```

## MCP Server (Claude Desktop)

`pprof-mcp` runs over stdio. Example config:

Linux: `~/.config/Claude/claude_desktop_config.json`
macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "pprof-mcp": {
      "command": "bash",
      "args": ["-lc", "cd /home/arreyder/repos/pprof-mcp && DD_API_KEY=... DD_APP_KEY=... go run ./cmd/pprof-mcp-server"],
      "env": {
        "DD_API_KEY": "...",
        "DD_APP_KEY": "...",
        "DD_SITE": "us3.datadoghq.com"
      }
    }
  }
}
```

## MCP Tools

See `docs/TOOLING_PROMPT.md` for the full tool list and usage guidance.

## Makefile Targets

- `make vendor` - sync `vendor/`
- `make test` - run tests with vendored deps
- `make build-profctl` - build CLI to `bin/profctl`
- `make build-server` - build MCP server to `bin/pprof-mcp-server`
- `make run-server` - run the MCP server over stdio

