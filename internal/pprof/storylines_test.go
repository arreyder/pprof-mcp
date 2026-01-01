package pprof

import (
	"testing"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
)

func TestFirstAppFrame(t *testing.T) {
	stack := []string{
		"runtime.goexit",
		"net/http.(*Server).Serve",
		"gitlab.com/ductone/c1/pkg/api.DoThing",
		"github.com/conductorone/other/pkg.Run",
	}
	prefixes := []string{"gitlab.com/ductone/c1"}
	frame := firstAppFrame(stack, prefixes)
	require.Equal(t, "gitlab.com/ductone/c1/pkg/api.DoThing", frame)
}

func TestDetectBestSampleIndex(t *testing.T) {
	tests := []struct {
		name        string
		sampleTypes []string
		expected    string
	}{
		{
			name:        "heap profile with alloc and inuse",
			sampleTypes: []string{"alloc_objects", "alloc_space", "inuse_objects", "inuse_space"},
			expected:    "alloc_space",
		},
		{
			name:        "cpu profile",
			sampleTypes: []string{"samples", "cpu"},
			expected:    "",
		},
		{
			name:        "goroutine profile",
			sampleTypes: []string{"goroutine", "goroutines"},
			expected:    "",
		},
		{
			name:        "only inuse (delta heap)",
			sampleTypes: []string{"inuse_objects", "inuse_space"},
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prof := &profile.Profile{}
			for _, st := range tt.sampleTypes {
				prof.SampleType = append(prof.SampleType, &profile.ValueType{Type: st})
			}
			result := detectBestSampleIndex(prof)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFindSampleIndex(t *testing.T) {
	prof := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "alloc_objects"},
			{Type: "alloc_space"},
			{Type: "inuse_objects"},
			{Type: "inuse_space"},
		},
		DefaultSampleType: "inuse_space",
	}

	tests := []struct {
		name        string
		sampleType  string
		expectedIdx int
	}{
		{"find alloc_space", "alloc_space", 1},
		{"find inuse_space", "inuse_space", 3},
		{"empty uses default", "", 3},
		{"not found returns 0", "nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := findSampleIndex(prof, tt.sampleType)
			require.Equal(t, tt.expectedIdx, idx)
		})
	}
}

