# Performance Regression Investigation

We noticed a performance regression in `{{SERVICE}}` in `{{ENV}}` around `{{TIMESTAMP}}`.

Please:
1. Find profiles from before and after the regression
2. Compare CPU usage to identify what changed
3. Compare heap allocations to see if memory behavior changed
4. Identify the specific functions that regressed
5. Suggest what code changes might have caused this
6. If possible, add a `pprof.regression_check` guard for the top regressed functions
