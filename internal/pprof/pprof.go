package pprof

import (
	"context"
	"fmt"
	"strings"

	"github.com/arreyder/pprof-mcp/internal/pprofparse"
	"github.com/arreyder/pprof-mcp/internal/textutil"
)

type TopParams struct {
	Profile     string
	Binary      string
	Cum         bool
	NodeCount   int
	Focus       string
	Ignore      string
	SampleIndex string
}

type TopResult struct {
	Command    string                `json:"command"`
	Raw        string                `json:"raw"`
	RawMeta    textutil.TruncateMeta `json:"raw_meta,omitempty"`
	Stderr     string                `json:"stderr,omitempty"`
	StderrMeta textutil.TruncateMeta `json:"stderr_meta,omitempty"`
	Rows       []pprofparse.TopRow   `json:"rows"`
	Summary    pprofparse.TopSummary `json:"summary"`
	Hints      []string              `json:"hints,omitempty"` // Contextual hints based on profile type
}

type PeekParams struct {
	Profile     string
	Binary      string
	Regex       string
	SampleIndex string
}

type PeekResult struct {
	Command    string                `json:"command"`
	Raw        string                `json:"raw"`
	RawMeta    textutil.TruncateMeta `json:"raw_meta,omitempty"`
	Stderr     string                `json:"stderr,omitempty"`
	StderrMeta textutil.TruncateMeta `json:"stderr_meta,omitempty"`
}

type ListParams struct {
	Profile     string
	Binary      string
	Function    string
	RepoRoot    string
	TrimPath    string
	SourcePaths []string // Additional source paths for vendored dependencies
}

type ListResult struct {
	Command    string                `json:"command"`
	Raw        string                `json:"raw"`
	RawMeta    textutil.TruncateMeta `json:"raw_meta,omitempty"`
	Stderr     string                `json:"stderr,omitempty"`
	StderrMeta textutil.TruncateMeta `json:"stderr_meta,omitempty"`
}

type TracesParams struct {
	Profile string
	Binary  string
	Lines   int
}

type TracesResult struct {
	Command    string                `json:"command"`
	Raw        string                `json:"raw"`
	TotalLines int                   `json:"total_lines"`
	Truncated  bool                  `json:"truncated"`
	RawMeta    textutil.TruncateMeta `json:"raw_meta,omitempty"`
	Stderr     string                `json:"stderr,omitempty"`
	StderrMeta textutil.TruncateMeta `json:"stderr_meta,omitempty"`
}

type DiffTopParams struct {
	Before      string
	After       string
	Binary      string
	Cum         bool
	NodeCount   int
	Focus       string
	Ignore      string
	SampleIndex string
}

type DiffTopResult struct {
	Commands map[string]string   `json:"commands"`
	Before   []pprofparse.TopRow `json:"before"`
	After    []pprofparse.TopRow `json:"after"`
	Deltas   []map[string]any    `json:"deltas"`
}

func RunTop(ctx context.Context, params TopParams) (TopResult, error) {
	if params.Profile == "" {
		return TopResult{}, fmt.Errorf("pprof top requires profile")
	}

	pprofArgs := []string{"tool", "pprof", "-top"}
	if params.Cum {
		pprofArgs = append(pprofArgs, "-cum")
	}
	if params.NodeCount > 0 {
		pprofArgs = append(pprofArgs, "-nodecount", fmt.Sprintf("%d", params.NodeCount))
	}
	if params.Focus != "" {
		pprofArgs = append(pprofArgs, "-focus", params.Focus)
	}
	if params.Ignore != "" {
		pprofArgs = append(pprofArgs, "-ignore", params.Ignore)
	}
	if params.SampleIndex != "" {
		pprofArgs = append(pprofArgs, "-sample_index", params.SampleIndex)
	}
	pprofArgs = append(pprofArgs, buildProfileArgs(params.Binary, params.Profile)...)

	output, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		if noMatches := wrapNoMatches(err, output.Stderr); noMatches != nil {
			return TopResult{}, noMatches
		}
		return TopResult{}, fmt.Errorf("pprof top failed: %w\n%s", err, output.Stderr)
	}

	report := pprofparse.ParseTop(output.Stdout)
	return TopResult{
		Command:    shellJoin(append([]string{"go"}, pprofArgs...)),
		Raw:        output.Stdout,
		RawMeta:    output.StdoutMeta,
		Stderr:     output.Stderr,
		StderrMeta: output.StderrMeta,
		Rows:       report.Rows,
		Summary:    report.Summary,
	}, nil
}

func RunPeek(ctx context.Context, params PeekParams) (PeekResult, error) {
	if params.Profile == "" || params.Regex == "" {
		return PeekResult{}, fmt.Errorf("pprof peek requires profile and regex")
	}

	pprofArgs := []string{"tool", "pprof", "-peek", params.Regex}
	if params.SampleIndex != "" {
		pprofArgs = append(pprofArgs, "-sample_index", params.SampleIndex)
	}
	pprofArgs = append(pprofArgs, buildProfileArgs(params.Binary, params.Profile)...)

	output, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		if noMatches := wrapNoMatches(err, output.Stderr); noMatches != nil {
			return PeekResult{}, noMatches
		}
		return PeekResult{}, fmt.Errorf("pprof peek failed: %w\n%s", err, output.Stderr)
	}

	return PeekResult{
		Command:    shellJoin(append([]string{"go"}, pprofArgs...)),
		Raw:        output.Stdout,
		RawMeta:    output.StdoutMeta,
		Stderr:     output.Stderr,
		StderrMeta: output.StderrMeta,
	}, nil
}

func RunList(ctx context.Context, params ListParams) (ListResult, error) {
	if params.Profile == "" || params.Function == "" {
		return ListResult{}, fmt.Errorf("pprof list requires profile and function")
	}
	trimPath := params.TrimPath
	if trimPath == "" {
		trimPath = "/xsrc"
	}
	repoRoot := params.RepoRoot
	if repoRoot == "" {
		repoRoot = "."
	}

	pprofArgs := []string{"tool", "pprof", "-list", params.Function, "-trim_path", trimPath}

	// Add primary source path
	pprofArgs = append(pprofArgs, "-source_path", repoRoot)

	// Add additional source paths for vendored/external dependencies
	for _, sp := range params.SourcePaths {
		if sp != "" {
			pprofArgs = append(pprofArgs, "-source_path", sp)
		}
	}

	pprofArgs = append(pprofArgs, buildProfileArgs(params.Binary, params.Profile)...)

	output, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		if noMatches := wrapNoMatches(err, output.Stderr); noMatches != nil {
			return ListResult{}, noMatches
		}
		return ListResult{}, fmt.Errorf("pprof list failed: %w\n%s", err, output.Stderr)
	}

	return ListResult{
		Command:    shellJoin(append([]string{"go"}, pprofArgs...)),
		Raw:        output.Stdout,
		RawMeta:    output.StdoutMeta,
		Stderr:     output.Stderr,
		StderrMeta: output.StderrMeta,
	}, nil
}

func RunTracesHead(ctx context.Context, params TracesParams) (TracesResult, error) {
	if params.Profile == "" {
		return TracesResult{}, fmt.Errorf("pprof traces_head requires profile")
	}
	lines := params.Lines
	if lines <= 0 {
		lines = 200
	}

	pprofArgs := []string{"tool", "pprof", "-traces"}
	pprofArgs = append(pprofArgs, buildProfileArgs(params.Binary, params.Profile)...)

	output, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		return TracesResult{}, fmt.Errorf("pprof traces failed: %w\n%s", err, output.Stderr)
	}

	allLines := strings.Split(strings.TrimSuffix(output.Stdout, "\n"), "\n")
	truncated := false
	if len(allLines) > lines {
		allLines = allLines[:lines]
		truncated = true
	}

	return TracesResult{
		Command:    shellJoin(append([]string{"go"}, pprofArgs...)),
		Raw:        strings.Join(allLines, "\n"),
		TotalLines: output.StdoutMeta.TotalLines,
		Truncated:  truncated,
		RawMeta:    output.StdoutMeta,
		Stderr:     output.Stderr,
		StderrMeta: output.StderrMeta,
	}, nil
}

func RunDiffTop(ctx context.Context, params DiffTopParams) (DiffTopResult, error) {
	if params.Before == "" || params.After == "" {
		return DiffTopResult{}, fmt.Errorf("pprof diff_top requires before and after")
	}

	before, err := RunTop(ctx, TopParams{
		Profile:     params.Before,
		Binary:      params.Binary,
		Cum:         params.Cum,
		NodeCount:   params.NodeCount,
		Focus:       params.Focus,
		Ignore:      params.Ignore,
		SampleIndex: params.SampleIndex,
	})
	if err != nil {
		return DiffTopResult{}, err
	}
	after, err := RunTop(ctx, TopParams{
		Profile:     params.After,
		Binary:      params.Binary,
		Cum:         params.Cum,
		NodeCount:   params.NodeCount,
		Focus:       params.Focus,
		Ignore:      params.Ignore,
		SampleIndex: params.SampleIndex,
	})
	if err != nil {
		return DiffTopResult{}, err
	}

	deltas := pprofparse.DiffTop(before.Rows, after.Rows, params.Cum)
	return DiffTopResult{
		Commands: map[string]string{
			"before": before.Command,
			"after":  after.Command,
		},
		Before: before.Rows,
		After:  after.Rows,
		Deltas: deltas,
	}, nil
}

func buildProfileArgs(binary, profile string) []string {
	if binary != "" {
		return []string{binary, profile}
	}
	return []string{profile}
}

// TagsParams for pprof.tags tool
type TagsParams struct {
	Profile     string
	Binary      string
	TagFocus    string // Regex to focus on samples with matching tags
	TagIgnore   string // Regex to ignore samples with matching tags
	TagShow     string // Specific tag key to show values for
	Cum         bool
	NodeCount   int
	SampleIndex string
}

type TagsResult struct {
	Command    string                `json:"command"`
	Raw        string                `json:"raw"`
	RawMeta    textutil.TruncateMeta `json:"raw_meta,omitempty"`
	Stderr     string                `json:"stderr,omitempty"`
	StderrMeta textutil.TruncateMeta `json:"stderr_meta,omitempty"`
	Tags       []string              `json:"tags,omitempty"` // Available tag keys when no filter applied
}

func RunTags(ctx context.Context, params TagsParams) (TagsResult, error) {
	if params.Profile == "" {
		return TagsResult{}, fmt.Errorf("pprof tags requires profile")
	}

	// If TagShow is specified, show values for that specific tag
	// Otherwise, run top with tag filters to show filtered results
	pprofArgs := []string{"tool", "pprof"}

	if params.TagShow != "" {
		// Use -tags to show tag information
		pprofArgs = append(pprofArgs, "-tags")
	} else {
		pprofArgs = append(pprofArgs, "-top")
	}

	if params.TagFocus != "" {
		pprofArgs = append(pprofArgs, "-tagfocus", params.TagFocus)
	}
	if params.TagIgnore != "" {
		pprofArgs = append(pprofArgs, "-tagignore", params.TagIgnore)
	}
	if params.Cum {
		pprofArgs = append(pprofArgs, "-cum")
	}
	if params.NodeCount > 0 {
		pprofArgs = append(pprofArgs, "-nodecount", fmt.Sprintf("%d", params.NodeCount))
	}
	if params.SampleIndex != "" {
		pprofArgs = append(pprofArgs, "-sample_index", params.SampleIndex)
	}

	pprofArgs = append(pprofArgs, buildProfileArgs(params.Binary, params.Profile)...)

	output, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		if noMatches := wrapNoMatches(err, output.Stderr); noMatches != nil {
			return TagsResult{}, noMatches
		}
		return TagsResult{}, fmt.Errorf("pprof tags failed: %w\n%s", err, output.Stderr)
	}

	result := TagsResult{
		Command:    shellJoin(append([]string{"go"}, pprofArgs...)),
		Raw:        output.Stdout,
		RawMeta:    output.StdoutMeta,
		Stderr:     output.Stderr,
		StderrMeta: output.StderrMeta,
	}

	// Parse tag keys if showing tags
	if params.TagShow != "" || (params.TagFocus == "" && params.TagIgnore == "") {
		result.Tags = parseTagKeys(output.Stdout)
	}

	return result, nil
}

func parseTagKeys(output string) []string {
	var tags []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Tag lines typically start with the tag name followed by colon
		if idx := strings.Index(trimmed, ":"); idx > 0 {
			tagKey := strings.TrimSpace(trimmed[:idx])
			if tagKey != "" && !strings.Contains(tagKey, " ") {
				tags = append(tags, tagKey)
			}
		}
	}
	return tags
}

// FlamegraphParams for pprof.flamegraph tool
type FlamegraphParams struct {
	Profile     string
	Binary      string
	OutputPath  string // Path to write the SVG file
	Focus       string
	Ignore      string
	TagFocus    string
	TagIgnore   string
	SampleIndex string
}

type FlamegraphResult struct {
	Command    string `json:"command"`
	OutputPath string `json:"output_path"`
	Message    string `json:"message"`
}

func RunFlamegraph(ctx context.Context, params FlamegraphParams) (FlamegraphResult, error) {
	if params.Profile == "" {
		return FlamegraphResult{}, fmt.Errorf("pprof flamegraph requires profile")
	}
	if params.OutputPath == "" {
		return FlamegraphResult{}, fmt.Errorf("pprof flamegraph requires output_path")
	}

	pprofArgs := []string{"tool", "pprof", "-svg", "-output", params.OutputPath}

	if params.Focus != "" {
		pprofArgs = append(pprofArgs, "-focus", params.Focus)
	}
	if params.Ignore != "" {
		pprofArgs = append(pprofArgs, "-ignore", params.Ignore)
	}
	if params.TagFocus != "" {
		pprofArgs = append(pprofArgs, "-tagfocus", params.TagFocus)
	}
	if params.TagIgnore != "" {
		pprofArgs = append(pprofArgs, "-tagignore", params.TagIgnore)
	}
	if params.SampleIndex != "" {
		pprofArgs = append(pprofArgs, "-sample_index", params.SampleIndex)
	}

	pprofArgs = append(pprofArgs, buildProfileArgs(params.Binary, params.Profile)...)

	output, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		if noMatches := wrapNoMatches(err, output.Stderr); noMatches != nil {
			return FlamegraphResult{}, noMatches
		}
		return FlamegraphResult{}, fmt.Errorf("pprof flamegraph failed: %w\n%s", err, output.Stderr)
	}

	return FlamegraphResult{
		Command:    shellJoin(append([]string{"go"}, pprofArgs...)),
		OutputPath: params.OutputPath,
		Message:    fmt.Sprintf("Flamegraph SVG written to %s", params.OutputPath),
	}, nil
}

// CallgraphParams for pprof.callgraph tool
type CallgraphParams struct {
	Profile     string
	Binary      string
	OutputPath  string // Path to write the DOT or SVG file
	Format      string // "dot", "svg", "png" (default: dot)
	Focus       string
	Ignore      string
	NodeCount   int
	EdgeFrac    float64 // Hide edges below this fraction
	NodeFrac    float64 // Hide nodes below this fraction
	SampleIndex string
}

type CallgraphResult struct {
	Command    string `json:"command"`
	OutputPath string `json:"output_path"`
	Format     string `json:"format"`
	Message    string `json:"message"`
}

func RunCallgraph(ctx context.Context, params CallgraphParams) (CallgraphResult, error) {
	if params.Profile == "" {
		return CallgraphResult{}, fmt.Errorf("pprof callgraph requires profile")
	}
	if params.OutputPath == "" {
		return CallgraphResult{}, fmt.Errorf("pprof callgraph requires output_path")
	}

	format := params.Format
	if format == "" {
		format = "dot"
	}

	pprofArgs := []string{"tool", "pprof", "-" + format, "-output", params.OutputPath}

	if params.Focus != "" {
		pprofArgs = append(pprofArgs, "-focus", params.Focus)
	}
	if params.Ignore != "" {
		pprofArgs = append(pprofArgs, "-ignore", params.Ignore)
	}
	if params.NodeCount > 0 {
		pprofArgs = append(pprofArgs, "-nodecount", fmt.Sprintf("%d", params.NodeCount))
	}
	if params.EdgeFrac > 0 {
		pprofArgs = append(pprofArgs, "-edgefraction", fmt.Sprintf("%f", params.EdgeFrac))
	}
	if params.NodeFrac > 0 {
		pprofArgs = append(pprofArgs, "-nodefraction", fmt.Sprintf("%f", params.NodeFrac))
	}
	if params.SampleIndex != "" {
		pprofArgs = append(pprofArgs, "-sample_index", params.SampleIndex)
	}

	pprofArgs = append(pprofArgs, buildProfileArgs(params.Binary, params.Profile)...)

	output, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		if noMatches := wrapNoMatches(err, output.Stderr); noMatches != nil {
			return CallgraphResult{}, noMatches
		}
		return CallgraphResult{}, fmt.Errorf("pprof callgraph failed: %w\n%s", err, output.Stderr)
	}

	return CallgraphResult{
		Command:    shellJoin(append([]string{"go"}, pprofArgs...)),
		OutputPath: params.OutputPath,
		Format:     format,
		Message:    fmt.Sprintf("Callgraph %s written to %s", format, params.OutputPath),
	}, nil
}

// FocusPathsParams for pprof.focus_paths tool
type FocusPathsParams struct {
	Profile     string
	Binary      string
	Function    string // Target function to find paths to
	Cum         bool
	NodeCount   int
	SampleIndex string
}

type FocusPathsResult struct {
	Command    string                `json:"command"`
	Raw        string                `json:"raw"`
	RawMeta    textutil.TruncateMeta `json:"raw_meta,omitempty"`
	Stderr     string                `json:"stderr,omitempty"`
	StderrMeta textutil.TruncateMeta `json:"stderr_meta,omitempty"`
}

func RunFocusPaths(ctx context.Context, params FocusPathsParams) (FocusPathsResult, error) {
	if params.Profile == "" {
		return FocusPathsResult{}, fmt.Errorf("pprof focus_paths requires profile")
	}
	if params.Function == "" {
		return FocusPathsResult{}, fmt.Errorf("pprof focus_paths requires function")
	}

	// Use -focus to show only paths containing the function
	// Combined with -traces to show the actual call paths
	pprofArgs := []string{"tool", "pprof", "-traces", "-focus", params.Function}

	if params.Cum {
		pprofArgs = append(pprofArgs, "-cum")
	}
	if params.NodeCount > 0 {
		pprofArgs = append(pprofArgs, "-nodecount", fmt.Sprintf("%d", params.NodeCount))
	}
	if params.SampleIndex != "" {
		pprofArgs = append(pprofArgs, "-sample_index", params.SampleIndex)
	}

	pprofArgs = append(pprofArgs, buildProfileArgs(params.Binary, params.Profile)...)

	output, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		if noMatches := wrapNoMatches(err, output.Stderr); noMatches != nil {
			return FocusPathsResult{}, noMatches
		}
		return FocusPathsResult{}, fmt.Errorf("pprof focus_paths failed: %w\n%s", err, output.Stderr)
	}

	return FocusPathsResult{
		Command:    shellJoin(append([]string{"go"}, pprofArgs...)),
		Raw:        output.Stdout,
		RawMeta:    output.StdoutMeta,
		Stderr:     output.Stderr,
		StderrMeta: output.StderrMeta,
	}, nil
}

// MergeParams for pprof.merge tool
type MergeParams struct {
	Profiles   []string // List of profile paths to merge
	Binary     string
	OutputPath string // Path to write the merged profile
}

type MergeResult struct {
	Command    string `json:"command"`
	OutputPath string `json:"output_path"`
	InputCount int    `json:"input_count"`
	Message    string `json:"message"`
}

func RunMerge(ctx context.Context, params MergeParams) (MergeResult, error) {
	if len(params.Profiles) < 2 {
		return MergeResult{}, fmt.Errorf("pprof merge requires at least 2 profiles")
	}
	if params.OutputPath == "" {
		return MergeResult{}, fmt.Errorf("pprof merge requires output_path")
	}

	// Use -proto to output merged profile in protobuf format
	pprofArgs := []string{"tool", "pprof", "-proto", "-output", params.OutputPath}

	if params.Binary != "" {
		pprofArgs = append(pprofArgs, params.Binary)
	}

	// Add all profile paths
	pprofArgs = append(pprofArgs, params.Profiles...)

	output, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		return MergeResult{}, fmt.Errorf("pprof merge failed: %w\n%s", err, output.Stderr)
	}

	return MergeResult{
		Command:    shellJoin(append([]string{"go"}, pprofArgs...)),
		OutputPath: params.OutputPath,
		InputCount: len(params.Profiles),
		Message:    fmt.Sprintf("Merged %d profiles into %s", len(params.Profiles), params.OutputPath),
	}, nil
}
