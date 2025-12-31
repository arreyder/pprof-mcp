// Package schema provides a Go implementation of the Model Context Protocol (MCP).
package schema

import (
	"encoding/json"
	"fmt"
)

// JSONRPCMessage represents any valid JSON-RPC object that can be decoded or encoded.
type JSONRPCMessage interface {
	IsJSONRPCMessage()
}

// JSONRPCRequest represents a request that expects a response.
type JSONRPCRequest struct {
	WithExtra
	JSONRPC string          `json:"jsonrpc"`
	ID      RequestID       `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsJSONRPCMessage implements the JSONRPCMessage interface.
func (*JSONRPCRequest) IsJSONRPCMessage() {}

// MarshalJSON implements the json.Marshaler interface for JSONRPCRequest.
func (r *JSONRPCRequest) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"jsonrpc": r.JSONRPC,
		"id":      r.ID,
		"method":  r.Method,
	}

	if len(r.Params) > 0 {
		fieldMap["params"] = r.Params
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for JSONRPCRequest.
func (r *JSONRPCRequest) UnmarshalJSON(data []byte) error {
	knownFields := []string{"jsonrpc", "id", "method", "params"}
	return unmarshalWithExtra(data, r, knownFields)
}

// JSONRPCNotification represents a notification which does not expect a response.
type JSONRPCNotification struct {
	WithExtra
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsJSONRPCMessage implements the JSONRPCMessage interface.
func (*JSONRPCNotification) IsJSONRPCMessage() {}

// MarshalJSON implements the json.Marshaler interface for JSONRPCNotification.
func (n *JSONRPCNotification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"jsonrpc": n.JSONRPC,
		"method":  n.Method,
	}

	if len(n.Params) > 0 {
		fieldMap["params"] = n.Params
	}

	return marshalWithExtra(n, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for JSONRPCNotification.
func (n *JSONRPCNotification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"jsonrpc", "method", "params"}
	return unmarshalWithExtra(data, n, knownFields)
}

// JSONRPCResponse represents a successful (non-error) response to a request.
type JSONRPCResponse struct {
	WithExtra
	JSONRPC string          `json:"jsonrpc"`
	ID      RequestID       `json:"id"`
	Result  json.RawMessage `json:"result"`
}

// IsJSONRPCMessage implements the JSONRPCMessage interface.
func (*JSONRPCResponse) IsJSONRPCMessage() {}

// MarshalJSON implements the json.Marshaler interface for JSONRPCResponse.
func (r *JSONRPCResponse) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"jsonrpc": r.JSONRPC,
		"id":      r.ID,
		"result":  r.Result,
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for JSONRPCResponse.
func (r *JSONRPCResponse) UnmarshalJSON(data []byte) error {
	knownFields := []string{"jsonrpc", "id", "result"}
	return unmarshalWithExtra(data, r, knownFields)
}

// JSONRPCError represents a response to a request that indicates an error occurred.
type JSONRPCError struct {
	WithExtra
	JSONRPC string          `json:"jsonrpc"`
	ID      RequestID       `json:"id"`
	Error   JSONRPCErrorObj `json:"error"`
}

// IsJSONRPCMessage implements the JSONRPCMessage interface.
func (*JSONRPCError) IsJSONRPCMessage() {}

// MarshalJSON implements the json.Marshaler interface for JSONRPCError.
func (e *JSONRPCError) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"jsonrpc": e.JSONRPC,
		"id":      e.ID,
		"error":   e.Error,
	}

	return marshalWithExtra(e, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for JSONRPCError.
func (e *JSONRPCError) UnmarshalJSON(data []byte) error {
	knownFields := []string{"jsonrpc", "id", "error"}
	return unmarshalWithExtra(data, e, knownFields)
}

// JSONRPCResponseUnion represents a union type that can be either a JSONRPCResponse or a JSONRPCError
type JSONRPCResponseUnion struct {
	WithExtra
	Success *JSONRPCResponse `json:"-"`
	Error   *JSONRPCError    `json:"-"`
}

// IsJSONRPCMessage implements the JSONRPCMessage interface.
func (*JSONRPCResponseUnion) IsJSONRPCMessage() {}

// MarshalJSON implements json.Marshaler
func (ru *JSONRPCResponseUnion) MarshalJSON() ([]byte, error) {
	if ru.Success != nil {
		return json.Marshal(ru.Success)
	}
	if ru.Error != nil {
		return json.Marshal(ru.Error)
	}

	return nil, fmt.Errorf("JSONRPCResponseUnion: no response type set")
}

// UnmarshalJSON implements json.Unmarshaler
func (ru *JSONRPCResponseUnion) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// Check for the presence of either "result" or "error" field
	if _, hasResult := m["result"]; hasResult {
		ru.Success = &JSONRPCResponse{}
		return json.Unmarshal(data, ru.Success)
	}

	if _, hasError := m["error"]; hasError {
		ru.Error = &JSONRPCError{}
		return json.Unmarshal(data, ru.Error)
	}

	return fmt.Errorf("JSONRPCResponseUnion: missing both 'result' and 'error' fields")
}

// NewJSONRPCResponseFromSuccess creates a new JSONRPCResponseUnion with JSONRPCResponse
func NewJSONRPCResponseFromSuccess(jsonrpc string, id RequestID, result json.RawMessage) JSONRPCResponseUnion {
	resp := JSONRPCResponse{
		JSONRPC: jsonrpc,
		ID:      id,
		Result:  result,
	}
	return JSONRPCResponseUnion{Success: &resp}
}

// NewJSONRPCResponseFromError creates a new JSONRPCResponseUnion with JSONRPCError
func NewJSONRPCResponseFromError(jsonrpc string, id RequestID, errorObj JSONRPCErrorObj) JSONRPCResponseUnion {
	errResp := JSONRPCError{
		JSONRPC: jsonrpc,
		ID:      id,
		Error:   errorObj,
	}
	return JSONRPCResponseUnion{Error: &errResp}
}

// IsSuccess returns true if this union contains JSONRPCResponse
func (ru *JSONRPCResponseUnion) IsSuccess() bool {
	return ru.Success != nil
}

// GetSuccess returns the JSONRPCResponse value if set, or nil
func (ru *JSONRPCResponseUnion) GetSuccess() *JSONRPCResponse {
	return ru.Success
}

// IsError returns true if this union contains JSONRPCError
func (ru *JSONRPCResponseUnion) IsError() bool {
	return ru.Error != nil
}

// GetError returns the JSONRPCError value if set, or nil
func (ru *JSONRPCResponseUnion) GetError() *JSONRPCError {
	return ru.Error
}

// JSONRPCErrorObj represents the error object in a JSON-RPC error response.
type JSONRPCErrorObj struct {
	WithExtra
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface for JSONRPCErrorObj.
func (e *JSONRPCErrorObj) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"code":    e.Code,
		"message": e.Message,
	}

	if len(e.Data) > 0 {
		fieldMap["data"] = e.Data
	}

	return marshalWithExtra(e, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for JSONRPCErrorObj.
func (e *JSONRPCErrorObj) UnmarshalJSON(data []byte) error {
	knownFields := []string{"code", "message", "data"}
	return unmarshalWithExtra(data, e, knownFields)
}

// JSONRPCBatchRequest represents a JSON-RPC batch request.
type JSONRPCBatchRequest []interface{} // Can contain JSONRPCRequest or JSONRPCNotification

// IsJSONRPCMessage implements the JSONRPCMessage interface.
func (JSONRPCBatchRequest) IsJSONRPCMessage() {}

// JSONRPCBatchResponse represents a JSON-RPC batch response.
type JSONRPCBatchResponse []interface{} // Can contain JSONRPCResponse or JSONRPCError

// IsJSONRPCMessage implements the JSONRPCMessage interface.
func (JSONRPCBatchResponse) IsJSONRPCMessage() {}

// Result represents a generic result structure.
type Result struct {
	WithExtra
	// All other fields are handled by the Extra map
}

// MarshalJSON implements the json.Marshaler interface for Result.
func (r *Result) MarshalJSON() ([]byte, error) {
	// Result has no explicit fields, just Extra
	return marshalWithExtra(r, map[string]interface{}{})
}

// UnmarshalJSON implements the json.Unmarshaler interface for Result.
func (r *Result) UnmarshalJSON(data []byte) error {
	// No known fields for Result, it's all Extra
	return unmarshalWithExtra(data, r, []string{})
}

// EmptyResult represents a response that indicates success but carries no data.
type EmptyResult Result

// MarshalJSON implements the json.Marshaler interface for EmptyResult.
func (r *EmptyResult) MarshalJSON() ([]byte, error) {
	// EmptyResult has no explicit fields, just Extra
	return marshalWithExtra(r, map[string]interface{}{})
}

// UnmarshalJSON implements the json.Unmarshaler interface for EmptyResult.
func (r *EmptyResult) UnmarshalJSON(data []byte) error {
	// No known fields for EmptyResult, it's all Extra
	return unmarshalWithExtra(data, r, []string{})
}

// Request represents a generic request structure.
type Request struct {
	WithExtra
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface for Request.
func (r *Request) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"method": r.Method,
	}

	if len(r.Params) > 0 {
		fieldMap["params"] = r.Params
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for Request.
func (r *Request) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, r, knownFields)
}

// Notification represents a generic notification structure.
type Notification struct {
	WithExtra
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface for Notification.
func (n *Notification) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"method": n.Method,
	}

	if len(n.Params) > 0 {
		fieldMap["params"] = n.Params
	}

	return marshalWithExtra(n, fieldMap)
}

// UnmarshalJSON implements the json.Unmarshaler interface for Notification.
func (n *Notification) UnmarshalJSON(data []byte) error {
	knownFields := []string{"method", "params"}
	return unmarshalWithExtra(data, n, knownFields)
}
