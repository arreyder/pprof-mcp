package server

import (
	"context"
	"net/http"
	"os"

	"github.com/conductorone/mcp-go-sdk/mcp/schema"
	"github.com/conductorone/mcp-go-sdk/mcp/server/transport"
)

// MCPServer provides a user-friendly interface for MCP servers
// and implements transport.Handler
type MCPServer struct {
	handler MCPHandler
	options *ServerOptions
}

// NewMCPServer creates a new MCP server with the given name and options
func NewMCPServer(name string, opts ...ServerOption) *MCPServer {
	// Create options with defaults
	options := applyOptions(opts...)

	// Override name if provided
	options.Name = name

	// Create a default tools provider if none provided but tools are added directly
	if options.ToolsProvider == nil && len(options.Tools) > 0 {
		toolsProvider := NewDefaultToolsProvider()
		for name, handler := range options.Tools {
			toolsProvider.AddTool(name, handler)
		}
		options.ToolsProvider = toolsProvider
	}

	// Create handler
	handler := NewDefaultHandler(options)

	return &MCPServer{
		handler: handler,
		options: options,
	}
}

// Handler returns the underlying MCPHandler
func (s *MCPServer) Handler() MCPHandler {
	return s.handler
}

// HandleRequest implements transport.Handler
func (s *MCPServer) HandleRequest(ctx context.Context, req schema.JSONRPCRequest) (schema.JSONRPCResponse, error) {
	return s.handler.HandleRequest(ctx, req)
}

// HandleNotification implements transport.Handler
func (s *MCPServer) HandleNotification(ctx context.Context, note schema.JSONRPCNotification) error {
	return s.handler.HandleNotification(ctx, note)
}

// SendNotification implements transport.Handler
func (s *MCPServer) SendNotification(ctx context.Context, note schema.JSONRPCNotification) error {
	return s.handler.SendNotification(ctx, note)
}

// AddTool registers a tool with the server
func (s *MCPServer) AddTool(name string, handler ToolHandlerFunc) {
	// If we don't have a provider yet, create one
	if s.options.ToolsProvider == nil {
		s.options.ToolsProvider = NewDefaultToolsProvider()
	}

	// If it's our default provider, add the tool
	if dp, ok := s.options.ToolsProvider.(*DefaultToolsProvider); ok {
		dp.AddTool(name, handler)
	} else {
		// Can't add tools to a custom provider
		// In a real implementation, we might log a warning here
	}
}

// ServeStdio starts serving the MCP server over stdio
func (s *MCPServer) ServeStdio(ctx context.Context) error {
	return transport.ServeStdio(ctx, s, os.Stdin, os.Stdout)
}

// ServeHTTP is a convenience method to start an HTTP server
func (s *MCPServer) ServeHTTP(addr string, opts ...transport.StreamOption) error {
	// Create default stream options
	streamOpts := transport.DefaultStreamOptions()

	// Apply all provided options
	for _, opt := range opts {
		opt(&streamOpts)
	}

	// Create a simple HTTP handler
	handler := transport.NewInMemoryStreamHandler(s.handler, streamOpts)

	// Start the HTTP server
	return http.ListenAndServe(addr, handler)
}

// RegisterNotificationHandler implements transport.MCPHandler
func (s *MCPServer) RegisterNotificationHandler(ctx context.Context, sessionID string, handler transport.NotificationHandler) error {
	return s.handler.RegisterNotificationHandler(ctx, sessionID, handler)
}

// UnregisterNotificationHandler implements transport.MCPHandler
func (s *MCPServer) UnregisterNotificationHandler(ctx context.Context, sessionID string) error {
	return s.handler.UnregisterNotificationHandler(ctx, sessionID)
}
