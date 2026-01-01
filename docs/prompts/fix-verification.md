# Performance Fix Verification

We deployed a fix for `{{FUNCTION}}` in `{{SERVICE}}` (`{{ENV}}`). The fix was deployed around `{{DEPLOY_TIME}}`.

Please:
1. Track the function's CPU usage over the last 72 hours
2. Compare profiles from before and after the deploy
3. Verify the function's resource usage decreased
4. Check for any unintended regressions in other areas
5. Quantify the improvement (% reduction in CPU/memory)
6. Provide an optional `pprof.regression_check` setup to prevent recurrence
