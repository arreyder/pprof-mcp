# d2.profile_branch_impact

Compare profiles between git branches to measure the performance impact of code changes in your local d2 development environment.

## Two-Phase Workflow (Recommended)

For long-running operations, use the two-phase workflow to review the plan before execution:

1. **Create Plan**: `d2.profile_branch_impact.plan` - Generate execution plan
2. **Review**: Examine steps and estimated time
3. **Execute**: `d2.profile_branch_impact.execute` - Run the plan (walk away!)

### Why Two-Phase?

- ✅ **Review before execution** - See exactly what will happen
- ✅ **Walk away** - Approve once, come back to results (~7+ minutes)
- ✅ **Time estimates** - Know how long it will take
- ✅ **No surprises** - Clear about git stashing and branch switching

## Overview

This tool automates the workflow of profiling a service before and after a code change:

1. ✅ Captures baseline profile from a reference branch (default: `main`)
2. ✅ Handles uncommitted changes automatically (stash/restore)
3. ✅ Switches to comparison branch (default: current branch)
4. ✅ Waits for Tilt to rebuild (detects live updates or pod restarts)
5. ✅ Captures post-change profile
6. ✅ Returns profile handles for both snapshots
7. ✅ Restores original branch and uncommitted changes

## Quick Start

### Two-Phase Workflow (Recommended for Long Operations)

**Step 1: Create the plan**

```typescript
// Natural language request:
"Create a plan to compare profiles between main and my-branch for ratelimit,
using 60 second samples with 20 second warmup"

// This calls:
mcp__pprof__d2_profile_branch_impact_plan({
  service: "ratelimit",
  out_dir: "/tmp/profile-comparison",
  before_ref: "main",
  after_ref: "my-branch",
  seconds: 60,
  warmup_delay: 20
})

// Returns:
{
  id: "a1b2c3d4e5f6g7h8",
  steps: [
    "Switch to main branch",
    "Wait for Tilt rebuild (timeout: 5m0s)",
    "Wait 20s for service warmup",
    "Profile ratelimit service for 60 seconds",
    "Switch to my-branch branch",
    "Wait for Tilt rebuild (timeout: 5m0s)",
    "Wait 20s for service warmup",
    "Profile ratelimit service for 60 seconds",
    "Compare profiles",
    "Switch back to main branch"
  ],
  estimated_time: "~12 minutes",
  current_branch: "main",
  has_uncommitted: false
}
```

**Step 2: Review and approve**

You'll be prompted: "Execute this plan?" → Approve once

**Step 3: Execute (walk away!)**

```typescript
// This runs automatically after approval:
mcp__pprof__d2_profile_branch_impact_execute({
  plan_id: "a1b2c3d4e5f6g7h8"
})

// Takes ~12 minutes, returns results when done
```

### One-Phase Usage (Quick)

For quick comparisons without review:

```typescript
mcp__pprof__d2_profile_branch_impact({
  service: "ratelimit",
  out_dir: "/tmp/profile-comparison"
})
```

This will:
- Profile `be-ratelimit` service on `main` branch
- Switch to your current branch
- Wait for Tilt rebuild
- Profile again
- Return handles for both profiles

### Custom Branches

Compare specific branches:

```typescript
mcp__pprof__d2_profile_branch_impact({
  service: "ratelimit",
  out_dir: "/tmp/profile-comparison",
  before_ref: "production",
  after_ref: "feature/cache-optimization"
})
```

### Full Options

```typescript
mcp__pprof__d2_profile_branch_impact({
  service: "ratelimit",           // Service to profile (fuzzy matching supported)
  out_dir: "/tmp/profiles",       // Output directory
  before_ref: "main",             // Baseline git ref (default: main)
  after_ref: "my-branch",         // Comparison git ref (default: current)
  seconds: 30,                    // CPU profile duration (default: 30)
  rebuild_timeout: 300,           // Max wait for rebuild in seconds (default: 300)
  warmup_delay: 15                // Warmup delay after rebuild in seconds (default: 15)
})
```

## How It Works

### Git Handling

**Uncommitted Changes:**
- Automatically detected with `git status --porcelain`
- Stashed with timestamped message: `d2_profile_branch_impact auto-stash <timestamp>`
- Restored after profiling completes

**Branch Switching:**
- Original branch is captured at start
- Switches to `before_ref` for baseline
- Switches to `after_ref` for comparison
- Returns to original branch on completion (even if errors occur)

### Tilt Rebuild Detection

The tool monitors Tilt's API to detect when rebuilds complete:

**Detection Methods:**

1. **Live Update (file sync)**
   - Monitors `LiveUpdate` resource
   - Detects when `lastFileTimeSynced` changes
   - Fastest rebuild method

2. **Container Restart**
   - Monitors `KubernetesDiscovery` resource
   - Detects when `startedAt` timestamp changes
   - Pod name stays the same

3. **Pod Recreate**
   - Monitors pod name in `KubernetesDiscovery`
   - Detects when pod name changes
   - Full rebuild with new pod

**Polling Strategy:**
- Initial 5-second delay (let Tilt detect change)
- Poll every 3 seconds for state changes
- Configurable timeout (default: 5 minutes)
- Warmup delay after detection (default: 15 seconds)

### Profile Collection

Uses the same `d2.profiles.download` mechanism:
- Discovers pod via kubectl (fuzzy service name matching)
- Port-forwards to debug server (port 4421)
- Downloads: CPU, heap, goroutines, mutex, block, allocs profiles
- Registers profile handles for analysis

## Output

Returns a structured result with:

```typescript
{
  service: "ratelimit",
  before_ref: "main",
  after_ref: "feature/cache-optimization",
  before_profiles: {
    service: "ratelimit",
    namespace: "default",
    pod_name: "be-ratelimit-abc123",
    pod_ip: "10.1.101.123",
    files: [
      { type: "cpu", handle: "handle:xyz...", bytes: 1234 },
      { type: "heap", handle: "handle:abc...", bytes: 5678 },
      // ... more profiles
    ]
  },
  after_profiles: {
    // Same structure as before_profiles
  },
  update_method: "live_update",  // or "pod_restart" or "pod_recreate"
  git_stashed: true,
  warnings: []  // Any non-fatal issues
}
```

## Analysis Workflow

After running `d2.profile_branch_impact`, analyze the results:

### 1. CPU Comparison

```typescript
// Get CPU handles from result
const beforeCPU = result.before_profiles.files.find(f => f.type === "cpu").handle
const afterCPU = result.after_profiles.files.find(f => f.type === "cpu").handle

// Compare
mcp__pprof__pprof_diff_top({
  before: beforeCPU,
  after: afterCPU,
  nodecount: 20
})
```

### 2. Heap Comparison

```typescript
const beforeHeap = result.before_profiles.files.find(f => f.type === "heap").handle
const afterHeap = result.after_profiles.files.find(f => f.type === "heap").handle

mcp__pprof__pprof_diff_top({
  before: beforeHeap,
  after: afterHeap,
  sample_index: "alloc_space",  // Show allocations
  nodecount: 20
})
```

### 3. Deep Analysis

```typescript
// Analyze specific profile
mcp__pprof__pprof_top({
  profile: afterCPU,
  nodecount: 20
})

// View flamegraph
mcp__pprof__pprof_flamegraph({
  profile: afterCPU,
  output_path: "/tmp/flamegraph.svg"
})

// Check hotspots
mcp__pprof__pprof_storylines({
  profile: afterCPU,
  repo_prefix: "gitlab.com/ductone/c1"
})
```

## Requirements

**Environment:**
- Local d2 development cluster running
- Tilt managing service deployments
- kubectl access to cluster
- Git repository with commits

**Service Requirements:**
- Service deployed via Tilt
- Debug server enabled (port 4421)
- Service has `app=<name>` label

## Troubleshooting

### "Failed waiting for rebuild"

**Cause:** Tilt didn't detect changes or rebuild timed out

**Solutions:**
- Increase `rebuild_timeout` parameter
- Check Tilt is running: `tilt logs <service>`
- Verify Tilt detected file changes
- Check service has proper Tiltfile configuration

### "Failed to find pod"

**Cause:** Service not running or name mismatch

**Solutions:**
- Verify service is running: `kubectl get pods | grep <service>`
- Use fuzzy matching: `"ratelimit"` finds `"be-ratelimit"`
- Check service deployed in `default` namespace
- Confirm Tilt has started the service

### "Failed to stash changes"

**Cause:** Uncommitted changes can't be stashed

**Solutions:**
- Manually commit or stash changes
- Check for untracked files that might conflict
- Ensure git working directory is clean

### Stashed changes not restored

**Check warnings in output:**
```typescript
if (result.warnings.length > 0) {
  console.log("Warnings:", result.warnings)
}
```

**Manual recovery:**
```bash
# List stashes
git stash list

# Find auto-stash
git stash list | grep "d2_profile_branch_impact"

# Restore manually
git stash pop stash@{N}
```

### Wrong branch after completion

**Check warnings field** - should indicate branch restore failure

**Manual recovery:**
```bash
# Check current branch
git branch --show-current

# Switch back
git checkout <original-branch>
```

## Best Practices

### 1. Profile Production-Like Load

The profiles are only as useful as the load during capture:

```bash
# Run load test in another terminal while profiling
hey -n 10000 -c 50 http://localhost:8080/api/endpoint
```

### 2. Use Longer CPU Profiles for Better Signal

```typescript
mcp__pprof__d2_profile_branch_impact({
  service: "ratelimit",
  out_dir: "/tmp/profiles",
  seconds: 60  // Longer profile for better data
})
```

### 3. Multiple Comparisons

Compare against multiple baselines:

```typescript
// vs main
const mainComparison = await d2_profile_branch_impact({
  service: "ratelimit",
  out_dir: "/tmp/main-comparison",
  before_ref: "main"
})

// vs production
const prodComparison = await d2_profile_branch_impact({
  service: "ratelimit",
  out_dir: "/tmp/prod-comparison",
  before_ref: "production"
})
```

### 4. Commit Before Profiling

While auto-stash works, cleaner to commit:

```bash
git add .
git commit -m "WIP: cache optimization"

# Now profile
# ... profiling ...

# Amend commit if needed
git commit --amend
```

## Examples

### Example 1: Quick Performance Check

```typescript
// Profile current branch vs main
const result = await mcp__pprof__d2_profile_branch_impact({
  service: "ratelimit",
  out_dir: "/tmp/quick-check"
})

// Compare CPU
const cpuBefore = result.before_profiles.files.find(f => f.type === "cpu").handle
const cpuAfter = result.after_profiles.files.find(f => f.type === "cpu").handle

await mcp__pprof__pprof_diff_top({
  before: cpuBefore,
  after: cpuAfter,
  nodecount: 10
})
```

### Example 2: Memory Leak Investigation

```typescript
const result = await mcp__pprof__d2_profile_branch_impact({
  service: "innkeeper",
  out_dir: "/tmp/memory-investigation",
  seconds: 60  // Longer profile
})

// Compare heap allocations
const heapBefore = result.before_profiles.files.find(f => f.type === "heap").handle
const heapAfter = result.after_profiles.files.find(f => f.type === "heap").handle

await mcp__pprof__pprof_diff_top({
  before: heapBefore,
  after: heapAfter,
  sample_index: "alloc_space",
  nodecount: 20
})

// Check for allocation paths
await mcp__pprof__pprof_alloc_paths({
  profile: heapAfter,
  min_percent: 1.0
})
```

### Example 3: Validate Optimization PR

```typescript
// Profile optimization branch
const result = await mcp__pprof__d2_profile_branch_impact({
  service: "ratelimit",
  out_dir: "/tmp/optimization-validation",
  before_ref: "main",
  after_ref: "feature/cache-getlimits",
  seconds: 60
})

console.log(`Update method: ${result.update_method}`)

// Compare all profile types
for (const profileType of ["cpu", "heap", "mutex"]) {
  const before = result.before_profiles.files.find(f => f.type === profileType).handle
  const after = result.after_profiles.files.find(f => f.type === profileType).handle

  console.log(`\n=== ${profileType.toUpperCase()} Comparison ===`)
  await mcp__pprof__pprof_diff_top({
    before: before,
    after: after,
    nodecount: 15
  })
}
```

## See Also

- [d2.profiles.download](./d2-profiles-download.md) - Individual profile downloads
- [pprof.diff_top](./pprof-diff-top.md) - Compare two profiles
- [pprof.top](./pprof-top.md) - Analyze single profile
- [Tilt Documentation](https://docs.tilt.dev/) - Tilt live update modes
