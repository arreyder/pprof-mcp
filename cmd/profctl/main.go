package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gofast-mcp/internal/datadog"
	"gofast-mcp/internal/pprof"
	"gofast-mcp/internal/services"
)

type jsonOutput map[string]any

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: profctl <download|pprof|repo|datadog>")
	}

	switch args[1] {
	case "download":
		return runDownload(args[2:], out)
	case "pprof":
		return runPprof(args[2:], out)
	case "repo":
		return runRepo(args[2:], out)
	case "datadog":
		return runDatadog(args[2:], out)
	default:
		return fmt.Errorf("unknown command: %s", args[1])
	}
}

func runDownload(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	service := fs.String("service", "", "Datadog service name")
	env := fs.String("env", "", "Datadog environment")
	outDir := fs.String("out", "", "output directory for profiles")
	ddSite := fs.String("dd_site", "", "Datadog site, defaults to DD_SITE or us3.datadoghq.com")
	hours := fs.Int("hours", 72, "time window in hours")
	profileID := fs.String("profile_id", "", "Datadog profile id (optional)")
	eventID := fs.String("event_id", "", "Datadog event id (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *service == "" || *env == "" || *outDir == "" {
		return errors.New("download requires --service, --env, and --out")
	}

	result, err := datadog.DownloadLatestBundle(context.Background(), datadog.DownloadParams{
		Service:   *service,
		Env:       *env,
		OutDir:    *outDir,
		Site:      *ddSite,
		Hours:     *hours,
		ProfileID: *profileID,
		EventID:   *eventID,
	})
	if err != nil {
		return err
	}

	cmdParts := []string{
		"profctl", "download",
		"--service", *service,
		"--env", *env,
		"--out", *outDir,
		"--hours", fmt.Sprintf("%d", *hours),
	}
	if *profileID != "" {
		cmdParts = append(cmdParts, "--profile_id", *profileID)
	}
	if *eventID != "" {
		cmdParts = append(cmdParts, "--event_id", *eventID)
	}
	if *ddSite != "" {
		cmdParts = append(cmdParts, "--dd_site", *ddSite)
	}
	payload := jsonOutput{
		"command": shellJoin(cmdParts),
		"result":  result,
	}
	return writeJSON(out, payload)
}

func runPprof(args []string, out io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: profctl pprof <top|peek|list|traces_head|diff_top|meta|storylines>")
	}

	switch args[0] {
	case "top":
		return runPprofTop(args[1:], out)
	case "peek":
		return runPprofPeek(args[1:], out)
	case "list":
		return runPprofList(args[1:], out)
	case "traces_head":
		return runPprofTracesHead(args[1:], out)
	case "diff_top":
		return runPprofDiffTop(args[1:], out)
	case "meta":
		return runPprofMeta(args[1:], out)
	case "storylines":
		return runPprofStorylines(args[1:], out)
	default:
		return fmt.Errorf("unknown pprof command: %s", args[0])
	}
}

func runPprofTop(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("pprof top", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profile := fs.String("profile", "", "path to .pprof profile")
	binary := fs.String("binary", "", "path to binary (optional)")
	cum := fs.Bool("cum", false, "use cumulative time")
	nodecount := fs.Int("nodecount", 0, "node count for top output")
	focus := fs.String("focus", "", "focus regex")
	ignore := fs.String("ignore", "", "ignore regex")
	sampleIndex := fs.String("sample_index", "", "pprof sample index")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := pprof.RunTop(context.Background(), pprof.TopParams{
		Profile:     *profile,
		Binary:      *binary,
		Cum:         *cum,
		NodeCount:   *nodecount,
		Focus:       *focus,
		Ignore:      *ignore,
		SampleIndex: *sampleIndex,
	})
	if err != nil {
		return err
	}

	payload := jsonOutput{
		"command": result.Command,
		"raw":     result.Raw,
		"rows":    result.Rows,
		"summary": result.Summary,
	}
	return writeJSON(out, payload)
}

func runPprofPeek(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("pprof peek", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profile := fs.String("profile", "", "path to .pprof profile")
	binary := fs.String("binary", "", "path to binary (optional)")
	regex := fs.String("regex", "", "regex or function to peek")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := pprof.RunPeek(context.Background(), pprof.PeekParams{
		Profile: *profile,
		Binary:  *binary,
		Regex:   *regex,
	})
	if err != nil {
		return err
	}

	payload := jsonOutput{
		"command": result.Command,
		"raw":     result.Raw,
	}
	return writeJSON(out, payload)
}

func runPprofList(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("pprof list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profile := fs.String("profile", "", "path to .pprof profile")
	binary := fs.String("binary", "", "path to binary (optional)")
	function := fs.String("function", "", "function or regex to list")
	repoRoot := fs.String("repo_root", ".", "repo root for source path")
	trimPath := fs.String("trim_path", "/xsrc", "trim path for sources")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := pprof.RunList(context.Background(), pprof.ListParams{
		Profile:  *profile,
		Binary:   *binary,
		Function: *function,
		RepoRoot: *repoRoot,
		TrimPath: *trimPath,
	})
	if err != nil {
		return err
	}

	payload := jsonOutput{
		"command": result.Command,
		"raw":     result.Raw,
	}
	return writeJSON(out, payload)
}

func runPprofTracesHead(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("pprof traces_head", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profile := fs.String("profile", "", "path to .pprof profile")
	binary := fs.String("binary", "", "path to binary (optional)")
	lines := fs.Int("lines", 200, "number of trace lines to keep")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := pprof.RunTracesHead(context.Background(), pprof.TracesParams{
		Profile: *profile,
		Binary:  *binary,
		Lines:   *lines,
	})
	if err != nil {
		return err
	}

	payload := jsonOutput{
		"command":     result.Command,
		"raw":         result.Raw,
		"total_lines": result.TotalLines,
		"truncated":   result.Truncated,
	}
	return writeJSON(out, payload)
}

func runPprofDiffTop(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("pprof diff_top", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	before := fs.String("before", "", "path to before .pprof profile")
	after := fs.String("after", "", "path to after .pprof profile")
	binary := fs.String("binary", "", "path to binary (optional)")
	cum := fs.Bool("cum", false, "use cumulative time")
	nodecount := fs.Int("nodecount", 0, "node count for top output")
	focus := fs.String("focus", "", "focus regex")
	ignore := fs.String("ignore", "", "ignore regex")
	sampleIndex := fs.String("sample_index", "", "pprof sample index")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := pprof.RunDiffTop(context.Background(), pprof.DiffTopParams{
		Before:      *before,
		After:       *after,
		Binary:      *binary,
		Cum:         *cum,
		NodeCount:   *nodecount,
		Focus:       *focus,
		Ignore:      *ignore,
		SampleIndex: *sampleIndex,
	})
	if err != nil {
		return err
	}

	payload := jsonOutput{
		"commands": result.Commands,
		"before":   result.Before,
		"after":    result.After,
		"deltas":   result.Deltas,
	}
	return writeJSON(out, payload)
}

func runPprofMeta(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("pprof meta", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profilePath := fs.String("profile", "", "path to .pprof profile")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	meta, err := pprof.RunMeta(*profilePath)
	if err != nil {
		return err
	}

	if !*jsonOut {
		_, err := fmt.Fprintf(out, "profile: %s\nkind: %s\n", meta.ProfilePath, meta.DetectedKind)
		return err
	}

	payload := jsonOutput{
		"command": pprof.FormatMetaCommand(*profilePath),
		"result":  meta,
	}
	return writeJSON(out, payload)
}

func runPprofStorylines(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("pprof storylines", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	profilePath := fs.String("profile", "", "path to cpu .pprof profile")
	n := fs.Int("n", 4, "number of storylines (2-6)")
	focus := fs.String("focus", "", "focus regex")
	ignore := fs.String("ignore", "", "ignore regex")
	repoRoot := fs.String("repo_root", ".", "repo root for source path")
	trimPath := fs.String("trim_path", "/xsrc", "trim path for sources")
	jsonOut := fs.Bool("json", false, "output JSON")
	var repoPrefixes multiFlag
	fs.Var(&repoPrefixes, "repo_prefix", "repo prefix to identify app-owned frames")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := pprof.RunStorylines(context.Background(), pprof.StorylinesParams{
		Profile:      *profilePath,
		N:            *n,
		Focus:        *focus,
		Ignore:       *ignore,
		RepoPrefixes: repoPrefixes,
		RepoRoot:     *repoRoot,
		TrimPath:     *trimPath,
	})
	if err != nil {
		return err
	}

	if !*jsonOut {
		for _, storyline := range result.Storylines {
			fmt.Fprintf(out, "- %s (cum=%s)\n", storyline.LeafHotspot, storyline.Cum)
		}
		return nil
	}

	payload := jsonOutput{
		"command": result.Command,
		"result":  result,
	}
	return writeJSON(out, payload)
}

func runRepo(args []string, out io.Writer) error {
	if len(args) < 2 || args[0] != "services" || args[1] != "discover" {
		return errors.New("usage: profctl repo services discover --repo_root <path>")
	}
	fs := flag.NewFlagSet("repo services discover", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repoRoot := fs.String("repo_root", ".", "path to repo root containing cmd/")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}

	results, err := services.Discover(*repoRoot)
	if err != nil {
		return err
	}

	payload := jsonOutput{
		"command":  shellJoin([]string{"profctl", "repo", "services", "discover", "--repo_root", *repoRoot}),
		"services": results,
	}
	return writeJSON(out, payload)
}

func runDatadog(args []string, out io.Writer) error {
	if len(args) < 2 || args[0] != "profiles" {
		return errors.New("usage: profctl datadog profiles <list|pick>")
	}
	switch args[1] {
	case "list":
		return runDatadogProfilesList(args[2:], out)
	case "pick":
		return runDatadogProfilesPick(args[2:], out)
	default:
		return fmt.Errorf("unknown datadog profiles command: %s", args[1])
	}
}

func runDatadogProfilesList(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("datadog profiles list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	service := fs.String("service", "", "Datadog service name")
	env := fs.String("env", "", "Datadog environment")
	from := fs.String("from", "", "ISO timestamp start")
	to := fs.String("to", "", "ISO timestamp end")
	hours := fs.Int("hours", 72, "time window in hours")
	limit := fs.Int("limit", 50, "max profiles to return")
	site := fs.String("site", "", "Datadog site (defaults to us3.datadoghq.com)")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := datadog.ListProfiles(context.Background(), datadog.ListProfilesParams{
		Service: *service,
		Env:     *env,
		From:    *from,
		To:      *to,
		Hours:   *hours,
		Limit:   *limit,
		Site:    *site,
	})
	if err != nil {
		return err
	}

	cmdParts := []string{
		"profctl", "datadog", "profiles", "list",
		"--service", *service,
		"--env", *env,
		"--hours", fmt.Sprintf("%d", *hours),
		"--limit", fmt.Sprintf("%d", *limit),
	}
	if *from != "" {
		cmdParts = append(cmdParts, "--from", *from)
	}
	if *to != "" {
		cmdParts = append(cmdParts, "--to", *to)
	}
	if *site != "" {
		cmdParts = append(cmdParts, "--site", *site)
	}

	if !*jsonOut {
		_, err := fmt.Fprintln(out, datadog.FormatCandidatesTable(result.Candidates))
		return err
	}

	payload := jsonOutput{
		"command":    shellJoin(cmdParts),
		"result":     result,
		"candidates": result.Candidates,
	}
	return writeJSON(out, payload)
}

func runDatadogProfilesPick(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("datadog profiles pick", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	service := fs.String("service", "", "Datadog service name")
	env := fs.String("env", "", "Datadog environment")
	from := fs.String("from", "", "ISO timestamp start")
	to := fs.String("to", "", "ISO timestamp end")
	hours := fs.Int("hours", 72, "time window in hours")
	limit := fs.Int("limit", 50, "max profiles to return")
	site := fs.String("site", "", "Datadog site (defaults to us3.datadoghq.com)")
	strategy := fs.String("strategy", "latest", "pick strategy: latest|closest_to_ts|most_samples|manual_index")
	target := fs.String("target_ts", "", "target ISO timestamp for closest_to_ts")
	index := fs.Int("index", -1, "manual index (0-based)")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := datadog.PickProfile(context.Background(), datadog.PickProfilesParams{
		Service:  *service,
		Env:      *env,
		From:     *from,
		To:       *to,
		Hours:    *hours,
		Limit:    *limit,
		Site:     *site,
		Strategy: datadog.PickStrategy(*strategy),
		TargetTS: *target,
		Index:    *index,
	})
	if err != nil {
		return err
	}

	cmdParts := []string{
		"profctl", "datadog", "profiles", "pick",
		"--service", *service,
		"--env", *env,
		"--strategy", *strategy,
		"--hours", fmt.Sprintf("%d", *hours),
		"--limit", fmt.Sprintf("%d", *limit),
	}
	if *from != "" {
		cmdParts = append(cmdParts, "--from", *from)
	}
	if *to != "" {
		cmdParts = append(cmdParts, "--to", *to)
	}
	if *site != "" {
		cmdParts = append(cmdParts, "--site", *site)
	}
	if *target != "" {
		cmdParts = append(cmdParts, "--target_ts", *target)
	}
	if *index >= 0 {
		cmdParts = append(cmdParts, "--index", fmt.Sprintf("%d", *index))
	}

	if !*jsonOut {
		_, err := fmt.Fprintf(out, "profile_id=%s event_id=%s timestamp=%s reason=%s\n", result.Candidate.ProfileID, result.Candidate.EventID, result.Candidate.Timestamp, result.Reason)
		return err
	}

	payload := jsonOutput{
		"command":  shellJoin(cmdParts),
		"result":   result,
		"profile":  result.Candidate,
		"warnings": result.Warnings,
	}
	return writeJSON(out, payload)
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func shellJoin(parts []string) string {
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted = append(quoted, shellQuote(part))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.ContainsAny(s, " \t\n\"'\\$&;|<>[]{}()") {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	}
	return s
}

func writeJSON(out io.Writer, payload any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
