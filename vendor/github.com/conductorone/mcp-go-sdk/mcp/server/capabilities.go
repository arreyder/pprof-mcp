package server

import (
	"context"

	"github.com/conductorone/mcp-go-sdk/mcp/schema"
)

// ToolsProvider handles tool-related capabilities
type ToolsProvider interface {
	// List returns available tools
	List(ctx context.Context) ([]schema.Tool, error)

	// Call executes a tool with the provided arguments
	Call(ctx context.Context, name string, arguments map[string]any) (*schema.CallToolResult, error)
}

// ToolHandlerFunc is a function that handles tool execution
type ToolHandlerFunc func(ctx context.Context, arguments map[string]any) (any, error)
