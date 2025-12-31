package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/conductorone/mcp-go-sdk/mcp/schema"
	"github.com/google/uuid"
	slogctx "github.com/veqryn/slog-context"
)

// Constants for HTTP headers
const (
	contentTypeHeader      = "Content-Type"
	contentTypeJSON        = "application/json"
	contentTypeEventStream = "text/event-stream"
	cacheControlHeader     = "Cache-Control"
	cacheControlNoCache    = "no-cache"
	connectionHeader       = "Connection"
	connectionKeepAlive    = "keep-alive"
	corsOriginHeader       = "Access-Control-Allow-Origin"
	corsOriginAll          = "*"
	acceptHeader           = "Accept"

	// Headers for protocol and host detection
	forwardedProtoHeader = "X-Forwarded-Proto"
	forwardedSSLHeader   = "X-Forwarded-SSL"
	forwardedHTTPSHeader = "X-Forwarded-HTTPS"
	forwardedHeader      = "Forwarded"
	forwardedHostHeader  = "X-Forwarded-Host"
)

// Constants for SSE events
const (
	sseEndpointEvent     = "event: endpoint"
	sseDataPrefix        = "data: "
	sseLineEnding        = "\n\n"
	sseConnectedMessage  = "Connected to MCP server"
	sseEndpointURLPrefix = "/messages"
)

// ErrConnectionClosed is returned when attempting to use a closed connection
var ErrConnectionClosed = errors.New("connection closed")

// SSEConnection represents a Server-Sent Events connection to a client
type SSEConnection interface {
	// Connection is the base interface that all connections must implement
	Connection

	// SessionID returns the associated session ID, if any
	SessionID() string

	// ClientInfo returns identifying information about the client
	ClientInfo() map[string]string
}

// sseConnection is the standard implementation of an SSE connection
type sseConnection struct {
	id         string
	sessionID  string
	clientInfo map[string]string
	reqCtx     context.Context
	writer     http.ResponseWriter
	flusher    http.Flusher
	closed     atomic.Bool
	mu         sync.Mutex
}

// NewSSEConnection creates a new SSE connection
func NewSSEConnection(w http.ResponseWriter, r *http.Request, sessionID string) (SSEConnection, error) {
	// Check if writer supports flushing (required for SSE)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported by underlying http.ResponseWriter")
	}

	// Set SSE headers
	h := w.Header()
	h.Set(contentTypeHeader, contentTypeEventStream)
	h.Set(cacheControlHeader, cacheControlNoCache)
	h.Set(connectionHeader, connectionKeepAlive)
	h.Set(corsOriginHeader, corsOriginAll)

	// Generate connection ID
	connID := uuid.New().String()

	// Extract client info from request
	clientInfo := map[string]string{
		"user-agent":  r.UserAgent(),
		"remote-addr": r.RemoteAddr,
	}

	return &sseConnection{
		id:         connID,
		sessionID:  sessionID,
		clientInfo: clientInfo,
		reqCtx:     r.Context(),
		writer:     w,
		flusher:    flusher,
	}, nil
}

// ID returns the unique connection identifier
func (c *sseConnection) ID() string {
	return c.id
}

// SessionID returns the associated session ID
func (c *sseConnection) SessionID() string {
	return c.sessionID
}

// ClientInfo returns client information
func (c *sseConnection) ClientInfo() map[string]string {
	return c.clientInfo
}

// Send delivers a message to the client
func (c *sseConnection) Send(ctx context.Context, message []byte) error {
	// Check context cancellation first
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	log := slogctx.FromCtx(ctx)

	if c.closed.Load() {
		log.DebugContext(ctx,
			"Failed to send message to closed client",
			"error", ErrConnectionClosed.Error(),
			"connection_id", c.id,
		)
		return ErrConnectionClosed
	}

	// Create buffer for the SSE message
	var buf bytes.Buffer
	_, _ = buf.WriteString(sseDataPrefix)
	_, _ = buf.Write(message)
	_, _ = buf.WriteString(sseLineEnding)

	// Check context again before writing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Write the entire message in one go
	_, err := io.Copy(c.writer, &buf)
	if err != nil {
		log.ErrorContext(ctx,
			"Failed to write SSE message",
			"error", err.Error(),
			"connection_id", c.id,
		)
		return fmt.Errorf("failed to write SSE message: %w", err)
	}

	c.flusher.Flush()
	log.Debug("Sent SSE message", "connection_id", c.id, "message_size", len(message))
	return nil
}

// Close terminates the connection
func (c *sseConnection) Close() error {
	// Just set it to closed and return
	ok := c.closed.CompareAndSwap(false, true)
	if ok {
		log := slogctx.FromCtx(c.reqCtx)
		log.Debug("Closing SSE connection", "connection_id", c.id, "session_id", c.sessionID)
	}
	return nil
}

// StreamHandler handles HTTP streamable transport for MCP
type StreamHandler struct {
	handler     MCPHandler
	options     StreamOptions
	connManager ConnectionManager
	messageBus  MessageBus
}

// NewStreamHandler creates a new HTTP stream handler
func NewStreamHandler(handler MCPHandler, options StreamOptions, connManager ConnectionManager, messageBus MessageBus) *StreamHandler {
	// If no connection manager is provided, use a default in-memory implementation
	if connManager == nil {
		connManager = NewInMemoryConnectionManager()
	}

	// If no message bus is provided, use a no-op implementation for single-instance mode
	if messageBus == nil {
		messageBus = NewInMemoryMessageBus()
	}

	return &StreamHandler{
		handler:     handler,
		options:     options,
		connManager: connManager,
		messageBus:  messageBus,
	}
}

// NewInMemoryStreamHandler creates a basic HTTP stream handler for single-instance deployments
func NewInMemoryStreamHandler(handler MCPHandler, options StreamOptions) *StreamHandler {
	return NewStreamHandler(handler, options, nil, nil)
}

// detectProtocol determines the protocol (http/https) based on request and configuration
func detectProtocol(r *http.Request, trustForwardedHeaders bool) string {
	// Check if we should trust forwarded headers
	if trustForwardedHeaders {
		// 1. Check X-Forwarded-Proto header (most common)
		if proto := r.Header.Get(forwardedProtoHeader); proto != "" {
			return strings.ToLower(proto)
		}

		// 2. Check X-Forwarded-SSL header
		if ssl := r.Header.Get(forwardedSSLHeader); ssl == "on" || ssl == "1" || ssl == "true" {
			return "https"
		}

		// 3. Check X-Forwarded-HTTPS header
		if https := r.Header.Get(forwardedHTTPSHeader); https == "on" || https == "1" || https == "true" {
			return "https"
		}

		// 4. Check Forwarded header (RFC 7239)
		// Example: Forwarded: for=192.0.2.60;proto=https;by=203.0.113.43
		if forwarded := r.Header.Get(forwardedHeader); forwarded != "" {
			parts := strings.Split(forwarded, ";")
			for _, part := range parts {
				if strings.HasPrefix(strings.ToLower(part), "proto=") {
					proto := strings.TrimPrefix(strings.ToLower(part), "proto=")
					return proto
				}
			}
		}
	}

	// 5. Fall back to TLS check
	if r.TLS != nil {
		return "https"
	}

	return "http"
}

// detectHost determines the host based on request and configuration
func detectHost(r *http.Request, trustForwardedHeaders bool) string {
	// Check if we should trust forwarded headers
	if trustForwardedHeaders {
		// 1. Check X-Forwarded-Host header
		if host := r.Header.Get(forwardedHostHeader); host != "" {
			return host
		}

		// 2. Check Forwarded header (RFC 7239)
		if forwarded := r.Header.Get(forwardedHeader); forwarded != "" {
			parts := strings.Split(forwarded, ";")
			for _, part := range parts {
				if strings.HasPrefix(strings.ToLower(part), "host=") {
					host := strings.TrimPrefix(strings.ToLower(part), "host=")
					// Remove quotes if present
					host = strings.Trim(host, "\"")
					return host
				}
			}
		}
	}

	// 3. Fall back to request host
	return r.Host
}

// buildEndpointURL constructs an endpoint URL using proper URL handling
func buildEndpointURL(r *http.Request, opts StreamOptions, endpointPath string) (string, error) {
	// If BaseURL is explicitly configured, use it
	if opts.BaseURL != "" {
		baseURL, err := url.Parse(opts.BaseURL)
		if err != nil {
			return "", fmt.Errorf("invalid base URL: %w", err)
		}

		// Join the paths correctly
		baseURL.Path = path.Join(baseURL.Path, endpointPath)
		return baseURL.String(), nil
	}

	// Otherwise build URL from request
	proto := detectProtocol(r, opts.TrustForwardedHeaders)
	host := detectHost(r, opts.TrustForwardedHeaders)

	// Construct URL using net/url
	u := url.URL{
		Scheme: proto,
		Host:   host,
		Path:   endpointPath,
	}

	return u.String(), nil
}

// HandleSSE handles SSE connection requests
func (h *StreamHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := slogctx.FromCtx(ctx)

	// Check if method is GET
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Error("SSE Connection rejected", "method", r.Method, "error", "Method not allowed")
		return
	}

	// Authenticate request if handler provided
	if h.options.AuthenticationHandler != nil {
		if err := h.options.AuthenticationHandler(r); err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			log.Error("SSE Connection rejected", "error", err.Error(), "status", http.StatusUnauthorized)
			return
		}
	}

	// Get or create session ID
	sessionID := r.Header.Get(h.options.SessionHeaderName)
	if sessionID == "" && h.options.SessionRequired {
		http.Error(w, "Session required", http.StatusBadRequest)
		log.Error("SSE Connection rejected", "error", "Session required", "status", http.StatusBadRequest)
		return
	}

	// Generate a session ID if needed but not required
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Create SSE connection
	conn, err := NewSSEConnection(w, r, sessionID)
	if err != nil {
		http.Error(w, "SSE not supported: "+err.Error(), http.StatusInternalServerError)
		log.Error("SSE Connection creation failed", "error", err.Error(), "status", http.StatusInternalServerError)
		return
	}

	// Register connection
	if err := h.connManager.AddConnection(ctx, conn); err != nil {
		http.Error(w, "Failed to register connection: "+err.Error(), http.StatusInternalServerError)
		log.Error("SSE Connection registration failed", "error", err.Error(), "status", http.StatusInternalServerError)
		return
	}

	// For the 2024-11-05 protocol, send an "endpoint:" event to instruct
	// the client where to send messages
	if r.Header.Get(acceptHeader) == contentTypeEventStream {
		// Use path package to handle the path manipulation
		currentPath := r.URL.Path
		messagesPath := path.Dir(currentPath)
		if messagesPath == "/" || messagesPath == "." {
			messagesPath = ""
		}
		messagesPath = path.Join(messagesPath, sseEndpointURLPrefix)

		// Build endpoint URL using our helper function
		endpoint, err := buildEndpointURL(r, h.options, messagesPath)
		if err != nil {
			log.Error("Failed to build endpoint URL", "error", err.Error())
			http.Error(w, "Failed to create endpoint URL", http.StatusInternalServerError)
			return
		}

		// Use buffer for writing event
		var buf strings.Builder
		buf.WriteString(sseEndpointEvent)
		buf.WriteString("\ndata: ")
		buf.WriteString(endpoint)
		buf.WriteString(sseLineEnding)

		// Write the entire buffer at once
		if _, err := io.WriteString(w, buf.String()); err != nil {
			log.Error("Failed to send endpoint event", "error", err.Error())
			http.Error(w, "Failed to send endpoint information", http.StatusInternalServerError)
			return
		}

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		log.Info("Sent endpoint event for legacy protocol", "endpoint", endpoint)
	}

	// Send a comment to establish the connection
	if err := conn.Send(ctx, []byte(sseConnectedMessage)); err != nil {
		http.Error(w, "Failed to send initial message: "+err.Error(), http.StatusInternalServerError)
		log.Error("Failed to send initial SSE message", "error", err.Error(), "status", http.StatusInternalServerError)
		return
	}

	// Set up handler for client notifications and server-to-client messages
	// This goroutine will handle sending notifications to the client
	done := make(chan struct{})
	go func() {
		defer close(done)

		// Create a child context that we can cancel manually if needed
		ctxWithCancel, cancel := context.WithCancel(ctx)
		defer cancel() // Ensure we cancel this context when we're done

		// Set up notification handler to send messages to the client
		notifyHandler := func(ctx context.Context, notif schema.JSONRPCNotification) error {
			// Check if the parent context is still valid
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			notifBytes, err := json.Marshal(notif)
			if err != nil {
				return fmt.Errorf("failed to marshal notification: %w", err)
			}
			return conn.Send(ctx, notifBytes)
		}

		// Register the notification handler
		h.handler.RegisterNotificationHandler(ctxWithCancel, sessionID, notifyHandler)
		defer h.handler.UnregisterNotificationHandler(ctx, sessionID)

		// Keep connection open until client disconnects or context is canceled
		<-ctx.Done()
	}()

	// Keep the connection open until client disconnects or context is canceled
	<-ctx.Done()

	// Wait for the notification handler goroutine to finish
	<-done

	// Unregister the connection when done
	h.connManager.RemoveConnection(ctx, conn.ID())

	if err := conn.Close(); err != nil {
		log.Warn("Error closing connection", "error", err)
	}
}

// HandleMessage processes message POST requests
func (h *StreamHandler) HandleMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := slogctx.FromCtx(ctx)

	// Debug output
	log.Debug("Message received",
		"remoteAddr", r.RemoteAddr,
		"contentLength", r.ContentLength,
		"headers", r.Header)

	// Check if method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Error("Message rejected", "method", r.Method, "error", "Method not allowed")
		return
	}

	// Authenticate request if handler provided
	if h.options.AuthenticationHandler != nil {
		if err := h.options.AuthenticationHandler(r); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			log.Error("Message rejected", "error", fmt.Errorf("authentication failed: %w", err), "status", http.StatusUnauthorized)
			return
		}
	}

	// Get session ID
	sessionID := r.Header.Get(h.options.SessionHeaderName)
	if sessionID == "" && h.options.SessionRequired {
		http.Error(w, "Session required", http.StatusBadRequest)
		log.Error("Message rejected", "error", "Session required", "status", http.StatusBadRequest)
		return
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		http.Error(w, "Request canceled", http.StatusRequestTimeout)
		log.Error("Message rejected", "error", ctx.Err(), "status", http.StatusRequestTimeout)
		return
	default:
	}

	// Set a reasonable limit for the request body size (e.g., 10MB)
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		statusCode := http.StatusBadRequest
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			statusCode = http.StatusRequestTimeout
		}
		http.Error(w, "Failed to read request body", statusCode)
		log.Error("Message rejected", "error", fmt.Errorf("failed to read body: %w", err), "status", statusCode)
		return
	}

	// Check for context cancellation again after potentially lengthy read
	select {
	case <-ctx.Done():
		http.Error(w, "Request canceled", http.StatusRequestTimeout)
		log.Error("Message rejected", "error", ctx.Err(), "status", http.StatusRequestTimeout)
		return
	default:
	}

	// Debug output
	log.Debug("Message body details", "length", len(body))
	if len(body) == 0 {
		// Special case for empty body - this is likely the initial connect POST from
		// the client in streamable mode. Respond with 200 OK and session ID.
		w.Header().Set(contentTypeHeader, contentTypeJSON)
		w.Header().Set(h.options.SessionHeaderName, uuid.New().String())
		w.WriteHeader(http.StatusOK)
		return
	}

	if log.Enabled(ctx, slog.LevelDebug) {
		log.Debug("Message body", "content", string(body))
	}

	// Determine if this is a request or a notification
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		log.Error("Message rejected", "error", fmt.Errorf("invalid JSON: %w", err), "status", http.StatusBadRequest)
		return
	}

	// Check if this is a request or notification based on presence of "id" field
	if _, hasID := payload["id"]; hasID {
		// This is a request
		var request schema.JSONRPCRequest
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
			log.Error("Message rejected", "error", fmt.Errorf("invalid JSON-RPC request: %w", err), "status", http.StatusBadRequest)
			return
		}

		// Check context before potentially lengthy operation
		select {
		case <-ctx.Done():
			http.Error(w, "Request canceled", http.StatusRequestTimeout)
			log.Error("Request processing canceled", "error", ctx.Err(), "status", http.StatusRequestTimeout)
			return
		default:
		}

		// Process the request
		response, err := h.handler.HandleRequest(ctx, request)
		if err != nil {
			// Consider if this should return a JSON-RPC error response instead of HTTP error
			http.Error(w, "Failed to process request", http.StatusInternalServerError)
			log.Error("Message processing failed", "error", fmt.Errorf("request processing error: %w", err), "status", http.StatusInternalServerError)
			return
		}

		// Marshal response
		responseBytes, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			log.Error("Response marshaling failed", "error", fmt.Errorf("json marshal error: %w", err), "status", http.StatusInternalServerError)
			return
		}

		// Check context again before writing response
		select {
		case <-ctx.Done():
			log.Error("Request canceled before response sent", "error", ctx.Err())
			return
		default:
		}

		// Set content type and write response
		w.Header().Set(contentTypeHeader, contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(responseBytes); err != nil {
			log.Error("Failed to write response", "error", err)
			// At this point, headers are already sent, so we can't change the status code
			return
		}
	} else {
		// This is a notification
		var notification schema.JSONRPCNotification
		if err := json.Unmarshal(body, &notification); err != nil {
			http.Error(w, "Invalid JSON-RPC notification", http.StatusBadRequest)
			log.Error("Message rejected", "error", fmt.Errorf("invalid JSON-RPC notification: %w", err), "status", http.StatusBadRequest)
			return
		}

		// Check context before potentially lengthy operation
		select {
		case <-ctx.Done():
			http.Error(w, "Request canceled", http.StatusRequestTimeout)
			log.Error("Notification processing canceled", "error", ctx.Err(), "status", http.StatusRequestTimeout)
			return
		default:
		}

		// Process the notification
		if err := h.handler.HandleNotification(ctx, notification); err != nil {
			http.Error(w, "Failed to process notification", http.StatusInternalServerError)
			log.Error("Notification processing failed", "error", fmt.Errorf("notification processing error: %w", err), "status", http.StatusInternalServerError)
			return
		}

		// Return a proper response for notifications
		// The client expects a valid JSON response with Content-Type set
		w.Header().Set(contentTypeHeader, contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("{}")); err != nil { // Empty JSON object
			log.Error("Failed to write notification response", "error", err)
			return
		}
	}
}

// ServeHTTP implements http.Handler
func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := slogctx.FromCtx(ctx)

	// Add custom response headers
	if h.options.ResponseHeaders != nil {
		for key, value := range h.options.ResponseHeaders {
			w.Header().Set(key, value)
		}
	}

	switch r.Method {
	case http.MethodGet:
		h.HandleSSE(w, r)
	case http.MethodPost:
		h.HandleMessage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Error("Method not allowed", "method", r.Method, "status", http.StatusMethodNotAllowed)
	}
}
