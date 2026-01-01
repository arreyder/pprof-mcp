package pprof

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

// RepoDetectionResult contains detected repository information from a profile.
type RepoDetectionResult struct {
	ModulePaths    []string          `json:"module_paths"`    // Detected Go module paths (e.g., gitlab.com/ductone/c1)
	DetectedRoot   string            `json:"detected_root"`   // Auto-detected local repo root
	DetectionNotes []string          `json:"detection_notes"` // Notes about how detection was done
	Confidence     string            `json:"confidence"`      // "high", "medium", "low", "none"
}

// DetectRepoFromProfile attempts to auto-detect repository information from a profile.
func DetectRepoFromProfile(prof *profile.Profile) RepoDetectionResult {
	result := RepoDetectionResult{
		ModulePaths:    []string{},
		DetectionNotes: []string{},
		Confidence:     "none",
	}

	// Collect unique module paths from function names
	moduleCounts := make(map[string]int)
	for _, loc := range prof.Location {
		for _, line := range loc.Line {
			if line.Function == nil {
				continue
			}
			name := line.Function.Name
			modPath := extractModulePath(name)
			if modPath != "" {
				moduleCounts[modPath]++
			}
		}
	}

	// Sort by frequency (most common first)
	type modCount struct {
		path  string
		count int
	}
	var sorted []modCount
	for path, count := range moduleCounts {
		sorted = append(sorted, modCount{path, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	// Take top module paths (likely the application's code)
	for i, mc := range sorted {
		if i >= 5 {
			break
		}
		result.ModulePaths = append(result.ModulePaths, mc.path)
	}

	if len(result.ModulePaths) == 0 {
		result.DetectionNotes = append(result.DetectionNotes, "no Go module paths detected in profile")
		return result
	}

	result.DetectionNotes = append(result.DetectionNotes,
		"detected module paths from function names")

	// Try to find local repo root
	for _, modPath := range result.ModulePaths {
		if root := findLocalRepoRoot(modPath); root != "" {
			result.DetectedRoot = root
			result.Confidence = "high"
			result.DetectionNotes = append(result.DetectionNotes,
				"found local repo at "+root+" matching "+modPath)
			break
		}
	}

	if result.DetectedRoot == "" {
		result.Confidence = "low"
		result.DetectionNotes = append(result.DetectionNotes,
			"could not find local repo matching detected modules")
	}

	return result
}

// extractModulePath extracts the Go module path from a fully qualified function name.
// e.g., "gitlab.com/ductone/c1/pkg/api.Handler" -> "gitlab.com/ductone/c1"
func extractModulePath(funcName string) string {
	// Skip runtime and standard library
	if strings.HasPrefix(funcName, "runtime.") ||
		strings.HasPrefix(funcName, "runtime/") ||
		!strings.Contains(funcName, ".") ||
		!strings.Contains(funcName, "/") {
		return ""
	}

	// Find the module path (everything before /pkg/, /cmd/, /internal/, or before the type/func name)
	parts := strings.Split(funcName, "/")

	// Look for common Go project structure markers
	for i, part := range parts {
		if part == "pkg" || part == "cmd" || part == "internal" || part == "vendor" {
			if i > 0 {
				return strings.Join(parts[:i], "/")
			}
		}
	}

	// If no marker found, try to find where the package name starts
	// (contains a dot that's part of a type or function name)
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.Contains(parts[i], ".") && !strings.HasPrefix(parts[i], ".") {
			// This part contains a function/type reference
			// Take everything before it, plus the package name
			if i >= 2 {
				return strings.Join(parts[:i], "/")
			}
		}
	}

	return ""
}

// findLocalRepoRoot tries to find a local directory that matches the module path.
func findLocalRepoRoot(modPath string) string {
	// Common locations to check
	homeDir, _ := os.UserHomeDir()
	searchPaths := []string{
		".",
		"..",
		"../..",
	}

	if homeDir != "" {
		searchPaths = append(searchPaths,
			filepath.Join(homeDir, "repos"),
			filepath.Join(homeDir, "src"),
			filepath.Join(homeDir, "go", "src"),
			filepath.Join(homeDir, "code"),
			filepath.Join(homeDir, "projects"),
		)
	}

	// Extract repo name from module path
	parts := strings.Split(modPath, "/")
	var repoName string
	if len(parts) >= 3 {
		repoName = parts[len(parts)-1] // e.g., "c1" from "gitlab.com/ductone/c1"
	}

	for _, searchPath := range searchPaths {
		// Try direct match
		candidatePath := filepath.Join(searchPath, repoName)
		if isValidRepoRoot(candidatePath, modPath) {
			absPath, err := filepath.Abs(candidatePath)
			if err == nil {
				return absPath
			}
			return candidatePath
		}

		// Try full module path structure (go/src/gitlab.com/ductone/c1)
		candidatePath = filepath.Join(searchPath, modPath)
		if isValidRepoRoot(candidatePath, modPath) {
			absPath, err := filepath.Abs(candidatePath)
			if err == nil {
				return absPath
			}
			return candidatePath
		}
	}

	return ""
}

// isValidRepoRoot checks if a path looks like a valid Go repo root.
func isValidRepoRoot(path, modPath string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check for go.mod
	goModPath := filepath.Join(path, "go.mod")
	if content, err := os.ReadFile(goModPath); err == nil {
		// Verify module path matches
		if strings.Contains(string(content), "module "+modPath) ||
			strings.Contains(string(content), "module \""+modPath+"\"") {
			return true
		}
	}

	// Check for common Go project markers
	markers := []string{"pkg", "cmd", "internal", "main.go"}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}

	return false
}
