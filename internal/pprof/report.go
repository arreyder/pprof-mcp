package pprof

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/arreyder/pprof-mcp/internal/pprofparse"
)

type ReportInput struct {
	Kind string         `json:"kind"`
	Data map[string]any `json:"data"`
}

type ReportParams struct {
	Title  string        `json:"title,omitempty"`
	Inputs []ReportInput `json:"inputs"`
}

type ReportResult struct {
	Markdown     string `json:"markdown"`
	SectionCount int    `json:"section_count"`
}

func GenerateReport(params ReportParams) (ReportResult, error) {
	if len(params.Inputs) == 0 {
		return ReportResult{}, fmt.Errorf("inputs are required")
	}

	title := params.Title
	if strings.TrimSpace(title) == "" {
		title = "Profiling Report"
	}

	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n")

	sections := 0
	for _, input := range params.Inputs {
		kind := strings.ToLower(strings.TrimSpace(input.Kind))
		data := unwrapReportData(input.Data)
		switch kind {
		case "discover", "pprof.discover":
			var report DiscoveryReport
			if err := decodeReportData(data, &report); err != nil {
				return ReportResult{}, err
			}
			sections += renderDiscoveryReport(&b, report)
		case "top", "pprof.top":
			var top TopResult
			if err := decodeReportData(data, &top); err != nil {
				return ReportResult{}, err
			}
			sections += renderTopReport(&b, top)
		case "alloc_paths", "pprof.alloc_paths":
			var alloc AllocPathsResult
			if err := decodeReportData(data, &alloc); err != nil {
				return ReportResult{}, err
			}
			sections += renderAllocPathsReport(&b, alloc)
		case "memory_sanity", "pprof.memory_sanity":
			var sanity MemorySanityResult
			if err := decodeReportData(data, &sanity); err != nil {
				return ReportResult{}, err
			}
			sections += renderMemorySanityReport(&b, sanity)
		case "overhead_report", "pprof.overhead_report":
			var overhead OverheadReport
			if err := decodeReportData(data, &overhead); err != nil {
				return ReportResult{}, err
			}
			sections += renderOverheadReport(&b, overhead)
		case "goroutine_analysis", "pprof.goroutine_analysis":
			var goroutine GoroutineAnalysisResult
			if err := decodeReportData(data, &goroutine); err != nil {
				return ReportResult{}, err
			}
			sections += renderGoroutineReport(&b, goroutine)
		default:
			sections += renderGenericReport(&b, input.Kind, data)
		}
	}

	return ReportResult{
		Markdown:     strings.TrimSpace(b.String()),
		SectionCount: sections,
	}, nil
}

func unwrapReportData(data map[string]any) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	if nested, ok := data["result"].(map[string]any); ok {
		return nested
	}
	return data
}

func decodeReportData(data map[string]any, target any) error {
	blob, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(blob, target)
}

func renderDiscoveryReport(b *strings.Builder, report DiscoveryReport) int {
	sections := 0

	b.WriteString("## Executive Summary\n")
	if report.Service != "" || report.Env != "" {
		b.WriteString(fmt.Sprintf("- Service: %s (%s)\n", report.Service, report.Env))
	}
	if report.Timestamp != "" {
		b.WriteString(fmt.Sprintf("- Profile timestamp: %s\n", report.Timestamp))
	}
	if report.CPU != nil && report.CPU.UtilizationPct > 0 {
		b.WriteString(fmt.Sprintf("- CPU utilization: %.1f%%\n", report.CPU.UtilizationPct))
	}
	if report.Heap != nil && report.Heap.AllocRate != "" {
		b.WriteString(fmt.Sprintf("- Heap alloc rate: %s\n", report.Heap.AllocRate))
	}
	if report.Goroutine != nil && report.Goroutine.TotalGoroutines > 0 {
		b.WriteString(fmt.Sprintf("- Goroutines: %d\n", report.Goroutine.TotalGoroutines))
	}
	b.WriteString("\n")
	sections++

	if len(report.Recommendations) > 0 {
		b.WriteString("## Recommendations\n")
		for _, rec := range report.Recommendations {
			b.WriteString(fmt.Sprintf("- [%s] %s: %s\n", strings.ToUpper(rec.Priority), rec.Area, rec.Suggestion))
		}
		b.WriteString("\n")
		sections++
	}

	if report.CPU != nil {
		b.WriteString("## CPU\n")
		if report.CPU.UtilizationPct > 0 {
			b.WriteString(fmt.Sprintf("- Utilization: %.1f%%\n", report.CPU.UtilizationPct))
		}
		if len(report.CPU.Overhead.Detections) > 0 {
			b.WriteString("- Overhead categories:\n")
			for _, det := range report.CPU.Overhead.Detections {
				b.WriteString(fmt.Sprintf("  - %s: %.1f%% (%s)\n", det.Category, det.Percentage, det.Severity))
			}
		}
		if len(report.CPU.TopFunctions) > 0 {
			b.WriteString("\n| Function | Flat | Flat% | Cum | Cum% |\n| --- | --- | --- | --- | --- |\n")
			for _, row := range limitTopRows(report.CPU.TopFunctions, 10) {
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", row.Name, row.Flat, row.FlatPct, row.Cum, row.CumPct))
			}
		}
		b.WriteString("\n\n")
		sections++
	}

	if report.Heap != nil {
		b.WriteString("## Heap\n")
		if report.Heap.AllocRate != "" {
			b.WriteString(fmt.Sprintf("- Allocation rate: %s\n", report.Heap.AllocRate))
		}
		if len(report.Heap.TopPaths) > 0 {
			b.WriteString("\n| Allocation Site | Rate | Share | Source |\n| --- | --- | --- | --- |\n")
			for _, path := range limitAllocPaths(report.Heap.TopPaths, 10) {
				b.WriteString(fmt.Sprintf("| %s | %s | %.1f%% | %s |\n", path.AllocSite, path.AllocRate, path.AllocPct, path.SourceLocation))
			}
		}
		if len(report.Heap.MemorySanity.Recommendations) > 0 {
			b.WriteString("\n- Memory sanity recommendations:\n")
			for _, rec := range report.Heap.MemorySanity.Recommendations {
				b.WriteString(fmt.Sprintf("  - %s\n", rec))
			}
		}
		b.WriteString("\n\n")
		sections++
	}

	if report.Mutex != nil {
		b.WriteString("## Contention\n")
		if report.Mutex.TotalDelay != "" {
			b.WriteString(fmt.Sprintf("- Total delay: %s\n", report.Mutex.TotalDelay))
		}
		if len(report.Mutex.TopContentions) > 0 {
			b.WriteString("\n| Function | Flat | Flat% | Cum | Cum% |\n| --- | --- | --- | --- | --- |\n")
			for _, row := range limitTopRows(report.Mutex.TopContentions, 10) {
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", row.Name, row.Flat, row.FlatPct, row.Cum, row.CumPct))
			}
		}
		b.WriteString("\n\n")
		sections++
	}

	if report.Goroutine != nil {
		b.WriteString("## Goroutines\n")
		if report.Goroutine.TotalGoroutines > 0 {
			b.WriteString(fmt.Sprintf("- Total: %d\n", report.Goroutine.TotalGoroutines))
		}
		if len(report.Goroutine.ByState) > 0 {
			b.WriteString("- By state:\n")
			for _, entry := range sortedStateCounts(report.Goroutine.ByState) {
				b.WriteString(fmt.Sprintf("  - %s: %d\n", entry.State, entry.Count))
			}
		}
		if len(report.Goroutine.TopWaitReasons) > 0 {
			b.WriteString("\n| Wait Reason | Count | Sample Stack |\n| --- | --- | --- |\n")
			for _, reason := range report.Goroutine.TopWaitReasons {
				b.WriteString(fmt.Sprintf("| %s | %d | %s |\n", reason.Reason, reason.Count, reason.SampleStack))
			}
		}
		b.WriteString("\n\n")
		sections++
	}

	if len(report.Warnings) > 0 {
		b.WriteString("## Warnings\n")
		for _, warning := range report.Warnings {
			b.WriteString(fmt.Sprintf("- %s\n", warning))
		}
		b.WriteString("\n")
		sections++
	}

	return sections
}

func renderTopReport(b *strings.Builder, top TopResult) int {
	if len(top.Rows) == 0 {
		return 0
	}
	b.WriteString("## Top Functions\n")
	b.WriteString("| Function | Flat | Flat% | Cum | Cum% |\n| --- | --- | --- | --- | --- |\n")
	for _, row := range limitTopRows(top.Rows, 10) {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", row.Name, row.Flat, row.FlatPct, row.Cum, row.CumPct))
	}
	b.WriteString("\n\n")
	return 1
}

func renderAllocPathsReport(b *strings.Builder, alloc AllocPathsResult) int {
	if len(alloc.Paths) == 0 {
		return 0
	}
	b.WriteString("## Allocation Paths\n")
	b.WriteString("| Allocation Site | Rate | Share | Source |\n| --- | --- | --- | --- |\n")
	for _, path := range limitAllocPaths(alloc.Paths, 10) {
		b.WriteString(fmt.Sprintf("| %s | %s | %.1f%% | %s |\n", path.AllocSite, path.AllocRate, path.AllocPct, path.SourceLocation))
	}
	b.WriteString("\n\n")
	return 1
}

func renderMemorySanityReport(b *strings.Builder, sanity MemorySanityResult) int {
	b.WriteString("## Memory Sanity\n")
	if sanity.Summary != "" {
		b.WriteString(fmt.Sprintf("- Summary: %s\n", sanity.Summary))
	}
	if len(sanity.Warnings) > 0 {
		b.WriteString("- Warnings:\n")
		for _, warning := range sanity.Warnings {
			b.WriteString(fmt.Sprintf("  - %s\n", warning))
		}
	}
	if len(sanity.Suspicions) > 0 {
		b.WriteString("- Suspicions:\n")
		for _, s := range sanity.Suspicions {
			b.WriteString(fmt.Sprintf("  - [%s] %s\n", strings.ToUpper(s.Severity), s.Description))
		}
	}
	if len(sanity.Recommendations) > 0 {
		b.WriteString("- Recommendations:\n")
		for _, rec := range sanity.Recommendations {
			b.WriteString(fmt.Sprintf("  - %s\n", rec))
		}
	}
	b.WriteString("\n\n")
	return 1
}

func renderOverheadReport(b *strings.Builder, overhead OverheadReport) int {
	if len(overhead.Detections) == 0 {
		return 0
	}
	b.WriteString("## Overhead\n")
	b.WriteString("| Category | Share | Severity |\n| --- | --- | --- |\n")
	for _, det := range overhead.Detections {
		b.WriteString(fmt.Sprintf("| %s | %.1f%% | %s |\n", det.Category, det.Percentage, det.Severity))
	}
	b.WriteString("\n\n")
	return 1
}

func renderGoroutineReport(b *strings.Builder, goroutine GoroutineAnalysisResult) int {
	if goroutine.TotalGoroutines == 0 && len(goroutine.TopWaitReasons) == 0 {
		return 0
	}
	b.WriteString("## Goroutines\n")
	if goroutine.TotalGoroutines > 0 {
		b.WriteString(fmt.Sprintf("- Total: %d\n", goroutine.TotalGoroutines))
	}
	if len(goroutine.ByState) > 0 {
		b.WriteString("- By state:\n")
		for _, entry := range sortedStateCounts(goroutine.ByState) {
			b.WriteString(fmt.Sprintf("  - %s: %d\n", entry.State, entry.Count))
		}
	}
	if len(goroutine.TopWaitReasons) > 0 {
		b.WriteString("\n| Wait Reason | Count | Sample Stack |\n| --- | --- | --- |\n")
		for _, reason := range goroutine.TopWaitReasons {
			b.WriteString(fmt.Sprintf("| %s | %d | %s |\n", reason.Reason, reason.Count, reason.SampleStack))
		}
	}
	b.WriteString("\n\n")
	return 1
}

func renderGenericReport(b *strings.Builder, kind string, data map[string]any) int {
	b.WriteString("## ")
	if strings.TrimSpace(kind) == "" {
		b.WriteString("Analysis")
	} else {
		b.WriteString(kind)
	}
	b.WriteString("\n")
	blob, _ := json.MarshalIndent(data, "", "  ")
	b.WriteString("```json\n")
	b.WriteString(string(blob))
	b.WriteString("\n```\n\n")
	return 1
}

func limitTopRows(rows []pprofparse.TopRow, limit int) []pprofparse.TopRow {
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func limitAllocPaths(paths []AllocPath, limit int) []AllocPath {
	if limit > 0 && len(paths) > limit {
		return paths[:limit]
	}
	return paths
}

type stateCount struct {
	State string
	Count int
}

func sortedStateCounts(states map[string]int) []stateCount {
	items := make([]stateCount, 0, len(states))
	for state, count := range states {
		items = append(items, stateCount{State: state, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})
	return items
}
