package main

import (
	"os"
	"testing"
)

func TestToolNameModeFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected toolNameMode
	}{
		{"", toolNameModeDefault},
		{"default", toolNameModeDefault},
		{"codex", toolNameModeCodex},
		{"CODEX", toolNameModeDefault}, // case sensitive, handled by caller
		{"invalid", toolNameModeDefault},
	}
	for _, tt := range tests {
		got := toolNameModeFromString(tt.input)
		if got != tt.expected {
			t.Errorf("toolNameModeFromString(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestToolNameForMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     toolNameMode
		expected string
	}{
		{"pprof.top", toolNameModeDefault, "pprof.top"},
		{"pprof.top", toolNameModeCodex, "pprof_top"},
		{"datadog.profiles.list", toolNameModeDefault, "datadog.profiles.list"},
		{"datadog.profiles.list", toolNameModeCodex, "datadog_profiles_list"},
		{"profiles.download_latest_bundle", toolNameModeDefault, "profiles.download_latest_bundle"},
		{"profiles.download_latest_bundle", toolNameModeCodex, "profiles_download_latest_bundle"},
		{"no_dots", toolNameModeDefault, "no_dots"},
		{"no_dots", toolNameModeCodex, "no_dots"},
	}
	for _, tt := range tests {
		got := toolNameForMode(tt.name, tt.mode)
		if got != tt.expected {
			t.Errorf("toolNameForMode(%q, %q) = %q, want %q", tt.name, tt.mode, got, tt.expected)
		}
	}
}

func TestToolNameModeFromEnv(t *testing.T) {
	tests := []struct {
		envValue string
		expected toolNameMode
	}{
		{"", toolNameModeDefault},
		{"codex", toolNameModeCodex},
		{"CODEX", toolNameModeCodex},
		{"  codex  ", toolNameModeCodex},
		{"default", toolNameModeDefault},
		{"invalid", toolNameModeDefault},
	}

	for _, tt := range tests {
		os.Setenv("PPROF_MCP_TOOL_NAME_MODE", tt.envValue)
		got := toolNameModeFromEnv()
		if got != tt.expected {
			t.Errorf("toolNameModeFromEnv() with env=%q = %q, want %q", tt.envValue, got, tt.expected)
		}
	}
	os.Unsetenv("PPROF_MCP_TOOL_NAME_MODE")
}
