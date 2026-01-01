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

func pprofTopOutputSchema() map[string]any {
	return NewObjectSchema(map[string]any{
		"command": prop("string", "pprof command"),
		"raw":     prop("string", "Raw pprof output"),
		"rows":    arrayPropSchema(pprofTopRowSchema(), "Top rows"),
		"summary": pprofTopSummarySchema(),
		"hints":   arrayPropSchema(prop("string", "Hint"), "Contextual hints based on profile type"),
	}, "command", "raw", "rows", "summary")
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
