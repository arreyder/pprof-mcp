package services

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ServiceInfo struct {
	Binary  string `json:"binary"`
	Service string `json:"service"`
	Path    string `json:"path"`
}

func Discover(repoRoot string) ([]ServiceInfo, error) {
	cmdDir := filepath.Join(repoRoot, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return nil, err
	}

	services := make([]ServiceInfo, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "be-") {
			continue
		}
		service := strings.TrimPrefix(name, "be-")
		service = strings.ReplaceAll(service, "-", "_")
		services = append(services, ServiceInfo{
			Binary:  name,
			Service: service,
			Path:    filepath.Join("cmd", name),
		})
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Binary < services[j].Binary
	})

	return services, nil
}

