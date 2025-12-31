// Package schema provides a Go implementation of the Model Context Protocol (MCP).
package schema

// MCPRequest is the interface implemented by all request types.
// It provides a marker for request types and a method to get the request method.
type MCPRequest interface {
	// IsMCPRequest indicates this type is a request
	IsMCPRequest() bool

	// GetRequestId returns the request ID
	GetRequestId() RequestID

	// SetRequestId sets the request ID
	SetRequestId(id RequestID)
}

// BaseRequest provides common functionality for all request types.
// It handles ID management that satisfies part of the MCPRequest interface.
type BaseRequest struct {
	// ID is the request ID, used for JSON-RPC
	ID StringNumber `json:"id,omitempty"`
}

// GetRequestId returns the request ID
func (b *BaseRequest) GetRequestId() RequestID {
	return b.ID
}

// SetRequestId sets the request ID
func (b *BaseRequest) SetRequestId(id RequestID) {
	b.ID = id
}

// InitializeRequestParams represents the parameters for an initialize request.
type InitializeRequestParams struct {
	WithExtra
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

// MarshalJSON implements json.Marshaler
func (p *InitializeRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"protocolVersion": p.ProtocolVersion,
		"capabilities":    p.Capabilities,
		"clientInfo":      p.ClientInfo,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *InitializeRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"protocolVersion", "capabilities", "clientInfo"}
	return unmarshalWithExtra(data, p, knownFields)
}

// InitializeRequest is sent from the client to the server when it first connects.
type InitializeRequest struct {
	WithExtra
	BaseRequest
	Params InitializeRequestParams `json:"params"`
}

// IsMCPRequest implements the MCPRequest interface.
func (i *InitializeRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (i *InitializeRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": i.Params,
	}

	return marshalWithExtra(i, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (i *InitializeRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, i, knownFields)
}

// GetConstants implements WithConstants interface
func (i *InitializeRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "initialize",
	}
}

// MCPMeta represents the _meta field as defined in the MCP schema.
// This matches the MCP schema.json which shows _meta as an object with an optional progressToken field.
type MCPMeta struct {
	WithExtra
	ProgressToken StringNumber `json:"progressToken,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (m *MCPMeta) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if !m.ProgressToken.IsNull() {
		fieldMap["progressToken"] = m.ProgressToken
	}

	return marshalWithExtra(m, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (m *MCPMeta) UnmarshalJSON(data []byte) error {
	knownFields := []string{"progressToken"}
	return unmarshalWithExtra(data, m, knownFields)
}

// PingRequestParams represents the parameters for a ping request.
type PingRequestParams struct {
	WithExtra
	Meta *MCPMeta `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *PingRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.Meta != nil && (!p.Meta.ProgressToken.IsNull() || len(p.Meta.Extra) > 0) {
		fieldMap["_meta"] = p.Meta
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *PingRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"_meta"}
	return unmarshalWithExtra(data, p, knownFields)
}

// PingRequest is issued by either the server or the client to check that the other party is still alive.
type PingRequest struct {
	WithExtra
	BaseRequest
	Params PingRequestParams `json:"params,omitempty"`
}

// IsMCPRequest implements the MCPRequest interface.
func (p *PingRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (p *PingRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if (p.Params.Meta != nil && !p.Params.Meta.ProgressToken.IsNull()) || len(p.Params.Extra) > 0 {
		fieldMap["params"] = p.Params
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *PingRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, p, knownFields)
}

// GetConstants implements WithConstants interface
func (p *PingRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "ping",
	}
}

// ListResourcesRequestParams represents the parameters for a list resources request.
type ListResourcesRequestParams struct {
	WithExtra
	Cursor string `json:"cursor,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *ListResourcesRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.Cursor != "" {
		fieldMap["cursor"] = p.Cursor
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *ListResourcesRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"cursor"}
	return unmarshalWithExtra(data, p, knownFields)
}

// ListResourcesRequest is sent from the client to the server to list all available resources.
type ListResourcesRequest struct {
	WithExtra
	BaseRequest
	Params ListResourcesRequestParams `json:"params,omitempty"`
}

// IsMCPRequest implements the MCPRequest interface.
func (l *ListResourcesRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (l *ListResourcesRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if len(l.Params.Extra) > 0 || l.Params.Cursor != "" {
		fieldMap["params"] = l.Params
	}

	return marshalWithExtra(l, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (l *ListResourcesRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, l, knownFields)
}

// GetConstants implements WithConstants interface
func (r *ListResourcesRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "resources/list",
	}
}

// ListResourceTemplatesRequestParams is the params for a ListResourceTemplatesRequest.
type ListResourceTemplatesRequestParams struct {
	WithExtra
	Cursor string `json:"cursor,omitempty"` // Pagination cursor
}

// UnmarshalJSON implements json.Unmarshaler
func (p *ListResourceTemplatesRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"cursor"}
	return unmarshalWithExtra(data, p, knownFields)
}

// ListResourceTemplatesRequest is sent from the client to the server to request a list of resource templates.
type ListResourceTemplatesRequest struct {
	WithExtra
	BaseRequest
	Params ListResourceTemplatesRequestParams `json:"params,omitempty"`
}

// IsMCPRequest implements the MCPRequest interface.
func (r *ListResourceTemplatesRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (r *ListResourceTemplatesRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if r.Params.Cursor != "" {
		fieldMap["params"] = r.Params
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ListResourceTemplatesRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, r, knownFields)
}

// GetConstants implements WithConstants interface
func (r *ListResourceTemplatesRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "resources/templates/list",
	}
}

// ReadResourceRequestParams represents the parameters for a read resource request.
type ReadResourceRequestParams struct {
	WithExtra
	URI string `json:"uri"`
}

// MarshalJSON implements json.Marshaler
func (p *ReadResourceRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri": p.URI,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *ReadResourceRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri"}
	return unmarshalWithExtra(data, p, knownFields)
}

// ReadResourceRequest is sent from the client to the server to request the contents of a resource.
type ReadResourceRequest struct {
	WithExtra
	BaseRequest
	Params ReadResourceRequestParams `json:"params"`
}

// IsMCPRequest implements the MCPRequest interface.
func (r *ReadResourceRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (r *ReadResourceRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": r.Params,
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ReadResourceRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, r, knownFields)
}

// GetConstants implements WithConstants interface
func (r *ReadResourceRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "resources/read",
	}
}

// SubscribeRequestParams represents the parameters for a subscribe request.
type SubscribeRequestParams struct {
	WithExtra
	URI string `json:"uri"`
}

// MarshalJSON implements json.Marshaler
func (p *SubscribeRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri": p.URI,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *SubscribeRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri"}
	return unmarshalWithExtra(data, p, knownFields)
}

// SubscribeRequest is sent from the client to request resources/updated notifications.
type SubscribeRequest struct {
	WithExtra
	BaseRequest
	Params SubscribeRequestParams `json:"params"`
}

// IsMCPRequest implements the MCPRequest interface.
func (s *SubscribeRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (s *SubscribeRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": s.Params,
	}

	return marshalWithExtra(s, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (s *SubscribeRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, s, knownFields)
}

// GetConstants implements WithConstants interface
func (s *SubscribeRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "resources/subscribe",
	}
}

// UnsubscribeRequestParams represents the parameters for an unsubscribe request.
type UnsubscribeRequestParams struct {
	WithExtra
	URI string `json:"uri"`
}

// MarshalJSON implements json.Marshaler
func (p *UnsubscribeRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri": p.URI,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *UnsubscribeRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri"}
	return unmarshalWithExtra(data, p, knownFields)
}

// UnsubscribeRequest is sent from the client to cancel a previous resources/subscribe.
type UnsubscribeRequest struct {
	WithExtra
	BaseRequest
	Params UnsubscribeRequestParams `json:"params"`
}

// IsMCPRequest implements the MCPRequest interface.
func (u *UnsubscribeRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (u *UnsubscribeRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": u.Params,
	}

	return marshalWithExtra(u, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (u *UnsubscribeRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, u, knownFields)
}

// GetConstants implements WithConstants interface
func (u *UnsubscribeRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "resources/unsubscribe",
	}
}

// ListPromptsRequestParams represents the parameters for a list prompts request.
type ListPromptsRequestParams struct {
	WithExtra
	Cursor string `json:"cursor,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *ListPromptsRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.Cursor != "" {
		fieldMap["cursor"] = p.Cursor
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *ListPromptsRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"cursor"}
	return unmarshalWithExtra(data, p, knownFields)
}

// ListPromptsRequest is sent from the client to the server to list all prompts.
type ListPromptsRequest struct {
	WithExtra
	BaseRequest
	Params ListPromptsRequestParams `json:"params,omitempty"`
}

// IsMCPRequest implements the MCPRequest interface.
func (l *ListPromptsRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (l *ListPromptsRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if len(l.Params.Extra) > 0 || l.Params.Cursor != "" {
		fieldMap["params"] = l.Params
	}

	return marshalWithExtra(l, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (l *ListPromptsRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, l, knownFields)
}

// GetConstants implements WithConstants interface
func (l *ListPromptsRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "prompts/list",
	}
}

// GetPromptRequestParams represents the parameters for a get prompt request.
type GetPromptRequestParams struct {
	WithExtra
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *GetPromptRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name": p.Name,
	}

	if len(p.Arguments) > 0 {
		fieldMap["arguments"] = p.Arguments
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *GetPromptRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "arguments"}
	return unmarshalWithExtra(data, p, knownFields)
}

// GetPromptRequest is sent from the client to the server to retrieve a specific prompt.
type GetPromptRequest struct {
	WithExtra
	BaseRequest
	Params GetPromptRequestParams `json:"params"`
}

// IsMCPRequest implements the MCPRequest interface.
func (g *GetPromptRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (g *GetPromptRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": g.Params,
	}

	return marshalWithExtra(g, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (g *GetPromptRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, g, knownFields)
}

// GetConstants implements WithConstants interface
func (g *GetPromptRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "prompts/get",
	}
}

// ListToolsRequestParams represents the parameters for a list tools request.
type ListToolsRequestParams struct {
	WithExtra
	Cursor string `json:"cursor,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *ListToolsRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.Cursor != "" {
		fieldMap["cursor"] = p.Cursor
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *ListToolsRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"cursor"}
	return unmarshalWithExtra(data, p, knownFields)
}

// ListToolsRequest is sent from the client to the server to list all tools.
type ListToolsRequest struct {
	WithExtra
	BaseRequest
	Params ListToolsRequestParams `json:"params,omitempty"`
}

// IsMCPRequest implements the MCPRequest interface.
func (l *ListToolsRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (l *ListToolsRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if len(l.Params.Extra) > 0 || l.Params.Cursor != "" {
		fieldMap["params"] = l.Params
	}

	return marshalWithExtra(l, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (l *ListToolsRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, l, knownFields)
}

// GetConstants implements WithConstants interface
func (l *ListToolsRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "tools/list",
	}
}

// CallToolRequestParams represents the parameters for a call tool request.
type CallToolRequestParams struct {
	WithExtra
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *CallToolRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name": p.Name,
	}

	if len(p.Arguments) > 0 {
		fieldMap["arguments"] = p.Arguments
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *CallToolRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "arguments"}
	return unmarshalWithExtra(data, p, knownFields)
}

// CallToolRequest is sent from the client to the server to call a tool.
type CallToolRequest struct {
	WithExtra
	BaseRequest
	Params CallToolRequestParams `json:"params"`
}

// IsMCPRequest implements the MCPRequest interface.
func (c *CallToolRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (c *CallToolRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": c.Params,
	}

	return marshalWithExtra(c, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (c *CallToolRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, c, knownFields)
}

// GetConstants implements WithConstants interface
func (c *CallToolRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "tools/call",
	}
}

// SetLevelRequestParams represents the parameters for a set level request.
type SetLevelRequestParams struct {
	WithExtra
	Level string `json:"level"`
}

// MarshalJSON implements json.Marshaler
func (p *SetLevelRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"level": p.Level,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *SetLevelRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"level"}
	return unmarshalWithExtra(data, p, knownFields)
}

// SetLevelRequest is sent from the client to the server to set the logging level.
type SetLevelRequest struct {
	WithExtra
	BaseRequest
	Params SetLevelRequestParams `json:"params"`
}

// IsMCPRequest implements the MCPRequest interface.
func (s *SetLevelRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (s *SetLevelRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": s.Params,
	}

	return marshalWithExtra(s, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (s *SetLevelRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, s, knownFields)
}

// GetConstants implements WithConstants interface
func (s *SetLevelRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "logging/setLevel",
	}
}

// CompleteRequestArgumentParams represents the nested argument field in a complete request.
type CompleteRequestArgumentParams struct {
	WithExtra
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MarshalJSON implements json.Marshaler
func (p *CompleteRequestArgumentParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name":  p.Name,
		"value": p.Value,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *CompleteRequestArgumentParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "value"}
	return unmarshalWithExtra(data, p, knownFields)
}

// CompleteRequestParams represents the parameters for a complete request.
type CompleteRequestParams struct {
	WithExtra
	Ref      ReferenceUnion                `json:"ref"`
	Argument CompleteRequestArgumentParams `json:"argument"`
}

// MarshalJSON implements json.Marshaler
func (p *CompleteRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"ref":      p.Ref,
		"argument": p.Argument,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *CompleteRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"ref", "argument"}
	return unmarshalWithExtra(data, p, knownFields)
}

// CompleteRequest is sent from the client to the server to request completions for an argument.
type CompleteRequest struct {
	WithExtra
	BaseRequest
	Params CompleteRequestParams `json:"params"`
}

// IsMCPRequest implements the MCPRequest interface.
func (c *CompleteRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (c *CompleteRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": c.Params,
	}

	return marshalWithExtra(c, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (c *CompleteRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, c, knownFields)
}

// GetConstants implements WithConstants interface
func (c *CompleteRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "completion/complete",
	}
}

// PromptReference identifies a prompt.
type PromptReference struct {
	WithExtra
	Name string `json:"name"`
}

// MarshalJSON implements json.Marshaler
func (p *PromptReference) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name": p.Name,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *PromptReference) UnmarshalJSON(data []byte) error {
	knownFields := []string{"type", "name"}
	return unmarshalWithExtra(data, p, knownFields)
}

// GetConstants implements WithConstants interface
func (p *PromptReference) GetConstants() map[string]string {
	return map[string]string{
		"type": "ref/prompt",
	}
}

// ResourceReference is a reference to a resource or resource template definition.
type ResourceReference struct {
	WithExtra
	URI string `json:"uri"`
}

// MarshalJSON implements json.Marshaler
func (r *ResourceReference) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri": r.URI,
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ResourceReference) UnmarshalJSON(data []byte) error {
	knownFields := []string{"type", "uri"}
	return unmarshalWithExtra(data, r, knownFields)
}

// GetConstants implements WithConstants interface
func (r *ResourceReference) GetConstants() map[string]string {
	return map[string]string{
		"type": "ref/resource",
	}
}

// SamplingMessage describes a message issued to or received from an LLM API.
type SamplingMessage struct {
	WithExtra
	Role    Role         `json:"role"`
	Content ContentUnion `json:"content"`
}

// MarshalJSON implements json.Marshaler
func (s *SamplingMessage) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"role":    s.Role,
		"content": s.Content,
	}

	return marshalWithExtra(s, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (s *SamplingMessage) UnmarshalJSON(data []byte) error {
	knownFields := []string{"role", "content"}
	return unmarshalWithExtra(data, s, knownFields)
}

// CreateMessageRequestParams represents the parameters for a create message request.
type CreateMessageRequestParams struct {
	WithExtra
	Messages         []SamplingMessage      `json:"messages"`
	ModelPreferences *ModelPreferences      `json:"modelPreferences,omitempty"`
	SystemPrompt     string                 `json:"systemPrompt,omitempty"`
	IncludeContext   string                 `json:"includeContext,omitempty"` // "none", "thisServer", "allServers"
	Temperature      *float64               `json:"temperature,omitempty"`
	MaxTokens        int                    `json:"maxTokens"`
	StopSequences    []string               `json:"stopSequences,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *CreateMessageRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"messages":  p.Messages,
		"maxTokens": p.MaxTokens,
	}

	if p.ModelPreferences != nil {
		fieldMap["modelPreferences"] = p.ModelPreferences
	}
	if p.SystemPrompt != "" {
		fieldMap["systemPrompt"] = p.SystemPrompt
	}
	if p.IncludeContext != "" {
		fieldMap["includeContext"] = p.IncludeContext
	}
	if p.Temperature != nil {
		fieldMap["temperature"] = p.Temperature
	}
	if len(p.StopSequences) > 0 {
		fieldMap["stopSequences"] = p.StopSequences
	}
	if len(p.Metadata) > 0 {
		fieldMap["metadata"] = p.Metadata
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *CreateMessageRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"messages", "modelPreferences", "systemPrompt", "includeContext", "temperature", "maxTokens", "stopSequences", "metadata"}
	return unmarshalWithExtra(data, p, knownFields)
}

// CreateMessageRequest is a request from the server to sample an LLM via the client.
type CreateMessageRequest struct {
	WithExtra
	BaseRequest
	Params CreateMessageRequestParams `json:"params"`
}

// IsMCPRequest implements the MCPRequest interface.
func (c *CreateMessageRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (c *CreateMessageRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": c.Params,
	}

	return marshalWithExtra(c, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (c *CreateMessageRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, c, knownFields)
}

// GetConstants implements WithConstants interface
func (c *CreateMessageRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "sampling/createMessage",
	}
}

// ListRootsRequestParams represents the parameters for a list roots request.
type ListRootsRequestParams struct {
	WithExtra
	Meta *MCPMeta `json:"_meta,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (p *ListRootsRequestParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.Meta != nil && (!p.Meta.ProgressToken.IsNull() || len(p.Meta.Extra) > 0) {
		fieldMap["_meta"] = p.Meta
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *ListRootsRequestParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"_meta"}
	return unmarshalWithExtra(data, p, knownFields)
}

// ListRootsRequest is sent from the server to request a list of root URIs from the client.
type ListRootsRequest struct {
	WithExtra
	BaseRequest
	Params ListRootsRequestParams `json:"params,omitempty"`
}

// IsMCPRequest implements the MCPRequest interface.
func (r *ListRootsRequest) IsMCPRequest() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (r *ListRootsRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if len(r.Params.Extra) > 0 || (r.Params.Meta != nil && !r.Params.Meta.ProgressToken.IsNull()) {
		fieldMap["params"] = r.Params
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *ListRootsRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params", "id"}
	return unmarshalWithExtra(data, r, knownFields)
}

// GetConstants implements WithConstants interface
func (r *ListRootsRequest) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "roots/list",
	}
}
