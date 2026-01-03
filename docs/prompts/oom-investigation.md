# OOM/Memory Investigation

The `{{SERVICE}}` service in `{{ENV}}` experienced an OOM kill at `{{TIMESTAMP}}`. Container RSS was `{{RSS_MB}}`MB but we need to understand why.

Please:
1. Find profiles around the OOM event using `datadog.profiles.near_event`
2. Analyze heap usage - is the Go heap the culprit?
3. Check for RSS/heap mismatch patterns (SQLite temp_store, CGO, goroutine stacks) using `pprof.memory_sanity`
4. Look at goroutine counts - are we leaking goroutines?
5. Identify what's driving memory growth and recommend fixes

## Checking for OOM Kill Events

OOMKilled events aren't in application logs - check kubelet/kubernetes sources:

```
# Search kubelet logs for container crashes
source:kubelet {{SERVICE}} (ContainerDied OR CrashLoopBackOff)

# Search for OOM signals (may not always be captured)
source:kubernetes (OOMKilled OR "exit code 137") {{SERVICE}}
```

Look for patterns like:
- `CrashLoopBackOff` with increasing back-off times
- `ContainerDied` events followed by `ContainerStarted`
- Multiple pods affected around the same time (indicates systemic issue vs single pod)
