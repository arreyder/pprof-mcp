package pprof

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/arreyder/pprof-mcp/internal/pprofdata"
	"github.com/arreyder/pprof-mcp/internal/pprofparse"
)

type ExplainOverheadParams struct {
	Profile     string
	Category    string
	Function    string
	DetailLevel string
}

type ExplainOverheadResult struct {
	Category               string                 `json:"category"`
	Explanation            ExplainOverheadBlock   `json:"explanation"`
	InYourProfile          *ExplainProfileSummary `json:"in_your_profile,omitempty"`
	OptimizationStrategies []ExplainStrategy      `json:"optimization_strategies"`
	Warnings               []string               `json:"warnings,omitempty"`
}

type ExplainOverheadBlock struct {
	Summary      string   `json:"summary"`
	Detailed     string   `json:"detailed"`
	WhySlow      []string `json:"why_slow"`
	CommonCauses []string `json:"common_causes"`
}

type ExplainProfileSummary struct {
	TotalPct        float64              `json:"total_pct"`
	TopContributors []ExplainContributor `json:"top_contributors"`
}

type ExplainContributor struct {
	Function string  `json:"function"`
	Pct      float64 `json:"pct"`
}

type ExplainStrategy struct {
	Strategy       string `json:"strategy"`
	ExpectedImpact string `json:"expected_impact"`
	Effort         string `json:"effort"`
	Description    string `json:"description"`
	Applicable     bool   `json:"applicable"`
	Reason         string `json:"reason,omitempty"`
}

type explanationsDB struct {
	Categories map[string]categoryExplanation `yaml:"categories"`
	Functions  map[string]functionExplanation `yaml:"functions"`
}

type categoryExplanation struct {
	Brief                  string             `yaml:"brief"`
	Standard               string             `yaml:"standard"`
	Detailed               string             `yaml:"detailed"`
	CommonCauses           []string           `yaml:"common_causes"`
	OptimizationStrategies []strategyTemplate `yaml:"optimization_strategies"`
}

type functionExplanation struct {
	Category     string   `yaml:"category"`
	Explanation  string   `yaml:"explanation"`
	WhyExpensive string   `yaml:"why_expensive"`
	Alternatives []string `yaml:"alternatives"`
}

type strategyTemplate struct {
	Strategy    string `yaml:"strategy"`
	Impact      string `yaml:"impact"`
	Effort      string `yaml:"effort"`
	Description string `yaml:"description"`
}

func RunExplainOverhead(ctx context.Context, params ExplainOverheadParams) (ExplainOverheadResult, error) {
	result := ExplainOverheadResult{
		OptimizationStrategies: []ExplainStrategy{},
		Warnings:               []string{},
	}
	category := strings.TrimSpace(params.Category)
	function := strings.TrimSpace(params.Function)
	if category == "" && function == "" {
		return result, fmt.Errorf("category or function is required")
	}

	db, err := loadExplanationsDB()
	if err != nil {
		return result, err
	}

	var catEntry categoryExplanation
	var fnEntry functionExplanation
	var ok bool

	if function != "" {
		fnEntry, ok = db.Functions[function]
		if !ok {
			return result, fmt.Errorf("no explanation found for function %q", function)
		}
		if category == "" {
			category = fnEntry.Category
		}
	}

	if category != "" {
		catEntry, ok = db.Categories[category]
		if !ok {
			return result, fmt.Errorf("no explanation found for category %q", category)
		}
	}

	result.Category = category
	result.Explanation = buildExplanation(params.DetailLevel, catEntry, fnEntry)
	result.OptimizationStrategies = buildStrategies(catEntry, nil)

	if strings.TrimSpace(params.Profile) != "" {
		summary, warnings := explainProfileSummary(ctx, params.Profile, category, function)
		if summary != nil {
			result.InYourProfile = summary
			result.OptimizationStrategies = buildStrategies(catEntry, summary)
		}
		result.Warnings = append(result.Warnings, warnings...)
	}

	return result, nil
}

func loadExplanationsDB() (explanationsDB, error) {
	var db explanationsDB
	content := pprofdata.ExplanationsYAML()
	if strings.TrimSpace(content) == "" {
		return db, fmt.Errorf("explanations.yaml is empty")
	}
	if err := yaml.Unmarshal([]byte(content), &db); err != nil {
		return db, err
	}
	return db, nil
}

func buildExplanation(detailLevel string, cat categoryExplanation, fn functionExplanation) ExplainOverheadBlock {
	level := strings.ToLower(strings.TrimSpace(detailLevel))
	if level == "" {
		level = "standard"
	}
	summary := cat.Brief
	detailed := cat.Standard
	switch level {
	case "brief":
		detailed = cat.Brief
	case "detailed":
		if cat.Detailed != "" {
			detailed = cat.Detailed
		}
	}

	whySlow := []string{}
	if fn.WhyExpensive != "" {
		whySlow = append(whySlow, fn.WhyExpensive)
	}

	if fn.Explanation != "" {
		detailed = fn.Explanation
		if summary == "" {
			summary = fn.Explanation
		}
	}

	return ExplainOverheadBlock{
		Summary:      strings.TrimSpace(summary),
		Detailed:     strings.TrimSpace(detailed),
		WhySlow:      whySlow,
		CommonCauses: cat.CommonCauses,
	}
}

func buildStrategies(cat categoryExplanation, summary *ExplainProfileSummary) []ExplainStrategy {
	strategies := make([]ExplainStrategy, 0, len(cat.OptimizationStrategies))
	for _, item := range cat.OptimizationStrategies {
		applicable := summary != nil && summary.TotalPct > 0
		reason := ""
		if applicable {
			reason = "Category appears in the supplied profile"
		}
		strategies = append(strategies, ExplainStrategy{
			Strategy:       item.Strategy,
			ExpectedImpact: item.Impact,
			Effort:         item.Effort,
			Description:    item.Description,
			Applicable:     applicable,
			Reason:         reason,
		})
	}
	return strategies
}

func explainProfileSummary(ctx context.Context, profilePath, category, function string) (*ExplainProfileSummary, []string) {
	warnings := []string{}
	if category != "" {
		if patterns := OverheadCategoryPatterns(category); len(patterns) > 0 {
			re := strings.Join(patterns, "|")
			top, err := RunTop(ctx, TopParams{
				Profile:   profilePath,
				Focus:     re,
				NodeCount: 10,
			})
			if err != nil {
				warnings = append(warnings, err.Error())
				return nil, warnings
			}
			return summarizeTopRows(top.Rows), warnings
		}
	}

	if function != "" {
		top, err := RunTop(ctx, TopParams{
			Profile:   profilePath,
			Focus:     regexp.QuoteMeta(function),
			NodeCount: 10,
		})
		if err != nil {
			warnings = append(warnings, err.Error())
			return nil, warnings
		}
		return summarizeTopRows(top.Rows), warnings
	}

	return nil, warnings
}

func summarizeTopRows(rows []pprofparse.TopRow) *ExplainProfileSummary {
	if len(rows) == 0 {
		return nil
	}
	summary := &ExplainProfileSummary{
		TopContributors: []ExplainContributor{},
	}
	for _, row := range rows {
		pct := parsePercent(row.CumPct)
		if pct <= 0 {
			pct = parsePercent(row.FlatPct)
		}
		summary.TotalPct += pct
		summary.TopContributors = append(summary.TopContributors, ExplainContributor{
			Function: row.Name,
			Pct:      roundPct(pct),
		})
	}
	summary.TotalPct = roundPct(summary.TotalPct)
	return summary
}
