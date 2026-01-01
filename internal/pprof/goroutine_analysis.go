package pprof

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

const (
	defaultLeakThreshold   = 1000
	defaultMaxLeakStacks   = 5
	defaultMaxWaitReasons  = 5
	defaultSignatureFrames = 8
)

type GoroutineAnalysisParams struct {
	Profile string
}

type GoroutineAnalysisResult struct {
	TotalGoroutines int                      `json:"total_goroutines"`
	ByState         map[string]int           `json:"by_state"`
	TopWaitReasons  []GoroutineWaitReason    `json:"top_wait_reasons"`
	PotentialLeaks  []GoroutineLeakCandidate `json:"potential_leaks"`
	Warnings        []string                 `json:"warnings,omitempty"`
}

type GoroutineWaitReason struct {
	Reason      string `json:"reason"`
	Count       int    `json:"count"`
	SampleStack string `json:"sample_stack"`
}

type GoroutineLeakCandidate struct {
	StackSignature string `json:"stack_signature"`
	Count          int    `json:"count"`
	Severity       string `json:"severity"`
	State          string `json:"state,omitempty"`
	WaitReason     string `json:"wait_reason,omitempty"`
}

type waitInfo struct {
	count       int
	sampleStack string
}

type leakInfo struct {
	count      int
	state      string
	waitReason string
}

func RunGoroutineAnalysis(params GoroutineAnalysisParams) (GoroutineAnalysisResult, error) {
	result := GoroutineAnalysisResult{
		ByState:        map[string]int{},
		TopWaitReasons: []GoroutineWaitReason{},
		PotentialLeaks: []GoroutineLeakCandidate{},
		Warnings:       []string{},
	}
	if params.Profile == "" {
		return result, fmt.Errorf("profile is required")
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
	if detectProfileKind(prof) != "goroutine" {
		result.Warnings = append(result.Warnings, "profile does not appear to be a goroutine profile; results may be inaccurate")
	}

	sampleIndex := findSampleTypeIndex(prof, []string{"goroutine", "goroutines"})

	waitReasons := map[string]*waitInfo{}

	leaks := map[string]*leakInfo{}

	for _, sample := range prof.Sample {
		count := sampleValue(sample, sampleIndex)
		if count <= 0 {
			count = 1
		}
		result.TotalGoroutines += count

		stack := stackFrames(sample)
		reason := detectWaitReason(stack)
		if reason == "" {
			reason = "unknown"
		}

		state := sampleState(sample)
		if state == "" {
			state = stateFromReason(reason)
		}
		if state == "" {
			state = "unknown"
		}
		result.ByState[state] += count

		if info, ok := waitReasons[reason]; ok {
			info.count += count
		} else {
			waitReasons[reason] = &waitInfo{
				count:       count,
				sampleStack: stackSignature(stack, 6),
			}
		}

		signature := stackSignature(stack, defaultSignatureFrames)
		if signature == "" {
			continue
		}
		if info, ok := leaks[signature]; ok {
			info.count += count
		} else {
			leaks[signature] = &leakInfo{
				count:      count,
				state:      state,
				waitReason: reason,
			}
		}
	}

	result.TopWaitReasons = topWaitReasons(waitReasons, defaultMaxWaitReasons)
	result.PotentialLeaks = topLeakCandidates(leaks, defaultLeakThreshold, defaultMaxLeakStacks)

	return result, nil
}

func findSampleTypeIndex(prof *profile.Profile, candidates []string) int {
	for _, name := range candidates {
		for idx, st := range prof.SampleType {
			if st.Type == name {
				return idx
			}
		}
	}
	if prof.DefaultSampleType != "" {
		for idx, st := range prof.SampleType {
			if st.Type == prof.DefaultSampleType {
				return idx
			}
		}
	}
	return 0
}

func sampleValue(sample *profile.Sample, idx int) int {
	if idx >= 0 && idx < len(sample.Value) {
		return int(sample.Value[idx])
	}
	if len(sample.Value) > 0 {
		return int(sample.Value[0])
	}
	return 1
}

func sampleState(sample *profile.Sample) string {
	if sample == nil {
		return ""
	}
	for _, key := range []string{"state", "status", "goroutine", "goroutine_state"} {
		if values, ok := sample.Label[key]; ok && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func stateFromReason(reason string) string {
	switch reason {
	case "syscall":
		return "syscall"
	case "chan receive", "chan send", "select", "mutex", "rwmutex", "cond", "io wait", "sleep", "timer", "network poll", "parked":
		return "waiting"
	default:
		return ""
	}
}

func detectWaitReason(stack []string) string {
	for _, frame := range stack {
		lower := strings.ToLower(frame)
		switch {
		case strings.Contains(lower, "runtime.chanrecv"):
			return "chan receive"
		case strings.Contains(lower, "runtime.chansend"):
			return "chan send"
		case strings.Contains(lower, "runtime.selectgo"):
			return "select"
		case strings.Contains(lower, "sync.(*mutex).lock") || strings.Contains(lower, "runtime.semacquire"):
			return "mutex"
		case strings.Contains(lower, "sync.(*rwmutex).") || strings.Contains(lower, "runtime.semacquiremutex"):
			return "rwmutex"
		case strings.Contains(lower, "sync.(*cond).wait"):
			return "cond"
		case strings.Contains(lower, "time.sleep") || strings.Contains(lower, "runtime.usleep"):
			return "sleep"
		case strings.Contains(lower, "net.(*polldesc).wait") || strings.Contains(lower, "internal/poll"):
			return "io wait"
		case strings.Contains(lower, "runtime.netpoll"):
			return "network poll"
		case strings.Contains(lower, "syscall.") || strings.Contains(lower, "runtime.cgocall"):
			return "syscall"
		case strings.Contains(lower, "runtime.gopark"):
			return "parked"
		}
	}
	return ""
}

func stackFrames(sample *profile.Sample) []string {
	if sample == nil {
		return nil
	}
	frames := make([]string, 0, len(sample.Location))
	for _, loc := range sample.Location {
		if loc == nil {
			continue
		}
		for _, line := range loc.Line {
			if line.Function == nil {
				continue
			}
			if line.Function.Name == "" {
				continue
			}
			frames = append(frames, line.Function.Name)
			break
		}
	}
	return frames
}

func stackSignature(frames []string, max int) string {
	if len(frames) == 0 {
		return ""
	}
	if max > 0 && len(frames) > max {
		frames = frames[:max]
	}
	return strings.Join(frames, " | ")
}

func topWaitReasons(reasons map[string]*waitInfo, limit int) []GoroutineWaitReason {
	items := make([]GoroutineWaitReason, 0, len(reasons))
	for reason, info := range reasons {
		items = append(items, GoroutineWaitReason{
			Reason:      reason,
			Count:       info.count,
			SampleStack: info.sampleStack,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func topLeakCandidates(leaks map[string]*leakInfo, threshold, limit int) []GoroutineLeakCandidate {
	candidates := make([]GoroutineLeakCandidate, 0, len(leaks))
	for signature, info := range leaks {
		severity := ""
		switch {
		case info.count >= threshold:
			severity = "high"
		case info.count >= threshold/2:
			severity = "medium"
		default:
			continue
		}
		candidates = append(candidates, GoroutineLeakCandidate{
			StackSignature: signature,
			Count:          info.count,
			Severity:       severity,
			State:          info.state,
			WaitReason:     info.waitReason,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Count > candidates[j].Count
	})
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}
