package pprof

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type MemorySanityParams struct {
	HeapProfile      string // Path to heap profile
	GoroutineProfile string // Optional path to goroutine profile
	Binary           string // Optional binary for symbol resolution
	ContainerRSSMB   int    // Optional: container RSS in MB for comparison
}

type MemorySanityResult struct {
	Summary        string         `json:"summary"`
	HeapInUseMB    float64        `json:"heap_inuse_mb"`
	HeapAllocMB    float64        `json:"heap_alloc_mb"`
	GoroutineCount int            `json:"goroutine_count,omitempty"`
	Warnings       []string       `json:"warnings"`
	Suspicions     []Suspicion    `json:"suspicions"`
	Recommendations []string      `json:"recommendations"`
}

type Suspicion struct {
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"` // low, medium, high
	Evidence    string  `json:"evidence,omitempty"`
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

	// Get heap stats
	heapTop, err := runPprofTop(ctx, params.HeapProfile, params.Binary, "inuse_space", 50)
	if err != nil {
		return result, fmt.Errorf("failed to get heap top: %w", err)
	}

	// Parse heap metrics
	result.HeapInUseMB, result.HeapAllocMB = parseHeapMetrics(heapTop)

	// Get goroutine count if profile provided
	if params.GoroutineProfile != "" {
		result.GoroutineCount = countGoroutines(ctx, params.GoroutineProfile, params.Binary)
	}

	// Analyze for suspicious patterns
	analyzeSQLitePatterns(heapTop, &result)
	analyzeFragmentationPatterns(heapTop, &result)
	analyzeGoroutineStackUsage(result.GoroutineCount, &result)
	analyzeCGOPatterns(heapTop, &result)
	analyzeRSSMismatch(params.ContainerRSSMB, result.HeapInUseMB, &result)

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
	// Look for lines like "Total: 124.5MB" or parse from individual entries
	lines := strings.Split(topOutput, "\n")
	for _, line := range lines {
		// Match patterns like "124.5MB" or "1.2GB"
		if strings.Contains(strings.ToLower(line), "total") {
			inuseMB = parseMemoryValue(line)
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

func analyzeSQLitePatterns(topOutput string, result *MemorySanityResult) {
	sqlitePatterns := []string{
		"sqlite",
		"glebarez",
		"goqu",
		"database/sql",
		"mattn/go-sqlite",
	}

	lowerOutput := strings.ToLower(topOutput)
	for _, pattern := range sqlitePatterns {
		if strings.Contains(lowerOutput, pattern) {
			result.Suspicions = append(result.Suspicions, Suspicion{
				Category:    "SQLite Memory",
				Description: "SQLite-related allocations detected in heap profile",
				Severity:    "medium",
				Evidence:    fmt.Sprintf("Found pattern: %s", pattern),
			})
			result.Recommendations = append(result.Recommendations,
				"Check SQLite pragma settings - temp_store=MEMORY can cause RSS growth outside Go heap",
				"Consider using temp_store=FILE or temp_store=DEFAULT",
				"Review connection pool size and query patterns",
			)
			break
		}
	}
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
		result.Suspicions = append(result.Suspicions, Suspicion{
			Category:    "Goroutine Stacks",
			Description: fmt.Sprintf("High goroutine count (%d) - stacks use at least %.1fMB", count, minStackMB),
			Severity:    "medium",
			Evidence:    fmt.Sprintf("%d goroutines * 2KB minimum = %.1fMB (can be much higher with deep stacks)", count, minStackMB),
		})
		result.Recommendations = append(result.Recommendations,
			"Review goroutine leaks - use pprof.goroutines profile to identify blocked goroutines",
			"Consider using worker pools instead of unbounded goroutine creation",
		)
	}

	if count > 10000 {
		result.Suspicions[len(result.Suspicions)-1].Severity = "high"
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
			Evidence:    fmt.Sprintf("Difference: %.0fMB unaccounted memory", rssMismatch),
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
	if result.GoroutineCount > 0 {
		sb.WriteString(fmt.Sprintf(" | Goroutines: %d", result.GoroutineCount))
	}
	sb.WriteString("\n")

	if len(result.Suspicions) == 0 {
		sb.WriteString("No obvious memory issues detected in heap profile.")
	} else {
		highSeverity := 0
		for _, s := range result.Suspicions {
			if s.Severity == "high" {
				highSeverity++
			}
		}
		sb.WriteString(fmt.Sprintf("Found %d potential issues (%d high severity)", len(result.Suspicions), highSeverity))
	}

	return sb.String()
}
