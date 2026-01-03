package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizePathStrictRejectsSymlinkEscape(t *testing.T) {
	baseDir := t.TempDir()
	outsideDir := t.TempDir()

	linkPath := filepath.Join(baseDir, "link")
	if err := os.Symlink(outsideDir, linkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := sanitizePathStrict(baseDir, "link/escape.txt")
	if err == nil {
		t.Fatalf("expected symlink escape to be rejected")
	}
}
