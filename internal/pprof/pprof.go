package pprof

import (
	"context"
	"fmt"
	"strings"

	"gofast-mcp/internal/pprofparse"
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
	Command string                `json:"command"`
	Raw     string                `json:"raw"`
	Rows    []pprofparse.TopRow   `json:"rows"`
	Summary pprofparse.TopSummary `json:"summary"`
}

type PeekParams struct {
	Profile string
	Binary  string
	Regex   string
}

type PeekResult struct {
	Command string `json:"command"`
	Raw     string `json:"raw"`
}

type ListParams struct {
	Profile  string
	Binary   string
	Function string
	RepoRoot string
	TrimPath string
}

type ListResult struct {
	Command string `json:"command"`
	Raw     string `json:"raw"`
}

type TracesParams struct {
	Profile string
	Binary  string
	Lines   int
}

type TracesResult struct {
	Command    string `json:"command"`
	Raw        string `json:"raw"`
	TotalLines int    `json:"total_lines"`
	Truncated  bool   `json:"truncated"`
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

	stdout, stderr, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		return TopResult{}, fmt.Errorf("pprof top failed: %w\n%s", err, stderr)
	}

	report := pprofparse.ParseTop(stdout)
	return TopResult{
		Command: shellJoin(append([]string{"go"}, pprofArgs...)),
		Raw:     stdout,
		Rows:    report.Rows,
		Summary: report.Summary,
	}, nil
}

func RunPeek(ctx context.Context, params PeekParams) (PeekResult, error) {
	if params.Profile == "" || params.Regex == "" {
		return PeekResult{}, fmt.Errorf("pprof peek requires profile and regex")
	}

	pprofArgs := []string{"tool", "pprof", "-peek", params.Regex}
	pprofArgs = append(pprofArgs, buildProfileArgs(params.Binary, params.Profile)...)

	stdout, stderr, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		return PeekResult{}, fmt.Errorf("pprof peek failed: %w\n%s", err, stderr)
	}

	return PeekResult{
		Command: shellJoin(append([]string{"go"}, pprofArgs...)),
		Raw:     stdout,
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

	pprofArgs := []string{"tool", "pprof", "-list", params.Function, "-source_path", repoRoot, "-trim_path", trimPath}
	pprofArgs = append(pprofArgs, buildProfileArgs(params.Binary, params.Profile)...)

	stdout, stderr, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		return ListResult{}, fmt.Errorf("pprof list failed: %w\n%s", err, stderr)
	}

	return ListResult{
		Command: shellJoin(append([]string{"go"}, pprofArgs...)),
		Raw:     stdout,
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

	stdout, stderr, err := runCommand(ctx, "go", pprofArgs...)
	if err != nil {
		return TracesResult{}, fmt.Errorf("pprof traces failed: %w\n%s", err, stderr)
	}

	allLines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	truncated := false
	if len(allLines) > lines {
		allLines = allLines[:lines]
		truncated = true
	}

	return TracesResult{
		Command:    shellJoin(append([]string{"go"}, pprofArgs...)),
		Raw:        strings.Join(allLines, "\n"),
		TotalLines: len(strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")),
		Truncated:  truncated,
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

