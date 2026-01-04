package datadog

import (
	"testing"
	"time"
)

func TestParseRetryAfterSeconds(t *testing.T) {
	delay := parseRetryAfter("5")
	if delay != 5*time.Second {
		t.Fatalf("expected 5s delay, got %v", delay)
	}
}

func TestParseRetryAfterDate(t *testing.T) {
	target := time.Now().Add(2 * time.Second).UTC()
	delay := parseRetryAfter(target.Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	if delay <= 0 {
		t.Fatalf("expected positive delay, got %v", delay)
	}
	if delay > 3*time.Second {
		t.Fatalf("expected delay <= 3s, got %v", delay)
	}
}

func TestParseRetryAfterCap(t *testing.T) {
	delay := parseRetryAfter("600")
	if delay != maxRetryAfter {
		t.Fatalf("expected cap at %v, got %v", maxRetryAfter, delay)
	}
}

func TestHostRateLimiterReserve(t *testing.T) {
	limiter := newHostRateLimiter(2, 2)
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	if delay := limiter.reserve("example.com", now); delay != 0 {
		t.Fatalf("expected first token immediately, got %v", delay)
	}
	if delay := limiter.reserve("example.com", now); delay != 0 {
		t.Fatalf("expected burst token immediately, got %v", delay)
	}

	delay := limiter.reserve("example.com", now)
	if delay < 490*time.Millisecond || delay > 510*time.Millisecond {
		t.Fatalf("expected ~500ms delay, got %v", delay)
	}

	delay = limiter.reserve("example.com", now.Add(500*time.Millisecond))
	if delay != 0 {
		t.Fatalf("expected token after refill, got %v", delay)
	}
}
