package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveBundlePaths(value any) (map[string]string, []string, error) {
	paths := map[string]string{}
	warnings := []string{}
	baseDir := strings.TrimSpace(os.Getenv("PPROF_MCP_BASEDIR"))
	if baseDir != "" {
		baseDir = filepath.Clean(baseDir)
	}

	switch typed := value.(type) {
	case string:
		if typed == "" {
			return paths, warnings, fmt.Errorf("bundle handle is required")
		}
		metas, err := findBundleMetas(typed)
		if err != nil {
			return nil, warnings, err
		}
		for _, meta := range metas {
			if meta.Type == "" {
				warnings = append(warnings, fmt.Sprintf("profile %q missing type", meta.Path))
				continue
			}
			path, err := sanitizePath(baseDir, meta.Path)
			if err != nil {
				return nil, warnings, err
			}
			if _, exists := paths[meta.Type]; !exists {
				paths[meta.Type] = path
			}
		}
	case []any:
		for _, item := range typed {
			obj, ok := item.(map[string]any)
			if !ok {
				return nil, warnings, fmt.Errorf("bundle entries must be objects with handle/type")
			}
			handle, ok := obj["handle"].(string)
			if !ok || strings.TrimSpace(handle) == "" {
				return nil, warnings, fmt.Errorf("bundle entry missing handle")
			}
			meta, err := resolveHandleMeta(handle)
			if err != nil {
				return nil, warnings, err
			}
			typ := ""
			if t, ok := obj["type"].(string); ok {
				typ = strings.TrimSpace(t)
			}
			if typ == "" {
				typ = meta.Type
			}
			if typ == "" {
				warnings = append(warnings, fmt.Sprintf("profile %q missing type", meta.Path))
				continue
			}
			path, err := sanitizePath(baseDir, meta.Path)
			if err != nil {
				return nil, warnings, err
			}
			if _, exists := paths[typ]; !exists {
				paths[typ] = path
			}
		}
	default:
		return nil, warnings, fmt.Errorf("bundle must be a handle or list of handles")
	}

	return paths, warnings, nil
}
