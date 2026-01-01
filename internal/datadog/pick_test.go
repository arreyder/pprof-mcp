package datadog

import (
	"testing"
)

func TestPickAnomalous(t *testing.T) {
	tests := []struct {
		name           string
		candidates     []ProfileCandidate
		wantOK         bool
		wantMinZScore  float64
		wantIdx        int // expected index of anomalous candidate
	}{
		{
			name:       "too few candidates",
			candidates: []ProfileCandidate{{}, {}},
			wantOK:     false,
		},
		{
			name: "no numeric fields",
			candidates: []ProfileCandidate{
				{ProfileID: "a"},
				{ProfileID: "b"},
				{ProfileID: "c"},
			},
			wantOK: false,
		},
		{
			name: "no anomaly - all values similar",
			candidates: []ProfileCandidate{
				{ProfileID: "a", NumericFields: map[string]float64{"cpu-samples": 100}},
				{ProfileID: "b", NumericFields: map[string]float64{"cpu-samples": 101}},
				{ProfileID: "c", NumericFields: map[string]float64{"cpu-samples": 99}},
				{ProfileID: "d", NumericFields: map[string]float64{"cpu-samples": 100}},
			},
			wantOK: false, // z-scores will be < 2
		},
		{
			name: "clear anomaly on cpu-samples",
			candidates: []ProfileCandidate{
				{ProfileID: "normal1", NumericFields: map[string]float64{"cpu-samples": 100}},
				{ProfileID: "normal2", NumericFields: map[string]float64{"cpu-samples": 100}},
				{ProfileID: "normal3", NumericFields: map[string]float64{"cpu-samples": 100}},
				{ProfileID: "normal4", NumericFields: map[string]float64{"cpu-samples": 100}},
				{ProfileID: "normal5", NumericFields: map[string]float64{"cpu-samples": 100}},
				{ProfileID: "anomaly", NumericFields: map[string]float64{"cpu-samples": 1000}}, // 10x normal
			},
			wantOK:        true,
			wantMinZScore: 2.0,
			wantIdx:       5,
		},
		{
			name: "anomaly on alloc_space",
			candidates: []ProfileCandidate{
				{ProfileID: "normal1", NumericFields: map[string]float64{"alloc_space": 1000000}},
				{ProfileID: "normal2", NumericFields: map[string]float64{"alloc_space": 1000000}},
				{ProfileID: "normal3", NumericFields: map[string]float64{"alloc_space": 1000000}},
				{ProfileID: "normal4", NumericFields: map[string]float64{"alloc_space": 1000000}},
				{ProfileID: "anomaly", NumericFields: map[string]float64{"alloc_space": 50000000}}, // 50x normal
				{ProfileID: "normal5", NumericFields: map[string]float64{"alloc_space": 1000000}},
			},
			wantOK:        true,
			wantMinZScore: 2.0,
			wantIdx:       4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate, zScore, field, ok := pickAnomalous(tt.candidates)

			if ok != tt.wantOK {
				t.Errorf("pickAnomalous() ok = %v, want %v", ok, tt.wantOK)
				return
			}

			if !tt.wantOK {
				return
			}

			if zScore < tt.wantMinZScore {
				t.Errorf("pickAnomalous() zScore = %v, want >= %v", zScore, tt.wantMinZScore)
			}

			if candidate.ProfileID != tt.candidates[tt.wantIdx].ProfileID {
				t.Errorf("pickAnomalous() candidate = %v, want %v", candidate.ProfileID, tt.candidates[tt.wantIdx].ProfileID)
			}

			if field == "" {
				t.Errorf("pickAnomalous() field is empty")
			}
		})
	}
}

func TestMeanStddev(t *testing.T) {
	tests := []struct {
		name       string
		values     []float64
		wantMean   float64
		wantStddev float64
		tolerance  float64
	}{
		{
			name:       "empty",
			values:     []float64{},
			wantMean:   0,
			wantStddev: 0,
		},
		{
			name:       "single value",
			values:     []float64{5},
			wantMean:   5,
			wantStddev: 0,
		},
		{
			name:       "uniform values",
			values:     []float64{10, 10, 10, 10},
			wantMean:   10,
			wantStddev: 0,
		},
		{
			name:       "simple variance",
			values:     []float64{2, 4, 4, 4, 5, 5, 7, 9},
			wantMean:   5,
			wantStddev: 2,
			tolerance:  0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mean, stddev := meanStddev(tt.values)

			if tt.tolerance == 0 {
				tt.tolerance = 0.0001
			}

			if abs(mean-tt.wantMean) > tt.tolerance {
				t.Errorf("meanStddev() mean = %v, want %v", mean, tt.wantMean)
			}

			if abs(stddev-tt.wantStddev) > tt.tolerance {
				t.Errorf("meanStddev() stddev = %v, want %v", stddev, tt.wantStddev)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
