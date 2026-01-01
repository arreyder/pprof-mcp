package pprof

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

// AllocPathsParams configures the allocation paths analysis.
type AllocPathsParams struct {
	Profile       string
	MinPercent    float64  // Minimum percentage to include (default: 1.0)
	MaxPaths      int      // Maximum paths to return (default: 20)
	RepoPrefixes  []string // Filter to paths containing these prefixes
	GroupBySource bool     // Group by source file instead of function
}

// AllocPath represents a single allocation path.
type AllocPath struct {
	AllocSite   string   `json:"alloc_site"`   // Where the allocation happens
	CallerChain []string `json:"caller_chain"` // Call stack leading to allocation
	AllocBytes  int64    `json:"alloc_bytes"`
	AllocBytesStr string `json:"alloc_bytes_str"`
	AllocPct    float64  `json:"alloc_pct"`
	AllocRate   string   `json:"alloc_rate,omitempty"` // e.g., "45MB/min"
	FirstAppFrame string `json:"first_app_frame,omitempty"`
}

// AllocPathsResult contains the allocation paths analysis.
type AllocPathsResult struct {
	ProfileKind   string      `json:"profile_kind"`
	TotalAlloc    int64       `json:"total_alloc"`
	TotalAllocStr string      `json:"total_alloc_str"`
	DurationSecs  float64     `json:"duration_secs,omitempty"`
	Paths         []AllocPath `json:"paths"`
	Warnings      []string    `json:"warnings,omitempty"`
}

// RunAllocPaths analyzes allocation paths in a heap profile.
func RunAllocPaths(params AllocPathsParams) (AllocPathsResult, error) {
	result := AllocPathsResult{
		Paths:    []AllocPath{},
		Warnings: []string{},
	}

	if params.Profile == "" {
		return result, fmt.Errorf("profile path required")
	}

	file, err := os.Open(params.Profile)
	if err != nil {
		return result, err
	}
	defer file.Close()

	prof, err := profile.Parse(file)
	if err != nil {
		return result, err
	}

	result.ProfileKind = detectProfileKind(prof)
	if result.ProfileKind != "heap" {
		result.Warnings = append(result.Warnings,
			"profile does not appear to be a heap profile; results may be inaccurate")
	}

	// Find alloc_space sample index
	allocIndex := -1
	for i, st := range prof.SampleType {
		if st.Type == "alloc_space" {
			allocIndex = i
			break
		}
	}
	if allocIndex == -1 {
		// Fall back to first sample type
		allocIndex = 0
		result.Warnings = append(result.Warnings,
			"alloc_space not found; using default sample type")
	}

	// Calculate total
	var total int64
	for _, sample := range prof.Sample {
		if allocIndex < len(sample.Value) {
			total += sample.Value[allocIndex]
		}
	}
	result.TotalAlloc = total
	result.TotalAllocStr = formatValue(total, "bytes")

	// Calculate duration for rate
	if prof.DurationNanos > 0 {
		result.DurationSecs = float64(prof.DurationNanos) / 1e9
	}

	if total == 0 {
		result.Warnings = append(result.Warnings, "no allocations in profile")
		return result, nil
	}

	// Set defaults
	minPct := params.MinPercent
	if minPct <= 0 {
		minPct = 1.0
	}
	maxPaths := params.MaxPaths
	if maxPaths <= 0 {
		maxPaths = 20
	}

	repoPrefixes := params.RepoPrefixes
	if len(repoPrefixes) == 0 {
		repoPrefixes = []string{"gitlab.com/ductone/c1", "github.com/conductorone"}
	}

	// Aggregate by allocation site
	type allocInfo struct {
		value       int64
		callerChain []string
		firstApp    string
	}
	allocSites := make(map[string]*allocInfo)

	for _, sample := range prof.Sample {
		value := int64(0)
		if allocIndex < len(sample.Value) {
			value = sample.Value[allocIndex]
		}
		if value == 0 {
			continue
		}

		// Build call chain
		var chain []string
		var allocSite string
		var firstApp string

		for i, loc := range sample.Location {
			for _, line := range loc.Line {
				if line.Function == nil {
					continue
				}
				funcName := line.Function.Name

				// Skip runtime internals for allocation site
				if i == 0 && isRuntimeAlloc(funcName) {
					continue
				}

				if allocSite == "" {
					allocSite = funcName
				}
				chain = append(chain, funcName)

				// Track first app frame
				if firstApp == "" {
					for _, prefix := range repoPrefixes {
						if strings.Contains(funcName, prefix) {
							firstApp = funcName
							break
						}
					}
				}
			}
		}

		if allocSite == "" {
			continue
		}

		// Filter by repo prefix if specified
		if len(repoPrefixes) > 0 {
			hasAppCode := false
			for _, frame := range chain {
				for _, prefix := range repoPrefixes {
					if strings.Contains(frame, prefix) {
						hasAppCode = true
						break
					}
				}
				if hasAppCode {
					break
				}
			}
			if !hasAppCode {
				continue
			}
		}

		// Aggregate
		key := allocSite
		if params.GroupBySource {
			// Use first app frame as key if available
			if firstApp != "" {
				key = firstApp
			}
		}

		if existing, ok := allocSites[key]; ok {
			existing.value += value
		} else {
			// Limit chain length
			if len(chain) > 8 {
				chain = chain[:8]
			}
			allocSites[key] = &allocInfo{
				value:       value,
				callerChain: chain,
				firstApp:    firstApp,
			}
		}
	}

	// Convert to paths and filter
	for site, info := range allocSites {
		pct := float64(info.value) / float64(total) * 100
		if pct < minPct {
			continue
		}

		path := AllocPath{
			AllocSite:     site,
			CallerChain:   info.callerChain,
			AllocBytes:    info.value,
			AllocBytesStr: formatValue(info.value, "bytes"),
			AllocPct:      pct,
			FirstAppFrame: info.firstApp,
		}

		// Calculate rate if duration available
		if result.DurationSecs > 0 {
			bytesPerMin := float64(info.value) / result.DurationSecs * 60
			path.AllocRate = formatValue(int64(bytesPerMin), "bytes") + "/min"
		}

		result.Paths = append(result.Paths, path)
	}

	// Sort by allocation size
	sort.Slice(result.Paths, func(i, j int) bool {
		return result.Paths[i].AllocBytes > result.Paths[j].AllocBytes
	})

	// Limit results
	if len(result.Paths) > maxPaths {
		result.Paths = result.Paths[:maxPaths]
	}

	return result, nil
}

func isRuntimeAlloc(funcName string) bool {
	runtimeAllocs := []string{
		"runtime.mallocgc",
		"runtime.newobject",
		"runtime.makeslice",
		"runtime.makemap",
		"runtime.mapassign",
		"runtime.growslice",
		"runtime.concatstrings",
		"runtime.rawstring",
		"runtime.slicebytetostring",
		"runtime.stringtoslicebyte",
	}
	for _, ra := range runtimeAllocs {
		if strings.HasPrefix(funcName, ra) {
			return true
		}
	}
	return false
}
