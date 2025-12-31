package pprofparse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTop(t *testing.T) {
	output := `Showing nodes accounting for 1.23s, 99% of 1.24s total
      flat  flat%   sum%        cum   cum%   name
     0.50s 40.32% 40.32%      0.70s 56.45%  main.run
     0.30s 24.19% 64.51%      0.40s 32.26%  runtime.mallocgc`

	report := ParseTop(output)
	require.Len(t, report.Rows, 2)
	require.Equal(t, "main.run", report.Rows[0].Name)
	require.NotNil(t, report.Rows[0].FlatSeconds)
	require.InDelta(t, 0.50, *report.Rows[0].FlatSeconds, 0.0001)
}

func TestParseTimeToSeconds(t *testing.T) {
	value, ok := parseTimeToSeconds("250ms")
	require.True(t, ok)
	require.InDelta(t, 0.25, value, 0.0001)
}

func TestDiffTop(t *testing.T) {
	before := []TopRow{{Name: "main.run", FlatSeconds: ptr(0.5), CumSeconds: ptr(0.7)}}
	after := []TopRow{{Name: "main.run", FlatSeconds: ptr(0.8), CumSeconds: ptr(1.0)}}

	deltas := DiffTop(before, after, false)
	require.Len(t, deltas, 1)
	require.Equal(t, "main.run", deltas[0]["name"])
	require.InDelta(t, 0.3, deltas[0]["delta_seconds"].(float64), 0.0001)
}

func ptr(val float64) *float64 {
	return &val
}

