package pprof

import (
	"context"
	"errors"
	"fmt"
)

type RegressionCheckParams struct {
	Profile     string
	SampleIndex string
	Checks      []RegressionCheckSpec
}

type RegressionCheckSpec struct {
	Function string  `json:"function"`
	Metric   string  `json:"metric"`
	Max      float64 `json:"max"`
}

type RegressionCheckSummary struct {
	Passed bool                    `json:"passed"`
	Checks []RegressionCheckResult `json:"checks"`
}

type RegressionCheckResult struct {
	Function  string  `json:"function"`
	Metric    string  `json:"metric"`
	Threshold float64 `json:"threshold"`
	Actual    float64 `json:"actual"`
	Passed    bool    `json:"passed"`
	Message   string  `json:"message,omitempty"`
}

func RunRegressionCheck(ctx context.Context, params RegressionCheckParams) (RegressionCheckSummary, error) {
	result := RegressionCheckSummary{
		Passed: true,
		Checks: []RegressionCheckResult{},
	}
	if params.Profile == "" {
		return result, fmt.Errorf("profile is required")
	}
	if len(params.Checks) == 0 {
		return result, fmt.Errorf("checks are required")
	}

	for _, check := range params.Checks {
		if check.Function == "" {
			return result, fmt.Errorf("check function is required")
		}
		metric := check.Metric
		if metric == "" {
			metric = "flat_pct"
		}
		if metric != "flat_pct" && metric != "cum_pct" {
			return result, fmt.Errorf("unsupported metric %q (use flat_pct or cum_pct)", metric)
		}

		actual, err := evaluateCheck(ctx, params.Profile, params.SampleIndex, check.Function, metric)
		if err != nil {
			return result, err
		}

		passed := actual <= check.Max
		entry := RegressionCheckResult{
			Function:  check.Function,
			Metric:    metric,
			Threshold: check.Max,
			Actual:    actual,
			Passed:    passed,
		}
		if !passed {
			entry.Message = fmt.Sprintf("%s %s (%.2f%%) exceeds threshold (%.2f%%)", check.Function, metric, actual, check.Max)
			result.Passed = false
		}
		result.Checks = append(result.Checks, entry)
	}

	return result, nil
}

func evaluateCheck(ctx context.Context, profilePath, sampleIndex, pattern, metric string) (float64, error) {
	topResult, err := RunTop(ctx, TopParams{
		Profile:     profilePath,
		NodeCount:   50,
		Focus:       pattern,
		SampleIndex: sampleIndex,
	})
	if err != nil {
		if errors.Is(err, ErrNoMatches) {
			return 0, nil
		}
		return 0, err
	}
	best := 0.0
	for _, row := range topResult.Rows {
		value := 0.0
		switch metric {
		case "flat_pct":
			value = parsePercent(row.FlatPct)
		case "cum_pct":
			value = parsePercent(row.CumPct)
		}
		if value > best {
			best = value
		}
	}
	return best, nil
}
