package main

func profileCandidateSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"profile_id":     prop("string", "Datadog profile ID"),
		"event_id":       prop("string", "Datadog event ID"),
		"timestamp":      prop("string", "Profile timestamp (RFC3339)"),
		"numeric_fields": numericFieldsSchema(),
	}, "profile_id", "event_id", "timestamp")
}

func numericFieldsSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "Numeric fields returned by Datadog for the profile",
		"additionalProperties": map[string]any{"type": "number"},
	}
}

func profileFileSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"type":   prop("string", "Profile type (cpu, heap, mutex, block, goroutines)"),
		"handle": prop("string", "Handle ID for the downloaded profile (use in pprof.* tools)"),
		"bytes":  prop("integer", "File size in bytes"),
	}, "type", "handle", "bytes")
}

func datadogProfilesListOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "CLI command equivalent"),
		"result": NewObjectSchema(map[string]any{
			"service":    prop("string", "Service name"),
			"env":        prop("string", "Environment"),
			"dd_site":    prop("string", "Datadog site"),
			"from_ts":    prop("string", "Resolved start time"),
			"to_ts":      prop("string", "Resolved end time"),
			"limit":      prop("integer", "Result limit"),
			"candidates": arrayPropSchema(profileCandidateSchema(), "Profile candidates"),
			"warnings":   arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "service", "env", "dd_site", "from_ts", "to_ts", "limit", "candidates"),
	}, "command", "result")
}

func datadogProfilesPickOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "CLI command equivalent"),
		"result": NewObjectSchema(map[string]any{
			"candidate": profileCandidateSchema(),
			"reason":    prop("string", "Selection reason"),
			"warnings":  arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "candidate", "reason"),
	}, "command", "result")
}

func profilesDownloadAutoOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "Command executed"),
		"mode":    prop("string", "Download mode used (d2 or datadog)"),
		"result": map[string]any{
			"type":                 "object",
			"description":          "Download result (schema varies by mode)",
			"additionalProperties": true,
		},
	}, "command", "mode", "result")
}

func downloadLatestBundleOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "CLI command equivalent"),
		"result": NewObjectSchema(map[string]any{
			"service":      prop("string", "Service name"),
			"env":          prop("string", "Environment"),
			"dd_site":      prop("string", "Datadog site"),
			"from_ts":      prop("string", "Resolved start time"),
			"to_ts":        prop("string", "Resolved end time"),
			"profile_id":   prop("string", "Profile ID"),
			"event_id":     prop("string", "Event ID"),
			"timestamp":    prop("string", "Profile timestamp"),
			"files":        arrayPropSchema(profileFileSchema(), "Downloaded profiles"),
			"metrics_path": prop("string", "Path to metrics file"),
			"warnings":     arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "service", "env", "dd_site", "from_ts", "to_ts", "profile_id", "event_id", "files"),
	}, "command", "result")
}

func d2DownloadOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "kubectl commands executed"),
		"result": NewObjectSchema(map[string]any{
			"service":   prop("string", "Service name"),
			"namespace": prop("string", "Kubernetes namespace"),
			"pod_name":  prop("string", "Pod name"),
			"pod_ip":    prop("string", "Pod IP address"),
			"files":     arrayPropSchema(profileFileSchema(), "Downloaded profiles"),
			"warnings":  arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "service", "namespace", "pod_name", "files"),
	}, "command", "result")
}

func pprofTopOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command":  prop("string", "pprof command"),
		"raw":      prop("string", "Raw pprof output"),
		"rows":     arrayPropSchema(pprofTopRowSchema(), "Top rows"),
		"summary":  pprofTopSummarySchema(),
		"hints":    arrayPropSchema(prop("string", "Hint"), "Contextual hints based on profile type"),
		"baseline": baselineComparisonSchema(),
	}, "command", "raw", "rows", "summary")
}

func baselineComparisonSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"key":              prop("string", "Baseline key"),
		"profile_kind":     prop("string", "Detected profile kind"),
		"sample_index":     prop("string", "Sample index used for baseline"),
		"baseline_samples": prop("integer", "Number of profiles in baseline"),
		"deviations": arrayPropSchema(NewObjectSchema(map[string]any{
			"function": prop("string", "Function name"),
			"metric":   prop("string", "Metric compared"),
			"current":  prop("number", "Current value"),
			"baseline": prop("number", "Baseline average"),
			"delta":    prop("number", "Difference from baseline"),
			"severity": prop("string", "Severity"),
		}, "function", "metric", "current", "baseline", "delta", "severity"), "Baseline deviations"),
		"warnings": arrayPropSchema(prop("string", "Warning"), "Warnings"),
	}, "key", "profile_kind", "baseline_samples", "deviations")
}

func pprofTopRowSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"flat":         prop("string", "Flat value"),
		"flat_pct":     prop("string", "Flat percent"),
		"sum_pct":      prop("string", "Cumulative sum percent"),
		"cum":          prop("string", "Cumulative value"),
		"cum_pct":      prop("string", "Cumulative percent"),
		"name":         prop("string", "Function name"),
		"flat_seconds": prop("number", "Flat time in seconds"),
		"cum_seconds":  prop("number", "Cumulative time in seconds"),
	}, "flat", "flat_pct", "sum_pct", "cum", "cum_pct", "name")
}

func pprofTopSummarySchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"header_lines": arrayPropSchema(prop("string", "Header line"), "Header lines"),
		"table_header": prop("string", "Table header"),
	}, "header_lines", "table_header")
}

func pprofGoroutineAnalysisOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchema(map[string]any{
			"total_goroutines": prop("integer", "Total goroutine count"),
			"by_state": map[string]any{
				"type":                 "object",
				"description":          "Goroutine counts by state",
				"additionalProperties": map[string]any{"type": "integer"},
			},
			"top_wait_reasons": arrayPropSchema(NewObjectSchema(map[string]any{
				"reason":       prop("string", "Wait reason"),
				"count":        prop("integer", "Goroutine count"),
				"sample_stack": prop("string", "Sample stack signature"),
			}, "reason", "count"), "Top wait reasons"),
			"potential_leaks": arrayPropSchema(NewObjectSchema(map[string]any{
				"stack_signature": prop("string", "Stack signature"),
				"count":           prop("integer", "Goroutine count"),
				"severity":        prop("string", "Severity"),
				"state":           prop("string", "Goroutine state"),
				"wait_reason":     prop("string", "Wait reason"),
			}, "stack_signature", "count", "severity"), "Potential leaks"),
			"warnings": arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "total_goroutines", "by_state"),
	}, "command", "result")
}

func pprofDiscoverOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchemaWithAdditional(map[string]any{
			"service":   prop("string", "Service name"),
			"env":       prop("string", "Environment"),
			"timestamp": prop("string", "Profile timestamp"),
			"profiles": arrayPropSchema(NewObjectSchema(map[string]any{
				"type":   prop("string", "Profile type"),
				"handle": prop("string", "Profile handle"),
				"bytes":  prop("integer", "File size in bytes"),
			}, "type", "handle"), "Downloaded profile handles"),
			"recommendations": arrayPropSchema(NewObjectSchema(map[string]any{
				"priority":   prop("string", "Recommendation priority"),
				"area":       prop("string", "Area of concern"),
				"suggestion": prop("string", "Suggested action"),
			}, "priority", "area", "suggestion"), "Prioritized recommendations"),
			"warnings": arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, true, "service", "env"),
	}, "command", "result")
}

func pprofGenerateReportOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchema(map[string]any{
			"markdown": prop("string", "Markdown report"),
		}, "markdown"),
	}, "command", "result")
}

func functionHistoryOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "CLI command equivalent"),
		"result": NewObjectSchema(map[string]any{
			"service":  prop("string", "Service name"),
			"env":      prop("string", "Environment"),
			"function": prop("string", "Function pattern"),
			"from_ts":  prop("string", "Resolved start time"),
			"to_ts":    prop("string", "Resolved end time"),
			"entries":  arrayPropSchema(functionHistoryEntrySchema(), "Function history entries"),
			"summary":  functionHistorySummarySchema(),
			"warnings": arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "service", "env", "function", "from_ts", "to_ts", "entries", "summary"),
		"table": prop("string", "Formatted table"),
	}, "command", "result", "table")
}

func functionHistoryEntrySchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"timestamp":    prop("string", "Profile timestamp (RFC3339)"),
		"profile_id":   prop("string", "Profile ID"),
		"event_id":     prop("string", "Event ID"),
		"flat_percent": prop("number", "Flat percent"),
		"cum_percent":  prop("number", "Cumulative percent"),
		"flat_value":   prop("string", "Flat value"),
		"cum_value":    prop("string", "Cumulative value"),
		"found":        prop("boolean", "Whether the function was found"),
	}, "timestamp", "profile_id", "event_id", "flat_percent", "cum_percent", "flat_value", "cum_value", "found")
}

func functionHistorySummarySchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"total_profiles":    prop("integer", "Total profiles searched"),
		"found_in_profiles": prop("integer", "Profiles where function was found"),
		"max_flat_percent":  prop("number", "Max flat percent"),
		"min_flat_percent":  prop("number", "Min flat percent"),
		"avg_flat_percent":  prop("number", "Average flat percent"),
	}, "total_profiles", "found_in_profiles", "max_flat_percent", "min_flat_percent", "avg_flat_percent")
}

func compareRangeOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command":   prop("string", "CLI command equivalent"),
		"result":    compareRangeResultSchema(),
		"formatted": prop("string", "Formatted comparison output"),
	}, "command", "result", "formatted")
}

func compareRangeResultSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"service":        prop("string", "Service name"),
		"env":            prop("string", "Environment"),
		"before_profile": profileSummarySchema(),
		"after_profile":  profileSummarySchema(),
		"diff":           prop("string", "Raw diff output"),
		"top_changes":    arrayPropSchema(functionDiffSchema(), "Top changes"),
		"warnings":       arrayPropSchema(prop("string", "Warning"), "Warnings"),
	}, "service", "env", "before_profile", "after_profile", "diff", "top_changes")
}

func profileSummarySchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"timestamp":  prop("string", "Profile timestamp (RFC3339)"),
		"profile_id": prop("string", "Profile ID"),
		"file_path":  prop("string", "Profile file path"),
	}, "timestamp", "profile_id", "file_path")
}

func functionDiffSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"function":    prop("string", "Function name"),
		"before_flat": prop("string", "Before flat value"),
		"after_flat":  prop("string", "After flat value"),
		"change":      prop("string", "Change summary"),
		"severity":    prop("string", "Severity"),
	}, "function", "before_flat", "after_flat", "change", "severity")
}

func pprofContentionAnalysisOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchema(map[string]any{
			"profile_type":      prop("string", "Profile type (mutex or block)"),
			"total_contentions": prop("integer", "Total contention count"),
			"total_delay":       prop("string", "Total delay across contentions"),
			"by_lock_site": arrayPropSchema(NewObjectSchema(map[string]any{
				"lock_site":       prop("string", "Lock function"),
				"source_location": prop("string", "Source location for lock site"),
				"contentions":     prop("integer", "Contention count"),
				"total_delay":     prop("string", "Total delay"),
				"avg_delay":       prop("string", "Average delay"),
				"top_waiters": arrayPropSchema(NewObjectSchema(map[string]any{
					"function": prop("string", "Waiting function"),
					"delay":    prop("string", "Total delay"),
				}, "function", "delay"), "Top waiting functions"),
			}, "lock_site", "contentions", "total_delay", "avg_delay", "top_waiters"), "Contention by lock site"),
			"patterns": arrayPropSchema(NewObjectSchema(map[string]any{
				"type":        prop("string", "Pattern type"),
				"severity":    prop("string", "Severity"),
				"description": prop("string", "Description"),
			}, "type", "severity", "description"), "Detected patterns"),
			"recommendations": arrayPropSchema(prop("string", "Recommendation"), "Recommendations"),
			"warnings":        arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "profile_type", "total_contentions", "total_delay", "by_lock_site", "patterns", "recommendations"),
	}, "command", "result")
}

func pprofCrossCorrelateOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchema(map[string]any{
			"correlations": arrayPropSchema(NewObjectSchema(map[string]any{
				"function":       prop("string", "Function name"),
				"combined_score": prop("number", "Combined score"),
				"cpu": NewObjectSchema(map[string]any{
					"flat_pct": prop("number", "CPU flat percent"),
					"rank":     prop("integer", "Rank in CPU top list"),
				}, "flat_pct", "rank"),
				"heap": NewObjectSchema(map[string]any{
					"alloc_pct": prop("number", "Allocation percent"),
					"rank":      prop("integer", "Rank in heap top list"),
				}, "alloc_pct", "rank"),
				"mutex": NewObjectSchema(map[string]any{
					"delay_pct": prop("number", "Contention delay percent"),
					"rank":      prop("integer", "Rank in mutex top list"),
				}, "delay_pct", "rank"),
				"insight": prop("string", "Insight summary"),
			}, "function", "combined_score", "insight"), "Cross-profile correlations"),
			"cpu_only_hotspots": arrayPropSchema(NewObjectSchema(map[string]any{
				"function": prop("string", "Function name"),
				"flat_pct": prop("number", "CPU flat percent"),
			}, "function", "flat_pct"), "CPU-only hotspots"),
			"heap_only_hotspots": arrayPropSchema(NewObjectSchema(map[string]any{
				"function":  prop("string", "Function name"),
				"alloc_pct": prop("number", "Heap allocation percent"),
			}, "function", "alloc_pct"), "Heap-only hotspots"),
			"mutex_only_hotspots": arrayPropSchema(NewObjectSchema(map[string]any{
				"function":  prop("string", "Function name"),
				"delay_pct": prop("number", "Mutex delay percent"),
			}, "function", "delay_pct"), "Mutex-only hotspots"),
			"warnings": arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "correlations", "cpu_only_hotspots", "heap_only_hotspots", "mutex_only_hotspots"),
	}, "command", "result")
}

func pprofHotspotSummaryOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchema(map[string]any{
			"cpu_top5": arrayPropSchema(NewObjectSchema(map[string]any{
				"function": prop("string", "Function name"),
				"flat_pct": prop("number", "CPU flat percent"),
			}, "function", "flat_pct"), "Top CPU hotspots"),
			"heap_top5": arrayPropSchema(NewObjectSchema(map[string]any{
				"function":  prop("string", "Function name"),
				"alloc_pct": prop("number", "Heap allocation percent"),
			}, "function", "alloc_pct"), "Top heap hotspots"),
			"mutex_top5": arrayPropSchema(NewObjectSchema(map[string]any{
				"function":  prop("string", "Function name"),
				"delay_pct": prop("number", "Mutex delay percent"),
			}, "function", "delay_pct"), "Top mutex hotspots"),
			"goroutine_count": prop("integer", "Total goroutines"),
			"warnings":        arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "cpu_top5", "heap_top5"),
	}, "command", "result")
}

func pprofRegressionCheckOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchema(map[string]any{
			"passed": prop("boolean", "Overall pass/fail"),
			"checks": arrayPropSchema(NewObjectSchema(map[string]any{
				"function":  prop("string", "Function pattern"),
				"metric":    prop("string", "Metric checked"),
				"threshold": prop("number", "Threshold value"),
				"actual":    prop("number", "Actual value"),
				"passed":    prop("boolean", "Whether check passed"),
				"message":   prop("string", "Failure message"),
			}, "function", "metric", "threshold", "actual", "passed"), "Check results"),
		}, "passed", "checks"),
	}, "command", "result")
}

func pprofTraceSourceOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchema(map[string]any{
			"call_chain": arrayPropSchema(NewObjectSchema(map[string]any{
				"function":       prop("string", "Function name"),
				"file":           prop("string", "Source file path"),
				"line":           prop("integer", "Line number"),
				"flat_pct":       prop("number", "Flat percent"),
				"cum_pct":        prop("number", "Cumulative percent"),
				"source_snippet": prop("string", "Annotated source snippet"),
				"is_vendor":      prop("boolean", "Whether frame is vendor code"),
				"vendor_package": prop("string", "Vendor module path"),
				"vendor_version": prop("string", "Vendor module version"),
				"source_error":   prop("string", "Source resolution error"),
			}, "function", "file", "line", "flat_pct", "cum_pct", "source_snippet", "is_vendor"), "Call chain frames"),
			"total_functions_traced": prop("integer", "Total functions traced"),
			"app_functions":          prop("integer", "Functions in app code"),
			"vendor_functions":       prop("integer", "Functions in vendor code"),
			"warnings":               arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "call_chain", "total_functions_traced", "app_functions", "vendor_functions"),
	}, "command", "result")
}

func pprofVendorAnalyzeOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchema(map[string]any{
			"vendor_hotspots": arrayPropSchema(NewObjectSchema(map[string]any{
				"package":        prop("string", "Package name"),
				"version":        prop("string", "Current version"),
				"total_flat_pct": prop("number", "Total flat percent"),
				"total_cum_pct":  prop("number", "Total cumulative percent"),
				"hot_functions": arrayPropSchema(NewObjectSchema(map[string]any{
					"name":     prop("string", "Function name"),
					"flat_pct": prop("number", "Flat percent"),
				}, "name", "flat_pct"), "Hot functions"),
				"repo_url":       prop("string", "Repository URL"),
				"latest_version": prop("string", "Latest version"),
				"known_issues": arrayPropSchema(NewObjectSchema(map[string]any{
					"pattern":        prop("string", "Pattern"),
					"severity":       prop("string", "Severity"),
					"issue":          prop("string", "Issue description"),
					"recommendation": prop("string", "Recommendation"),
				}, "pattern", "severity", "issue", "recommendation"), "Known issues"),
			}, "package", "total_flat_pct", "total_cum_pct", "hot_functions"), "Vendor hotspots"),
			"total_vendor_pct": prop("number", "Total vendor percentage"),
			"total_app_pct":    prop("number", "Total app percentage"),
			"warnings":         arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "vendor_hotspots", "total_vendor_pct", "total_app_pct"),
	}, "command", "result")
}

func pprofExplainOverheadOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchema(map[string]any{
			"category": prop("string", "Overhead category"),
			"explanation": NewObjectSchema(map[string]any{
				"summary":       prop("string", "Summary"),
				"detailed":      prop("string", "Detailed explanation"),
				"why_slow":      arrayPropSchema(prop("string", "Why slow"), "Why slow"),
				"common_causes": arrayPropSchema(prop("string", "Common cause"), "Common causes"),
			}, "summary", "detailed", "why_slow", "common_causes"),
			"in_your_profile": NewObjectSchema(map[string]any{
				"total_pct": prop("number", "Total percent in profile"),
				"top_contributors": arrayPropSchema(NewObjectSchema(map[string]any{
					"function": prop("string", "Function name"),
					"pct":      prop("number", "Percent"),
				}, "function", "pct"), "Top contributors"),
			}, "total_pct", "top_contributors"),
			"optimization_strategies": arrayPropSchema(NewObjectSchema(map[string]any{
				"strategy":        prop("string", "Strategy"),
				"expected_impact": prop("string", "Expected impact"),
				"effort":          prop("string", "Effort"),
				"description":     prop("string", "Description"),
				"applicable":      prop("boolean", "Applicable"),
				"reason":          prop("string", "Applicability reason"),
			}, "strategy", "expected_impact", "effort", "description", "applicable"), "Optimization strategies"),
			"warnings": arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "category", "explanation", "optimization_strategies"),
	}, "command", "result")
}

func pprofSuggestFixOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"result": NewObjectSchema(map[string]any{
			"issue": prop("string", "Issue identifier"),
			"analysis": NewObjectSchema(map[string]any{
				"overhead_pct": prop("number", "Overhead percent"),
				"top_functions": arrayPropSchema(NewObjectSchema(map[string]any{
					"function": prop("string", "Function name"),
					"pct":      prop("number", "Percent"),
				}, "function", "pct"), "Top functions"),
			}, "overhead_pct", "top_functions"),
			"applicable_fixes": arrayPropSchema(NewObjectSchema(map[string]any{
				"fix_id":          prop("string", "Fix identifier"),
				"description":     prop("string", "Description"),
				"expected_impact": NewObjectSchemaWithAdditional(map[string]any{}, true),
				"files_to_modify": arrayPropSchema(NewObjectSchema(map[string]any{
					"path":          prop("string", "File path"),
					"is_vendor":     prop("boolean", "Is vendor file"),
					"upstream_repo": prop("string", "Upstream repo"),
					"changes": arrayPropSchema(NewObjectSchema(map[string]any{
						"line":   prop("integer", "Line number"),
						"before": prop("string", "Before"),
						"after":  prop("string", "After"),
					}, "line", "before", "after"), "Line changes"),
				}, "path", "is_vendor", "changes"), "Files to modify"),
				"diff":               prop("string", "Unified diff"),
				"pr_description":     prop("string", "PR description"),
				"considerations":     arrayPropSchema(prop("string", "Consideration"), "Considerations"),
				"is_vendored":        prop("boolean", "Is vendored"),
				"upstream_pr_needed": prop("boolean", "Upstream PR needed"),
			}, "fix_id", "description", "expected_impact", "files_to_modify", "diff", "pr_description", "considerations", "is_vendored", "upstream_pr_needed"), "Applicable fixes"),
			"next_steps": arrayPropSchema(prop("string", "Next step"), "Next steps"),
			"warnings":   arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "issue", "analysis", "applicable_fixes", "next_steps"),
	}, "command", "result")
}

func datadogProfilesAggregateOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "CLI command equivalent"),
		"result": NewObjectSchema(map[string]any{
			"handle":          prop("string", "Handle for merged profile"),
			"profile_type":    prop("string", "Profile type"),
			"profiles_merged": prop("integer", "Number of profiles merged"),
			"time_range": NewObjectSchema(map[string]any{
				"from": prop("string", "Start time"),
				"to":   prop("string", "End time"),
			}, "from", "to"),
			"total_duration": prop("string", "Total duration of merged profile"),
			"hint":           prop("string", "Usage hint"),
			"warnings":       arrayPropSchema(prop("string", "Warning"), "Warnings"),
		}, "handle", "profiles_merged", "time_range"),
	}, "command", "result")
}
