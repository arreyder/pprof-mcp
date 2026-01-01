package d2

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GetToken retrieves the debug token from the pod via the local port-forward
func GetToken(ctx context.Context, localPort int) (string, error) {
	url := fmt.Sprintf("https://127.0.0.1:%d/debug/token", localPort)

	// Create HTTP client that skips TLS verification (self-signed certs in dev)
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add the required header to reveal the token
	req.Header.Set("Ductone-Profile", "true")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	// Response format: "token: <token>\n"
	tokenStr := string(body)
	tokenStr = strings.TrimSpace(tokenStr)
	tokenStr = strings.TrimPrefix(tokenStr, "token:")
	tokenStr = strings.TrimSpace(tokenStr)

	if tokenStr == "" {
		return "", fmt.Errorf("received empty token")
	}

	return tokenStr, nil
}
