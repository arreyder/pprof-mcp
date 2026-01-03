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
	"time"
)

// MetricsAtTimestampParams configures the metrics query.
type MetricsAtTimestampParams struct {
	Service   string
	Env       string
	Site      string
	Timestamp string   // RFC3339 format
	Window    string   // Duration around timestamp (e.g., "5m", "15m")
	Metrics   []string // Specific metrics to query (optional)
	PodName   string   // Optional pod name filter
}

// MetricDataPoint represents a single data point.
type MetricDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MetricSeries represents a metric's time series.
type MetricSeries struct {
	Name       string            `json:"name"`
	Tags       map[string]string `json:"tags,omitempty"`
	Points     []MetricDataPoint `json:"points"`
	Unit       string            `json:"unit,omitempty"`
	AvgValue   float64           `json:"avg_value"`
	MaxValue   float64           `json:"max_value"`
	MinValue   float64           `json:"min_value"`
	LastValue  float64           `json:"last_value"`
}

// MetricsAtTimestampResult contains metrics around the specified time.
type MetricsAtTimestampResult struct {
	Service     string         `json:"service"`
	Env         string         `json:"env"`
	DDSite      string         `json:"dd_site"`
	CenterTime  time.Time      `json:"center_time"`
	FromTime    time.Time      `json:"from_time"`
	ToTime      time.Time      `json:"to_time"`
	PodName     string         `json:"pod_name,omitempty"`
	Metrics     []MetricSeries `json:"metrics"`
	Summary     MetricsSummary `json:"summary"`
	Warnings    []string       `json:"warnings,omitempty"`
}

// MetricsSummary provides a quick overview of key metrics.
type MetricsSummary struct {
	GoGoroutines     *float64 `json:"go_goroutines,omitempty"`
	GoHeapInuseBytes *float64 `json:"go_heap_inuse_bytes,omitempty"`
	GoAllocBytes     *float64 `json:"go_alloc_bytes,omitempty"`
	GoGCPauseNs      *float64 `json:"go_gc_pause_ns,omitempty"`
	ContainerRSSMB   *float64 `json:"container_rss_mb,omitempty"`
	ContainerCPU     *float64 `json:"container_cpu_percent,omitempty"`
}

// Default metrics to query for Go services
var defaultGoMetrics = []string{
	"go.goroutines",
	"go.memstats.heap_inuse_bytes",
	"go.memstats.heap_alloc_bytes",
	"go.memstats.heap_sys_bytes",
	"go.memstats.alloc_bytes",
	"go.memstats.gc_pause_ns",
	"go.gc.pause_ns",
	"go.gc.count",
	"container.memory.rss",
	"container.memory.usage",
	"container.cpu.usage",
	"kubernetes.memory.rss",
	"kubernetes.cpu.usage.total",
}

// QueryMetricsAtTimestamp fetches metrics around a specific timestamp.
func QueryMetricsAtTimestamp(ctx context.Context, params MetricsAtTimestampParams) (MetricsAtTimestampResult, error) {
	result := MetricsAtTimestampResult{
		Service:  params.Service,
		Env:      params.Env,
		PodName:  params.PodName,
		Metrics:  []MetricSeries{},
		Warnings: []string{},
	}

	if params.Service == "" {
		return result, fmt.Errorf("service is required")
	}

	// Parse timestamp
	centerTime, err := parseMetricTimestamp(params.Timestamp)
	if err != nil {
		return result, fmt.Errorf("invalid timestamp: %w", err)
	}
	result.CenterTime = centerTime

	// Parse window
	window := 5 * time.Minute // default
	if params.Window != "" {
		parsed, err := time.ParseDuration(params.Window)
		if err != nil {
			return result, fmt.Errorf("invalid window duration: %w", err)
		}
		window = parsed
	}

	result.FromTime = centerTime.Add(-window)
	result.ToTime = centerTime.Add(window)

	site := params.Site
	if site == "" {
		site = os.Getenv("DD_SITE")
	}
	if site == "" {
		site = defaultSite
	}
	result.DDSite = site

	apiKey, appKey, err := loadKeys()
	if err != nil {
		return result, err
	}

	// Determine which metrics to query
	metricsToQuery := params.Metrics
	if len(metricsToQuery) == 0 {
		metricsToQuery = defaultGoMetrics
	}

	// Build tag filter
	tagFilter := buildTagFilter(params.Service, params.Env, params.PodName)

	// Query each metric
	for _, metricName := range metricsToQuery {
		series, err := queryMetricSeries(ctx, site, apiKey, appKey, metricName, tagFilter, result.FromTime, result.ToTime)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("query for %s failed: %v", metricName, err))
			continue
		}

		if len(series.Points) > 0 {
			result.Metrics = append(result.Metrics, series)
		}
	}

	// Sort metrics by name
	sort.Slice(result.Metrics, func(i, j int) bool {
		return result.Metrics[i].Name < result.Metrics[j].Name
	})

	// Build summary
	result.Summary = buildMetricsSummary(result.Metrics)

	return result, nil
}

func parseMetricTimestamp(ts string) (time.Time, error) {
	if ts == "" {
		return time.Now(), nil
	}

	// Try RFC3339
	t, err := time.Parse(time.RFC3339, ts)
	if err == nil {
		return t, nil
	}

	// Try RFC3339Nano
	t, err = time.Parse(time.RFC3339Nano, ts)
	if err == nil {
		return t, nil
	}

	// Try Unix timestamp (seconds)
	var unix int64
	if _, err := fmt.Sscanf(ts, "%d", &unix); err == nil {
		return time.Unix(unix, 0), nil
	}

	return time.Time{}, fmt.Errorf("unrecognized timestamp format: %s", ts)
}

func buildTagFilter(service, env, podName string) string {
	parts := []string{}

	if service != "" {
		// Try common service tag patterns
		parts = append(parts, fmt.Sprintf("service:%s", service))
	}
	if env != "" {
		parts = append(parts, fmt.Sprintf("env:%s", env))
	}
	if podName != "" {
		parts = append(parts, fmt.Sprintf("pod_name:%s", podName))
	}

	return strings.Join(parts, ",")
}

func queryMetricSeries(ctx context.Context, site, apiKey, appKey, metricName, tagFilter string, from, to time.Time) (MetricSeries, error) {
	series := MetricSeries{
		Name:   metricName,
		Points: []MetricDataPoint{},
		Tags:   map[string]string{},
	}

	// Build query
	query := metricName
	if tagFilter != "" {
		query = fmt.Sprintf("%s{%s}", metricName, tagFilter)
	}

	// Use the v1 query endpoint
	queryURL := fmt.Sprintf("https://api.%s/api/v1/query", site)
	params := url.Values{}
	params.Set("from", fmt.Sprintf("%d", from.Unix()))
	params.Set("to", fmt.Sprintf("%d", to.Unix()))
	params.Set("query", query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL+"?"+params.Encode(), nil)
	if err != nil {
		return series, err
	}
	req.Header.Set("DD-API-KEY", apiKey)
	req.Header.Set("DD-APPLICATION-KEY", appKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return series, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return series, fmt.Errorf("query failed: status %d", resp.StatusCode)
	}

	var result struct {
		Series []struct {
			Metric     string          `json:"metric"`
			PointList  [][]float64     `json:"pointlist"`
			TagSet     []string        `json:"tag_set"`
			Unit       []struct {
				Name string `json:"name"`
			} `json:"unit"`
		} `json:"series"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return series, err
	}

	if len(result.Series) == 0 {
		return series, nil
	}

	// Use first series
	s := result.Series[0]
	series.Name = s.Metric

	// Parse tags
	for _, tag := range s.TagSet {
		parts := strings.SplitN(tag, ":", 2)
		if len(parts) == 2 {
			series.Tags[parts[0]] = parts[1]
		}
	}

	// Parse unit
	if len(s.Unit) > 0 {
		series.Unit = s.Unit[0].Name
	}

	// Parse points
	var sum, min, max float64
	first := true
	for _, point := range s.PointList {
		if len(point) >= 2 {
			ts := time.Unix(int64(point[0]/1000), 0)
			val := point[1]

			series.Points = append(series.Points, MetricDataPoint{
				Timestamp: ts,
				Value:     val,
			})

			sum += val
			if first {
				min = val
				max = val
				first = false
			} else {
				if val < min {
					min = val
				}
				if val > max {
					max = val
				}
			}
		}
	}

	if len(series.Points) > 0 {
		series.AvgValue = sum / float64(len(series.Points))
		series.MinValue = min
		series.MaxValue = max
		series.LastValue = series.Points[len(series.Points)-1].Value
	}

	return series, nil
}

func buildMetricsSummary(metrics []MetricSeries) MetricsSummary {
	summary := MetricsSummary{}

	for _, m := range metrics {
		if len(m.Points) == 0 {
			continue
		}

		val := m.LastValue

		switch {
		case strings.Contains(m.Name, "go.goroutines"):
			summary.GoGoroutines = &val
		case strings.Contains(m.Name, "heap_inuse"):
			summary.GoHeapInuseBytes = &val
		case strings.Contains(m.Name, "alloc_bytes") || strings.Contains(m.Name, "heap_alloc"):
			summary.GoAllocBytes = &val
		case strings.Contains(m.Name, "gc_pause") || strings.Contains(m.Name, "pause_ns"):
			summary.GoGCPauseNs = &val
		case strings.Contains(m.Name, "container.memory.rss") || strings.Contains(m.Name, "kubernetes.memory.rss"):
			mb := val / (1024 * 1024)
			summary.ContainerRSSMB = &mb
		case strings.Contains(m.Name, "container.cpu") || strings.Contains(m.Name, "kubernetes.cpu"):
			summary.ContainerCPU = &val
		}
	}

	return summary
}

// FormatMetricsAtTimestamp formats the result as a readable table.
func FormatMetricsAtTimestamp(result MetricsAtTimestampResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Service: %s, Env: %s\n", result.Service, result.Env))
	sb.WriteString(fmt.Sprintf("Time Window: %s to %s (center: %s)\n",
		result.FromTime.Format(time.RFC3339),
		result.ToTime.Format(time.RFC3339),
		result.CenterTime.Format(time.RFC3339)))
	if result.PodName != "" {
		sb.WriteString(fmt.Sprintf("Pod: %s\n", result.PodName))
	}
	sb.WriteString("\n")

	// Summary
	sb.WriteString("=== Summary ===\n")
	if result.Summary.GoGoroutines != nil {
		sb.WriteString(fmt.Sprintf("  Goroutines:     %.0f\n", *result.Summary.GoGoroutines))
	}
	if result.Summary.GoHeapInuseBytes != nil {
		mb := *result.Summary.GoHeapInuseBytes / (1024 * 1024)
		sb.WriteString(fmt.Sprintf("  Heap In-Use:    %.1f MB\n", mb))
	}
	if result.Summary.GoAllocBytes != nil {
		mb := *result.Summary.GoAllocBytes / (1024 * 1024)
		sb.WriteString(fmt.Sprintf("  Heap Alloc:     %.1f MB\n", mb))
	}
	if result.Summary.ContainerRSSMB != nil {
		sb.WriteString(fmt.Sprintf("  Container RSS:  %.1f MB\n", *result.Summary.ContainerRSSMB))
	}
	if result.Summary.ContainerCPU != nil {
		sb.WriteString(fmt.Sprintf("  Container CPU:  %.2f%%\n", *result.Summary.ContainerCPU))
	}
	sb.WriteString("\n")

	// Detailed metrics
	sb.WriteString("=== Metrics ===\n")
	sb.WriteString(fmt.Sprintf("%-50s  %12s  %12s  %12s  %s\n", "METRIC", "LAST", "AVG", "MAX", "UNIT"))
	sb.WriteString(strings.Repeat("-", 100) + "\n")

	for _, m := range result.Metrics {
		unit := m.Unit
		if unit == "" {
			unit = "-"
		}
		sb.WriteString(fmt.Sprintf("%-50s  %12.2f  %12.2f  %12.2f  %s\n",
			m.Name, m.LastValue, m.AvgValue, m.MaxValue, unit))
	}

	return sb.String()
}
