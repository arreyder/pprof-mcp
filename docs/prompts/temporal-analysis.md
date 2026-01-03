# Temporal Worker Analysis

I need to understand the Temporal SDK worker configuration for `{{SERVICE}}` in `{{ENV}}`. I want to infer the actual runtime settings from a goroutine profile:

1. How many activity task pollers are configured? (default is 2)
2. How many workflow task pollers are configured? (default is 2)
3. How many workflows are currently cached in the sticky cache?
4. How many activities are actively executing?
5. Are there any local activities or sessions?

Please download the profiles and use `pprof.temporal_analysis` on the goroutine profile. Compare the inferred settings against these Temporal SDK defaults:

| Setting | Default |
|---------|---------|
| MaxConcurrentActivityTaskPollers | 2 |
| MaxConcurrentWorkflowTaskPollers | 2 |
| MaxConcurrentActivityExecutionSize | 1000 |
| MaxConcurrentWorkflowTaskExecutionSize | 1000 |
| MaxConcurrentLocalActivityExecutionSize | 1000 |
| WorkflowCacheSize (sticky cache) | 10000 |

Also use `pprof.goroutine_categorize` with preset `temporal` to get a breakdown of all Temporal-related goroutines. If there's high goroutine count, this helps identify which subsystem is responsible.

If you have access to the timestamp of the profile, also query `datadog.metrics_at_timestamp` to correlate with runtime metrics like goroutine count and memory usage.
