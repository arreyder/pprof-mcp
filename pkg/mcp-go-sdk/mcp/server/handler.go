package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/conductorone/mcp-go-sdk/mcp/schema"
	"github.com/conductorone/mcp-go-sdk/mcp/server/transport"
)

// Context keys
type contextKey string

const (
	ctxKeySessionID contextKey = "sessionID"
)

// MCPHandler defines the core functionality to handle MCP messages
type MCPHandler interface {
	// HandleRequest processes a JSON-RPC request and returns a response
	HandleRequest(ctx context.Context, req schema.JSONRPCRequest) (schema.JSONRPCResponse, error)

	// HandleNotification processes a JSON-RPC notification (no response)
	HandleNotification(ctx context.Context, note schema.JSONRPCNotification) error

	// SendNotification allows sending a notification to the client
	SendNotification(ctx context.Context, note schema.JSONRPCNotification) error

	// RegisterNotificationHandler registers a function to handle notifications for a specific session
	RegisterNotificationHandler(ctx context.Context, sessionID string, handler transport.NotificationHandler) error

	// UnregisterNotificationHandler removes a notification handler for a session
	UnregisterNotificationHandler(ctx context.Context, sessionID string) error
}

// DefaultHandler is the default implementation of MCPHandler
type DefaultHandler struct {
	// Server options
	options *ServerOptions

	// Initialized flag
	initialized bool

	// Notification handlers
	notificationHandlers map[string]transport.NotificationHandler
	handlersMu           sync.RWMutex
}

// NewDefaultHandler creates a new default handler with the given options
func NewDefaultHandler(options *ServerOptions) *DefaultHandler {
	if options == nil {
		options = &ServerOptions{
			Name: "unknown-server",
		}
	}

	return &DefaultHandler{
		options: options,
	}
}

// HandleRequest processes a JSON-RPC request and returns a response
func (h *DefaultHandler) HandleRequest(ctx context.Context, req schema.JSONRPCRequest) (schema.JSONRPCResponse, error) {
	// Handle initialization specially (allowed before initialized state)
	if req.Method == "initialize" {
		if h.initialized {
			// Already initialized, return error
			return createErrorResponse(req.ID, schema.InvalidRequest, "Server already initialized"), nil
		}

		result, err := h.handleInitialize(ctx, req)
		if err != nil {
			return createErrorResponse(req.ID, schema.InternalError, err.Error()), nil
		}

		// Create success response
		resp := schema.JSONRPCResponse{
			JSONRPC: schema.JSONRPCVersion,
			ID:      req.ID,
			Result:  result,
		}
		return resp, nil
	}

	// All other methods require the server to be initialized
	if !h.initialized {
		return createErrorResponse(req.ID, schema.InvalidRequest, "Server not initialized"), nil
	}

	// Handle various request methods
	var result json.RawMessage
	var err error

	switch req.Method {
	case "ping":
		result, err = h.handlePing(ctx, req)
	case "tools/list":
		result, err = h.handleToolsList(ctx, req)
	case "tools/call":
		result, err = h.handleToolsCall(ctx, req)
	default:
		err = fmt.Errorf("%w: %s", ErrMethodNotFound, req.Method)
	}

	if err != nil {
		var code int
		var message string

		switch {
		case errors.Is(err, ErrMethodNotFound):
			code = schema.MethodNotFound
			message = fmt.Sprintf("Method not found: %s", req.Method)
		case errors.Is(err, ErrInvalidParams):
			code = schema.InvalidParams
			message = "Invalid parameters"
		default:
			code = schema.InternalError
			message = err.Error()
		}

		return createErrorResponse(req.ID, code, message), nil
	}

	// Create success response
	resp := schema.JSONRPCResponse{
		JSONRPC: schema.JSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}
	return resp, nil
}

// HandleNotification processes a JSON-RPC notification (no response needed)
func (h *DefaultHandler) HandleNotification(ctx context.Context, note schema.JSONRPCNotification) error {
	// All notifications require the server to be initialized
	if !h.initialized && note.Method != "notifications/initialized" {
		return ErrNotInitialized
	}

	switch note.Method {
	case "notifications/initialized":
		// Client is confirming initialization is complete
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrMethodNotFound, note.Method)
	}
}

// SendNotification allows sending a notification to the client
func (h *DefaultHandler) SendNotification(ctx context.Context, note schema.JSONRPCNotification) error {
	// Extract session ID from context if available
	sessionID, _ := ctx.Value(ctxKeySessionID).(string)
	if sessionID == "" {
		// No specific session, broadcast to all handlers
		return h.broadcastNotification(ctx, note)
	}

	// Send to specific session
	h.handlersMu.RLock()
	handler, ok := h.notificationHandlers[sessionID]
	h.handlersMu.RUnlock()

	if !ok {
		return errors.New("no notification handler registered for session")
	}

	return handler(ctx, note)
}

// handleInitialize processes an initialization request
func (h *DefaultHandler) handleInitialize(ctx context.Context, req schema.JSONRPCRequest) (json.RawMessage, error) {
	var params schema.InitializeRequestParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, ErrInvalidParams
	}

	// Check if the client's protocol version is supported
	selectedVersion := ""
	for _, version := range h.options.SupportedProtocolVersions {
		if version == params.ProtocolVersion {
			selectedVersion = version
			break
		}
	}

	// If client version not supported, use our most preferred version
	if selectedVersion == "" {
		selectedVersion = h.options.SupportedProtocolVersions[0]
	}

	// Create capabilities
	capabilities := schema.ServerCapabilities{}

	// Add tools capability if we have a provider
	if h.options.ToolsProvider != nil {
		capabilities.Tools = &schema.ToolsCapability{
			Supported:   true,
			ListChanged: true,
		}
	}

	// Create and marshal the result
	result := schema.InitializeResult{
		ProtocolVersion: selectedVersion,
		ServerInfo: schema.Implementation{
			Name:    h.options.Name,
			Version: h.options.Version,
		},
		Capabilities: capabilities,
		Instructions: h.options.Instructions,
	}

	// Mark as initialized
	h.initialized = true

	return json.Marshal(result)
}

// handlePing processes a ping request
func (h *DefaultHandler) handlePing(ctx context.Context, req schema.JSONRPCRequest) (json.RawMessage, error) {
	// Simple echo response
	return json.Marshal(map[string]interface{}{
		"result": "pong",
	})
}

// handleToolsList processes a tools/list request
func (h *DefaultHandler) handleToolsList(ctx context.Context, req schema.JSONRPCRequest) (json.RawMessage, error) {
	// Check if tools are supported
	if h.options.ToolsProvider == nil {
		return nil, ErrNotSupported
	}

	// Parse request parameters (params can be null/missing for tools/list)
	var params struct {
		Cursor string `json:"cursor,omitempty"`
	}

	if len(req.Params) > 0 && string(req.Params) != "null" {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, ErrInvalidParams
		}
	}

	// Get tools from provider
	tools, err := h.options.ToolsProvider.List(ctx)
	if err != nil {
		return nil, err
	}

	// Create and marshal result
	result := schema.ListToolsResult{
		Tools: tools,
		// No pagination in this simple implementation
	}

	return json.Marshal(result)
}

// handleToolsCall processes a tools/call request
func (h *DefaultHandler) handleToolsCall(ctx context.Context, req schema.JSONRPCRequest) (json.RawMessage, error) {
	// Check if tools are supported
	if h.options.ToolsProvider == nil {
		return nil, ErrNotSupported
	}

	// Parse request parameters
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, ErrInvalidParams
	}

	// Call the tool
	result, err := h.options.ToolsProvider.Call(ctx, params.Name, params.Arguments)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

// createErrorResponse creates a JSON-RPC error response
func createErrorResponse(id schema.RequestID, code int, message string) schema.JSONRPCResponse {
	result, _ := json.Marshal(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})

	return schema.JSONRPCResponse{
		JSONRPC: schema.JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
}

// RegisterNotificationHandler registers a function to handle notifications for a specific session
func (h *DefaultHandler) RegisterNotificationHandler(ctx context.Context, sessionID string, handler transport.NotificationHandler) error {
	h.handlersMu.Lock()
	defer h.handlersMu.Unlock()

	if h.notificationHandlers == nil {
		h.notificationHandlers = make(map[string]transport.NotificationHandler)
	}

	h.notificationHandlers[sessionID] = handler
	return nil
}

// UnregisterNotificationHandler removes a notification handler for a session
func (h *DefaultHandler) UnregisterNotificationHandler(ctx context.Context, sessionID string) error {
	h.handlersMu.Lock()
	defer h.handlersMu.Unlock()

	if h.notificationHandlers != nil {
		delete(h.notificationHandlers, sessionID)
	}

	return nil
}

// Add a helper method for broadcasting notifications
func (h *DefaultHandler) broadcastNotification(ctx context.Context, note schema.JSONRPCNotification) error {
	h.handlersMu.RLock()
	handlers := make([]transport.NotificationHandler, 0, len(h.notificationHandlers))
	for _, handler := range h.notificationHandlers {
		handlers = append(handlers, handler)
	}
	h.handlersMu.RUnlock()

	if len(handlers) == 0 {
		return errors.New("no notification handlers registered")
	}

	var lastErr error
	for _, handler := range handlers {
		if err := handler(ctx, note); err != nil {
			lastErr = err
		}
	}

	return lastErr
}
