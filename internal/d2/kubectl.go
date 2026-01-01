package d2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// PodInfo contains information about a discovered pod
type PodInfo struct {
	Name      string
	Namespace string
	IP        string
	Status    string
}

// PortForward manages a kubectl port-forward session
type PortForward struct {
	cmd        *exec.Cmd
	localPort  int
	remotePort int
	cancel     context.CancelFunc
}

// FindPod discovers a pod for the given service in the default namespace
func FindPod(ctx context.Context, service string) (*PodInfo, error) {
	// Use app label to find pod
	label := fmt.Sprintf("app=%s", service)

	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", "default",
		"-l", label,
		"-o", "json")

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("kubectl get pods failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("kubectl get pods failed: %w", err)
	}

	var result struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Status struct {
				Phase string `json:"phase"`
				PodIP string `json:"podIP"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse kubectl output: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no pods found for service %s (label: %s)", service, label)
	}

	// Use the first running pod
	for _, item := range result.Items {
		if item.Status.Phase == "Running" {
			return &PodInfo{
				Name:      item.Metadata.Name,
				Namespace: item.Metadata.Namespace,
				IP:        item.Status.PodIP,
				Status:    item.Status.Phase,
			}, nil
		}
	}

	return nil, fmt.Errorf("no running pods found for service %s", service)
}

// StartPortForward starts a kubectl port-forward to the pod's debug port
func StartPortForward(ctx context.Context, pod *PodInfo, remotePort int) (*PortForward, error) {
	// Find an available local port
	localPort, err := findAvailablePort()
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}

	// Create a cancellable context for the port-forward command
	fwdCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(fwdCtx, "kubectl", "port-forward",
		"-n", pod.Namespace,
		pod.Name,
		fmt.Sprintf("%d:%d", localPort, remotePort))

	// Start the port-forward in the background
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start port-forward: %w", err)
	}

	pf := &PortForward{
		cmd:        cmd,
		localPort:  localPort,
		remotePort: remotePort,
		cancel:     cancel,
	}

	// Wait for port-forward to be ready
	if err := pf.waitForReady(ctx); err != nil {
		pf.Stop()
		return nil, err
	}

	return pf, nil
}

// LocalPort returns the local port being forwarded
func (pf *PortForward) LocalPort() int {
	return pf.localPort
}

// Stop terminates the port-forward
func (pf *PortForward) Stop() {
	if pf.cancel != nil {
		pf.cancel()
	}
	if pf.cmd != nil && pf.cmd.Process != nil {
		_ = pf.cmd.Process.Kill()
		_ = pf.cmd.Wait()
	}
}

// waitForReady waits for the port-forward to be ready by attempting to connect
func (pf *PortForward) waitForReady(ctx context.Context) error {
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	addr := fmt.Sprintf("127.0.0.1:%d", pf.localPort)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for port-forward to be ready")
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				// Give it a bit more time to stabilize
				time.Sleep(200 * time.Millisecond)
				return nil
			}
		}
	}
}

// findAvailablePort finds an available local port
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// ListServices returns a list of available services in the default namespace
func ListServices(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", "default",
		"-o", "json")

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("kubectl get pods failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("kubectl get pods failed: %w", err)
	}

	var result struct {
		Items []struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse kubectl output: %w", err)
	}

	// Collect unique service names from app labels
	serviceSet := make(map[string]bool)
	for _, item := range result.Items {
		if item.Status.Phase == "Running" {
			if app, ok := item.Metadata.Labels["app"]; ok {
				// Filter to services that look like backend services
				if strings.HasPrefix(app, "be-") || strings.HasPrefix(app, "pub-") {
					serviceSet[app] = true
				}
			}
		}
	}

	services := make([]string, 0, len(serviceSet))
	for service := range serviceSet {
		services = append(services, service)
	}

	return services, nil
}
