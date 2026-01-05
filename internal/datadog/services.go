package datadog

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// ListServicesParams configures the service discovery request.
type ListServicesParams struct {
	Env     string // Optional: filter by environment prefix
	Site    string // Datadog site
	Minutes int    // How far back to look (default: 15)
}

// ServiceInfo represents a discovered service with profiling enabled.
type ServiceInfo struct {
	Name         string   `json:"name"`
	Environments []string `json:"environments,omitempty"`
	LastSeen     string   `json:"last_seen,omitempty"`
}

// ListServicesResult contains discovered services.
type ListServicesResult struct {
	Services []ServiceInfo `json:"services"`
	DDSite   string        `json:"dd_site"`
	CachedAt string        `json:"cached_at,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
}

// ListServicesWithProfiling discovers all services that have profiling enabled
// by querying recent profiles and extracting unique service names.
func ListServicesWithProfiling(ctx context.Context, params ListServicesParams) (ListServicesResult, error) {
	site := params.Site
	if site == "" {
		site = os.Getenv("DD_SITE")
	}
	if site == "" {
		site = defaultSite
	}

	minutes := params.Minutes
	if minutes <= 0 {
		minutes = 15
	}

	apiKey, appKey, err := loadKeys()
	if err != nil {
		return ListServicesResult{}, err
	}

	now := time.Now()
	toTS := now.UTC().Format(time.RFC3339)
	fromTS := now.Add(-time.Duration(minutes) * time.Minute).UTC().Format(time.RFC3339)

	// Build query - try without service filter first, just env wildcard
	query := "*"
	if params.Env != "" {
		query = fmt.Sprintf("env:%s*", params.Env)
	}

	payload := map[string]any{
		"filter": map[string]any{
			"from":  fromTS,
			"to":    toTS,
			"query": query,
		},
		"sort": map[string]any{
			"field": "timestamp",
			"order": "desc",
		},
		"limit": 500, // High limit to capture more services
	}

	listResp, err := doRequest(ctx, "POST", fmt.Sprintf("https://%s/api/unstable/profiles/list", site), apiKey, appKey, payload)
	if err != nil {
		return ListServicesResult{}, fmt.Errorf("failed to list profiles: %w", err)
	}

	// Extract unique services from profile list response
	services, warnings := extractServicesFromResponse(listResp)

	// Filter by env prefix if specified
	if params.Env != "" {
		services = FilterServicesByEnvPrefix(services, params.Env)
	}

	return ListServicesResult{
		Services: services,
		DDSite:   site,
		Warnings: warnings,
	}, nil
}

// extractServicesFromResponse parses the profile list response and extracts unique services.
func extractServicesFromResponse(resp map[string]any) ([]ServiceInfo, []string) {
	var warnings []string

	data, ok := resp["data"].([]any)
	if !ok {
		return nil, []string{"unexpected response format: missing data array"}
	}

	// Map to track unique service+env combinations
	serviceEnvs := make(map[string]map[string]string) // service -> env -> lastSeen

	for _, item := range data {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}

		// Extract service and env from attributes
		attrs, _ := entry["attributes"].(map[string]any)
		if attrs == nil {
			attrs = entry // Fallback to top-level
		}

		service := extractServiceName(attrs)
		env := extractEnvName(attrs)
		timestamp := getStringNested(entry, "attributes", "timestamp")
		if timestamp == "" {
			timestamp = getString(entry, "timestamp")
		}

		if service == "" {
			continue
		}

		if serviceEnvs[service] == nil {
			serviceEnvs[service] = make(map[string]string)
		}

		// Track most recent timestamp per env
		if existing, ok := serviceEnvs[service][env]; !ok || timestamp > existing {
			serviceEnvs[service][env] = timestamp
		}
	}

	// Convert to ServiceInfo list
	var services []ServiceInfo
	for name, envMap := range serviceEnvs {
		var envs []string
		var lastSeen string
		for env, ts := range envMap {
			if env != "" {
				envs = append(envs, env)
			}
			if ts > lastSeen {
				lastSeen = ts
			}
		}
		sort.Strings(envs)
		services = append(services, ServiceInfo{
			Name:         name,
			Environments: envs,
			LastSeen:     lastSeen,
		})
	}

	// Sort by service name
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	return services, warnings
}

// extractServiceName tries to find the service name from profile attributes.
func extractServiceName(attrs map[string]any) string {
	// Try common field names
	for _, key := range []string{"service", "service_name", "service-name"} {
		if v := getString(attrs, key); v != "" {
			return v
		}
	}

	// Try tags array
	if tags, ok := attrs["tags"].([]any); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				if strings.HasPrefix(tagStr, "service:") {
					return strings.TrimPrefix(tagStr, "service:")
				}
			}
		}
	}

	return ""
}

// extractEnvName tries to find the environment name from profile attributes.
func extractEnvName(attrs map[string]any) string {
	// Try common field names
	for _, key := range []string{"env", "environment"} {
		if v := getString(attrs, key); v != "" {
			return v
		}
	}

	// Try tags array
	if tags, ok := attrs["tags"].([]any); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				if strings.HasPrefix(tagStr, "env:") {
					return strings.TrimPrefix(tagStr, "env:")
				}
			}
		}
	}

	return ""
}

// FilterServicesByEnvPrefix filters services to only those with environments matching the prefix.
func FilterServicesByEnvPrefix(services []ServiceInfo, prefix string) []ServiceInfo {
	prefix = strings.ToLower(prefix)
	var filtered []ServiceInfo

	for _, svc := range services {
		var matchingEnvs []string
		for _, env := range svc.Environments {
			if strings.HasPrefix(strings.ToLower(env), prefix) {
				matchingEnvs = append(matchingEnvs, env)
			}
		}
		if len(matchingEnvs) > 0 {
			filtered = append(filtered, ServiceInfo{
				Name:         svc.Name,
				Environments: matchingEnvs,
				LastSeen:     svc.LastSeen,
			})
		}
	}

	return filtered
}

// FormatServicesTable formats the service list as a readable table.
func FormatServicesTable(services []ServiceInfo) string {
	if len(services) == 0 {
		return "No services found with profiling enabled"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-30s  %-50s  %s\n", "SERVICE", "ENVIRONMENTS", "LAST SEEN"))
	sb.WriteString(strings.Repeat("-", 100) + "\n")

	for _, svc := range services {
		envs := strings.Join(svc.Environments, ", ")
		if len(envs) > 50 {
			envs = envs[:47] + "..."
		}
		lastSeen := svc.LastSeen
		if len(lastSeen) > 19 {
			lastSeen = lastSeen[:19] // Trim to datetime without timezone
		}
		sb.WriteString(fmt.Sprintf("%-30s  %-50s  %s\n", svc.Name, envs, lastSeen))
	}

	return sb.String()
}
