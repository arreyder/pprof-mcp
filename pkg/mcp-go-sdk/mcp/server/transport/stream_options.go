package transport

import (
	"net/http"
)

// StreamOptions configures the HTTP stream handler
type StreamOptions struct {
	// SessionRequired determines if sessions are mandatory
	SessionRequired bool

	// SessionHeaderName is the HTTP header used for session IDs
	// Default: "Mcp-Session-Id"
	SessionHeaderName string

	// AuthenticationHandler validates requests before processing
	// If this returns an error, the request is rejected with HTTP 401
	AuthenticationHandler func(r *http.Request) error

	// AuthorizationHandler determines if a request is authorized for specific actions
	// If this returns an error, the request is rejected with HTTP 403
	AuthorizationHandler func(r *http.Request, action string) error

	// ResponseHeaders are custom headers to include in all responses
	ResponseHeaders map[string]string

	// SSEEventBufferSize determines buffer size for SSE events
	// Default: 10
	SSEEventBufferSize int

	// BaseURL explicitly sets the base URL for endpoints
	// If not set, the server will attempt to detect it from request information
	// Example: "https://api.example.com"
	BaseURL string

	// TrustForwardedHeaders determines if the server should trust X-Forwarded-* headers
	// for protocol/host detection. Only enable in environments with trusted reverse proxies.
	// Default: false
	TrustForwardedHeaders bool
}

// StreamOption is a function that configures StreamOptions
type StreamOption func(*StreamOptions)

// DefaultStreamOptions returns the default options for stream handling
func DefaultStreamOptions() StreamOptions {
	return StreamOptions{
		SessionRequired:       false,
		SessionHeaderName:     "Mcp-Session-Id",
		SSEEventBufferSize:    10,
		ResponseHeaders:       make(map[string]string),
		TrustForwardedHeaders: false,
	}
}

// WithSessionRequired sets whether sessions are required
func WithSessionRequired(required bool) StreamOption {
	return func(o *StreamOptions) {
		o.SessionRequired = required
	}
}

// WithSessionHeaderName sets the session header name
func WithSessionHeaderName(name string) StreamOption {
	return func(o *StreamOptions) {
		o.SessionHeaderName = name
	}
}

// WithAuthHandler sets the authentication handler
func WithAuthHandler(handler func(r *http.Request) error) StreamOption {
	return func(o *StreamOptions) {
		o.AuthenticationHandler = handler
	}
}

// WithAuthorizationHandler sets the authorization handler
func WithAuthorizationHandler(handler func(r *http.Request, action string) error) StreamOption {
	return func(o *StreamOptions) {
		o.AuthorizationHandler = handler
	}
}

// WithResponseHeader adds a custom response header
func WithResponseHeader(key, value string) StreamOption {
	return func(o *StreamOptions) {
		o.ResponseHeaders[key] = value
	}
}

// WithSSEEventBufferSize sets the buffer size for SSE events
func WithSSEEventBufferSize(size int) StreamOption {
	return func(o *StreamOptions) {
		if size > 0 {
			o.SSEEventBufferSize = size
		}
	}
}

// WithBaseURL sets an explicit base URL for all endpoints
func WithBaseURL(baseURL string) StreamOption {
	return func(o *StreamOptions) {
		o.BaseURL = baseURL
	}
}

// WithTrustForwardedHeaders configures whether to trust X-Forwarded-* headers
func WithTrustForwardedHeaders(trust bool) StreamOption {
	return func(o *StreamOptions) {
		o.TrustForwardedHeaders = trust
	}
}
