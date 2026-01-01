package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var pathArgKeys = map[string]bool{
	"profile":           true,
	"binary":            true,
	"output_path":       true,
	"repo_root":         true,
	"out_dir":           true,
	"heap_profile":      true,
	"goroutine_profile": true,
	"before":            true,
	"after":             true,
	"baseline_path":     true,
}

var pathSliceArgKeys = map[string]bool{
	"profiles":     true,
	"source_paths": true,
}

func sanitizeArgs(args map[string]any) (map[string]any, error) {
	if len(args) == 0 {
		return args, nil
	}
	baseDir := strings.TrimSpace(os.Getenv("PPROF_MCP_BASEDIR"))
	if baseDir != "" {
		baseDir = filepath.Clean(baseDir)
	}

	cleaned := make(map[string]any, len(args))
	for key, value := range args {
		switch {
		case pathArgKeys[key]:
			str, ok := value.(string)
			if !ok {
				cleaned[key] = value
				continue
			}
			var path string
			var err error
			if isHandle(str) {
				path, err = resolveHandlePath(baseDir, str)
			} else {
				path, err = sanitizePath(baseDir, str)
			}
			if err != nil {
				expected := "valid path"
				if baseDir != "" {
					expected = fmt.Sprintf("path within base dir %q", baseDir)
				}
				return nil, &ValidationError{
					Field:    key,
					Message:  err.Error(),
					Expected: expected,
					Received: redactValue(key, str),
				}
			}
			cleaned[key] = path
		case pathSliceArgKeys[key]:
			paths, ok := sliceValue(value)
			if !ok {
				cleaned[key] = value
				continue
			}
			out := make([]string, 0, len(paths))
			for _, raw := range paths {
				str, ok := raw.(string)
				if !ok {
					return nil, fmt.Errorf("invalid path value in %q", key)
				}
				var path string
				var err error
				if isHandle(str) {
					path, err = resolveHandlePath(baseDir, str)
				} else {
					path, err = sanitizePath(baseDir, str)
				}
				if err != nil {
					expected := "valid path"
					if baseDir != "" {
						expected = fmt.Sprintf("path within base dir %q", baseDir)
					}
					return nil, &ValidationError{
						Field:    key,
						Message:  err.Error(),
						Expected: expected,
						Received: redactValue(key, str),
					}
				}
				out = append(out, path)
			}
			cleaned[key] = out
		default:
			cleaned[key] = value
		}
	}
	return cleaned, nil
}

func sanitizePath(baseDir, value string) (string, error) {
	if value == "" {
		return value, nil
	}
	cleaned := filepath.Clean(value)
	if baseDir == "" {
		return cleaned, nil
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("invalid base dir %q: %w", baseDir, err)
	}

	var absPath string
	if filepath.IsAbs(cleaned) {
		absPath, err = filepath.Abs(cleaned)
	} else {
		absPath, err = filepath.Abs(filepath.Join(baseAbs, cleaned))
	}
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", value, err)
	}

	rel, err := filepath.Rel(baseAbs, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside base dir %q", value, baseAbs)
	}
	return absPath, nil
}
