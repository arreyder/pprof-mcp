package main

import (
	"os"
	"strings"
)

type toolNameMode string

const (
	toolNameModeDefault toolNameMode = "default"
	toolNameModeCodex   toolNameMode = "codex"
)

func toolNameModeFromEnv() toolNameMode {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("PPROF_MCP_TOOL_NAME_MODE")))
	return toolNameModeFromString(mode)
}

func toolNameModeFromString(value string) toolNameMode {
	if value == string(toolNameModeCodex) {
		return toolNameModeCodex
	}
	return toolNameModeDefault
}

func toolNameForMode(name string, mode toolNameMode) string {
	if mode == toolNameModeCodex {
		return strings.ReplaceAll(name, ".", "_")
	}
	return name
}
