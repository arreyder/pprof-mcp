// Package schema provides a Go implementation of the Model Context Protocol (MCP).
package schema

import (
	"encoding/json"
)

// InitializeResult is the response to an Initialize request.
type InitializeResult struct {
	WithExtra
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
	Meta            *MCPMeta           `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (i *InitializeResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"protocolVersion": i.ProtocolVersion,
		"serverInfo":      i.ServerInfo,
		"capabilities":    i.Capabilities,
	}

	if i.Instructions != "" {
		fieldMap["instructions"] = i.Instructions
	}

	if i.Meta != nil && (!i.Meta.ProgressToken.IsNull() || len(i.Meta.Extra) > 0) {
		fieldMap["_meta"] = i.Meta
	}

	return marshalWithExtra(i, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (i *InitializeResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"protocolVersion", "capabilities", "serverInfo", "instructions", "_meta"}
	return unmarshalWithExtra(data, i, knownFields)
}

// ListResourcesResult is the server's response to a resources/list request.
type ListResourcesResult struct {
	WithExtra
	NextCursor string     `json:"nextCursor,omitempty"`
	Resources  []Resource `json:"resources"`
	Meta       *MCPMeta   `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *ListResourcesResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"resources": r.Resources,
	}

	if r.NextCursor != "" {
		fieldMap["nextCursor"] = r.NextCursor
	}

	if r.Meta != nil && (!r.Meta.ProgressToken.IsNull() || len(r.Meta.Extra) > 0) {
		fieldMap["_meta"] = r.Meta
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ListResourcesResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"nextCursor", "resources", "_meta"}
	return unmarshalWithExtra(data, r, knownFields)
}

// ListResourceTemplatesResult is the server's response to a resources/templates/list request.
type ListResourceTemplatesResult struct {
	WithExtra
	NextCursor        string             `json:"nextCursor,omitempty"`
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	Meta              *MCPMeta           `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *ListResourceTemplatesResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"resourceTemplates": r.ResourceTemplates,
	}

	if r.NextCursor != "" {
		fieldMap["nextCursor"] = r.NextCursor
	}

	if r.Meta != nil && (!r.Meta.ProgressToken.IsNull() || len(r.Meta.Extra) > 0) {
		fieldMap["_meta"] = r.Meta
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ListResourceTemplatesResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"nextCursor", "resourceTemplates", "_meta"}
	return unmarshalWithExtra(data, r, knownFields)
}

// ReadResourceResult is the server's response to a resources/read request.
type ReadResourceResult struct {
	WithExtra
	Contents []ResourceContentsUnion `json:"contents"` // TextResourceContents or BlobResourceContents
	Meta     *MCPMeta                `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *ReadResourceResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"contents": r.Contents,
	}

	if r.Meta != nil && (!r.Meta.ProgressToken.IsNull() || len(r.Meta.Extra) > 0) {
		fieldMap["_meta"] = r.Meta
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ReadResourceResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"contents", "_meta"}
	return unmarshalWithExtra(data, r, knownFields)
}

// ListPromptsResult is the server's response to a prompts/list request.
type ListPromptsResult struct {
	WithExtra
	NextCursor string   `json:"nextCursor,omitempty"`
	Prompts    []Prompt `json:"prompts"`
	Meta       *MCPMeta `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *ListPromptsResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"prompts": r.Prompts,
	}

	if r.NextCursor != "" {
		fieldMap["nextCursor"] = r.NextCursor
	}

	if r.Meta != nil && (!r.Meta.ProgressToken.IsNull() || len(r.Meta.Extra) > 0) {
		fieldMap["_meta"] = r.Meta
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ListPromptsResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"nextCursor", "prompts", "_meta"}
	return unmarshalWithExtra(data, r, knownFields)
}

// Prompt represents a prompt or prompt template that the server offers.
type Prompt struct {
	WithExtra
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *Prompt) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name": p.Name,
	}

	if p.Description != "" {
		fieldMap["description"] = p.Description
	}
	if len(p.Arguments) > 0 {
		fieldMap["arguments"] = p.Arguments
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *Prompt) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "description", "arguments"}
	return unmarshalWithExtra(data, p, knownFields)
}

// PromptArgument describes an argument that a prompt can accept.
type PromptArgument struct {
	WithExtra
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *PromptArgument) MarshalJSON() ([]byte, error) {
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
func (p *PromptArgument) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "description", "required"}
	return unmarshalWithExtra(data, p, knownFields)
}

// GetPromptResult is the server's response to a prompts/get request.
type GetPromptResult struct {
	WithExtra
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
	Meta        *MCPMeta        `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *GetPromptResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"messages": r.Messages,
	}

	if r.Description != "" {
		fieldMap["description"] = r.Description
	}

	if r.Meta != nil && (!r.Meta.ProgressToken.IsNull() || len(r.Meta.Extra) > 0) {
		fieldMap["_meta"] = r.Meta
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *GetPromptResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"description", "messages", "_meta"}
	return unmarshalWithExtra(data, r, knownFields)
}

// PromptMessage describes a message returned as part of a prompt.
type PromptMessage struct {
	WithExtra
	Role    Role               `json:"role"`
	Content PromptContentUnion `json:"content"`
}

// MarshalJSON implements json.Marshaler
func (m *PromptMessage) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"role":    m.Role,
		"content": m.Content,
	}

	return marshalWithExtra(m, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (m *PromptMessage) UnmarshalJSON(data []byte) error {
	knownFields := []string{"role", "content"}
	return unmarshalWithExtra(data, m, knownFields)
}

// ListToolsResult is the server's response to a tools/list request.
type ListToolsResult struct {
	WithExtra
	NextCursor string   `json:"nextCursor,omitempty"`
	Tools      []Tool   `json:"tools"`
	Meta       *MCPMeta `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *ListToolsResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"tools": r.Tools,
	}

	if r.NextCursor != "" {
		fieldMap["nextCursor"] = r.NextCursor
	}

	if r.Meta != nil && (!r.Meta.ProgressToken.IsNull() || len(r.Meta.Extra) > 0) {
		fieldMap["_meta"] = r.Meta
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ListToolsResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"nextCursor", "tools", "_meta"}
	return unmarshalWithExtra(data, r, knownFields)
}

// Tool represents a definition for a tool the client can call.
type Tool struct {
	WithExtra
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	InputSchema ToolInputSchema  `json:"inputSchema"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (t *Tool) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name":        t.Name,
		"inputSchema": t.InputSchema,
	}

	if t.Description != "" {
		fieldMap["description"] = t.Description
	}
	if t.Annotations != nil {
		fieldMap["annotations"] = t.Annotations
	}

	return marshalWithExtra(t, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (t *Tool) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "description", "inputSchema", "annotations"}
	return unmarshalWithExtra(data, t, knownFields)
}

// ToolInputSchema represents a JSON Schema object defining the expected parameters for a tool.
type ToolInputSchema struct {
	WithExtra
	Type       string                     `json:"type"` // Always "object"
	Properties map[string]json.RawMessage `json:"properties,omitempty"`
	Required   []string                   `json:"required,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (s *ToolInputSchema) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"type": s.Type,
	}

	if len(s.Properties) > 0 {
		fieldMap["properties"] = s.Properties
	}
	if len(s.Required) > 0 {
		fieldMap["required"] = s.Required
	}

	return marshalWithExtra(s, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (s *ToolInputSchema) UnmarshalJSON(data []byte) error {
	knownFields := []string{"type", "properties", "required"}
	return unmarshalWithExtra(data, s, knownFields)
}

// ToolAnnotations contains additional properties describing a Tool to clients.
type ToolAnnotations struct {
	WithExtra
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    bool   `json:"readOnlyHint,omitempty"`
	DestructiveHint bool   `json:"destructiveHint,omitempty"`
	IdempotentHint  bool   `json:"idempotentHint,omitempty"`
	OpenWorldHint   bool   `json:"openWorldHint,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (a *ToolAnnotations) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if a.Title != "" {
		fieldMap["title"] = a.Title
	}
	if a.ReadOnlyHint {
		fieldMap["readOnlyHint"] = a.ReadOnlyHint
	}
	if a.DestructiveHint {
		fieldMap["destructiveHint"] = a.DestructiveHint
	}
	if a.IdempotentHint {
		fieldMap["idempotentHint"] = a.IdempotentHint
	}
	if a.OpenWorldHint {
		fieldMap["openWorldHint"] = a.OpenWorldHint
	}

	return marshalWithExtra(a, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (a *ToolAnnotations) UnmarshalJSON(data []byte) error {
	knownFields := []string{"title", "readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"}
	return unmarshalWithExtra(data, a, knownFields)
}

// CallToolResult is the server's response to a tool call.
type CallToolResult struct {
	WithExtra
	Content []PromptContentUnion `json:"content"`
	IsError bool                 `json:"isError,omitempty"`
	Meta    *MCPMeta             `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *CallToolResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"content": r.Content,
	}

	if r.IsError {
		fieldMap["isError"] = r.IsError
	}

	if r.Meta != nil && (!r.Meta.ProgressToken.IsNull() || len(r.Meta.Extra) > 0) {
		fieldMap["_meta"] = r.Meta
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *CallToolResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"content", "isError", "_meta"}
	return unmarshalWithExtra(data, r, knownFields)
}

// Completion represents completion data in a completion result.
type Completion struct {
	WithExtra
	Values  []string `json:"values"`
	Total   int      `json:"total,omitempty"`
	HasMore bool     `json:"hasMore,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (c *Completion) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"values": c.Values,
	}

	if c.Total != 0 {
		fieldMap["total"] = c.Total
	}
	if c.HasMore {
		fieldMap["hasMore"] = c.HasMore
	}

	return marshalWithExtra(c, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (c *Completion) UnmarshalJSON(data []byte) error {
	knownFields := []string{"values", "total", "hasMore"}
	return unmarshalWithExtra(data, c, knownFields)
}

// CompleteResult is the server's response to a completion/complete request.
type CompleteResult struct {
	WithExtra
	Completion Completion `json:"completion"`
	Meta       *MCPMeta   `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *CompleteResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"completion": r.Completion,
	}

	if r.Meta != nil && (!r.Meta.ProgressToken.IsNull() || len(r.Meta.Extra) > 0) {
		fieldMap["_meta"] = r.Meta
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *CompleteResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"completion", "_meta"}
	return unmarshalWithExtra(data, r, knownFields)
}

// CreateMessageResult is the client's response to a sampling/createMessage request.
type CreateMessageResult struct {
	WithExtra
	Role       Role         `json:"role"`
	Content    ContentUnion `json:"content"`
	Model      string       `json:"model"`
	StopReason string       `json:"stopReason,omitempty"`
	Meta       *MCPMeta     `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *CreateMessageResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"role":    r.Role,
		"content": r.Content,
		"model":   r.Model,
	}

	if r.StopReason != "" {
		fieldMap["stopReason"] = r.StopReason
	}

	if r.Meta != nil && (!r.Meta.ProgressToken.IsNull() || len(r.Meta.Extra) > 0) {
		fieldMap["_meta"] = r.Meta
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *CreateMessageResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"role", "content", "model", "stopReason", "_meta"}
	return unmarshalWithExtra(data, r, knownFields)
}

// ListRootsResult is the client's response to a roots/list request from the server.
type ListRootsResult struct {
	WithExtra
	Roots []Root   `json:"roots"`
	Meta  *MCPMeta `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *ListRootsResult) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"roots": r.Roots,
	}

	if r.Meta != nil && (!r.Meta.ProgressToken.IsNull() || len(r.Meta.Extra) > 0) {
		fieldMap["_meta"] = r.Meta
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ListRootsResult) UnmarshalJSON(data []byte) error {
	knownFields := []string{"roots", "_meta"}
	return unmarshalWithExtra(data, r, knownFields)
}
