package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/conductorone/mcp-go-sdk/mcp/schema"
)

// StdioHandler handles communication over stdin/stdout
type StdioHandler struct {
	handler MCPHandler
	reader  *bufio.Reader
	writer  *bufio.Writer
	mu      sync.Mutex
}

// NewStdioHandler creates a new stdio handler
func NewStdioHandler(handler MCPHandler, r io.Reader, w io.Writer) *StdioHandler {
	return &StdioHandler{
		handler: handler,
		reader:  bufio.NewReader(r),
		writer:  bufio.NewWriter(w),
	}
}

// ProcessMessage handles an incoming JSON-RPC message
func (h *StdioHandler) ProcessMessage(ctx context.Context, rawMsg []byte) error {
	// Try to decode as a single message first
	var msg json.RawMessage
	if err := json.Unmarshal(rawMsg, &msg); err != nil {
		// Not valid JSON, return error
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Check message type
	var rawObj map[string]json.RawMessage
	if err := json.Unmarshal(msg, &rawObj); err != nil {
		return fmt.Errorf("invalid JSON-RPC message: %w", err)
	}

	// Check if it's a request (has both "id" and "method") or notification (has "method" but no "id")
	_, hasID := rawObj["id"]
	_, hasMethod := rawObj["method"]

	if hasMethod {
		if hasID {
			// It's a request
			var req schema.JSONRPCRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				return fmt.Errorf("invalid JSON-RPC request: %w", err)
			}

			// Process the request
			resp, err := h.handler.HandleRequest(ctx, req)
			if err != nil {
				return fmt.Errorf("error handling request: %w", err)
			}

			// Send the response
			return h.sendResponse(resp)
		} else {
			// It's a notification
			var note schema.JSONRPCNotification
			if err := json.Unmarshal(msg, &note); err != nil {
				return fmt.Errorf("invalid JSON-RPC notification: %w", err)
			}

			// Process the notification (no response needed)
			return h.handler.HandleNotification(ctx, note)
		}
	}

	// Not a valid JSON-RPC message
	return fmt.Errorf("invalid JSON-RPC message: missing method")
}

// sendResponse sends a JSON-RPC response
func (h *StdioHandler) sendResponse(resp schema.JSONRPCResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Encode response
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("error encoding response: %w", err)
	}

	// Write response with newline
	if _, err := h.writer.Write(data); err != nil {
		return fmt.Errorf("error writing response: %w", err)
	}
	if _, err := h.writer.WriteString("\n"); err != nil {
		return fmt.Errorf("error writing newline: %w", err)
	}

	// Flush writer
	return h.writer.Flush()
}

// ServeStdio serves an MCP handler over stdio
func ServeStdio(ctx context.Context, handler MCPHandler, r io.Reader, w io.Writer) error {
	stdioHandler := NewStdioHandler(handler, r, w)
	reader := bufio.NewReader(r)

	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}

		// Read a line of input
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				// EOF means the pipe was closed, which is normal termination
				return nil
			}
			return fmt.Errorf("error reading input: %w", err)
		}

		// Process the message
		if err := stdioHandler.ProcessMessage(ctx, line); err != nil {
			// Log the error but don't terminate the server
			fmt.Fprintf(w, "{\"jsonrpc\":\"2.0\",\"id\":null,\"error\":{\"code\":-32700,\"message\":\"Error processing message: %s\"}}\n", err.Error())
		}
	}
}
