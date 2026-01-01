# OOM/Memory Investigation

The `{{SERVICE}}` service in `{{ENV}}` experienced an OOM kill at `{{TIMESTAMP}}`. Container RSS was `{{RSS_MB}}`MB but we need to understand why.

Please:
1. Find profiles around the OOM event
2. Analyze heap usage - is the Go heap the culprit?
3. Check for RSS/heap mismatch patterns (SQLite, CGO, goroutine stacks)
4. Look at goroutine counts - are we leaking goroutines?
5. Identify what's driving memory growth and recommend fixes
