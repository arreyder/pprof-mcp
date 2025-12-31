package server

import (
	"errors"
)

// Error definitions used by the MCP server
var (
	// ErrUnsupportedProtocolVersion indicates an unsupported protocol version
	ErrUnsupportedProtocolVersion = errors.New("unsupported protocol version")

	// ErrMethodNotFound indicates an unknown method
	ErrMethodNotFound = errors.New("method not found")

	// ErrInvalidParams indicates invalid parameters were provided
	ErrInvalidParams = errors.New("invalid parameters")

	// ErrInternalError indicates a server-side error occurred
	ErrInternalError = errors.New("internal server error")

	// ErrNotInitialized indicates the server hasn't been initialized
	ErrNotInitialized = errors.New("server not initialized")

	// ErrSessionRequired indicates a session is required but not provided
	ErrSessionRequired = errors.New("session required")

	// ErrNotSupported indicates a capability is not supported
	ErrNotSupported = errors.New("capability not supported")
)
