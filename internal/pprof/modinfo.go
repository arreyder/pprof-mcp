package pprof

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ModInfo struct {
	ModulePath string
	Versions   map[string]string
}

func ParseGoMod(repoRoot string) (ModInfo, error) {
	path := filepath.Join(repoRoot, "go.mod")
	content, err := os.ReadFile(path)
	if err != nil {
		return ModInfo{}, err
	}

	info := ModInfo{
		Versions: map[string]string{},
	}
	lines := strings.Split(string(content), "\n")
	inRequire := false

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		line = strings.Split(line, "//")[0]
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "module ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				info.ModulePath = trimQuotes(fields[1])
			}
			continue
		}

		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if inRequire {
			if strings.HasPrefix(line, ")") {
				inRequire = false
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				info.Versions[trimQuotes(fields[0])] = trimQuotes(fields[1])
			}
			continue
		}

		if strings.HasPrefix(line, "require ") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				info.Versions[trimQuotes(fields[1])] = trimQuotes(fields[2])
			} else {
				return info, fmt.Errorf("invalid require line: %q", line)
			}
		}
	}

	return info, nil
}

func moduleVersionForPackage(info ModInfo, packagePath string) (string, string) {
	if packagePath == "" {
		return "", ""
	}
	var bestModule string
	for module := range info.Versions {
		if strings.HasPrefix(packagePath, module) {
			if len(module) > len(bestModule) {
				bestModule = module
			}
		}
	}
	if bestModule == "" {
		return "", ""
	}
	return bestModule, info.Versions[bestModule]
}

func trimQuotes(value string) string {
	return strings.Trim(value, "\"")
}
