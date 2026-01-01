package pprof

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNoMatches indicates a regex/pattern matched no symbols in a profile.
var ErrNoMatches = errors.New("pprof: no matches found")

func wrapNoMatches(err error, stderr string) error {
	if err == nil {
		return nil
	}
	if !strings.Contains(stderr, "no matches found for regexp") {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrNoMatches, strings.TrimSpace(stderr))
}
