package d2

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	debugPort = 4421
)

// DownloadParams contains parameters for downloading profiles from d2
type DownloadParams struct {
	Service   string
	Namespace string // defaults to "default"
	OutDir    string
	Seconds   int // duration for CPU profile (default 30)
}

// DownloadResult contains the results of a profile download
type DownloadResult struct {
	Service   string        `json:"service"`
	Namespace string        `json:"namespace"`
	PodName   string        `json:"pod_name"`
	PodIP     string        `json:"pod_ip"`
	Files     []ProfileFile `json:"files"`
	Warnings  []string      `json:"warnings,omitempty"`
}

// ProfileFile represents a downloaded profile file
type ProfileFile struct {
	Type  string `json:"type"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// profileEndpoint represents a pprof endpoint to download
type profileEndpoint struct {
	name     string
	path     string
	filename string
	seconds  int // for CPU profile
}

// DownloadProfiles downloads pprof profiles from a d2 service
func DownloadProfiles(ctx context.Context, params DownloadParams) (DownloadResult, error) {
	if params.Service == "" {
		return DownloadResult{}, fmt.Errorf("service is required")
	}

	if params.OutDir == "" {
		return DownloadResult{}, fmt.Errorf("out_dir is required")
	}

	if params.Namespace == "" {
		params.Namespace = "default"
	}

	seconds := params.Seconds
	if seconds <= 0 {
		seconds = 30
	}

	result := DownloadResult{
		Service:   params.Service,
		Namespace: params.Namespace,
		Files:     []ProfileFile{},
		Warnings:  []string{},
	}

	// Step 1: Find the pod
	pod, err := FindPod(ctx, params.Service)
	if err != nil {
		return result, fmt.Errorf("failed to find pod: %w", err)
	}

	result.PodName = pod.Name
	result.PodIP = pod.IP

	// Step 2: Start port-forward
	pf, err := StartPortForward(ctx, pod, debugPort)
	if err != nil {
		return result, fmt.Errorf("failed to start port-forward: %w", err)
	}
	defer pf.Stop()

	localPort := pf.LocalPort()

	// Step 3: Get auth token
	token, err := GetToken(ctx, localPort)
	if err != nil {
		return result, fmt.Errorf("failed to get token: %w", err)
	}

	// Step 4: Create output directory
	if err := os.MkdirAll(params.OutDir, 0755); err != nil {
		return result, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Step 5: Download all profile types
	endpoints := []profileEndpoint{
		{name: "cpu", path: "/debug/pprof/profile", filename: "cpu.pprof", seconds: seconds},
		{name: "heap", path: "/debug/pprof/heap", filename: "heap.pprof"},
		{name: "goroutine", path: "/debug/pprof/goroutine", filename: "goroutines.pprof"},
		{name: "mutex", path: "/debug/pprof/mutex", filename: "mutex.pprof"},
		{name: "block", path: "/debug/pprof/block", filename: "block.pprof"},
		{name: "allocs", path: "/debug/pprof/allocs", filename: "allocs.pprof"},
	}

	for _, ep := range endpoints {
		file, err := downloadProfile(ctx, localPort, token, ep, params.OutDir, params.Service)
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Failed to download %s profile: %v", ep.name, err))
			continue
		}
		result.Files = append(result.Files, file)
	}

	if len(result.Files) == 0 {
		return result, fmt.Errorf("failed to download any profiles")
	}

	return result, nil
}

// downloadProfile downloads a single profile from the specified endpoint
func downloadProfile(ctx context.Context, localPort int, token string, ep profileEndpoint, outDir, service string) (ProfileFile, error) {
	url := fmt.Sprintf("https://127.0.0.1:%d%s", localPort, ep.path)

	// Add seconds parameter for CPU profile
	if ep.seconds > 0 {
		url = fmt.Sprintf("%s?seconds=%d", url, ep.seconds)
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: time.Duration(ep.seconds+60) * time.Second, // Extra time for CPU profile
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ProfileFile{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Add auth token header
	req.Header.Set("Ductone-Token", token)

	resp, err := client.Do(req)
	if err != nil {
		return ProfileFile{}, fmt.Errorf("failed to download profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ProfileFile{}, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Generate filename with timestamp
	timestamp := time.Now().UTC().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s", service, timestamp, ep.filename)
	outPath := filepath.Join(outDir, filename)

	// Write profile to file
	outFile, err := os.Create(outPath)
	if err != nil {
		return ProfileFile{}, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return ProfileFile{}, fmt.Errorf("failed to write profile: %w", err)
	}

	// Convert type name to match Datadog convention
	typeName := ep.name
	if typeName == "goroutine" {
		typeName = "goroutines"
	}

	return ProfileFile{
		Type:  typeName,
		Path:  outPath,
		Bytes: written,
	}, nil
}

// ListAvailableServices returns a list of available services that can be profiled
func ListAvailableServices(ctx context.Context) ([]string, error) {
	return ListServices(ctx)
}

// NormalizeServiceName ensures the service name follows the expected format
func NormalizeServiceName(service string) string {
	// Remove common prefixes if provided with pod name
	service = strings.TrimSpace(service)

	// If it's already in the correct format, return it
	if strings.HasPrefix(service, "be-") || strings.HasPrefix(service, "pub-") {
		return service
	}

	// Try to infer from common patterns
	// e.g., "innkeeper" -> "be-innkeeper"
	if !strings.Contains(service, "-") {
		return "be-" + service
	}

	return service
}
