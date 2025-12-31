package transport

import (
	"context"

	"github.com/conductorone/mcp-go-sdk/mcp/schema"
)

// NotificationHandler is a function that processes notifications for a client
type NotificationHandler func(ctx context.Context, notification schema.JSONRPCNotification) error

// MCPHandler defines the core functionality to handle MCP messages
// This interface is implemented by the server package and used by
// the transport implementations to process messages.
type MCPHandler interface {
	// HandleRequest processes a JSON-RPC request and returns a response
	HandleRequest(ctx context.Context, req schema.JSONRPCRequest) (schema.JSONRPCResponse, error)

	// HandleNotification processes a JSON-RPC notification (no response)
	HandleNotification(ctx context.Context, note schema.JSONRPCNotification) error

	// SendNotification allows sending a notification to the client
	SendNotification(ctx context.Context, note schema.JSONRPCNotification) error

	// RegisterNotificationHandler registers a function to handle notifications for a specific session
	RegisterNotificationHandler(ctx context.Context, sessionID string, handler NotificationHandler) error

	// UnregisterNotificationHandler removes a notification handler for a session
	UnregisterNotificationHandler(ctx context.Context, sessionID string) error
}
