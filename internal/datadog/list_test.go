package datadog

import (
	"testing"
	"time"
)

func TestParseRelativeOrAbsoluteTimeNow(t *testing.T) {
	start := time.Now().UTC()
	value, err := parseRelativeOrAbsoluteTime("now", start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if parsed.Before(start.Add(-2*time.Second)) || parsed.After(time.Now().UTC().Add(2*time.Second)) {
		t.Fatalf("unexpected now value: %s", value)
	}
}

func TestParseRelativeOrAbsoluteTimeRelative(t *testing.T) {
	start := time.Now().UTC()
	value, err := parseRelativeOrAbsoluteTime("-1h", start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	expected := start.Add(-1 * time.Hour)
	if parsed.Before(expected.Add(-2*time.Second)) || parsed.After(expected.Add(2*time.Second)) {
		t.Fatalf("unexpected relative value: %s", value)
	}
}

func TestParseRelativeOrAbsoluteTimeRFC3339(t *testing.T) {
	value, err := parseRelativeOrAbsoluteTime("2025-01-02T03:04:05Z", time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "2025-01-02T03:04:05Z" {
		t.Fatalf("unexpected value: %s", value)
	}
}
