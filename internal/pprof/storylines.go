package pprof

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/pprof/profile"

	"github.com/arreyder/pprof-mcp/internal/pprofparse"
)

type StorylinesParams struct {
	Profile      string
	N            int
	Focus        string
	Ignore       string
	RepoPrefixes []string
	RepoRoot     string
	TrimPath     string
	SampleIndex  string // Optional: if empty, auto-detects based on profile type
}

type StorylinesResult struct {
	Command    string      `json:"command"`
	Storylines []Storyline `json:"storylines"`
	Warnings   []string    `json:"warnings,omitempty"`
}

type Storyline struct {
	LeafHotspot string            `json:"leaf_hotspot"`
	Cum         string            `json:"cum"`
	CumPct      string            `json:"cum_pct"`
	CallChain   []string          `json:"call_chain"`
	FirstApp    string            `json:"first_app_frame"`
	Evidence    StorylineEvidence `json:"evidence"`
	Warnings    []string          `json:"warnings,omitempty"`
}

type StorylineEvidence struct {
	TopRow   map[string]any  `json:"top_row"`
	PeekLeaf EvidenceOutput  `json:"peek_leaf"`
	PeekApp  EvidenceOutput  `json:"peek_first_app"`
	ListApp  EvidenceOutput  `json:"list_first_app"`
}

type EvidenceOutput struct {
	Command   string `json:"command"`
	Raw       string `json:"raw"`
	Truncated bool   `json:"truncated"`
}

func RunStorylines(ctx context.Context, params StorylinesParams) (StorylinesResult, error) {
	if params.Profile == "" {
		return StorylinesResult{}, fmt.Errorf("profile is required")
	}
	count := params.N
	if count == 0 {
		count = 4
	}
	if count < 2 {
		count = 2
	}
	if count > 6 {
		count = 6
	}

	repoPrefixes := params.RepoPrefixes
	if len(repoPrefixes) == 0 {
		repoPrefixes = []string{"gitlab.com/ductone/c1", "github.com/conductorone"}
	}

	// Parse profile first to detect type and auto-select sample_index
	prof, err := parseProfile(params.Profile)
	if err != nil {
		return StorylinesResult{}, err
	}

	// Auto-detect sample_index for heap profiles
	sampleIndex := params.SampleIndex
	if sampleIndex == "" {
		sampleIndex = detectBestSampleIndex(prof)
	}

	topReport, err := RunTop(ctx, TopParams{
		Profile:     params.Profile,
		Cum:         true,
		NodeCount:   40,
		Focus:       params.Focus,
		Ignore:      params.Ignore,
		SampleIndex: sampleIndex,
	})
	if err != nil {
		return StorylinesResult{}, err
	}

	// Find the value index matching our sample_index for call chain analysis
	defaultIndex := findSampleIndex(prof, sampleIndex)

	storylines := []Storyline{}
	for _, row := range topReport.Rows {
		if len(storylines) >= count {
			break
		}
		storyline := buildStoryline(ctx, row, prof, defaultIndex, repoPrefixes, params, sampleIndex)
		storylines = append(storylines, storyline)
	}

	return StorylinesResult{
		Command:    topReport.Command,
		Storylines: storylines,
	}, nil
}

// detectBestSampleIndex returns the best sample index for analysis based on profile type.
// For heap profiles, uses alloc_space (total allocations) over inuse_space (current live).
// For other profiles, returns empty string to use pprof defaults.
func detectBestSampleIndex(prof *profile.Profile) string {
	hasAllocSpace := false
	hasInuseSpace := false
	for _, st := range prof.SampleType {
		switch st.Type {
		case "alloc_space", "alloc_objects":
			hasAllocSpace = true
		case "inuse_space", "inuse_objects":
			hasInuseSpace = true
		}
	}
	// If this looks like a heap profile, prefer alloc_space for allocation analysis
	if hasAllocSpace && hasInuseSpace {
		return "alloc_space"
	}
	return ""
}

// findSampleIndex returns the index of the given sample type in the profile.
func findSampleIndex(prof *profile.Profile, sampleType string) int {
	if sampleType == "" {
		// Use default
		if prof.DefaultSampleType != "" {
			for i, st := range prof.SampleType {
				if st.Type == prof.DefaultSampleType {
					return i
				}
			}
		}
		return 0
	}
	for i, st := range prof.SampleType {
		if st.Type == sampleType {
			return i
		}
	}
	return 0
}

func buildStoryline(ctx context.Context, row pprofparse.TopRow, prof *profile.Profile, valueIndex int, prefixes []string, params StorylinesParams, sampleIndex string) Storyline {
	warnings := []string{}
	leaf := row.Name

	chain, firstApp := findCallChain(prof, leaf, valueIndex, prefixes)
	if len(chain) == 0 {
		warnings = append(warnings, "no call chain inferred; leaf not found in samples")
	}
	if firstApp == "" {
		warnings = append(warnings, "no app-owned frame found")
	}

	peekLeaf := runEvidencePeek(ctx, params.Profile, leaf, sampleIndex)
	peekApp := EvidenceOutput{}
	listApp := EvidenceOutput{}
	if firstApp != "" {
		peekApp = runEvidencePeek(ctx, params.Profile, firstApp, sampleIndex)
		if params.RepoRoot != "" {
			listApp = runEvidenceList(ctx, params.Profile, firstApp, params)
		}
	}

	return Storyline{
		LeafHotspot: leaf,
		Cum:         row.Cum,
		CumPct:      row.CumPct,
		CallChain:   chain,
		FirstApp:    firstApp,
		Evidence: StorylineEvidence{
			TopRow: map[string]any{
				"flat":     row.Flat,
				"flat_pct": row.FlatPct,
				"sum_pct":  row.SumPct,
				"cum":      row.Cum,
				"cum_pct":  row.CumPct,
				"name":     row.Name,
			},
			PeekLeaf: peekLeaf,
			PeekApp:  peekApp,
			ListApp:  listApp,
		},
		Warnings: warnings,
	}
}

func parseProfile(path string) (*profile.Profile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return profile.Parse(file)
}

func findCallChain(prof *profile.Profile, leaf string, valueIndex int, prefixes []string) ([]string, string) {
	var bestChain []string
	var bestFirstApp string
	var bestValue int64
	for _, sample := range prof.Sample {
		stack := sampleStack(sample)
		if !stackContains(stack, leaf) {
			continue
		}
		value := int64(0)
		if valueIndex < len(sample.Value) {
			value = sample.Value[valueIndex]
		}
		if value > bestValue {
			bestValue = value
			bestChain = stack
			bestFirstApp = firstAppFrame(stack, prefixes)
		}
	}

	if len(bestChain) == 0 {
		return nil, ""
	}

	reverse(bestChain)
	if len(bestChain) > 12 {
		bestChain = bestChain[len(bestChain)-12:]
	}
	return bestChain, bestFirstApp
}

func sampleStack(sample *profile.Sample) []string {
	frames := []string{}
	for _, loc := range sample.Location {
		name := functionName(loc)
		if name == "" {
			continue
		}
		frames = append(frames, name)
	}
	return frames
}

func functionName(loc *profile.Location) string {
	if loc == nil {
		return ""
	}
	if len(loc.Line) > 0 && loc.Line[0].Function != nil {
		return loc.Line[0].Function.Name
	}
	return ""
}

func stackContains(stack []string, target string) bool {
	for _, frame := range stack {
		if frame == target {
			return true
		}
	}
	return false
}

func firstAppFrame(stack []string, prefixes []string) string {
	for _, frame := range stack {
		for _, prefix := range prefixes {
			if strings.HasPrefix(frame, prefix) || strings.Contains(frame, prefix) {
				return frame
			}
		}
	}
	return ""
}

func runEvidencePeek(ctx context.Context, profilePath, symbol, sampleIndex string) EvidenceOutput {
	result, err := RunPeek(ctx, PeekParams{Profile: profilePath, Regex: symbol, SampleIndex: sampleIndex})
	if err != nil {
		return EvidenceOutput{}
	}
	raw, truncated := truncate(result.Raw, 4000)
	return EvidenceOutput{Command: result.Command, Raw: raw, Truncated: truncated}
}

func runEvidenceList(ctx context.Context, profilePath, symbol string, params StorylinesParams) EvidenceOutput {
	result, err := RunList(ctx, ListParams{
		Profile:  profilePath,
		Function: symbol,
		RepoRoot: params.RepoRoot,
		TrimPath: params.TrimPath,
	})
	if err != nil {
		return EvidenceOutput{}
	}
	raw, truncated := truncate(result.Raw, 4000)
	return EvidenceOutput{Command: result.Command, Raw: raw, Truncated: truncated}
}

func truncate(value string, limit int) (string, bool) {
	if len(value) <= limit {
		return value, false
	}
	return value[:limit], true
}

func reverse(items []string) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

