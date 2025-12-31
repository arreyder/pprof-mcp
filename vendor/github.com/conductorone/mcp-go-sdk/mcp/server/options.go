package server

import (
	"github.com/conductorone/mcp-go-sdk/mcp/schema"
)

// ServerOption configures a server
type ServerOption func(*ServerOptions)

// ServerOptions holds all server configuration
type ServerOptions struct {
	// Name is the server name
	Name string

	// Version is the server version
	Version string

	// Instructions for the server
	Instructions string

	// Tools map
	Tools map[string]ToolHandlerFunc

	// ToolsProvider for handling tool operations
	ToolsProvider ToolsProvider

	// SupportedProtocolVersions lists supported protocol versions in order of preference
	SupportedProtocolVersions []string
}

// WithName sets the server name
func WithName(name string) ServerOption {
	return func(o *ServerOptions) {
		o.Name = name
	}
}

// WithVersion sets the server version
func WithVersion(version string) ServerOption {
	return func(o *ServerOptions) {
		o.Version = version
	}
}

// WithInstructions sets the server instructions
func WithInstructions(instructions string) ServerOption {
	return func(o *ServerOptions) {
		o.Instructions = instructions
	}
}

// WithTool adds a tool to the server
func WithTool(name string, handler ToolHandlerFunc) ServerOption {
	return func(o *ServerOptions) {
		if o.Tools == nil {
			o.Tools = make(map[string]ToolHandlerFunc)
		}
		o.Tools[name] = handler
	}
}

// WithToolsProvider sets a custom tools provider
func WithToolsProvider(provider ToolsProvider) ServerOption {
	return func(o *ServerOptions) {
		o.ToolsProvider = provider
	}
}

// WithSupportedProtocolVersions sets the supported protocol versions
func WithSupportedProtocolVersions(versions []string) ServerOption {
	return func(o *ServerOptions) {
		o.SupportedProtocolVersions = versions
	}
}

// applyOptions applies the given options to a new ServerOptions instance
func applyOptions(opts ...ServerOption) *ServerOptions {
	// Default options
	options := &ServerOptions{
		Name:                      "mcp-go-server",
		Version:                   "0.1.0",
		SupportedProtocolVersions: schema.DefaultSupportedProtocolVersions,
	}

	// Apply all options
	for _, opt := range opts {
		opt(options)
	}

	return options
}
