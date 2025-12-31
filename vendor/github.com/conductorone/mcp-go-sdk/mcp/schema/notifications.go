// Package schema provides a Go implementation of the Model Context Protocol (MCP).
package schema

// MCPNotification is the interface implemented by all notification types.
// It provides a marker for notification types and a method to get the notification method.
type MCPNotification interface {
	// IsMCPNotification indicates this type is a notification
	IsMCPNotification() bool
}

// InitializedNotificationParams represents the parameters for an initialized notification.
type InitializedNotificationParams struct {
	WithExtra
	Meta *MCPMeta `json:"_meta,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface for InitializedNotificationParams.
func (p *InitializedNotificationParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.Meta != nil && (!p.Meta.ProgressToken.IsNull() || len(p.Meta.Extra) > 0) {
		fieldMap["_meta"] = p.Meta
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for InitializedNotificationParams.
func (p *InitializedNotificationParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"_meta"}
	return unmarshalWithExtra(data, p, knownFields)
}

// InitializedNotification is sent from the client to the server after initialization has finished.
type InitializedNotification struct {
	WithExtra
	Params InitializedNotificationParams `json:"params,omitempty"`
}

// IsMCPNotification implements the MCPNotification interface.
func (n *InitializedNotification) IsMCPNotification() bool {
	return true
}

// GetConstants implements WithConstants interface for InitializedNotification.
func (n *InitializedNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
}

// MarshalJSON implements the json.Marshaler interface for InitializedNotification.
func (n *InitializedNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": n.Params,
	}

	return marshalWithExtra(n, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for InitializedNotification.
func (n *InitializedNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, n, knownFields)
}

// CancelledNotificationParams represents the parameters for a cancelled notification.
type CancelledNotificationParams struct {
	WithExtra
	RequestID RequestID `json:"requestId"`
	Reason    string    `json:"reason,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface for CancelledNotificationParams.
func (p *CancelledNotificationParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"requestId": p.RequestID,
	}

	if p.Reason != "" {
		fieldMap["reason"] = p.Reason
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for CancelledNotificationParams.
func (p *CancelledNotificationParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"requestId", "reason"}
	return unmarshalWithExtra(data, p, knownFields)
}

// CancelledNotification is sent by either side to indicate cancellation of a previously-issued request.
type CancelledNotification struct {
	WithExtra
	Params CancelledNotificationParams `json:"params"`
}

// IsMCPNotification implements the MCPNotification interface.
func (n *CancelledNotification) IsMCPNotification() bool {
	return true
}

// GetConstants implements WithConstants interface for CancelledNotification.
func (n *CancelledNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "notifications/cancelled",
	}
}

// MarshalJSON implements the json.Marshaler interface for CancelledNotification.
func (n *CancelledNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": n.Params,
	}

	return marshalWithExtra(n, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for CancelledNotification.
func (n *CancelledNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, n, knownFields)
}

// ProgressNotificationParams represents the parameters for a progress notification.
type ProgressNotificationParams struct {
	WithExtra
	ProgressToken StringNumber `json:"progressToken"`
	Progress      float64      `json:"progress"`
	Total         float64      `json:"total,omitempty"`
	Message       string       `json:"message,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface for ProgressParams.
func (p *ProgressNotificationParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"progressToken": p.ProgressToken,
		"progress":      p.Progress,
	}

	if p.Total != 0 {
		fieldMap["total"] = p.Total
	}
	if p.Message != "" {
		fieldMap["message"] = p.Message
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for ProgressParams.
func (p *ProgressNotificationParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"progressToken", "progress", "total", "message"}
	return unmarshalWithExtra(data, p, knownFields)
}

// ProgressNotification is sent from the client to the server to notify about progress.
type ProgressNotification struct {
	WithExtra
	Params ProgressNotificationParams `json:"params"`
}

// IsMCPNotification implements the MCPNotification interface.
func (p *ProgressNotification) IsMCPNotification() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (p *ProgressNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": p.Params,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *ProgressNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, p, knownFields)
}

// GetConstants implements WithConstants interface
func (p *ProgressNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "logging/progress",
	}
}

// ResourceListChangedNotificationParams represents the parameters for a resource list changed notification.
type ResourceListChangedNotificationParams struct {
	WithExtra
	Meta *MCPMeta `json:"_meta,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface for ResourceListChangedNotificationParams.
func (p *ResourceListChangedNotificationParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.Meta != nil && (!p.Meta.ProgressToken.IsNull() || len(p.Meta.Extra) > 0) {
		fieldMap["_meta"] = p.Meta
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for ResourceListChangedNotificationParams.
func (p *ResourceListChangedNotificationParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"_meta"}
	return unmarshalWithExtra(data, p, knownFields)
}

// ResourceListChangedNotification informs the client that the list of resources has changed.
type ResourceListChangedNotification struct {
	WithExtra
	Params ResourceListChangedNotificationParams `json:"params,omitempty"`
}

// IsMCPNotification implements the MCPNotification interface.
func (n *ResourceListChangedNotification) IsMCPNotification() bool {
	return true
}

// GetConstants implements WithConstants interface for ResourceListChangedNotification.
func (n *ResourceListChangedNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "notifications/resources/list_changed",
	}
}

// MarshalJSON implements the json.Marshaler interface for ResourceListChangedNotification.
func (n *ResourceListChangedNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if len(n.Params.Extra) > 0 {
		fieldMap["params"] = n.Params
	}

	return marshalWithExtra(n, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for ResourceListChangedNotification.
func (n *ResourceListChangedNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, n, knownFields)
}

// ResourceUpdatedNotificationParams represents the parameters for a resource updated notification.
type ResourceUpdatedNotificationParams struct {
	WithExtra
	URI string `json:"uri"`
}

// MarshalJSON implements the json.Marshaler interface for ResourceUpdatedNotificationParams.
func (p *ResourceUpdatedNotificationParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri": p.URI,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for ResourceUpdatedNotificationParams.
func (p *ResourceUpdatedNotificationParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri"}
	return unmarshalWithExtra(data, p, knownFields)
}

// ResourceUpdatedNotification informs the client that a resource has changed.
type ResourceUpdatedNotification struct {
	WithExtra
	Params ResourceUpdatedNotificationParams `json:"params"`
}

// IsMCPNotification implements the MCPNotification interface.
func (n *ResourceUpdatedNotification) IsMCPNotification() bool {
	return true
}

// GetConstants implements WithConstants interface for ResourceUpdatedNotification.
func (n *ResourceUpdatedNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "notifications/resources/updated",
	}
}

// MarshalJSON implements the json.Marshaler interface for ResourceUpdatedNotification.
func (n *ResourceUpdatedNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": n.Params,
	}

	return marshalWithExtra(n, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for ResourceUpdatedNotification.
func (n *ResourceUpdatedNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, n, knownFields)
}

// PromptListChangedNotificationParams represents the parameters for a prompt list changed notification.
type PromptListChangedNotificationParams struct {
	WithExtra
	Meta *MCPMeta `json:"_meta,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface for PromptListChangedNotificationParams.
func (p *PromptListChangedNotificationParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.Meta != nil && (!p.Meta.ProgressToken.IsNull() || len(p.Meta.Extra) > 0) {
		fieldMap["_meta"] = p.Meta
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for PromptListChangedNotificationParams.
func (p *PromptListChangedNotificationParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"_meta"}
	return unmarshalWithExtra(data, p, knownFields)
}

// PromptListChangedNotification informs the client that the list of prompts has changed.
type PromptListChangedNotification struct {
	WithExtra
	Params PromptListChangedNotificationParams `json:"params,omitempty"`
}

// IsMCPNotification implements the MCPNotification interface.
func (n *PromptListChangedNotification) IsMCPNotification() bool {
	return true
}

// GetConstants implements WithConstants interface for PromptListChangedNotification.
func (n *PromptListChangedNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "notifications/prompts/list_changed",
	}
}

// MarshalJSON implements the json.Marshaler interface for PromptListChangedNotification.
func (n *PromptListChangedNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if len(n.Params.Extra) > 0 {
		fieldMap["params"] = n.Params
	}

	return marshalWithExtra(n, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for PromptListChangedNotification.
func (n *PromptListChangedNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, n, knownFields)
}

// ToolListChangedNotificationParams represents the parameters for a tool list changed notification.
type ToolListChangedNotificationParams struct {
	WithExtra
	Meta *MCPMeta `json:"_meta,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface for ToolListChangedNotificationParams.
func (p *ToolListChangedNotificationParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.Meta != nil && (!p.Meta.ProgressToken.IsNull() || len(p.Meta.Extra) > 0) {
		fieldMap["_meta"] = p.Meta
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for ToolListChangedNotificationParams.
func (p *ToolListChangedNotificationParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"_meta"}
	return unmarshalWithExtra(data, p, knownFields)
}

// ToolListChangedNotification informs the client that the list of tools has changed.
type ToolListChangedNotification struct {
	WithExtra
	Params ToolListChangedNotificationParams `json:"params,omitempty"`
}

// IsMCPNotification implements the MCPNotification interface.
func (n *ToolListChangedNotification) IsMCPNotification() bool {
	return true
}

// GetConstants implements WithConstants interface for ToolListChangedNotification.
func (n *ToolListChangedNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "notifications/tools/list_changed",
	}
}

// MarshalJSON implements the json.Marshaler interface for ToolListChangedNotification.
func (n *ToolListChangedNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if len(n.Params.Extra) > 0 {
		fieldMap["params"] = n.Params
	}

	return marshalWithExtra(n, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for ToolListChangedNotification.
func (n *ToolListChangedNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, n, knownFields)
}

// LoggingLevel is the severity of a log message.
type LoggingLevel string

const (
	LoggingLevelDebug     LoggingLevel = "debug"
	LoggingLevelInfo      LoggingLevel = "info"
	LoggingLevelNotice    LoggingLevel = "notice"
	LoggingLevelWarning   LoggingLevel = "warning"
	LoggingLevelError     LoggingLevel = "error"
	LoggingLevelCritical  LoggingLevel = "critical"
	LoggingLevelAlert     LoggingLevel = "alert"
	LoggingLevelEmergency LoggingLevel = "emergency"
)

// LoggingMessageNotificationParams represents the parameters for a logging message notification.
type LoggingMessageNotificationParams struct {
	WithExtra
	Level  LoggingLevel `json:"level"`
	Logger string       `json:"logger,omitempty"`
	Data   interface{}  `json:"data"`
}

// MarshalJSON implements the json.Marshaler interface for LoggingMessageNotificationParams.
func (p *LoggingMessageNotificationParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"level": p.Level,
		"data":  p.Data,
	}

	if p.Logger != "" {
		fieldMap["logger"] = p.Logger
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for LoggingMessageNotificationParams.
func (p *LoggingMessageNotificationParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"level", "logger", "data"}
	return unmarshalWithExtra(data, p, knownFields)
}

// LoggingMessageNotification is a notification of a log message passed from server to client.
type LoggingMessageNotification struct {
	WithExtra
	Params LoggingMessageNotificationParams `json:"params"`
}

// IsMCPNotification implements the MCPNotification interface.
func (n *LoggingMessageNotification) IsMCPNotification() bool {
	return true
}

// GetConstants implements WithConstants interface for LoggingMessageNotification.
func (n *LoggingMessageNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "notifications/message",
	}
}

// MarshalJSON implements the json.Marshaler interface for LoggingMessageNotification.
func (n *LoggingMessageNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": n.Params,
	}

	return marshalWithExtra(n, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for LoggingMessageNotification.
func (n *LoggingMessageNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, n, knownFields)
}

// RootsListChangedNotificationParams represents the parameters for a roots list changed notification.
type RootsListChangedNotificationParams struct {
	WithExtra
	Meta *MCPMeta `json:"_meta,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface for RootsListChangedNotificationParams.
func (p *RootsListChangedNotificationParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if p.Meta != nil && (!p.Meta.ProgressToken.IsNull() || len(p.Meta.Extra) > 0) {
		fieldMap["_meta"] = p.Meta
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for RootsListChangedNotificationParams.
func (p *RootsListChangedNotificationParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"_meta"}
	return unmarshalWithExtra(data, p, knownFields)
}

// RootsListChangedNotification informs the server that the list of roots has changed.
type RootsListChangedNotification struct {
	WithExtra
	Params RootsListChangedNotificationParams `json:"params,omitempty"`
}

// IsMCPNotification implements the MCPNotification interface.
func (n *RootsListChangedNotification) IsMCPNotification() bool {
	return true
}

// GetConstants implements WithConstants interface for RootsListChangedNotification.
func (n *RootsListChangedNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "notifications/roots/list_changed",
	}
}

// MarshalJSON implements the json.Marshaler interface for RootsListChangedNotification.
func (n *RootsListChangedNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if len(n.Params.Extra) > 0 {
		fieldMap["params"] = n.Params
	}

	return marshalWithExtra(n, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for RootsListChangedNotification.
func (n *RootsListChangedNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, n, knownFields)
}

// SubscribeParams represents the parameters for a subscribe notification.
type SubscribeParams struct {
	WithExtra
	URI string `json:"uri"`
}

// MarshalJSON implements json.Marshaler
func (p *SubscribeParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri": p.URI,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *SubscribeParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri"}
	return unmarshalWithExtra(data, p, knownFields)
}

// SubscribeNotification is sent from the client to the server to subscribe to a resource.
type SubscribeNotification struct {
	WithExtra
	Params SubscribeParams `json:"params"`
}

// IsMCPNotification implements the MCPNotification interface.
func (s *SubscribeNotification) IsMCPNotification() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (s *SubscribeNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": s.Params,
	}

	return marshalWithExtra(s, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (s *SubscribeNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, s, knownFields)
}

// GetConstants implements WithConstants interface
func (s *SubscribeNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "resources/subscribe",
	}
}

// UnsubscribeParams represents the parameters for an unsubscribe notification.
type UnsubscribeParams struct {
	WithExtra
	URI string `json:"uri"`
}

// MarshalJSON implements json.Marshaler
func (p *UnsubscribeParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri": p.URI,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *UnsubscribeParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri"}
	return unmarshalWithExtra(data, p, knownFields)
}

// UnsubscribeNotification is sent from the client to the server to unsubscribe from a resource.
type UnsubscribeNotification struct {
	WithExtra
	Params UnsubscribeParams `json:"params"`
}

// IsMCPNotification implements the MCPNotification interface.
func (u *UnsubscribeNotification) IsMCPNotification() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (u *UnsubscribeNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": u.Params,
	}

	return marshalWithExtra(u, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (u *UnsubscribeNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, u, knownFields)
}

// GetConstants implements WithConstants interface
func (u *UnsubscribeNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "resources/unsubscribe",
	}
}

// SetLevelParams represents the parameters for a SetLevel notification.
type SetLevelParams struct {
	WithExtra
	Level string `json:"level"`
}

// MarshalJSON implements json.Marshaler
func (p *SetLevelParams) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"level": p.Level,
	}

	return marshalWithExtra(p, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *SetLevelParams) UnmarshalJSON(data []byte) error {
	knownFields := []string{"level"}
	return unmarshalWithExtra(data, p, knownFields)
}

// SetLevelNotification is sent from the client to the server to set the logging level.
type SetLevelNotification struct {
	WithExtra
	Params SetLevelParams `json:"params"`
}

// IsMCPNotification implements the MCPNotification interface.
func (s *SetLevelNotification) IsMCPNotification() bool {
	return true
}

// MarshalJSON implements json.Marshaler
func (s *SetLevelNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"params": s.Params,
	}

	return marshalWithExtra(s, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (s *SetLevelNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, s, knownFields)
}

// GetConstants implements WithConstants interface
func (s *SetLevelNotification) GetConstants() map[string]string {
	return map[string]string{
		"jsonrpc": "2.0",
		"method":  "logging/setLevel",
	}
}
