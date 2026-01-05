package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveBaselineStoreAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baselines.json")

	store := baselineStore{
		Entries: map[string]*baselineEntry{
			"service:env|cpu|default": {
				Key:         "service:env|cpu|default",
				ProfileKind: "cpu",
				SampleIndex: "default",
				UpdatedAt:   "2024-01-01T00:00:00Z",
				Samples:     1,
				Functions: map[string]*baselineFunction{
					"main.main": {AvgFlatPct: 10.0, AvgCumPct: 12.0, Count: 1},
				},
			},
		},
	}

	if err := saveBaselineStore(path, store); err != nil {
		t.Fatalf("saveBaselineStore failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read baseline store: %v", err)
	}

	var decoded baselineStore
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("baseline store invalid JSON: %v", err)
	}
	if _, ok := decoded.Entries["service:env|cpu|default"]; !ok {
		t.Fatalf("expected baseline entry to be present")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".pprof-mcp-baselines-") {
			t.Fatalf("unexpected temp file left behind: %s", entry.Name())
		}
	}
}
