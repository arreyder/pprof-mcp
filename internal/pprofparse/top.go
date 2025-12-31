package pprofparse

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type TopSummary struct {
	HeaderLines []string `json:"header_lines"`
	TableHeader string   `json:"table_header"`
}

type TopRow struct {
	Flat        string   `json:"flat"`
	FlatPct     string   `json:"flat_pct"`
	SumPct      string   `json:"sum_pct"`
	Cum         string   `json:"cum"`
	CumPct      string   `json:"cum_pct"`
	Name        string   `json:"name"`
	FlatSeconds *float64 `json:"flat_seconds,omitempty"`
	CumSeconds  *float64 `json:"cum_seconds,omitempty"`
}

type TopReport struct {
	Summary TopSummary `json:"summary"`
	Rows    []TopRow   `json:"rows"`
}

var tableHeaderPattern = regexp.MustCompile(`(?i)^\s*flat\s+flat%\s+sum%\s+cum\s+cum%\s+name\s*$`)

func ParseTop(output string) TopReport {
	lines := strings.Split(output, "\n")
	report := TopReport{}
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if tableHeaderPattern.MatchString(trimmed) {
			report.Summary.TableHeader = trimmed
			inTable = true
			continue
		}
		if !inTable {
			report.Summary.HeaderLines = append(report.Summary.HeaderLines, trimmed)
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 6 {
			continue
		}
		name := strings.Join(fields[5:], " ")
		row := TopRow{
			Flat:    fields[0],
			FlatPct: fields[1],
			SumPct:  fields[2],
			Cum:     fields[3],
			CumPct:  fields[4],
			Name:    name,
		}
		if seconds, ok := parseTimeToSeconds(fields[0]); ok {
			row.FlatSeconds = &seconds
		}
		if seconds, ok := parseTimeToSeconds(fields[3]); ok {
			row.CumSeconds = &seconds
		}
		report.Rows = append(report.Rows, row)
	}
	return report
}

func DiffTop(before []TopRow, after []TopRow, useCum bool) []map[string]any {
	beforeMap := map[string]TopRow{}
	for _, row := range before {
		beforeMap[row.Name] = row
	}
	afterMap := map[string]TopRow{}
	for _, row := range after {
		afterMap[row.Name] = row
	}

	keys := make([]string, 0, len(beforeMap)+len(afterMap))
	seen := map[string]struct{}{}
	for name := range beforeMap {
		keys = append(keys, name)
		seen[name] = struct{}{}
	}
	for name := range afterMap {
		if _, ok := seen[name]; !ok {
			keys = append(keys, name)
		}
	}

	deltas := make([]map[string]any, 0, len(keys))
	for _, name := range keys {
		beforeRow := beforeMap[name]
		afterRow := afterMap[name]
		beforeVal := pickValue(beforeRow, useCum)
		afterVal := pickValue(afterRow, useCum)
		delta := afterVal - beforeVal
		deltas = append(deltas, map[string]any{
			"name":                name,
			"before_flat":         beforeRow.Flat,
			"after_flat":          afterRow.Flat,
			"before_cum":          beforeRow.Cum,
			"after_cum":           afterRow.Cum,
			"before_flat_seconds": beforeRow.FlatSeconds,
			"after_flat_seconds":  afterRow.FlatSeconds,
			"before_cum_seconds":  beforeRow.CumSeconds,
			"after_cum_seconds":   afterRow.CumSeconds,
			"delta_seconds":       delta,
		})
	}

	sort.Slice(deltas, func(i, j int) bool {
		return deltas[i]["delta_seconds"].(float64) > deltas[j]["delta_seconds"].(float64)
	})

	return deltas
}

func pickValue(row TopRow, useCum bool) float64 {
	if useCum {
		if row.CumSeconds != nil {
			return *row.CumSeconds
		}
		return 0
	}
	if row.FlatSeconds != nil {
		return *row.FlatSeconds
	}
	return 0
}

func parseTimeToSeconds(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "0" {
		return 0, true
	}
	units := []struct {
		suffix string
		mult   float64
	}{
		{"ns", 1e-9},
		{"us", 1e-6},
		{"ms", 1e-3},
		{"s", 1},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			parsed, err := strconv.ParseFloat(strings.TrimSuffix(value, unit.suffix), 64)
			if err != nil {
				return 0, false
			}
			return parsed * unit.mult, true
		}
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

