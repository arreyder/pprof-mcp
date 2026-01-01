package d2

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BranchImpactParams contains parameters for comparing profiles between branches
type BranchImpactParams struct {
	Service       string
	BeforeRef     string // default: "main"
	AfterRef      string // default: current branch
	OutDir        string
	Seconds       int           // CPU profile duration (default: 30)
	RebuildTimeout time.Duration // default: 5 minutes
	WarmupDelay   time.Duration // default: 15 seconds
}

// BranchImpactResult contains the results of a branch comparison
type BranchImpactResult struct {
	Service       string                `json:"service"`
	BeforeRef     string                `json:"before_ref"`
	AfterRef      string                `json:"after_ref"`
	BeforeProfiles DownloadResult       `json:"before_profiles"`
	AfterProfiles  DownloadResult       `json:"after_profiles"`
	UpdateMethod  string                `json:"update_method"` // "live_update", "pod_restart", or "pod_recreate"
	GitStashed    bool                  `json:"git_stashed"`
	Warnings      []string              `json:"warnings,omitempty"`
}

// TiltState captures the current state of a Tilt resource
type TiltState struct {
	PodName           string
	StartedAt         time.Time
	LastFileTimeSynced *time.Time
	ContainerID       string
}

// CompareBranches profiles a service on two different git branches
func CompareBranches(ctx context.Context, params BranchImpactParams) (BranchImpactResult, error) {
	// Set defaults
	if params.BeforeRef == "" {
		params.BeforeRef = "main"
	}
	if params.Seconds <= 0 {
		params.Seconds = 30
	}
	if params.RebuildTimeout == 0 {
		params.RebuildTimeout = 5 * time.Minute
	}
	if params.WarmupDelay == 0 {
		params.WarmupDelay = 15 * time.Second
	}

	result := BranchImpactResult{
		Service:    params.Service,
		BeforeRef:  params.BeforeRef,
		GitStashed: false,
		Warnings:   []string{},
	}

	// Get current branch
	currentBranch, err := getCurrentBranch(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Determine after ref
	if params.AfterRef == "" {
		params.AfterRef = currentBranch
	}
	result.AfterRef = params.AfterRef

	// Check for uncommitted changes
	hasChanges, err := hasUncommittedChanges(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to check git status: %w", err)
	}

	if hasChanges {
		// Auto-stash changes
		if err := gitStash(ctx); err != nil {
			return result, fmt.Errorf("failed to stash changes: %w", err)
		}
		result.GitStashed = true
	}

	// Ensure we restore state on exit
	defer func() {
		// Switch back to original branch
		if err := gitCheckout(ctx, currentBranch); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to restore branch %s: %v", currentBranch, err))
		}

		// Restore stashed changes
		if result.GitStashed {
			if err := gitStashPop(ctx); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to restore stashed changes: %v", err))
			}
		}
	}()

	// Step 1: Capture baseline profile from before_ref
	if err := gitCheckout(ctx, params.BeforeRef); err != nil {
		return result, fmt.Errorf("failed to checkout %s: %w", params.BeforeRef, err)
	}

	// Wait for rebuild after switching to before_ref
	updateMethod, err := waitForRebuild(ctx, params.Service, params.RebuildTimeout, params.WarmupDelay)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("rebuild detection warning: %v", err))
		// Continue anyway - maybe service was already on this branch
	}

	beforeProfiles, err := DownloadProfiles(ctx, DownloadParams{
		Service: params.Service,
		OutDir:  params.OutDir + "/before",
		Seconds: params.Seconds,
	})
	if err != nil {
		return result, fmt.Errorf("failed to download before profiles: %w", err)
	}
	result.BeforeProfiles = beforeProfiles

	// Step 2: Switch to after_ref
	if err := gitCheckout(ctx, params.AfterRef); err != nil {
		return result, fmt.Errorf("failed to checkout %s: %w", params.AfterRef, err)
	}

	// Wait for rebuild
	updateMethod, err = waitForRebuild(ctx, params.Service, params.RebuildTimeout, params.WarmupDelay)
	if err != nil {
		return result, fmt.Errorf("failed waiting for rebuild: %w", err)
	}
	result.UpdateMethod = updateMethod

	// Step 3: Capture after profile
	afterProfiles, err := DownloadProfiles(ctx, DownloadParams{
		Service: params.Service,
		OutDir:  params.OutDir + "/after",
		Seconds: params.Seconds,
	})
	if err != nil {
		return result, fmt.Errorf("failed to download after profiles: %w", err)
	}
	result.AfterProfiles = afterProfiles

	return result, nil
}

// waitForRebuild waits for Tilt to rebuild the service after a git change
func waitForRebuild(ctx context.Context, service string, timeout, warmupDelay time.Duration) (string, error) {
	// Capture initial state
	initialState, err := getTiltState(ctx, service)
	if err != nil {
		return "", fmt.Errorf("failed to get initial tilt state: %w", err)
	}

	// Initial delay to let Tilt detect the change
	time.Sleep(5 * time.Second)

	// Poll for changes
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Until(deadline)):
			return "", fmt.Errorf("timeout waiting for rebuild after %v", timeout)
		case <-ticker.C:
			currentState, err := getTiltState(ctx, service)
			if err != nil {
				// Tilt state unavailable, continue polling
				continue
			}

			// Detect what changed
			if currentState.PodName != initialState.PodName {
				// Pod was recreated (full rebuild)
				time.Sleep(warmupDelay)
				return "pod_recreate", nil
			}

			if !currentState.StartedAt.Equal(initialState.StartedAt) {
				// Container restarted
				time.Sleep(warmupDelay)
				return "pod_restart", nil
			}

			if currentState.LastFileTimeSynced != nil && initialState.LastFileTimeSynced != nil {
				if currentState.LastFileTimeSynced.After(*initialState.LastFileTimeSynced) {
					// Live update happened
					time.Sleep(warmupDelay)
					return "live_update", nil
				}
			} else if currentState.LastFileTimeSynced != nil && initialState.LastFileTimeSynced == nil {
				// First live update
				time.Sleep(warmupDelay)
				return "live_update", nil
			}
		}
	}
}

// findTiltResource finds the exact Tilt resource name for a service (with fuzzy matching)
func findTiltResource(ctx context.Context, service string) (string, error) {
	// Try exact match first
	cmd := exec.CommandContext(ctx, "tilt", "get", "kubernetesdiscovery", service, "-o", "json")
	if err := cmd.Run(); err == nil {
		return service, nil
	}

	// Fuzzy match - list all resources and find one containing the service name
	cmd = exec.CommandContext(ctx, "tilt", "get", "kubernetesdiscovery", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list kubernetesdiscovery resources: %w", err)
	}

	var result struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse kubernetesdiscovery list: %w", err)
	}

	serviceLower := strings.ToLower(service)
	for _, item := range result.Items {
		nameLower := strings.ToLower(item.Metadata.Name)
		if strings.Contains(nameLower, serviceLower) || strings.Contains(serviceLower, nameLower) {
			return item.Metadata.Name, nil
		}
	}

	return "", fmt.Errorf("no tilt resource found matching %q", service)
}

// getTiltState queries Tilt API for current service state
func getTiltState(ctx context.Context, service string) (*TiltState, error) {
	state := &TiltState{}

	// Find the exact Tilt resource name (might be be-ratelimit when service is ratelimit)
	tiltResourceName, err := findTiltResource(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("failed to find tilt resource: %w", err)
	}

	// Get KubernetesDiscovery state (pod name, startedAt)
	kdCmd := exec.CommandContext(ctx, "tilt", "get", "kubernetesdiscovery", tiltResourceName, "-o", "json")
	kdOutput, err := kdCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetesdiscovery: %w", err)
	}

	var kdResult struct {
		Status struct {
			Pods []struct {
				Name       string `json:"name"`
				Containers []struct {
					State struct {
						Running *struct {
							StartedAt time.Time `json:"startedAt"`
						} `json:"running"`
					} `json:"state"`
				} `json:"containers"`
			} `json:"pods"`
		} `json:"status"`
	}

	if err := json.Unmarshal(kdOutput, &kdResult); err != nil {
		return nil, fmt.Errorf("failed to parse kubernetesdiscovery output: %w", err)
	}

	if len(kdResult.Status.Pods) > 0 {
		state.PodName = kdResult.Status.Pods[0].Name
		if len(kdResult.Status.Pods[0].Containers) > 0 {
			if running := kdResult.Status.Pods[0].Containers[0].State.Running; running != nil {
				state.StartedAt = running.StartedAt
			}
		}
	}

	// Get LiveUpdate state (lastFileTimeSynced)
	luCmd := exec.CommandContext(ctx, "tilt", "get", "liveupdate", "-o", "json")
	luOutput, err := luCmd.Output()
	if err != nil {
		// LiveUpdate might not exist, that's ok
		return state, nil
	}

	var luResult struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Containers []struct {
					PodName            string     `json:"podName"`
					LastFileTimeSynced *time.Time `json:"lastFileTimeSynced"`
					ContainerID        string     `json:"containerID"`
				} `json:"containers"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(luOutput, &luResult); err != nil {
		return state, nil // Ignore LiveUpdate parse errors
	}

	// Find matching LiveUpdate resource (use tiltResourceName for matching)
	for _, item := range luResult.Items {
		if strings.Contains(item.Metadata.Name, tiltResourceName) {
			if len(item.Status.Containers) > 0 {
				state.LastFileTimeSynced = item.Status.Containers[0].LastFileTimeSynced
				state.ContainerID = item.Status.Containers[0].ContainerID
			}
			break
		}
	}

	return state, nil
}

// Git helper functions

func getCurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func hasUncommittedChanges(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

func gitStash(ctx context.Context) error {
	timestamp := time.Now().Format("20060102-150405")
	message := fmt.Sprintf("d2_profile_branch_impact auto-stash %s", timestamp)
	cmd := exec.CommandContext(ctx, "git", "stash", "push", "-m", message)
	return cmd.Run()
}

func gitStashPop(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "stash", "pop")
	return cmd.Run()
}

func gitCheckout(ctx context.Context, ref string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", ref)
	return cmd.Run()
}
