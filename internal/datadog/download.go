package datadog

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const defaultSite = "us3.datadoghq.com"

var profileTypes = map[string]string{
	"cpu.pprof":         "cpu",
	"delta-heap.pprof":  "heap",
	"delta-mutex.pprof": "mutex",
	"delta-block.pprof": "block",
	"goroutines.pprof":  "goroutines",
}

type DownloadParams struct {
	Service string
	Env     string
	OutDir  string
	Site    string
	Hours   int
	Now     time.Time
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
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", apiKey)
	req.Header.Set("DD-APPLICATION-KEY", appKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("datadog list failed: status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if _, ok := result["errors"]; ok {
		return nil, fmt.Errorf("datadog list returned errors: %v", result["errors"])
	}

	return result, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("DD-API-KEY", apiKey)
	req.Header.Set("DD-APPLICATION-KEY", appKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("profile download failed: status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
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
