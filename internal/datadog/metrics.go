package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

type MetricsDiscoverParams struct {
	Service string
	Env     string
	Site    string
	Query   string // Optional additional filter
}

type MetricInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Unit        string `json:"unit,omitempty"`
}

type MetricsDiscoverResult struct {
	Service  string       `json:"service"`
	Env      string       `json:"env"`
	DDSite   string       `json:"dd_site"`
	Query    string       `json:"query"`
	Metrics  []MetricInfo `json:"metrics"`
	Warnings []string     `json:"warnings,omitempty"`
}

// DiscoverMetrics finds available Datadog metrics that match the service/env pattern.
// It searches for common Go runtime metrics, container metrics, and service-specific metrics.
func DiscoverMetrics(ctx context.Context, params MetricsDiscoverParams) (MetricsDiscoverResult, error) {
	if params.Service == "" {
		return MetricsDiscoverResult{}, fmt.Errorf("service is required")
	}

	site := params.Site
	if site == "" {
		site = os.Getenv("DD_SITE")
	}
	if site == "" {
		site = defaultSite
	}

	apiKey, appKey, err := loadKeys()
	if err != nil {
		return MetricsDiscoverResult{}, err
	}

	// Build search queries for common metric patterns
	searchPatterns := buildSearchPatterns(params.Service, params.Env, params.Query)

	var allMetrics []MetricInfo
	var warnings []string
	seenMetrics := make(map[string]bool)

	for _, pattern := range searchPatterns {
		metrics, err := searchMetrics(ctx, site, apiKey, appKey, pattern)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("search for %q failed: %v", pattern, err))
			continue
		}

		for _, m := range metrics {
			if !seenMetrics[m.Name] {
				seenMetrics[m.Name] = true
				allMetrics = append(allMetrics, m)
			}
		}
	}

	// Sort metrics by relevance (Go runtime first, then container, then others)
	sortMetricsByRelevance(allMetrics)

	return MetricsDiscoverResult{
		Service:  params.Service,
		Env:      params.Env,
		DDSite:   site,
		Query:    params.Query,
		Metrics:  allMetrics,
		Warnings: warnings,
	}, nil
}

func buildSearchPatterns(service, env, additionalQuery string) []string {
	patterns := []string{}

	// Go runtime metrics (most useful for profiling correlation)
	goPatterns := []string{
		"go.memstats",
		"go.goroutines",
		"go.gc",
		"go.heap",
		"go.alloc",
		"runtime.go",
	}
	for _, p := range goPatterns {
		patterns = append(patterns, p)
	}

	// Container/k8s metrics (useful for RSS/memory investigation)
	containerPatterns := []string{
		"container.memory",
		"container.cpu",
		"kubernetes.memory",
		"kubernetes.cpu",
		"docker.memory",
		"docker.cpu",
	}
	for _, p := range containerPatterns {
		patterns = append(patterns, p)
	}

	// Service-specific patterns
	if service != "" {
		// Service name variations
		patterns = append(patterns, service)
		patterns = append(patterns, strings.ReplaceAll(service, "-", "_"))
		patterns = append(patterns, strings.ReplaceAll(service, "_", "-"))
	}

	// User-provided additional query
	if additionalQuery != "" {
		patterns = append(patterns, additionalQuery)
	}

	return patterns
}

func searchMetrics(ctx context.Context, site, apiKey, appKey, query string) ([]MetricInfo, error) {
	// Use the v1 metrics search endpoint
	searchURL := fmt.Sprintf("https://api.%s/api/v1/search?q=metrics:%s", site, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("DD-API-KEY", apiKey)
	req.Header.Set("DD-APPLICATION-KEY", appKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("metrics search failed: status %d", resp.StatusCode)
	}

	var result struct {
		Results struct {
			Metrics []string `json:"metrics"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	metrics := make([]MetricInfo, 0, len(result.Results.Metrics))
	for _, name := range result.Results.Metrics {
		metrics = append(metrics, MetricInfo{
			Name: name,
			Type: inferMetricType(name),
		})
	}

	return metrics, nil
}

func inferMetricType(name string) string {
	lower := strings.ToLower(name)

	switch {
	case strings.Contains(lower, "count"):
		return "count"
	case strings.Contains(lower, "gauge"):
		return "gauge"
	case strings.Contains(lower, "rate"):
		return "rate"
	case strings.Contains(lower, "histogram"):
		return "histogram"
	case strings.Contains(lower, "bytes") || strings.Contains(lower, "memory") || strings.Contains(lower, "heap"):
		return "gauge"
	case strings.Contains(lower, "goroutine"):
		return "gauge"
	case strings.Contains(lower, "gc"):
		return "gauge"
	default:
		return ""
	}
}

func sortMetricsByRelevance(metrics []MetricInfo) {
	priority := func(name string) int {
		lower := strings.ToLower(name)
		switch {
		case strings.HasPrefix(lower, "go."):
			return 0 // Go runtime - highest priority
		case strings.HasPrefix(lower, "runtime."):
			return 1
		case strings.Contains(lower, "container.memory"):
			return 2
		case strings.Contains(lower, "container.cpu"):
			return 3
		case strings.Contains(lower, "kubernetes."):
			return 4
		case strings.Contains(lower, "docker."):
			return 5
		default:
			return 10
		}
	}

	sort.SliceStable(metrics, func(i, j int) bool {
		pi, pj := priority(metrics[i].Name), priority(metrics[j].Name)
		if pi != pj {
			return pi < pj
		}
		return metrics[i].Name < metrics[j].Name
	})
}

// FormatMetricsTable formats metrics as a readable table
func FormatMetricsTable(metrics []MetricInfo) string {
	if len(metrics) == 0 {
		return "No metrics found"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-60s  %s\n", "METRIC NAME", "TYPE"))
	sb.WriteString(strings.Repeat("-", 70) + "\n")

	for _, m := range metrics {
		metricType := m.Type
		if metricType == "" {
			metricType = "-"
		}
		sb.WriteString(fmt.Sprintf("%-60s  %s\n", m.Name, metricType))
	}

	return sb.String()
}
