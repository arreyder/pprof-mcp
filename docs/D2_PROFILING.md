# D2 Local Development Profiling

This document explains how to profile services running in your local d2 development environment.

## Quick Start

**Recommended**: Use the smart wrapper that auto-detects your environment:

```bash
# In d2 (with d2=true environment variable)
export d2=true
profiles.download service=be-innkeeper out_dir=./profiles

# In production/staging
profiles.download service=be-innkeeper env=prod out_dir=./profiles
```

The `profiles.download` tool automatically detects if you're in d2 (checks `d2=true` env var) and uses the appropriate backend.

## Overview

The d2 profiling tool (`d2.profiles.download` or via smart wrapper `profiles.download`) allows you to download pprof profiles from services running in your local Kubernetes cluster (deployed by Tilt) without needing Datadog credentials or production access.

## How It Works

1. **Pod Discovery**: Uses `kubectl` to find the service pod by its `app` label
2. **Port Forwarding**: Automatically sets up `kubectl port-forward` to port 1337 (debug server)
3. **Token Auth**: Retrieves the authentication token from the pod's debug server
4. **Profile Download**: Downloads CPU, heap, mutex, block, goroutine, and allocs profiles
5. **Handle Registration**: Registers profiles with the same handle system as Datadog downloads

All existing `pprof.*` analysis tools work seamlessly with d2-downloaded profiles.

## Requirements

- **kubectl**: Must have access to your local cluster
- **Context**: Default kubectl context pointing to d2 cluster
- **Tilt**: Services must be running (deployed by Tilt)
- **Debug Server**: Services must have debug server enabled on port 1337

## Usage

### Basic Usage

```bash
# Download profiles from be-innkeeper
d2.profiles.download service=be-innkeeper out_dir=./profiles

# Download with custom CPU profile duration
d2.profiles.download service=pub-api out_dir=./profiles seconds=60
```

### Via MCP (Claude Code)

```
Download profiles from be-innkeeper service in d2 to ./profiles directory
```

The tool will:
1. Find the pod running be-innkeeper
2. Set up port-forward
3. Get auth token
4. Download all profile types
5. Return handles for analysis

### Example Output

```json
{
  "command": "kubectl port-forward -n default be-innkeeper-d59886fd6-phwnd 1337:1337",
  "result": {
    "service": "be-innkeeper",
    "namespace": "default",
    "pod_name": "be-innkeeper-d59886fd6-phwnd",
    "pod_ip": "10.1.45.23",
    "files": [
      {
        "type": "cpu",
        "handle": "handle:abc123...",
        "bytes": 245678
      },
      {
        "type": "heap",
        "handle": "handle:def456...",
        "bytes": 123456
      },
      ...
    ]
  }
}
```

## Analyzing Profiles

Once downloaded, use any `pprof.*` tool with the returned handles:

```
# Show top CPU consumers
pprof.top profile=handle:abc123 cum=true nodecount=20

# Find allocation hot paths
pprof.alloc_paths profile=handle:def456

# Compare before/after optimization
pprof.diff_top before=handle:abc123 after=handle:xyz789
```

## Workflow Example: Profile → Change → Profile

This workflow is ideal for local development optimization:

1. **Baseline Profile**
```
d2.profiles.download service=be-innkeeper out_dir=./profiles
# Returns handle:baseline123
```

2. **Analyze**
```
pprof.top profile=handle:baseline123
pprof.storylines profile=handle:baseline123 repo_prefix=gitlab.com/ductone/c1
```

3. **Make Code Changes**
- Edit code in ../c1/
- Tilt automatically rebuilds and redeploys

4. **Profile Again**
```
d2.profiles.download service=be-innkeeper out_dir=./profiles
# Returns handle:after456
```

5. **Compare**
```
pprof.diff_top before=handle:baseline123 after=handle:after456
```

## Supported Services

All services deployed by Tilt with debug server enabled:

- `be-conductor`
- `be-db-stream`
- `be-innkeeper`
- `be-notification`
- `be-ratelimit`
- `be-session`
- `be-support-dashboard`
- `be-temporal-webhooks`
- `be-temporal-sync`
- `be-temporal-worker`
- `be-vault`
- `pub-accounts`
- `pub-api`
- `pub-auth`
- `pub-connector-api`
- `pub-notify`
- `pub-websocket`
- `pub-slack`
- `pg-analyze`

## Profile Types

The tool downloads the following profile types:

| Type | Description |
|------|-------------|
| `cpu` | CPU profile (default 30s, configurable) |
| `heap` | Memory allocations and in-use memory |
| `goroutine` | Goroutine stack traces |
| `mutex` | Mutex contention profile |
| `block` | Blocking profile |
| `allocs` | All memory allocations (not just in-use) |

## Troubleshooting

### "No pods found for service"
- Check if service is running: `kubectl get pods -n default`
- Verify service name matches pod label: `kubectl get pods -l app=be-innkeeper`

### "Failed to get token"
- Ensure debug server is running on port 1337
- Check pod logs for errors
- Verify TLS certificates are valid

### "Port-forward failed"
- Check kubectl context: `kubectl config current-context`
- Ensure you have permissions to port-forward
- Try manually: `kubectl port-forward -n default <pod-name> 1337:1337`

### "Timeout waiting for port-forward"
- Pod may be restarting
- Debug server may not be listening
- Network issues

## Architecture

```
┌─────────────────────┐
│  MCP Client         │
│  (Claude Code)      │
└──────────┬──────────┘
           │
           │ d2.profiles.download
           ▼
┌─────────────────────┐
│  pprof-mcp-server   │
│  internal/d2/       │
└──────────┬──────────┘
           │
           ├─► kubectl get pods (find pod)
           │
           ├─► kubectl port-forward (setup tunnel)
           │
           ├─► PUT /debug/token (get auth token)
           │
           └─► GET /debug/pprof/* (download profiles)
                   │
                   ▼
           ┌─────────────────────┐
           │  Pod in Namespace   │
           │  :1337 debug server │
           │  (upprof + udebug)  │
           └─────────────────────┘
```

## Implementation Details

- **Package**: `internal/d2/`
- **Files**:
  - `kubectl.go` - Pod discovery and port-forwarding
  - `token.go` - Token retrieval
  - `download.go` - Profile download orchestration
- **Tool Handler**: `cmd/pprof-mcp-server/main.go:d2DownloadTool`

## Future Enhancements

Potential improvements:
1. List available services: `d2.profiles.list_services`
2. Multi-pod profiling for load-balanced services
3. Continuous profiling mode (profile every N seconds)
4. Direct comparison with production (d2 vs prod)
5. Integration with Tilt for automatic profiling on rebuild
