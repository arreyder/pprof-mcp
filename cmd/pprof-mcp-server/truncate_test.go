package main

import (
	"testing"

	"github.com/arreyder/pprof-mcp/internal/textutil"
)

func TestTruncateLinesNoLimit(t *testing.T) {
	raw := "a\nb\n"
	result := textutil.TruncateText(raw, textutil.TruncateOptions{MaxLines: 0})
	if result.Text != raw {
		t.Fatalf("expected raw output, got %q", result.Text)
	}
	if result.Meta.TotalLines != 2 {
		t.Fatalf("expected total 2, got %d", result.Meta.TotalLines)
	}
	if result.Meta.Truncated {
		t.Fatalf("expected not truncated")
	}
}

func TestTruncateLinesLimit(t *testing.T) {
	raw := "a\nb\n"
	result := textutil.TruncateText(raw, textutil.TruncateOptions{MaxLines: 1})
	if result.Text != "a" {
		t.Fatalf("expected trimmed output 'a', got %q", result.Text)
	}
	if result.Meta.TotalLines != 2 {
		t.Fatalf("expected total 2, got %d", result.Meta.TotalLines)
	}
	if !result.Meta.Truncated {
		t.Fatalf("expected truncated")
	}
}

func TestTruncateLinesEmpty(t *testing.T) {
	raw := ""
	result := textutil.TruncateText(raw, textutil.TruncateOptions{MaxLines: 3})
	if result.Text != raw {
		t.Fatalf("expected empty output, got %q", result.Text)
	}
	if result.Meta.TotalLines != 0 {
		t.Fatalf("expected total 0, got %d", result.Meta.TotalLines)
	}
	if result.Meta.Truncated {
		t.Fatalf("expected not truncated")
	}
}
