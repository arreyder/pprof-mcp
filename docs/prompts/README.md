# Profiling Prompts

Ready-to-use prompts for common profiling scenarios. Replace `{{PLACEHOLDERS}}` with your values.

## Available Prompts

| Prompt | Use Case |
|--------|----------|
| [discovery.md](discovery.md) | First look at an unknown service |
| [regression.md](regression.md) | Investigate a performance regression |
| [oom-investigation.md](oom-investigation.md) | Debug OOM kills and memory issues |
| [fix-verification.md](fix-verification.md) | Verify a performance fix worked |
| [tenant-analysis.md](tenant-analysis.md) | Analyze issues for a specific tenant |
| [temporal-analysis.md](temporal-analysis.md) | Infer Temporal SDK worker settings from profiles |

## Usage

1. Copy the prompt content
2. Replace placeholders (`{{SERVICE}}`, `{{ENV}}`, etc.)
3. Paste into Claude Code with pprof-mcp configured

Or reference directly:
```
@docs/prompts/discovery.md - analyze myservice in prod
```
