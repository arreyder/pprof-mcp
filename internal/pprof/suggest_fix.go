package pprof

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/yaml.v3"

	"github.com/arreyder/pprof-mcp/internal/pprofdata"
	"github.com/arreyder/pprof-mcp/internal/pprofparse"
)

const (
	defaultSuggestNodeCount = 200
	protojsonThresholdPct   = 10.0
)

type SuggestFixParams struct {
	Profile        string
	Issue          string
	RepoRoot       string
	TargetFunction string
}

type SuggestFixResult struct {
	Issue           string             `json:"issue"`
	Analysis        SuggestFixAnalysis `json:"analysis"`
	ApplicableFixes []FixSuggestion    `json:"applicable_fixes"`
	NextSteps       []string           `json:"next_steps"`
	Warnings        []string           `json:"warnings,omitempty"`
}

type SuggestFixAnalysis struct {
	OverheadPct float64           `json:"overhead_pct"`
	TopFuncs    []SuggestFunction `json:"top_functions"`
}

type SuggestFunction struct {
	Function string  `json:"function"`
	Pct      float64 `json:"pct"`
}

type FixSuggestion struct {
	FixID            string            `json:"fix_id"`
	Description      string            `json:"description"`
	ExpectedImpact   map[string]string `json:"expected_impact"`
	FilesToModify    []FixFileChange   `json:"files_to_modify"`
	Diff             string            `json:"diff"`
	PRDescription    string            `json:"pr_description"`
	Considerations   []string          `json:"considerations"`
	IsVendored       bool              `json:"is_vendored"`
	UpstreamPRNeeded bool              `json:"upstream_pr_needed"`
}

type FixFileChange struct {
	Path         string          `json:"path"`
	IsVendor     bool            `json:"is_vendor"`
	UpstreamRepo string          `json:"upstream_repo,omitempty"`
	Changes      []FixLineChange `json:"changes"`
}

type FixLineChange struct {
	Line   int    `json:"line"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type fixTemplateDB struct {
	Fixes map[string]fixTemplate `yaml:"fixes"`
}

type fixTemplate struct {
	IssueID           string             `yaml:"issue_id"`
	Description       string             `yaml:"description"`
	ApplicableWhen    []string           `yaml:"applicable_when"`
	DetectionPatterns []string           `yaml:"detection_patterns"`
	Template          fixTemplateSnippet `yaml:"template"`
	Considerations    []string           `yaml:"considerations"`
	ExpectedImpact    map[string]string  `yaml:"expected_impact"`
	PRTemplate        string             `yaml:"pr_template"`
}

type fixTemplateSnippet struct {
	Before string `yaml:"before"`
	After  string `yaml:"after"`
}

func RunSuggestFix(ctx context.Context, params SuggestFixParams) (SuggestFixResult, error) {
	result := SuggestFixResult{
		ApplicableFixes: []FixSuggestion{},
		NextSteps:       []string{},
		Warnings:        []string{},
	}
	if strings.TrimSpace(params.Profile) == "" {
		return result, fmt.Errorf("profile is required")
	}
	if strings.TrimSpace(params.Issue) == "" {
		return result, fmt.Errorf("issue is required")
	}

	templates, err := loadFixTemplates()
	if err != nil {
		return result, err
	}

	top, err := RunTop(ctx, TopParams{
		Profile:   params.Profile,
		NodeCount: defaultSuggestNodeCount,
	})
	if err != nil {
		return result, err
	}

	result.Issue = params.Issue
	result.Analysis = buildSuggestAnalysis(top.Rows)

	for id, tmpl := range templates.Fixes {
		if tmpl.IssueID != params.Issue {
			continue
		}

		if !fixApplicable(tmpl, top.Rows) {
			continue
		}

		fix := FixSuggestion{
			FixID:          id,
			Description:    tmpl.Description,
			ExpectedImpact: tmpl.ExpectedImpact,
			FilesToModify:  []FixFileChange{},
			Considerations: tmpl.Considerations,
		}

		if params.RepoRoot != "" {
			files, diff, vendor, upstream := generateFixDiff(params.RepoRoot, tmpl)
			fix.FilesToModify = files
			fix.Diff = diff
			fix.IsVendored = vendor
			fix.UpstreamPRNeeded = vendor
			if vendor && upstream != "" {
				for i := range fix.FilesToModify {
					if fix.FilesToModify[i].IsVendor {
						fix.FilesToModify[i].UpstreamRepo = upstream
					}
				}
			}
		}

		fix.PRDescription = renderPRTemplate(tmpl.PRTemplate, result.Analysis)
		result.ApplicableFixes = append(result.ApplicableFixes, fix)
	}

	if len(result.ApplicableFixes) == 0 {
		result.Warnings = append(result.Warnings, "no applicable fixes detected for the supplied issue")
	}

	result.NextSteps = buildNextSteps(result.ApplicableFixes)
	return result, nil
}

func loadFixTemplates() (fixTemplateDB, error) {
	var db fixTemplateDB
	content := pprofdata.FixTemplatesYAML()
	if strings.TrimSpace(content) == "" {
		return db, fmt.Errorf("fix_templates.yaml is empty")
	}
	if err := yaml.Unmarshal([]byte(content), &db); err != nil {
		return db, err
	}
	return db, nil
}

func buildSuggestAnalysis(rows []pprofparse.TopRow) SuggestFixAnalysis {
	analysis := SuggestFixAnalysis{
		TopFuncs: []SuggestFunction{},
	}
	for _, row := range rows {
		pct := parsePercent(row.CumPct)
		if pct == 0 {
			pct = parsePercent(row.FlatPct)
		}
		if pct == 0 {
			continue
		}
		analysis.OverheadPct += pct
		analysis.TopFuncs = append(analysis.TopFuncs, SuggestFunction{
			Function: row.Name,
			Pct:      roundPct(pct),
		})
	}
	analysis.OverheadPct = roundPct(analysis.OverheadPct)
	if len(analysis.TopFuncs) > 5 {
		analysis.TopFuncs = analysis.TopFuncs[:5]
	}
	return analysis
}

func fixApplicable(tmpl fixTemplate, rows []pprofparse.TopRow) bool {
	if len(tmpl.DetectionPatterns) == 0 {
		return true
	}
	for _, row := range rows {
		flat := parsePercent(row.FlatPct)
		cum := parsePercent(row.CumPct)
		pct := flat
		if cum > pct {
			pct = cum
		}
		if pct < protojsonThresholdPct {
			continue
		}
		for _, pattern := range tmpl.DetectionPatterns {
			if strings.Contains(row.Name, pattern) {
				return true
			}
		}
	}
	return false
}

func generateFixDiff(repoRoot string, tmpl fixTemplate) ([]FixFileChange, string, bool, string) {
	files := []FixFileChange{}
	if tmpl.IssueID != "protojson_overhead" {
		return files, "", false, ""
	}

	changedFiles := map[string]string{}
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		original := string(content)
		if !strings.Contains(original, "protojson.") && !strings.Contains(original, "encoding/protojson") {
			return nil
		}
		modified := strings.ReplaceAll(original, "google.golang.org/protobuf/encoding/protojson", "google.golang.org/protobuf/proto")
		modified = strings.ReplaceAll(modified, "protojson.", "proto.")
		if modified == original {
			return nil
		}
		changedFiles[path] = modified
		return nil
	})
	if err != nil {
		return files, "", false, ""
	}

	allDiffs := []string{}
	isVendored := false
	upstreamRepo := ""

	for path, modified := range changedFiles {
		original, _ := os.ReadFile(path)
		fileDiff := unifiedDiff(path, string(original), modified)
		if fileDiff != "" {
			allDiffs = append(allDiffs, fileDiff)
		}
		isVendor := strings.Contains(path, string(filepath.Separator)+"vendor"+string(filepath.Separator))
		if isVendor {
			isVendored = true
			if upstreamRepo == "" {
				upstreamRepo = repoURLForPackage(pathToVendorModule(path))
			}
		}
		files = append(files, FixFileChange{
			Path:     path,
			IsVendor: isVendor,
			Changes:  lineChanges(string(original), modified),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files, strings.Join(allDiffs, "\n"), isVendored, upstreamRepo
}

func unifiedDiff(path, original, modified string) string {
	if original == modified {
		return ""
	}
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(original),
		B:        difflib.SplitLines(modified),
		FromFile: "a/" + filepath.ToSlash(path),
		ToFile:   "b/" + filepath.ToSlash(path),
		Context:  2,
	}
	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(text)
}

func lineChanges(original, modified string) []FixLineChange {
	changes := []FixLineChange{}
	origLines := strings.Split(original, "\n")
	modLines := strings.Split(modified, "\n")
	maxLines := len(origLines)
	if len(modLines) > maxLines {
		maxLines = len(modLines)
	}
	for i := 0; i < maxLines; i++ {
		var before, after string
		if i < len(origLines) {
			before = origLines[i]
		}
		if i < len(modLines) {
			after = modLines[i]
		}
		if before != after {
			changes = append(changes, FixLineChange{
				Line:   i + 1,
				Before: before,
				After:  after,
			})
		}
	}
	return changes
}

func renderPRTemplate(template string, analysis SuggestFixAnalysis) string {
	if template == "" {
		return ""
	}
	topFuncs := []string{}
	for _, fn := range analysis.TopFuncs {
		topFuncs = append(topFuncs, fmt.Sprintf("- %s (%.2f%%)", fn.Function, fn.Pct))
	}
	out := strings.ReplaceAll(template, "{service}", "service")
	out = strings.ReplaceAll(out, "{overhead_pct}", fmt.Sprintf("%.2f", analysis.OverheadPct))
	out = strings.ReplaceAll(out, "{top_functions}", strings.Join(topFuncs, "\n"))
	return strings.TrimSpace(out)
}

func buildNextSteps(fixes []FixSuggestion) []string {
	if len(fixes) == 0 {
		return nil
	}
	steps := []string{}
	for _, fix := range fixes {
		if fix.UpstreamPRNeeded {
			steps = append(steps, "Open PR in upstream dependency for vendor change")
		}
		if len(fix.FilesToModify) > 0 {
			steps = append(steps, "Apply suggested changes and run unit tests")
		}
	}
	return steps
}

func pathToVendorModule(path string) string {
	parts := strings.Split(path, string(filepath.Separator)+"vendor"+string(filepath.Separator))
	if len(parts) < 2 {
		return ""
	}
	modulePath := parts[1]
	segments := strings.Split(modulePath, string(filepath.Separator))
	if len(segments) >= 3 {
		return strings.Join(segments[:3], "/")
	}
	return strings.ReplaceAll(modulePath, string(filepath.Separator), "/")
}

func parseRegexList(patterns []string) []*regexp.Regexp {
	out := []*regexp.Regexp{}
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err == nil {
			out = append(out, re)
		}
	}
	return out
}
