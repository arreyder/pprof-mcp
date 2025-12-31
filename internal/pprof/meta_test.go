package pprof

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
)

func TestRunMeta(t *testing.T) {
	prof := &profile.Profile{
		SampleType: []*profile.ValueType{{Type: "samples", Unit: "count"}},
		Sample: []*profile.Sample{
			{Value: []int64{10}},
			{Value: []int64{5}},
		},
		DefaultSampleType: "samples",
		Period:            1000000,
		DurationNanos:     5000000000,
		TimeNanos:         123456789,
		Comments:          []string{"go version go1.25.5"},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test_cpu.pprof")
	file, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, prof.Write(file))
	require.NoError(t, file.Close())

	meta, err := RunMeta(path)
	require.NoError(t, err)
	require.Equal(t, "cpu", meta.DetectedKind)
	require.Len(t, meta.SampleTypes, 1)
	require.Equal(t, int64(15), meta.Totals[0].Total)
	require.Equal(t, 0, meta.DefaultSampleIndex)
	require.NotNil(t, meta.GoVersion)
}

