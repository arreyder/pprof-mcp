package pprofdata

import _ "embed"

//go:embed known_perf_issues.yaml
var knownPerfIssuesYAML string

//go:embed explanations.yaml
var explanationsYAML string

//go:embed fix_templates.yaml
var fixTemplatesYAML string

func KnownPerfIssuesYAML() string {
	return knownPerfIssuesYAML
}

func ExplanationsYAML() string {
	return explanationsYAML
}

func FixTemplatesYAML() string {
	return fixTemplatesYAML
}
