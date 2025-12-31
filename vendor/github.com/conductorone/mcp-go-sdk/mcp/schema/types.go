// Package schema provides a Go implementation of the Model Context Protocol (MCP).
//
// MCP defines a standard way for AI models and applications to exchange context
// through resources, tools, and prompts.
package schema

import (
	"encoding/json"
	"time"
)

// ProtocolVersion represents the MCP protocol version.
// The initial version supported is "2024-11-05".
const (
	ProtocolVersion20241105 = "2024-11-05"
	ProtocolVersionLatest   = ProtocolVersion20241105
	ProtocolVersion20250326 = "2025-03-26"
)

// JSONRPC constants
const (
	JSONRPCVersion = "2.0"
)

// Error codes as defined in the JSON-RPC 2.0 specification
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Role types for messages
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// RequestID represents a uniquely identifying ID for a request in JSON-RPC.
// In the MCP schema, this is defined as string | number.
type RequestID = StringNumber

// ClientInfo contains information about the client.
type ClientInfo struct {
	WithExtra
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MarshalJSON implements json.Marshaler
func (c *ClientInfo) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name":    c.Name,
		"version": c.Version,
	}

	return marshalWithExtra(c, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (c *ClientInfo) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "version"}
	return unmarshalWithExtra(data, c, knownFields)
}

// ServerInfo contains information about the server.
type ServerInfo struct {
	WithExtra
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MarshalJSON implements json.Marshaler
func (s *ServerInfo) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name":    s.Name,
		"version": s.Version,
	}

	return marshalWithExtra(s, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (s *ServerInfo) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "version"}
	return unmarshalWithExtra(data, s, knownFields)
}

// Resource represents a resource that can be exposed by the server.
type Resource struct {
	WithExtra
	URI         string            `json:"uri"`
	Title       string            `json:"title,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	MediaType   string            `json:"mediaType,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *Resource) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri": r.URI,
	}

	if r.Title != "" {
		fieldMap["title"] = r.Title
	}
	if r.Description != "" {
		fieldMap["description"] = r.Description
	}
	if r.MediaType != "" {
		fieldMap["mediaType"] = r.MediaType
	}
	if len(r.Metadata) > 0 {
		fieldMap["metadata"] = r.Metadata
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *Resource) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri", "title", "description", "mediaType", "metadata"}
	return unmarshalWithExtra(data, r, knownFields)
}

// ResourceData contains the content of a resource along with metadata.
type ResourceData struct {
	WithExtra
	URI       string         `json:"uri"`
	MediaType string         `json:"mediaType,omitempty"`
	Content   []ContentItem  `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *ResourceData) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri":     r.URI,
		"content": r.Content,
	}

	if r.MediaType != "" {
		fieldMap["mediaType"] = r.MediaType
	}
	if len(r.Metadata) > 0 {
		fieldMap["metadata"] = r.Metadata
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ResourceData) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri", "mediaType", "content", "metadata"}
	return unmarshalWithExtra(data, r, knownFields)
}

// ContentItem represents a single content item in a resource or tool result.
type ContentItem struct {
	WithExtra
	Type  string `json:"type"`            // e.g., "text", "image"
	Text  string `json:"text,omitempty"`  // For text content
	Image *Image `json:"image,omitempty"` // For image content
}

// MarshalJSON implements json.Marshaler
func (c *ContentItem) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"type": c.Type,
	}

	if c.Text != "" {
		fieldMap["text"] = c.Text
	}
	if c.Image != nil {
		fieldMap["image"] = c.Image
	}

	return marshalWithExtra(c, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (c *ContentItem) UnmarshalJSON(data []byte) error {
	knownFields := []string{"type", "text", "image"}
	return unmarshalWithExtra(data, c, knownFields)
}

// Image represents an image in a content item.
type Image struct {
	WithExtra
	URL string `json:"url,omitempty"`
	// Additional image properties can be added as needed
}

// MarshalJSON implements json.Marshaler
func (i *Image) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if i.URL != "" {
		fieldMap["url"] = i.URL
	}

	return marshalWithExtra(i, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (i *Image) UnmarshalJSON(data []byte) error {
	knownFields := []string{"url"}
	return unmarshalWithExtra(data, i, knownFields)
}

// ToolInfo contains information about a tool.
type ToolInfo struct {
	WithExtra
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"` // JSON Schema for input parameters
}

// MarshalJSON implements json.Marshaler
func (t *ToolInfo) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name": t.Name,
	}

	if t.Description != "" {
		fieldMap["description"] = t.Description
	}
	if len(t.InputSchema) > 0 {
		fieldMap["inputSchema"] = t.InputSchema
	}

	return marshalWithExtra(t, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (t *ToolInfo) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "description", "inputSchema"}
	return unmarshalWithExtra(data, t, knownFields)
}

// ToolOutput represents the result of a tool call.
type ToolOutput struct {
	WithExtra
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (t *ToolOutput) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"content": t.Content,
	}

	if t.IsError {
		fieldMap["isError"] = t.IsError
	}

	return marshalWithExtra(t, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (t *ToolOutput) UnmarshalJSON(data []byte) error {
	knownFields := []string{"content", "isError"}
	return unmarshalWithExtra(data, t, knownFields)
}

// ToolCallParams contains parameters for calling a tool.
type ToolCallParams struct {
	WithExtra
	Name   string         `json:"name"`
	Params map[string]any `json:"params"`
}

// MarshalJSON implements json.Marshaler
func (t *ToolCallParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name": t.Name,
	}

	if len(t.Params) > 0 {
		fieldMap["params"] = t.Params
	}

	return marshalWithExtra(t, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (t *ToolCallParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "params"}
	return unmarshalWithExtra(data, t, knownFields)
}

// PromptInfo contains information about a prompt.
type PromptInfo struct {
	WithExtra
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Args        []PromptArg `json:"args,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *PromptInfo) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name": p.Name,
	}

	if p.Description != "" {
		fieldMap["description"] = p.Description
	}
	if len(p.Args) > 0 {
		fieldMap["args"] = p.Args
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *PromptInfo) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "description", "args"}
	return unmarshalWithExtra(data, p, knownFields)
}

// PromptArg defines an argument for a prompt.
type PromptArg struct {
	WithExtra
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *PromptArg) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name": p.Name,
	}

	if p.Description != "" {
		fieldMap["description"] = p.Description
	}
	if p.Required {
		fieldMap["required"] = p.Required
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *PromptArg) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "description", "required"}
	return unmarshalWithExtra(data, p, knownFields)
}

// PromptContent represents the content of a prompt.
type PromptContent struct {
	WithExtra
	Messages []Message `json:"messages,omitempty"`
	// Could add other prompt formats as needed
}

// MarshalJSON implements json.Marshaler
func (p *PromptContent) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if len(p.Messages) > 0 {
		fieldMap["messages"] = p.Messages
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *PromptContent) UnmarshalJSON(data []byte) error {
	knownFields := []string{"messages"}
	return unmarshalWithExtra(data, p, knownFields)
}

// Message represents a message in a prompt (e.g., for chat-based models).
type Message struct {
	WithExtra
	Role    string `json:"role"` // e.g., "system", "user", "assistant"
	Content string `json:"content"`
}

// MarshalJSON implements json.Marshaler
func (m *Message) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"role":    m.Role,
		"content": m.Content,
	}

	return marshalWithExtra(m, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (m *Message) UnmarshalJSON(data []byte) error {
	knownFields := []string{"role", "content"}
	return unmarshalWithExtra(data, m, knownFields)
}

// Progress represents a progress notification.
type Progress struct {
	WithExtra
	ID        string  `json:"id"`
	Completed float64 `json:"completed"`
	Message   string  `json:"message,omitempty"`
	Timestamp string  `json:"timestamp,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *Progress) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"id":        p.ID,
		"completed": p.Completed,
	}

	if p.Message != "" {
		fieldMap["message"] = p.Message
	}
	if p.Timestamp != "" {
		fieldMap["timestamp"] = p.Timestamp
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *Progress) UnmarshalJSON(data []byte) error {
	knownFields := []string{"id", "completed", "message", "timestamp"}
	return unmarshalWithExtra(data, p, knownFields)
}

// NewProgress creates a new Progress notification with the current timestamp.
func NewProgress(id string, completed float64, message string) Progress {
	return Progress{
		ID:        id,
		Completed: completed,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// Capabilities represents the capabilities that a server supports.
type Capabilities struct {
	WithExtra
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (c *Capabilities) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if c.Resources != nil {
		fieldMap["resources"] = c.Resources
	}
	if c.Tools != nil {
		fieldMap["tools"] = c.Tools
	}
	if c.Prompts != nil {
		fieldMap["prompts"] = c.Prompts
	}

	return marshalWithExtra(c, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (c *Capabilities) UnmarshalJSON(data []byte) error {
	knownFields := []string{"resources", "tools", "prompts"}
	return unmarshalWithExtra(data, c, knownFields)
}

// ResourcesCapability indicates server support for resources.
type ResourcesCapability struct {
	WithExtra
	ListChanged bool `json:"listChanged,omitempty"`
	Subscribe   bool `json:"subscribe,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *ResourcesCapability) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if r.ListChanged {
		fieldMap["listChanged"] = r.ListChanged
	}
	if r.Subscribe {
		fieldMap["subscribe"] = r.Subscribe
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ResourcesCapability) UnmarshalJSON(data []byte) error {
	knownFields := []string{"listChanged", "subscribe"}
	return unmarshalWithExtra(data, r, knownFields)
}

// ToolsCapability indicates server support for tools.
type ToolsCapability struct {
	WithExtra
	ListChanged bool   `json:"listChanged,omitempty"`
	Supported   bool   `json:"supported,omitempty"`
	Version     string `json:"version,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (t *ToolsCapability) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if t.ListChanged {
		fieldMap["listChanged"] = t.ListChanged
	}

	if t.Supported {
		fieldMap["supported"] = t.Supported
	}

	if t.Version != "" {
		fieldMap["version"] = t.Version
	}

	return marshalWithExtra(t, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (t *ToolsCapability) UnmarshalJSON(data []byte) error {
	knownFields := []string{"listChanged", "supported", "version"}
	return unmarshalWithExtra(data, t, knownFields)
}

// PromptsCapability indicates server support for prompts.
type PromptsCapability struct {
	WithExtra
	ListChanged bool `json:"listChanged,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *PromptsCapability) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.ListChanged {
		fieldMap["listChanged"] = p.ListChanged
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *PromptsCapability) UnmarshalJSON(data []byte) error {
	knownFields := []string{"listChanged"}
	return unmarshalWithExtra(data, p, knownFields)
}
