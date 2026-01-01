package pprof

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

const (
	defaultTraceDepth       = 10
	defaultTraceContextLine = 5
)

type TraceSourceParams struct {
	Profile      string
	Function     string
	RepoRoot     string
	MaxDepth     int
	ShowVendor   bool
	ContextLines int
}

type TraceSourceResult struct {
	CallChain            []TraceSourceFrame `json:"call_chain"`
	TotalFunctionsTraced int                `json:"total_functions_traced"`
	AppFunctions         int                `json:"app_functions"`
	VendorFunctions      int                `json:"vendor_functions"`
	Warnings             []string           `json:"warnings,omitempty"`
}

type TraceSourceFrame struct {
	Function      string  `json:"function"`
	File          string  `json:"file"`
	Line          int     `json:"line"`
	FlatPct       float64 `json:"flat_pct"`
	CumPct        float64 `json:"cum_pct"`
	SourceSnippet string  `json:"source_snippet"`
	IsVendor      bool    `json:"is_vendor"`
	VendorPackage string  `json:"vendor_package,omitempty"`
	VendorVersion string  `json:"vendor_version,omitempty"`
	SourceError   string  `json:"source_error,omitempty"`
}

type functionStat struct {
	flat int64
	cum  int64
}

type traceFrame struct {
	function string
	file     string
	line     int
}

func RunTraceSource(params TraceSourceParams) (TraceSourceResult, error) {
	result := TraceSourceResult{
		CallChain: []TraceSourceFrame{},
		Warnings:  []string{},
	}
	if strings.TrimSpace(params.Profile) == "" {
		return result, fmt.Errorf("profile is required")
	}
	if strings.TrimSpace(params.Function) == "" {
		return result, fmt.Errorf("function is required")
	}

	maxDepth := params.MaxDepth
	if maxDepth <= 0 {
		maxDepth = defaultTraceDepth
	}
	contextLines := params.ContextLines
	if contextLines <= 0 {
		contextLines = defaultTraceContextLine
	}
	repoRoot := strings.TrimSpace(params.RepoRoot)
	if repoRoot == "" {
		repoRoot = "."
	}

	profileFile, err := os.Open(params.Profile)
	if err != nil {
		return result, err
	}
	defer profileFile.Close()

	prof, err := profile.Parse(profileFile)
	if err != nil {
		return result, err
	}

	reFunc, err := regexp.Compile(params.Function)
	if err != nil {
		return result, fmt.Errorf("invalid function regex: %w", err)
	}

	statMap, totalValue := computeFunctionStats(prof)
	frames, matchIndex, err := pickSampleTrace(prof, reFunc)
	if err != nil {
		return result, err
	}

	if maxDepth > 0 && len(frames) > maxDepth {
		start := 0
		if matchIndex >= 0 {
			start = matchIndex - maxDepth/2
			if start < 0 {
				start = 0
			}
			if start+maxDepth > len(frames) {
				start = len(frames) - maxDepth
			}
		} else {
			start = len(frames) - maxDepth
		}
		frames = frames[start : start+maxDepth]
	}

	modInfo, modErr := ParseGoMod(repoRoot)
	if modErr != nil {
		result.Warnings = append(result.Warnings, "go.mod not found or unreadable; version info omitted")
	}

	for _, frame := range frames {
		stat := statMap[frame.function]
		flatPct := percentOf(stat.flat, totalValue)
		cumPct := percentOf(stat.cum, totalValue)
		resolved, isVendor, vendorPackage, vendorVersion, sourceErr := resolveSourceFile(frame, repoRoot, params.ShowVendor, modInfo)
		if sourceErr != nil && resolved == "" {
			result.Warnings = append(result.Warnings, sourceErr.Error())
		}

		if isVendor {
			result.VendorFunctions++
		} else {
			result.AppFunctions++
		}
		if !params.ShowVendor && isVendor {
			continue
		}

		snippet, snippetErr := readSnippet(resolved, frame.line, contextLines)
		sourceErrText := ""
		if snippetErr != nil {
			sourceErrText = snippetErr.Error()
			if snippet == "" {
				snippet = fmt.Sprintf("source not available: %s", snippetErr.Error())
			}
		}

		result.CallChain = append(result.CallChain, TraceSourceFrame{
			Function:      frame.function,
			File:          resolved,
			Line:          frame.line,
			FlatPct:       flatPct,
			CumPct:        cumPct,
			SourceSnippet: snippet,
			IsVendor:      isVendor,
			VendorPackage: vendorPackage,
			VendorVersion: vendorVersion,
			SourceError:   sourceErrText,
		})
	}

	result.TotalFunctionsTraced = len(result.CallChain)
	return result, nil
}

func computeFunctionStats(prof *profile.Profile) (map[string]functionStat, int64) {
	stats := map[string]functionStat{}
	var total int64
	if prof == nil || len(prof.Sample) == 0 {
		return stats, total
	}
	sampleIndex := 0
	for _, sample := range prof.Sample {
		value := sampleValueAt(sample, sampleIndex)
		total += value
		if value == 0 {
			continue
		}

		if len(sample.Location) > 0 {
			if fn := locationFunction(sample.Location[0]); fn != "" {
				stat := stats[fn]
				stat.flat += value
				stats[fn] = stat
			}
		}

		seen := map[string]bool{}
		for _, loc := range sample.Location {
			fn := locationFunction(loc)
			if fn == "" || seen[fn] {
				continue
			}
			seen[fn] = true
			stat := stats[fn]
			stat.cum += value
			stats[fn] = stat
		}
	}
	return stats, total
}

func pickSampleTrace(prof *profile.Profile, reFunc *regexp.Regexp) ([]traceFrame, int, error) {
	if prof == nil {
		return nil, -1, fmt.Errorf("profile is empty")
	}
	sampleIndex := 0
	var bestSample *profile.Sample
	var bestValue int64
	var matchIndex int

	for _, sample := range prof.Sample {
		frames := sampleTraceFrames(sample)
		matched := false
		for i, frame := range frames {
			if reFunc.MatchString(frame.function) {
				matched = true
				matchIndex = i
				break
			}
		}
		if !matched {
			continue
		}
		value := sampleValueAt(sample, sampleIndex)
		if value > bestValue {
			bestValue = value
			bestSample = sample
		}
	}

	if bestSample == nil {
		return nil, -1, fmt.Errorf("%w: no matching frames for %q", ErrNoMatches, reFunc.String())
	}

	frames := sampleTraceFrames(bestSample)
	if len(frames) == 0 {
		return nil, -1, fmt.Errorf("matched sample had no frames")
	}

	matchIndex = -1
	for i, frame := range frames {
		if reFunc.MatchString(frame.function) {
			matchIndex = i
			break
		}
	}

	return frames, matchIndex, nil
}

func sampleTraceFrames(sample *profile.Sample) []traceFrame {
	if sample == nil {
		return nil
	}
	frames := make([]traceFrame, 0, len(sample.Location))
	for i := len(sample.Location) - 1; i >= 0; i-- {
		loc := sample.Location[i]
		fn, file, line := locationDetails(loc)
		if fn == "" {
			continue
		}
		frames = append(frames, traceFrame{
			function: fn,
			file:     file,
			line:     line,
		})
	}
	return frames
}

func locationFunction(loc *profile.Location) string {
	if loc == nil {
		return ""
	}
	for _, line := range loc.Line {
		if line.Function != nil && line.Function.Name != "" {
			return line.Function.Name
		}
	}
	return ""
}

func locationDetails(loc *profile.Location) (string, string, int) {
	if loc == nil {
		return "", "", 0
	}
	for _, line := range loc.Line {
		if line.Function == nil || line.Function.Name == "" {
			continue
		}
		file := line.Function.Filename
		return line.Function.Name, file, int(line.Line)
	}
	return "", "", 0
}

func sampleValueAt(sample *profile.Sample, idx int) int64 {
	if sample == nil || len(sample.Value) == 0 {
		return 0
	}
	if idx >= 0 && idx < len(sample.Value) {
		return sample.Value[idx]
	}
	return sample.Value[0]
}

func percentOf(value int64, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return roundPct(float64(value) / float64(total) * 100)
}

func roundPct(value float64) float64 {
	if value == 0 {
		return 0
	}
	const factor = 100
	return float64(int(value*factor+0.5)) / factor
}

func resolveSourceFile(frame traceFrame, repoRoot string, showVendor bool, modInfo ModInfo) (string, bool, string, string, error) {
	if frame.file == "" {
		return "", false, "", "", fmt.Errorf("no source file for %s", frame.function)
	}

	candidates := []string{}
	frameFile := filepath.Clean(frame.file)

	if filepath.IsAbs(frameFile) {
		if fileExists(frameFile) {
			return frameFile, isVendorPath(frameFile), "", "", nil
		}
	}

	trimmed := strings.TrimPrefix(frameFile, "/xsrc/")
	if trimmed != frameFile {
		candidates = append(candidates, filepath.Join(repoRoot, trimmed))
	}
	candidates = append(candidates, filepath.Join(repoRoot, frameFile))

	if idx := strings.Index(frameFile, string(filepath.Separator)+"vendor"+string(filepath.Separator)); idx >= 0 {
		after := frameFile[idx+len(string(filepath.Separator))+len("vendor")+len(string(filepath.Separator)):]
		candidates = append(candidates, filepath.Join(repoRoot, "vendor", after))
	}

	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate, isVendorPath(candidate), "", "", nil
		}
	}

	packagePath := functionPackagePath(frame.function)
	modulePath, moduleVersion := moduleVersionForPackage(modInfo, packagePath)
	if showVendor && packagePath != "" {
		base := filepath.Base(frameFile)
		if base != "" {
			vendorCandidate := filepath.Join(repoRoot, "vendor", packagePath, base)
			if fileExists(vendorCandidate) {
				return vendorCandidate, true, modulePath, moduleVersion, nil
			}
		}
	}

	if modulePath != "" {
		modCandidate := findModuleCacheFile(modulePath, packagePath, filepath.Base(frameFile))
		if modCandidate != "" {
			return modCandidate, true, modulePath, moduleVersion, nil
		}
	}

	return "", false, modulePath, moduleVersion, fmt.Errorf("source file not found for %s", frame.function)
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isVendorPath(path string) bool {
	return strings.Contains(path, string(filepath.Separator)+"vendor"+string(filepath.Separator))
}

func readSnippet(path string, line int, contextLines int) (string, error) {
	if path == "" {
		return "", fmt.Errorf("source path not resolved")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if line <= 0 {
		return "", fmt.Errorf("invalid line number")
	}

	start := line - contextLines
	if start < 1 {
		start = 1
	}
	end := line + contextLines

	var b strings.Builder
	scanner := bufio.NewScanner(file)
	current := 1
	for scanner.Scan() {
		text := scanner.Text()
		if current >= start && current <= end {
			prefix := fmt.Sprintf("%d: ", current)
			if current == line {
				prefix = fmt.Sprintf(">>>%d: ", current)
			}
			b.WriteString(prefix)
			b.WriteString(text)
			b.WriteString("\n")
		}
		if current > end {
			break
		}
		current++
	}
	if err := scanner.Err(); err != nil {
		return b.String(), err
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func functionPackagePath(function string) string {
	function = strings.TrimSpace(function)
	if function == "" {
		return ""
	}
	parts := strings.Split(function, "/")
	last := parts[len(parts)-1]
	pkgSegment := strings.Split(last, ".")[0]
	if pkgSegment == "" {
		return ""
	}
	if len(parts) == 1 {
		return pkgSegment
	}
	return strings.Join(append(parts[:len(parts)-1], pkgSegment), "/")
}

func findModuleCacheFile(modulePath, packagePath, baseFile string) string {
	if modulePath == "" || baseFile == "" {
		return ""
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			gopath = filepath.Join(home, "go")
		}
	}
	if gopath == "" {
		return ""
	}

	glob := filepath.Join(gopath, "pkg", "mod", modulePath+"@*")
	matches, _ := filepath.Glob(glob)
	if len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	moduleDir := matches[len(matches)-1]

	subPath := strings.TrimPrefix(packagePath, modulePath)
	subPath = strings.TrimPrefix(subPath, "/")
	candidate := filepath.Join(moduleDir, subPath, baseFile)
	if fileExists(candidate) {
		return candidate
	}
	return ""
}
