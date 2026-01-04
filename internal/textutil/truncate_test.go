package textutil

import (
	"strings"
	"testing"
)

func TestTruncateTextMaxBytesSingleLine(t *testing.T) {
	raw := strings.Repeat("a", 10)
	result := TruncateText(raw, TruncateOptions{MaxBytes: 5, Strategy: StrategyHead})
	if result.Text != "aaaaa" {
		t.Fatalf("expected truncated text, got %q", result.Text)
	}
	if result.Meta.TotalBytes != 10 {
		t.Fatalf("expected total bytes 10, got %d", result.Meta.TotalBytes)
	}
	if result.Meta.TotalLines != 1 {
		t.Fatalf("expected total lines 1, got %d", result.Meta.TotalLines)
	}
	if !result.Meta.Truncated {
		t.Fatalf("expected truncated=true")
	}
	if result.Meta.TruncatedReason != "max_bytes" {
		t.Fatalf("expected truncated_reason=max_bytes, got %q", result.Meta.TruncatedReason)
	}
	if result.Meta.Strategy != string(StrategyHead) {
		t.Fatalf("expected strategy head, got %q", result.Meta.Strategy)
	}
}

func TestTruncateTextMaxLines(t *testing.T) {
	raw := "a\nb\nc\n"
	result := TruncateText(raw, TruncateOptions{MaxLines: 2, Strategy: StrategyHead})
	if result.Text != "a\nb" {
		t.Fatalf("expected first 2 lines, got %q", result.Text)
	}
	if result.Meta.TotalLines != 3 {
		t.Fatalf("expected total lines 3, got %d", result.Meta.TotalLines)
	}
	if !result.Meta.Truncated {
		t.Fatalf("expected truncated=true")
	}
	if result.Meta.TruncatedReason != "max_lines" {
		t.Fatalf("expected truncated_reason=max_lines, got %q", result.Meta.TruncatedReason)
	}
}

func TestTruncateTextHeadTailStrategy(t *testing.T) {
	raw := "one\ntwo\nthree\nfour\nfive\n"
	result := TruncateText(raw, TruncateOptions{MaxLines: 3, Strategy: StrategyHeadTail})
	if !strings.Contains(result.Text, truncateMarker) {
		t.Fatalf("expected truncate marker, got %q", result.Text)
	}
	if result.Meta.Strategy != string(StrategyHeadTail) {
		t.Fatalf("expected strategy head_tail, got %q", result.Meta.Strategy)
	}
	if !result.Meta.Truncated {
		t.Fatalf("expected truncated=true")
	}
}
