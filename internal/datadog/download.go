package datadog

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultSite = "us3.datadoghq.com"

const (
	defaultMaxRetries     = 5
	defaultRateLimitRPS   = 2.0
	defaultRateLimitBurst = 4
	defaultBaseBackoff    = 200 * time.Millisecond
	defaultMaxBackoff     = 5 * time.Second
	maxRetryAfter         = 60 * time.Second
	maxRequestBodyBytes   = 1 << 20
)

var profileTypes = map[string]string{
	"cpu.pprof":         "cpu",
	"delta-heap.pprof":  "heap",
	"delta-mutex.pprof": "mutex",
	"delta-block.pprof": "block",
	"goroutines.pprof":  "goroutines",
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

type DownloadParams struct {
	Service   string
	Env       string
	OutDir    string
	Site      string
	Hours     int
	Now       time.Time
	ProfileID string
	EventID   string
}

type DownloadResult struct {
	Service     string        `json:"service"`
	Env         string        `json:"env"`
	DDSite      string        `json:"dd_site"`
	FromTS      string        `json:"from_ts"`
	ToTS        string        `json:"to_ts"`
	ProfileID   string        `json:"profile_id"`
	EventID     string        `json:"event_id"`
	Timestamp   string        `json:"timestamp"`
	Files       []ProfileFile `json:"files"`
	MetricsPath string        `json:"metrics_path,omitempty"`
	Warnings    []string      `json:"warnings,omitempty"`
}

type ProfileFile struct {
	Type  string `json:"type"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

func DownloadLatestBundle(ctx context.Context, params DownloadParams) (DownloadResult, error) {
	if params.Service == "" || params.Env == "" || params.OutDir == "" {
		return DownloadResult{}, errors.New("service, env, and out_dir are required")
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
		return DownloadResult{}, err
	}

	hours := params.Hours
	if hours <= 0 {
		hours = 72
	}

	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	from := now.Add(-time.Duration(hours) * time.Hour)

	fromTS := from.Format(time.RFC3339)
	toTS := now.Format(time.RFC3339)

	profileID := params.ProfileID
	eventID := params.EventID
	timestamp := ""
	resultWarnings := []string{}

	if profileID != "" || eventID != "" {
		if profileID == "" || eventID == "" {
			return DownloadResult{}, errors.New("profile_id and event_id must be provided together")
		}
		resultWarnings = append(resultWarnings, "timestamp unavailable; profile selected explicitly")
	} else {
		listPayload := map[string]any{
			"filter": map[string]any{
				"from":  fromTS,
				"to":    toTS,
				"query": fmt.Sprintf("service:%s env:%s", params.Service, params.Env),
			},
			"sort": map[string]any{
				"field": "timestamp",
				"order": "desc",
			},
			"limit": 1,
		}

		listResp, err := doRequest(ctx, http.MethodPost, fmt.Sprintf("https://%s/api/unstable/profiles/list", site), apiKey, appKey, listPayload)
		if err != nil {
			return DownloadResult{}, err
		}

		profileID, eventID, timestamp, err = extractProfileMetadata(listResp)
		if err != nil {
			return DownloadResult{}, err
		}
	}

	downloadURL := fmt.Sprintf("https://%s/api/ui/profiling/profiles/%s/download?eventId=%s", site, profileID, eventID)
	zipBytes, err := downloadZip(ctx, downloadURL, apiKey, appKey)
	if err != nil {
		return DownloadResult{}, err
	}

	files, metricsPath, err := extractProfiles(zipBytes, params.Service, params.Env, params.OutDir)
	if err != nil {
		return DownloadResult{}, err
	}

	result := DownloadResult{
		Service:     params.Service,
		Env:         params.Env,
		DDSite:      site,
		FromTS:      fromTS,
		ToTS:        toTS,
		ProfileID:   profileID,
		EventID:     eventID,
		Timestamp:   timestamp,
		Files:       files,
		MetricsPath: metricsPath,
		Warnings:    resultWarnings,
	}

	return result, nil
}

func doRequest(ctx context.Context, method, url, apiKey, appKey string, payload any) (map[string]any, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	respBody, status, err := doRequestWithRetry(ctx, method, url, apiKey, appKey, bodyBytes, "application/json", 60*time.Second)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, fmt.Errorf("datadog list failed: status %d: %s", status, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if _, ok := result["errors"]; ok {
		return nil, fmt.Errorf("datadog list returned errors: %v", result["errors"])
	}

	return result, nil
}

func doRequestWithRetry(ctx context.Context, method, urlStr, apiKey, appKey string, body []byte, contentType string, timeout time.Duration) ([]byte, int, error) {
	if len(body) > maxRequestBodyBytes {
		return nil, 0, fmt.Errorf("datadog request body too large (%d bytes)", len(body))
	}
	attempts := maxRetries()
	if attempts < 1 {
		attempts = 1
	}
	client := &http.Client{Timeout: timeout}
	host := hostFromURL(urlStr)
	limiter := getRateLimiter()

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := limiter.Wait(ctx, host); err != nil {
			return nil, 0, err
		}
		req, err := newRequest(ctx, method, urlStr, apiKey, appKey, body, contentType)
		if err != nil {
			return nil, 0, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, readErr
		}
		if !shouldRetry(resp.StatusCode) {
			return respBody, resp.StatusCode, nil
		}
		if attempt == attempts {
			return respBody, resp.StatusCode, fmt.Errorf("datadog request failed: status %d: %s", resp.StatusCode, string(respBody))
		}
		wait := retryDelay(resp, attempt)
		if err := sleepWithContext(ctx, wait); err != nil {
			return nil, resp.StatusCode, err
		}
	}
	return nil, 0, errors.New("datadog request failed")
}

func newRequest(ctx context.Context, method, urlStr, apiKey, appKey string, body []byte, contentType string) (*http.Request, error) {
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, reader)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if apiKey != "" {
		req.Header.Set("DD-API-KEY", apiKey)
	}
	if appKey != "" {
		req.Header.Set("DD-APPLICATION-KEY", appKey)
	}
	return req, nil
}

func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	backoff := backoffDelay(attempt)
	if resp.StatusCode == http.StatusTooManyRequests {
		if retryAfter := parseRetryAfter(resp.Header.Get("Retry-After")); retryAfter > 0 {
			return retryAfter
		}
	}
	return backoff
}

func backoffDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := defaultBaseBackoff * time.Duration(1<<uint(attempt-1))
	if delay > defaultMaxBackoff {
		delay = defaultMaxBackoff
	}
	jitterRange := delay / 4
	if jitterRange <= 0 {
		return delay
	}
	jitter := time.Duration(rand.Int63n(int64(jitterRange)))
	return delay + jitter
}

func parseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		return capRetryAfter(time.Duration(seconds) * time.Second)
	}
	if parsed, err := http.ParseTime(raw); err == nil {
		wait := time.Until(parsed)
		if wait < 0 {
			return 0
		}
		return capRetryAfter(wait)
	}
	return 0
}

func capRetryAfter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	if delay > maxRetryAfter {
		return maxRetryAfter
	}
	return delay
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

type hostRateLimiter struct {
	mu      sync.Mutex
	rps     float64
	burst   float64
	buckets map[string]*rateBucket
}

type rateBucket struct {
	tokens float64
	last   time.Time
}

func newHostRateLimiter(rps float64, burst int) *hostRateLimiter {
	if rps <= 0 || burst <= 0 {
		return &hostRateLimiter{
			rps:     0,
			burst:   0,
			buckets: map[string]*rateBucket{},
		}
	}
	return &hostRateLimiter{
		rps:     rps,
		burst:   float64(burst),
		buckets: map[string]*rateBucket{},
	}
}

func (l *hostRateLimiter) Wait(ctx context.Context, host string) error {
	if l == nil || l.rps <= 0 || l.burst <= 0 || host == "" {
		return nil
	}
	for {
		wait := l.reserve(host, time.Now())
		if wait <= 0 {
			return nil
		}
		if err := sleepWithContext(ctx, wait); err != nil {
			return err
		}
	}
}

func (l *hostRateLimiter) reserve(host string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket := l.buckets[host]
	if bucket == nil {
		bucket = &rateBucket{
			tokens: l.burst,
			last:   now,
		}
		l.buckets[host] = bucket
	}
	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	bucket.tokens = minFloat64(l.burst, bucket.tokens+elapsed*l.rps)
	bucket.last = now
	if bucket.tokens >= 1 {
		bucket.tokens -= 1
		return 0
	}
	missing := 1 - bucket.tokens
	if missing < 0 {
		missing = 0
	}
	if l.rps <= 0 {
		return 0
	}
	return time.Duration(missing / l.rps * float64(time.Second))
}

var (
	rateLimiterOnce sync.Once
	rateLimiter     *hostRateLimiter
)

func getRateLimiter() *hostRateLimiter {
	rateLimiterOnce.Do(func() {
		rateLimiter = newHostRateLimiter(rateLimitRPS(), rateLimitBurst())
	})
	return rateLimiter
}

func rateLimitRPS() float64 {
	raw := strings.TrimSpace(os.Getenv("PPROF_MCP_DD_RPS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("PPROF_MCP_DD_RATE_LIMIT_RPS"))
	}
	if raw == "" {
		return defaultRateLimitRPS
	}
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return defaultRateLimitRPS
	}
	if val <= 0 {
		return 0
	}
	return val
}

func rateLimitBurst() int {
	raw := strings.TrimSpace(os.Getenv("PPROF_MCP_DD_BURST"))
	if raw == "" {
		return defaultRateLimitBurst
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val < 1 {
		return defaultRateLimitBurst
	}
	return val
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxRetries() int {
	raw := strings.TrimSpace(os.Getenv("PPROF_MCP_DD_MAX_RETRIES"))
	if raw == "" {
		return defaultMaxRetries
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val < 1 {
		return defaultMaxRetries
	}
	return val
}

func extractProfileMetadata(listResp map[string]any) (string, string, string, error) {
	data, ok := listResp["data"].([]any)
	if !ok || len(data) == 0 {
		return "", "", "", errors.New("no profiles found")
	}
	entry, ok := data[0].(map[string]any)
	if !ok {
		return "", "", "", errors.New("unexpected datadog response format")
	}

	// profile-id is nested inside attributes
	profileID := getStringNested(entry, "attributes", "profile-id")
	if profileID == "" {
		profileID = getString(entry, "profile-id")
	}
	if profileID == "" {
		profileID = getString(entry, "profile_id")
	}
	eventID := getString(entry, "id")
	timestamp := getStringNested(entry, "attributes", "timestamp")
	if timestamp == "" {
		timestamp = getString(entry, "timestamp")
	}

	if profileID == "" || eventID == "" {
		return "", "", "", errors.New("missing profile id or event id in response")
	}

	return profileID, eventID, timestamp, nil
}

func downloadZip(ctx context.Context, url, apiKey, appKey string) ([]byte, error) {
	respBody, status, err := doRequestWithRetry(ctx, http.MethodGet, url, apiKey, appKey, nil, "", 120*time.Second)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("profile download failed: status %d: %s", status, string(respBody))
	}
	return respBody, nil
}

func extractProfiles(zipBytes []byte, service, env, outDir string) ([]ProfileFile, string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, "", err
	}

	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, "", err
	}

	workDir, err := os.MkdirTemp("", "gofast-profiles-*")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(workDir)

	for _, file := range reader.File {
		// Sanitize the file name to prevent path traversal attacks
		cleanName := filepath.Clean(file.Name)
		if cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
			return nil, "", fmt.Errorf("invalid path in zip: %s", file.Name)
		}
		path := filepath.Join(workDir, cleanName)

		// Verify the path is within the work directory
		rel, err := filepath.Rel(workDir, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, "", fmt.Errorf("path traversal detected in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return nil, "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, "", err
		}
		in, err := file.Open()
		if err != nil {
			return nil, "", err
		}
		out, err := os.Create(path)
		if err != nil {
			in.Close()
			return nil, "", err
		}
		if _, err := io.Copy(out, in); err != nil {
			in.Close()
			out.Close()
			return nil, "", err
		}
		in.Close()
		out.Close()
	}

	pprofFiles := []string{}
	metricsPath := ""
	walkErr := filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() == "metrics.json" {
			metricsPath = path
			return nil
		}
		if strings.HasSuffix(info.Name(), ".pprof") {
			pprofFiles = append(pprofFiles, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, "", walkErr
	}

	if len(pprofFiles) == 0 {
		return nil, "", errors.New("no .pprof files found in bundle")
	}

	sort.Strings(pprofFiles)
	var outputs []ProfileFile
	for _, pprofPath := range pprofFiles {
		base := filepath.Base(pprofPath)
		dest := filepath.Join(outDir, fmt.Sprintf("%s_%s_%s", service, env, base))
		if err := copyFile(pprofPath, dest); err != nil {
			return nil, "", err
		}
		info, err := os.Stat(dest)
		if err != nil {
			return nil, "", err
		}
		fileType := "unknown"
		if mapped, ok := profileTypes[base]; ok {
			fileType = mapped
		}
		outputs = append(outputs, ProfileFile{
			Type:  fileType,
			Path:  dest,
			Bytes: info.Size(),
		})
	}

	metricsOut := ""
	if metricsPath != "" {
		metricsOut = filepath.Join(outDir, fmt.Sprintf("%s_%s_metrics.json", service, env))
		if err := copyFile(metricsPath, metricsOut); err != nil {
			return nil, "", err
		}
	}

	return outputs, metricsOut, nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func getString(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func getStringNested(m map[string]any, key, nested string) string {
	if val, ok := m[key]; ok {
		if sub, ok := val.(map[string]any); ok {
			return getString(sub, nested)
		}
	}
	return ""
}
