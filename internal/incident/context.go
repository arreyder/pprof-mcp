// Package incident provides incident context integration for pprof-mcp.
// It reads the incident context set by c1-ops-mcp to auto-route profiles.
package incident

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	contextDir  = ".c1-ops"
	contextFile = "incident-context.json"
)

// Context represents the current incident context (read from c1-ops-mcp).
type Context struct {
	ID          string    `json:"id"`
	Description string    `json:"description,omitempty"`
	OpenedAt    time.Time `json:"opened_at"`
	BaseDir     string    `json:"base_dir"`
}

func contextPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, contextDir, contextFile), nil
}

// Current returns the current incident context, or nil if none is open.
func Current() (*Context, error) {
	path, err := contextPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var ctx Context
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, err
	}

	return &ctx, nil
}

// ProfilesDir returns the profiles directory for the current incident.
// Returns empty string if no incident is open.
func ProfilesDir() string {
	ctx, err := Current()
	if err != nil || ctx == nil {
		return ""
	}
	return filepath.Join(ctx.BaseDir, "profiles")
}

// ResolveOutDir returns the appropriate output directory:
// - If outDir is provided, use it
// - If an incident is open, use the incident's profiles directory
// - Otherwise return empty string (caller should error)
func ResolveOutDir(outDir string) (string, string) {
	if outDir != "" {
		return outDir, ""
	}

	ctx, err := Current()
	if err != nil || ctx == nil {
		return "", ""
	}

	profilesDir := filepath.Join(ctx.BaseDir, "profiles")
	return profilesDir, ctx.ID
}
