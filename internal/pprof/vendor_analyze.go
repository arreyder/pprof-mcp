package pprof

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/arreyder/pprof-mcp/internal/pprofdata"
)

const (
	defaultVendorMinPct = 1.0
	maxVendorFunctions  = 5
)

type VendorAnalyzeParams struct {
	Profile      string
	RepoRoot     string
	MinPct       float64
	CheckUpdates bool
}

type VendorAnalyzeResult struct {
	VendorHotspots []VendorHotspot `json:"vendor_hotspots"`
	TotalVendorPct float64         `json:"total_vendor_pct"`
	TotalAppPct    float64         `json:"total_app_pct"`
	Warnings       []string        `json:"warnings,omitempty"`
}

type VendorHotspot struct {
	Package      string           `json:"package"`
	Version      string           `json:"version,omitempty"`
	TotalFlatPct float64          `json:"total_flat_pct"`
	TotalCumPct  float64          `json:"total_cum_pct"`
	HotFunctions []VendorFunction `json:"hot_functions"`
	RepoURL      string           `json:"repo_url,omitempty"`
	Latest       string           `json:"latest_version,omitempty"`
	KnownIssues  []KnownIssue     `json:"known_issues,omitempty"`
}

type VendorFunction struct {
	Name    string  `json:"name"`
	FlatPct float64 `json:"flat_pct"`
}

type KnownIssue struct {
	Pattern        string `json:"pattern"`
	Severity       string `json:"severity"`
	Issue          string `json:"issue"`
	Recommendation string `json:"recommendation"`
}

type perfIssueDB struct {
	Packages map[string]perfIssuePackage `yaml:"packages"`
}

type perfIssuePackage struct {
	RepoURL  string             `yaml:"repo_url"`
	Patterns []perfIssuePattern `yaml:"patterns"`
}

type perfIssuePattern struct {
	Match          string `yaml:"match"`
	Severity       string `yaml:"severity"`
	Issue          string `yaml:"issue"`
	Recommendation string `yaml:"recommendation"`
}

type vendorHotspotBuilder struct {
	pkg         string
	version     string
	totalFlat   float64
	totalCum    float64
	functions   []VendorFunction
	knownIssues []KnownIssue
	repoURL     string
	latest      string
}

func RunVendorAnalyze(ctx context.Context, params VendorAnalyzeParams) (VendorAnalyzeResult, error) {
	result := VendorAnalyzeResult{
		VendorHotspots: []VendorHotspot{},
		Warnings:       []string{},
	}
	if strings.TrimSpace(params.Profile) == "" {
		return result, fmt.Errorf("profile is required")
	}

	minPct := params.MinPct
	if minPct <= 0 {
		minPct = defaultVendorMinPct
	}
	repoRoot := strings.TrimSpace(params.RepoRoot)
	if repoRoot == "" {
		repoRoot = "."
	}

	top, err := RunTop(ctx, TopParams{
		Profile:   params.Profile,
		NodeCount: 200,
	})
	if err != nil {
		return result, err
	}

	modInfo, modErr := ParseGoMod(repoRoot)
	if modErr != nil {
		result.Warnings = append(result.Warnings, "go.mod not found or unreadable; version info omitted")
	}

	issuesDB, err := loadPerfIssueDB()
	if err != nil {
		result.Warnings = append(result.Warnings, "known_perf_issues.yaml unavailable; known issues omitted")
	}

	hotspots := map[string]*vendorHotspotBuilder{}
	for _, row := range top.Rows {
		flat := parsePercent(row.FlatPct)
		cum := parsePercent(row.CumPct)
		if flat < minPct && cum < minPct {
			continue
		}

		funcName := strings.TrimSpace(row.Name)
		if funcName == "" {
			continue
		}
		isApp := modInfo.ModulePath != "" && strings.HasPrefix(funcName, modInfo.ModulePath)
		if isApp {
			result.TotalAppPct += flat
			continue
		}
		result.TotalVendorPct += flat

		packagePath := functionPackagePath(funcName)
		if packagePath == "" {
			packagePath = funcName
		}
		modulePath, version := moduleVersionForPackage(modInfo, packagePath)
		packageKey := packagePath
		if modulePath != "" {
			packageKey = modulePath
		}

		builder, ok := hotspots[packageKey]
		if !ok {
			builder = &vendorHotspotBuilder{
				pkg:     packageKey,
				version: version,
				repoURL: repoURLForPackage(packageKey),
			}
			hotspots[packageKey] = builder
		}
		builder.totalFlat += flat
		builder.totalCum += cum
		builder.functions = append(builder.functions, VendorFunction{
			Name:    funcName,
			FlatPct: flat,
		})
	}

	for _, builder := range hotspots {
		sort.Slice(builder.functions, func(i, j int) bool {
			return builder.functions[i].FlatPct > builder.functions[j].FlatPct
		})
		if len(builder.functions) > maxVendorFunctions {
			builder.functions = builder.functions[:maxVendorFunctions]
		}
		builder.knownIssues = matchKnownIssues(issuesDB, builder.pkg, builder.functions)
		if params.CheckUpdates && builder.pkg != "" {
			if latest, err := lookupLatestVersion(ctx, builder.pkg); err == nil {
				builder.latest = latest
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("update check failed for %s: %v", builder.pkg, err))
			}
		}

		result.VendorHotspots = append(result.VendorHotspots, VendorHotspot{
			Package:      builder.pkg,
			Version:      builder.version,
			TotalFlatPct: roundPct(builder.totalFlat),
			TotalCumPct:  roundPct(builder.totalCum),
			HotFunctions: builder.functions,
			RepoURL:      builder.repoURL,
			Latest:       builder.latest,
			KnownIssues:  builder.knownIssues,
		})
	}

	sort.Slice(result.VendorHotspots, func(i, j int) bool {
		return result.VendorHotspots[i].TotalFlatPct > result.VendorHotspots[j].TotalFlatPct
	})
	return result, nil
}

func loadPerfIssueDB() (perfIssueDB, error) {
	var db perfIssueDB
	content := pprofdata.KnownPerfIssuesYAML()
	if strings.TrimSpace(content) == "" {
		return db, fmt.Errorf("known_perf_issues.yaml is empty")
	}
	if err := yaml.Unmarshal([]byte(content), &db); err != nil {
		return db, err
	}
	return db, nil
}

func matchKnownIssues(db perfIssueDB, packageKey string, functions []VendorFunction) []KnownIssue {
	pkg, ok := db.Packages[packageKey]
	if !ok {
		return nil
	}
	issues := []KnownIssue{}
	for _, pattern := range pkg.Patterns {
		re, err := regexp.Compile(pattern.Match)
		if err != nil {
			continue
		}
		for _, fn := range functions {
			if re.MatchString(fn.Name) {
				issues = append(issues, KnownIssue{
					Pattern:        pattern.Match,
					Severity:       pattern.Severity,
					Issue:          pattern.Issue,
					Recommendation: pattern.Recommendation,
				})
				break
			}
		}
	}
	return issues
}

func repoURLForPackage(pkg string) string {
	switch {
	case strings.HasPrefix(pkg, "github.com/"):
		parts := strings.Split(pkg, "/")
		if len(parts) >= 3 {
			return "https://github.com/" + strings.Join(parts[1:3], "/")
		}
	case strings.HasPrefix(pkg, "gitlab.com/"):
		parts := strings.Split(pkg, "/")
		if len(parts) >= 3 {
			return "https://gitlab.com/" + strings.Join(parts[1:3], "/")
		}
	}
	return ""
}

func lookupLatestVersion(ctx context.Context, modulePath string) (string, error) {
	stdout, stderr, err := runCommand(ctx, "go", "list", "-m", "-u", "-json", modulePath)
	if err != nil {
		return "", fmt.Errorf("go list failed: %w (%s)", err, strings.TrimSpace(stderr))
	}
	var payload struct {
		Latest *struct {
			Version string `json:"Version"`
		} `json:"Latest"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return "", err
	}
	if payload.Latest == nil {
		return "", nil
	}
	return payload.Latest.Version, nil
}
