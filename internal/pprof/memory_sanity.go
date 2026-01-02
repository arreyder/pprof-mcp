package pprof

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type MemorySanityParams struct {
	HeapProfile      string // Path to heap profile
	GoroutineProfile string // Optional path to goroutine profile
	CPUProfile       string // Optional path to CPU profile for cross-referencing
	Binary           string // Optional binary for symbol resolution
	ContainerRSSMB   int    // Optional: container RSS in MB for comparison
	RepoRoot         string // Optional: repository root for code scanning
}

type MemorySanityResult struct {
	Summary         string         `json:"summary"`
	HeapInUseMB     float64        `json:"heap_inuse_mb"`
	HeapAllocMB     float64        `json:"heap_alloc_mb"`
	GoroutineCount  int            `json:"goroutine_count,omitempty"`
	Warnings        []string       `json:"warnings"`
	Suspicions      []Suspicion    `json:"suspicions"`
	CodeFindings    []CodeFinding  `json:"code_findings,omitempty"`
	Recommendations []string       `json:"recommendations"`
}

// CodeFinding represents a problematic pattern found in the codebase
type CodeFinding struct {
	Category    string `json:"category"`
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Pattern     string `json:"pattern"`
	Snippet     string `json:"snippet,omitempty"`
	Explanation string `json:"explanation"`
	IsVendor    bool   `json:"is_vendor"` // true if found in vendor/ directory
}

type Suspicion struct {
	Category    string `json:"category"`
	Description string `json:"description"`
	Severity    string `json:"severity"`   // low, medium, high
	Confidence  string `json:"confidence"` // confirmed, likely, suspected, possible
	Evidence    string `json:"evidence,omitempty"`
}

// RunMemorySanity analyzes a heap profile for patterns that cause RSS growth beyond Go heap.
func RunMemorySanity(ctx context.Context, params MemorySanityParams) (MemorySanityResult, error) {
	if params.HeapProfile == "" {
		return MemorySanityResult{}, fmt.Errorf("heap_profile is required")
	}

	result := MemorySanityResult{
		Warnings:        []string{},
		Suspicions:      []Suspicion{},
		Recommendations: []string{},
	}

	// Get heap stats (in-use memory)
	heapTop, err := runPprofTop(ctx, params.HeapProfile, params.Binary, "inuse_space", 50)
	if err != nil {
		return result, fmt.Errorf("failed to get heap top: %w", err)
	}

	// Get allocation stats (total allocations) - critical for detecting high-churn patterns
	allocTop, err := runPprofTop(ctx, params.HeapProfile, params.Binary, "alloc_space", 50)
	if err != nil {
		// Non-fatal - continue with just inuse analysis
		result.Warnings = append(result.Warnings, "Could not analyze alloc_space")
		allocTop = ""
	}

	// Get CPU profile data if provided - used to confirm off-heap allocations
	cpuTop := ""
	if params.CPUProfile != "" {
		cpuTop, err = runPprofTop(ctx, params.CPUProfile, params.Binary, "", 50)
		if err != nil {
			result.Warnings = append(result.Warnings, "Could not analyze CPU profile")
			cpuTop = ""
		}
	}

	// Parse heap metrics
	result.HeapInUseMB, result.HeapAllocMB = parseHeapMetrics(heapTop)
	if allocTop != "" {
		_, result.HeapAllocMB = parseAllocMetrics(allocTop)
	}

	// Get goroutine count if profile provided
	if params.GoroutineProfile != "" {
		result.GoroutineCount = countGoroutines(ctx, params.GoroutineProfile, params.Binary)
	}

	// Analyze for suspicious patterns - check heap, alloc, and CPU outputs
	combinedHeapOutput := heapTop + "\n" + allocTop
	foundCategories := analyzeOffHeapPatterns(combinedHeapOutput, cpuTop, result.HeapInUseMB, result.HeapAllocMB, &result)
	analyzeFragmentationPatterns(heapTop, &result)
	analyzeGoroutineStackUsage(result.GoroutineCount, &result)
	analyzeCGOPatterns(combinedHeapOutput, &result)
	analyzeRSSMismatch(params.ContainerRSSMB, result.HeapInUseMB, &result)

	// Scan codebase for problematic patterns if repo_root provided
	if params.RepoRoot != "" && len(foundCategories) > 0 {
		result.CodeFindings = scanCodebaseForPatterns(ctx, params.RepoRoot, foundCategories)
		if len(result.CodeFindings) > 0 {
			// Upgrade confidence if we found code evidence
			// Prioritize non-vendor findings for confidence upgrade
			for i := range result.Suspicions {
				if result.Suspicions[i].Confidence == "suspected" || result.Suspicions[i].Confidence == "possible" {
					// First look for application code (non-vendor) findings
					var bestMatch *CodeFinding
					for j := range result.CodeFindings {
						cf := &result.CodeFindings[j]
						if cf.Category == result.Suspicions[i].Category ||
							(result.Suspicions[i].Category == "SQLite Off-Heap Memory" && cf.Category == "SQLite") {
							if bestMatch == nil || (!cf.IsVendor && bestMatch.IsVendor) {
								bestMatch = cf
							}
						}
					}
					if bestMatch != nil {
						if !bestMatch.IsVendor {
							result.Suspicions[i].Confidence = "confirmed"
							result.Suspicions[i].Evidence += fmt.Sprintf("; CODE FOUND: %s:%d", bestMatch.File, bestMatch.Line)
						} else {
							// Vendor code finding - upgrade to "likely" not "confirmed"
							if result.Suspicions[i].Confidence == "possible" {
								result.Suspicions[i].Confidence = "likely"
							}
							result.Suspicions[i].Evidence += fmt.Sprintf("; vendor code: %s:%d", bestMatch.File, bestMatch.Line)
						}
					}
				}
			}
		}
	} else if params.RepoRoot == "" && len(foundCategories) > 0 {
		result.Warnings = append(result.Warnings, "Provide repo_root parameter to scan codebase for problematic patterns")
	}

	// Generate summary
	result.Summary = generateSummary(&result)

	return result, nil
}

func runPprofTop(ctx context.Context, profile, binary, sampleIndex string, nodeCount int) (string, error) {
	args := []string{"tool", "pprof", "-top"}
	if binary != "" {
		args = append(args, "-symbolize=force")
	}
	args = append(args, fmt.Sprintf("-nodecount=%d", nodeCount))
	if sampleIndex != "" {
		args = append(args, fmt.Sprintf("-sample_index=%s", sampleIndex))
	}
	if binary != "" {
		args = append(args, binary)
	}
	args = append(args, profile)

	cmd := exec.CommandContext(ctx, "go", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pprof failed: %w: %s", err, string(out))
	}
	return string(out), nil
}

func parseHeapMetrics(topOutput string) (inuseMB, allocMB float64) {
	// Look for lines like "Total: 124.5MB" or "Total samples = 26.67GB"
	lines := strings.Split(topOutput, "\n")
	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "total") {
			inuseMB = parseMemoryValue(line)
			break
		}
	}
	return inuseMB, allocMB
}

func parseAllocMetrics(topOutput string) (inuseMB, allocMB float64) {
	// Look for "Total samples = X" line in alloc_space output
	lines := strings.Split(topOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Total samples") {
			allocMB = parseMemoryValue(line)
			break
		}
	}
	return inuseMB, allocMB
}

func parseMemoryValue(s string) float64 {
	// Match patterns like "124.5MB" or "1.2GB"
	re := regexp.MustCompile(`(\d+\.?\d*)(MB|GB|KB|B)`)
	matches := re.FindStringSubmatch(s)
	if len(matches) < 3 {
		return 0
	}

	val, _ := strconv.ParseFloat(matches[1], 64)
	switch matches[2] {
	case "GB":
		return val * 1024
	case "MB":
		return val
	case "KB":
		return val / 1024
	case "B":
		return val / (1024 * 1024)
	}
	return val
}

func countGoroutines(ctx context.Context, profile, binary string) int {
	args := []string{"tool", "pprof", "-top", "-nodecount=1"}
	if binary != "" {
		args = append(args, binary)
	}
	args = append(args, profile)

	cmd := exec.CommandContext(ctx, "go", args...)
	out, _ := cmd.CombinedOutput()

	// Parse goroutine count from output
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "goroutine") {
			re := regexp.MustCompile(`(\d+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				count, _ := strconv.Atoi(matches[1])
				return count
			}
		}
	}
	return 0
}

// offHeapPattern defines a pattern that may indicate off-heap memory allocation
type offHeapPattern struct {
	pattern     string
	category    string
	description string
	severity    string // base severity, may be escalated
}

// analyzeOffHeapPatterns detects patterns that allocate memory outside Go heap control.
// It cross-references heap and CPU profiles to determine confidence levels.
// Returns a map of categories found for use in code scanning.
func analyzeOffHeapPatterns(heapOutput, cpuOutput string, heapInUseMB, heapAllocMB float64, result *MemorySanityResult) map[string]bool {
	lowerHeapOutput := strings.ToLower(heapOutput)
	lowerCPUOutput := strings.ToLower(cpuOutput)
	hasCPUProfile := cpuOutput != ""

	// Track what we find in each profile type
	type finding struct {
		inHeap bool
		inCPU  bool
	}
	findings := make(map[string]*finding)

	// Patterns to look for
	patterns := []offHeapPattern{
		// SQLite patterns
		{"glebarez/go-sqlite", "SQLite", "go-sqlite detected", "medium"},
		{"mattn/go-sqlite", "SQLite", "go-sqlite3 (CGO) detected", "medium"},
		{"modernc.org/sqlite", "SQLite", "modernc SQLite detected", "medium"},

		// libc allocation patterns - these are the smoking gun for off-heap memory
		{"modernc.org/libc.(*tls).alloc", "libc-alloc", "libc.(*TLS).Alloc - direct off-heap allocation", "high"},
		{"modernc.org/libc.xmalloc", "libc-alloc", "libc Xmalloc - direct off-heap allocation", "high"},
		{"modernc.org/libc.xrealloc", "libc-alloc", "libc Xrealloc - direct off-heap allocation", "high"},
		{"modernc.org/libc.xmemcpy", "libc-ops", "libc Xmemcpy - off-heap memory operations", "medium"},
		{"modernc.org/libc.xmemcmp", "libc-ops", "libc Xmemcmp - off-heap memory operations", "medium"},
		{"modernc.org/libc.(*tls).free", "libc-ops", "libc.(*TLS).Free - off-heap memory operations", "medium"},

		// Compression libraries
		{"klauspost/compress/zstd", "Compression", "zstd compression detected", "medium"},
		{"klauspost/compress/zlib", "Compression", "zlib compression detected", "medium"},
		{"klauspost/compress/gzip", "Compression", "gzip compression detected", "medium"},
	}

	// Check each pattern in both profiles
	for _, p := range patterns {
		lowerPattern := strings.ToLower(p.pattern)
		f := &finding{}
		if strings.Contains(lowerHeapOutput, lowerPattern) {
			f.inHeap = true
		}
		if hasCPUProfile && strings.Contains(lowerCPUOutput, lowerPattern) {
			f.inCPU = true
		}
		if f.inHeap || f.inCPU {
			findings[p.pattern] = f
		}
	}

	// Determine what categories we found
	hasSQLite := findings["glebarez/go-sqlite"] != nil || findings["mattn/go-sqlite"] != nil || findings["modernc.org/sqlite"] != nil
	hasLibcAlloc := findings["modernc.org/libc.(*tls).alloc"] != nil || findings["modernc.org/libc.xmalloc"] != nil || findings["modernc.org/libc.xrealloc"] != nil
	hasLibcOps := findings["modernc.org/libc.xmemcpy"] != nil || findings["modernc.org/libc.xmemcmp"] != nil || findings["modernc.org/libc.(*tls).free"] != nil
	hasCompression := findings["klauspost/compress/zstd"] != nil || findings["klauspost/compress/zlib"] != nil || findings["klauspost/compress/gzip"] != nil

	// High churn indicator
	highChurn := heapAllocMB > 1024 && heapInUseMB < 500

	// Build evidence strings
	var evidenceParts []string
	if hasSQLite {
		evidenceParts = append(evidenceParts, "SQLite in heap profile")
	}
	if hasLibcAlloc {
		if hasCPUProfile {
			evidenceParts = append(evidenceParts, "libc.Alloc in CPU profile (CONFIRMED off-heap allocation)")
		} else {
			evidenceParts = append(evidenceParts, "libc.Alloc detected")
		}
	}
	if hasLibcOps {
		evidenceParts = append(evidenceParts, "libc memory ops detected")
	}
	if highChurn {
		evidenceParts = append(evidenceParts, fmt.Sprintf("HIGH CHURN: %.0fMB allocated, only %.0fMB in-use", heapAllocMB, heapInUseMB))
	}

	// Determine confidence level for SQLite off-heap memory
	if hasSQLite {
		var confidence, severity, description string
		evidence := strings.Join(evidenceParts, "; ")

		// Confidence levels based on evidence combination
		switch {
		case hasLibcAlloc && hasCPUProfile && highChurn:
			// Best case: we see SQLite + libc.Alloc in CPU + high churn
			confidence = "confirmed"
			severity = "high"
			description = "SQLite is allocating memory outside Go heap via libc (likely temp_store=MEMORY)"
		case hasLibcAlloc && hasCPUProfile:
			// Good: we see SQLite + libc.Alloc in CPU
			confidence = "likely"
			severity = "high"
			description = "SQLite with libc allocations detected - probable off-heap memory usage"
		case (hasLibcAlloc || hasLibcOps) && highChurn:
			// Medium: we see libc patterns + high churn but no CPU profile
			confidence = "likely"
			severity = "high"
			description = "SQLite with high allocation churn and libc patterns - probable off-heap memory"
			if !hasCPUProfile {
				evidence += " (provide CPU profile for confirmation)"
			}
		case highChurn:
			// We see SQLite + high churn but no libc patterns
			confidence = "suspected"
			severity = "medium"
			description = "SQLite detected with high allocation churn - possible off-heap memory via temp_store=MEMORY"
			if !hasCPUProfile {
				evidence += " (provide CPU profile for confirmation)"
			}
		default:
			// Just SQLite, no strong indicators
			confidence = "possible"
			severity = "low"
			description = "SQLite detected - check temp_store setting if experiencing memory issues"
			if !hasCPUProfile {
				evidence += " (provide CPU profile for better analysis)"
			}
		}

		result.Suspicions = append(result.Suspicions, Suspicion{
			Category:    "SQLite Off-Heap Memory",
			Description: description,
			Severity:    severity,
			Confidence:  confidence,
			Evidence:    evidence,
		})

		// Add recommendations based on confidence
		if confidence == "confirmed" || confidence == "likely" {
			result.Recommendations = append(result.Recommendations,
				"CRITICAL: Check for PRAGMA temp_store=MEMORY in SQLite connections",
				"Use PRAGMA temp_store=FILE or temp_store=DEFAULT to use disk-based temp storage",
				"Monitor container RSS directly - Go heap metrics won't show this memory",
			)
		} else {
			result.Recommendations = append(result.Recommendations,
				"Check SQLite PRAGMA temp_store setting - MEMORY mode allocates outside Go heap",
				"Consider PRAGMA temp_store=FILE if experiencing memory issues",
			)
		}
	}

	// Compression analysis
	if hasCompression {
		confidence := "possible"
		severity := "low"
		description := "Compression library detected - may hold internal buffers"

		if highChurn {
			confidence = "suspected"
			severity = "medium"
			description = "Compression with high allocation churn - buffers may contribute to memory pressure"
		}

		var compressors []string
		if findings["klauspost/compress/zstd"] != nil {
			compressors = append(compressors, "zstd")
		}
		if findings["klauspost/compress/zlib"] != nil {
			compressors = append(compressors, "zlib")
		}
		if findings["klauspost/compress/gzip"] != nil {
			compressors = append(compressors, "gzip")
		}

		result.Suspicions = append(result.Suspicions, Suspicion{
			Category:    "Compression Buffers",
			Description: description,
			Severity:    severity,
			Confidence:  confidence,
			Evidence:    fmt.Sprintf("Found: %s", strings.Join(compressors, ", ")),
		})

		result.Recommendations = append(result.Recommendations,
			"Reuse compression encoders/decoders instead of creating new ones",
			"Call encoder.Close() promptly to release internal buffers",
		)
	}

	// If we have libc patterns but no SQLite, still flag it
	if (hasLibcAlloc || hasLibcOps) && !hasSQLite {
		confidence := "suspected"
		if hasLibcAlloc && hasCPUProfile {
			confidence = "likely"
		}

		result.Suspicions = append(result.Suspicions, Suspicion{
			Category:    "libc Memory Allocation",
			Description: "modernc.org/libc allocations detected without obvious source",
			Severity:    "medium",
			Confidence:  confidence,
			Evidence:    "libc memory operations found - some library is allocating outside Go heap",
		})
	}

	// Return categories found for code scanning
	foundCategories := make(map[string]bool)
	if hasSQLite {
		foundCategories["SQLite"] = true
	}
	if hasLibcAlloc {
		foundCategories["libc-alloc"] = true
	}
	if hasLibcOps {
		foundCategories["libc-ops"] = true
	}
	if hasCompression {
		foundCategories["Compression"] = true
	}
	return foundCategories
}

func analyzeFragmentationPatterns(topOutput string, result *MemorySanityResult) {
	// Look for patterns that suggest memory fragmentation
	fragmentationPatterns := []string{
		"runtime.mallocgc",
		"mcache",
		"mspan",
	}

	lowerOutput := strings.ToLower(topOutput)
	for _, pattern := range fragmentationPatterns {
		if strings.Contains(lowerOutput, pattern) {
			// Check if it's a significant portion
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Runtime memory allocation pattern detected: %s", pattern))
		}
	}
}

func analyzeGoroutineStackUsage(count int, result *MemorySanityResult) {
	if count == 0 {
		return
	}

	// Each goroutine starts with 2KB stack but can grow
	// Estimate minimum stack memory: count * 2KB
	minStackMB := float64(count) * 2 / 1024

	if count > 1000 {
		severity := "medium"
		confidence := "confirmed" // goroutine count is a fact, not a guess
		if count > 10000 {
			severity = "high"
		}

		result.Suspicions = append(result.Suspicions, Suspicion{
			Category:    "Goroutine Stacks",
			Description: fmt.Sprintf("High goroutine count (%d) - stacks use at least %.1fMB", count, minStackMB),
			Severity:    severity,
			Confidence:  confidence,
			Evidence:    fmt.Sprintf("%d goroutines * 2KB minimum = %.1fMB (can be much higher with deep stacks)", count, minStackMB),
		})
		result.Recommendations = append(result.Recommendations,
			"Review goroutine leaks - use pprof.goroutines profile to identify blocked goroutines",
			"Consider using worker pools instead of unbounded goroutine creation",
		)
	}
}

func analyzeCGOPatterns(topOutput string, result *MemorySanityResult) {
	cgoPatterns := []string{
		"cgo",
		"_Cfunc",
		"_cgo",
	}

	lowerOutput := strings.ToLower(topOutput)
	for _, pattern := range cgoPatterns {
		if strings.Contains(lowerOutput, pattern) {
			result.Suspicions = append(result.Suspicions, Suspicion{
				Category:    "CGO Allocations",
				Description: "CGO allocations detected - these allocate outside Go heap",
				Severity:    "medium",
				Confidence:  "confirmed", // CGO pattern is definitive
				Evidence:    fmt.Sprintf("Found CGO pattern: %s", pattern),
			})
			result.Recommendations = append(result.Recommendations,
				"CGO allocations are not tracked by Go heap - use system memory profilers",
				"Consider pure-Go alternatives if CGO memory is problematic",
			)
			break
		}
	}
}

func analyzeRSSMismatch(containerRSSMB int, heapInUseMB float64, result *MemorySanityResult) {
	if containerRSSMB <= 0 || heapInUseMB <= 0 {
		return
	}

	rssMismatch := float64(containerRSSMB) - heapInUseMB

	if rssMismatch > 500 { // More than 500MB difference
		result.Suspicions = append(result.Suspicions, Suspicion{
			Category:    "RSS/Heap Mismatch",
			Description: fmt.Sprintf("Container RSS (%.0fMB) significantly exceeds Go heap (%.1fMB) by %.0fMB", float64(containerRSSMB), heapInUseMB, rssMismatch),
			Severity:    "high",
			Confidence:  "confirmed", // RSS vs heap is a measured fact
			Evidence:    fmt.Sprintf("Difference: %.0fMB unaccounted memory - this memory is outside Go heap", rssMismatch),
		})
		result.Recommendations = append(result.Recommendations,
			"Large RSS/heap mismatch indicates memory outside Go heap control",
			"Common causes: SQLite temp_store=MEMORY, CGO allocations, mmap'd files, MADV_FREE pages",
			"Set GODEBUG=madvdontneed=1 to release memory to OS immediately",
			"Set GOMEMLIMIT to trigger GC before hitting container limits",
		)
	}
}

func generateSummary(result *MemorySanityResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Heap in-use: %.1fMB", result.HeapInUseMB))
	if result.HeapAllocMB > 0 {
		sb.WriteString(fmt.Sprintf(" | Total allocated: %.1fMB", result.HeapAllocMB))
	}
	if result.GoroutineCount > 0 {
		sb.WriteString(fmt.Sprintf(" | Goroutines: %d", result.GoroutineCount))
	}
	sb.WriteString("\n")

	if len(result.Suspicions) == 0 {
		sb.WriteString("No obvious memory issues detected in heap profile.")
	} else {
		// Count by confidence level
		confirmed := 0
		likely := 0
		suspected := 0
		for _, s := range result.Suspicions {
			switch s.Confidence {
			case "confirmed":
				confirmed++
			case "likely":
				likely++
			case "suspected":
				suspected++
			}
		}

		sb.WriteString(fmt.Sprintf("Found %d potential issues", len(result.Suspicions)))
		if confirmed > 0 || likely > 0 {
			sb.WriteString(fmt.Sprintf(" (%d confirmed, %d likely, %d suspected)", confirmed, likely, suspected))
		}
	}

	return sb.String()
}

// codePattern defines a pattern to search for in the codebase
type codePattern struct {
	category    string
	pattern     string   // grep pattern (regex)
	fileGlob    string   // file pattern to search (e.g., "*.go")
	explanation string
}

// scanCodebaseForPatterns searches the repo for known problematic patterns
func scanCodebaseForPatterns(ctx context.Context, repoRoot string, categories map[string]bool) []CodeFinding {
	if repoRoot == "" {
		return nil
	}

	var findings []CodeFinding
	var patterns []codePattern

	// Only search for patterns relevant to what we found in profiles
	if categories["SQLite"] || categories["libc-alloc"] || categories["libc-ops"] {
		patterns = append(patterns, []codePattern{
			{
				category:    "SQLite",
				pattern:     `temp_store\s*=\s*['"]?MEMORY['"]?`,
				fileGlob:    "*.go",
				explanation: "temp_store=MEMORY causes SQLite to allocate temp tables in memory outside Go heap",
			},
			{
				category:    "SQLite",
				pattern:     `PRAGMA\s+temp_store\s*=\s*2`,
				fileGlob:    "*.go",
				explanation: "PRAGMA temp_store=2 is equivalent to MEMORY mode",
			},
			{
				category:    "SQLite",
				pattern:     `temp_store=memory`,
				fileGlob:    "*.go",
				explanation: "temp_store=memory in connection string allocates outside Go heap",
			},
			{
				category:    "SQLite",
				pattern:     `_pragma=temp_store`,
				fileGlob:    "*.go",
				explanation: "SQLite pragma configuration - check if temp_store is set to MEMORY",
			},
		}...)
	}

	if categories["Compression"] {
		patterns = append(patterns, []codePattern{
			{
				category:    "Compression",
				pattern:     `zstd\.NewWriter\(`,
				fileGlob:    "*.go",
				explanation: "zstd.NewWriter creates encoder with internal buffers - ensure Close() is called and consider pooling",
			},
			{
				category:    "Compression",
				pattern:     `zstd\.NewReader\(`,
				fileGlob:    "*.go",
				explanation: "zstd.NewReader creates decoder with internal buffers - ensure Close() is called",
			},
			{
				category:    "Compression",
				pattern:     `gzip\.NewWriter\(`,
				fileGlob:    "*.go",
				explanation: "gzip.NewWriter - ensure Close() is called to release buffers",
			},
		}...)
	}

	for _, p := range patterns {
		matches := grepPattern(ctx, repoRoot, p.pattern, p.fileGlob)
		for _, m := range matches {
			findings = append(findings, CodeFinding{
				Category:    p.category,
				File:        m.file,
				Line:        m.line,
				Pattern:     p.pattern,
				Snippet:     m.snippet,
				Explanation: p.explanation,
				IsVendor:    strings.HasPrefix(m.file, "vendor/") || strings.Contains(m.file, "/vendor/"),
			})
		}
	}

	// Sort findings: application code first, then vendor
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].IsVendor != findings[j].IsVendor {
			return !findings[i].IsVendor // non-vendor (false) comes first
		}
		return findings[i].File < findings[j].File
	})

	return findings
}

type grepMatch struct {
	file    string
	line    int
	snippet string
}

// grepPattern searches for a pattern in the repo using grep
func grepPattern(ctx context.Context, repoRoot, pattern, fileGlob string) []grepMatch {
	// Use grep -r with extended regex
	args := []string{"-r", "-n", "-E", "--include=" + fileGlob, pattern, repoRoot}
	cmd := exec.CommandContext(ctx, "grep", args...)
	out, err := cmd.Output()
	if err != nil {
		// grep returns exit 1 if no matches - that's okay
		return nil
	}

	var matches []grepMatch
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Parse grep output: file:line:content
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		lineNum, _ := strconv.Atoi(parts[1])
		// Make file path relative to repo root
		file := parts[0]
		if strings.HasPrefix(file, repoRoot) {
			file = strings.TrimPrefix(file, repoRoot)
			file = strings.TrimPrefix(file, "/")
		}
		matches = append(matches, grepMatch{
			file:    file,
			line:    lineNum,
			snippet: strings.TrimSpace(parts[2]),
		})
	}
	return matches
}
