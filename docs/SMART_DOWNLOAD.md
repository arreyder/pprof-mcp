# Smart Profile Download

The `profiles.download` tool automatically detects your environment and uses the appropriate download method.

## How It Works

```
profiles.download
       │
       ├─ Check D2 environment variable
       │
       ├─ D2=true? ──► Use d2.profiles.download
       │                └─ kubectl + port-forward
       │
       └─ Otherwise ──► Use profiles.download_latest_bundle
                         └─ Datadog API
```

## Environment Detection

The tool checks for `d2=true` environment variable (lowercase) to determine the mode:

```bash
# Set in d2 environment
export d2=true

# Now profiles.download will use local kubectl
profiles.download service=be-innkeeper out_dir=./profiles
```

**Note**: Both `d2` (lowercase) and `D2` (uppercase) are supported for flexibility.

## Usage Examples

### D2 Local Development

```bash
# Ensure d2=true is set
export d2=true

# Download profiles (auto-detects d2 mode)
profiles.download service=be-innkeeper out_dir=./profiles

# Optional: Specify CPU profile duration
profiles.download service=pub-api out_dir=./profiles seconds=60
```

### Production/Staging

```bash
# Ensure d2 is not set or false
unset d2

# Download profiles (auto-detects Datadog mode)
profiles.download service=be-innkeeper env=prod out_dir=./profiles

# With additional Datadog parameters
profiles.download service=be-innkeeper env=prod out_dir=./profiles hours=24 site=us3.datadoghog.com
```

## Parameters

### Common (Both Modes)
- `service` (required) - Service name
- `out_dir` (required) - Output directory

### D2 Mode Only
- `seconds` (optional, default: 30) - CPU profile duration

### Datadog Mode Only
- `env` (required) - Environment (prod, staging)
- `hours` (optional, default: 72) - Hours to look back
- `dd_site` / `site` (optional) - Datadog site
- `profile_id` (optional) - Specific profile ID
- `event_id` (optional) - Specific event ID (required if profile_id set)

## Output

The tool returns a unified output format:

```json
{
  "command": "kubectl port-forward ... (d2 mode)",
  "mode": "d2",
  "result": {
    "service": "be-innkeeper",
    "namespace": "default",
    "pod_name": "be-innkeeper-d59886fd6-phwnd",
    "files": [
      {
        "type": "cpu",
        "handle": "handle:abc123...",
        "bytes": 245678
      },
      ...
    ]
  }
}
```

Or for Datadog mode:

```json
{
  "command": "./bin/profctl download ... (datadog mode)",
  "mode": "datadog",
  "result": {
    "service": "be-innkeeper",
    "env": "prod",
    "dd_site": "us3.datadoghq.com",
    "profile_id": "...",
    "files": [...]
  }
}
```

## Error Handling

### Missing Required Parameters

**D2 Mode**: Only needs `service` and `out_dir`

**Datadog Mode**: Requires `env` parameter when not in d2:

```
Error: env parameter required for Datadog mode (not in d2 environment)
```

To fix: Either set `D2=true` or provide `env` parameter.

## When to Use Explicit Tools

Most of the time, use `profiles.download` (smart wrapper). Use explicit tools when:

1. **`d2.profiles.download`**: You want to force d2 mode even without d2=true
2. **`profiles.download_latest_bundle`**: You want to force Datadog mode even with d2=true

## Setting d2 Environment Variable

### Temporarily (Current Session)
```bash
export d2=true
```

### Permanently (Shell Config)
```bash
# Add to ~/.bashrc or ~/.zshrc
echo 'export d2=true' >> ~/.bashrc
source ~/.bashrc
```

### In Docker/Kubernetes
```yaml
env:
  - name: d2
    value: "true"
```

### Claude Code Configuration

Add to your Claude Code settings to set d2 for all MCP server invocations:

```json
{
  "mcpServers": {
    "pprof-mcp": {
      "command": "/path/to/pprof-mcp-server",
      "env": {
        "d2": "true",
        "DD_API_KEY": "your-api-key",
        "DD_APP_KEY": "your-app-key"
      }
    }
  }
}
```

## Benefits

1. **Unified Interface**: One tool for all environments
2. **Less Mental Overhead**: Don't think about which tool to use
3. **Consistent Workflow**: Same command works locally and in production
4. **Future-Proof**: New download methods can be added transparently

## Complete Workflow Example

```bash
# Set environment
export d2=true

# Download baseline
profiles.download service=be-innkeeper out_dir=./profiles
# Returns handle:baseline123

# Analyze
pprof.top profile=handle:baseline123

# Make changes in ../c1/
# (Tilt auto-rebuilds)

# Download again
profiles.download service=be-innkeeper out_dir=./profiles
# Returns handle:after456

# Compare
pprof.diff_top before=handle:baseline123 after=handle:after456
```

## Troubleshooting

### "d2 download failed: No pods found"
- d2=true is set but service not running
- Check: `kubectl get pods -n default`
- Fix: Start service in Tilt

### "datadog download failed: 401 Unauthorized"
- d2 is not set and Datadog credentials missing
- Check: DD_API_KEY and DD_APP_KEY environment variables
- Fix: Set credentials or set d2=true for local mode

### "env parameter required for Datadog mode"
- d2 is not set (or false) but `env` parameter not provided
- Fix: Either `export d2=true` or add `env=prod` parameter
