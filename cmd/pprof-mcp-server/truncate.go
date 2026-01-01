package main

import "strings"

func truncateLines(raw string, maxLines int) (string, int, bool) {
	if maxLines <= 0 {
		return raw, countLines(raw), false
	}
	lines := splitLines(raw)
	if len(lines) == 0 {
		return raw, 0, false
	}
	if len(lines) <= maxLines {
		return raw, len(lines), false
	}
	return strings.Join(lines[:maxLines], "\n"), len(lines), true
}

func splitLines(raw string) []string {
	trimmed := strings.TrimSuffix(raw, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func countLines(raw string) int {
	return len(splitLines(raw))
}
