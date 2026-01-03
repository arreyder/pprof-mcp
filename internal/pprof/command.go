package pprof

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/arreyder/pprof-mcp/internal/textutil"
)

type commandOutput struct {
	Stdout     string
	Stderr     string
	StdoutMeta textutil.TruncateMeta
	StderrMeta textutil.TruncateMeta
}

func runCommand(ctx context.Context, name string, args ...string) (commandOutput, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// Stream stdout/stderr into capped buffers to avoid unbounded memory usage.
	stdoutBuf := newCappedBuffer(maxStdoutBytes())
	stderrBuf := newCappedBuffer(maxStderrBytes())
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf
	err := cmd.Run()
	return commandOutput{
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		StdoutMeta: stdoutBuf.Meta(),
		StderrMeta: stderrBuf.Meta(),
	}, err
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

const (
	defaultMaxStdoutBytes = 1_000_000
	defaultMaxStderrBytes = 200_000
)

var (
	maxStdoutBytesOnce sync.Once
	maxStdoutBytesVal  int
	maxStderrBytesOnce sync.Once
	maxStderrBytesVal  int
)

func maxStdoutBytes() int {
	maxStdoutBytesOnce.Do(func() {
		maxStdoutBytesVal = readMaxBytesEnv("PPROF_MCP_MAX_STDOUT_BYTES", defaultMaxStdoutBytes)
	})
	return maxStdoutBytesVal
}

func maxStderrBytes() int {
	maxStderrBytesOnce.Do(func() {
		maxStderrBytesVal = readMaxBytesEnv("PPROF_MCP_MAX_STDERR_BYTES", defaultMaxStderrBytes)
	})
	return maxStderrBytesVal
}

func readMaxBytesEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

type cappedBuffer struct {
	buf       bytes.Buffer
	maxBytes  int
	total     int
	lines     int
	lastByte  byte
	hasData   bool
	truncated bool
}

func newCappedBuffer(maxBytes int) *cappedBuffer {
	if maxBytes <= 0 {
		maxBytes = defaultMaxStdoutBytes
	}
	return &cappedBuffer{maxBytes: maxBytes}
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	c.total += len(p)
	if len(p) > 0 {
		c.lastByte = p[len(p)-1]
		c.hasData = true
	}
	for _, b := range p {
		if b == '\n' {
			c.lines++
		}
	}
	if c.maxBytes <= 0 {
		_, err := c.buf.Write(p)
		return len(p), err
	}
	remaining := c.maxBytes - c.buf.Len()
	if remaining > 0 {
		if len(p) <= remaining {
			_, _ = c.buf.Write(p)
		} else {
			_, _ = c.buf.Write(p[:remaining])
			c.truncated = true
		}
	} else {
		c.truncated = true
	}
	return len(p), nil
}

func (c *cappedBuffer) String() string {
	return c.buf.String()
}

func (c *cappedBuffer) Meta() textutil.TruncateMeta {
	lines := c.lines
	if c.hasData && c.lastByte != '\n' {
		lines++
	}
	meta := textutil.TruncateMeta{
		TotalLines: lines,
		TotalBytes: c.total,
		Truncated:  c.truncated,
	}
	if meta.Truncated {
		meta.TruncatedReason = "max_bytes"
	}
	return meta
}
