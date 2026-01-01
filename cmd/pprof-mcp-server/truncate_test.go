package main

import "testing"

func TestTruncateLinesNoLimit(t *testing.T) {
	raw := "a\nb\n"
	trimmed, total, truncated := truncateLines(raw, 0)
	if trimmed != raw {
		t.Fatalf("expected raw output, got %q", trimmed)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if truncated {
		t.Fatalf("expected not truncated")
	}
}

func TestTruncateLinesLimit(t *testing.T) {
	raw := "a\nb\n"
	trimmed, total, truncated := truncateLines(raw, 1)
	if trimmed != "a" {
		t.Fatalf("expected trimmed output 'a', got %q", trimmed)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if !truncated {
		t.Fatalf("expected truncated")
	}
}

func TestTruncateLinesEmpty(t *testing.T) {
	raw := ""
	trimmed, total, truncated := truncateLines(raw, 3)
	if trimmed != raw {
		t.Fatalf("expected empty output, got %q", trimmed)
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %d", total)
	}
	if truncated {
		t.Fatalf("expected not truncated")
	}
}
