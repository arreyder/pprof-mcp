// Package schema provides a Go implementation of the Model Context Protocol (MCP).
package schema

import (
	"encoding/json"
	"fmt"
)

// TextContent represents text provided to or from an LLM.
type TextContent struct {
	WithExtra
	Text string `json:"text"`
}

// MarshalJSON implements json.Marshaler
func (tc *TextContent) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"text": tc.Text,
	}

	return marshalWithExtra(tc, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (tc *TextContent) UnmarshalJSON(data []byte) error {
	knownFields := []string{"type", "text"}
	return unmarshalWithExtra(data, tc, knownFields)
}

// GetConstants implements WithConstants interface
func (tc *TextContent) GetConstants() map[string]string {
	return map[string]string{
		"type": "text",
	}
}

// NewTextContent creates a new TextContent with the given text.
func NewTextContent(text string) TextContent {
	return TextContent{
		Text: text,
	}
}

// ImageContent represents an image provided to or from an LLM.
type ImageContent struct {
	WithExtra
	Data     string `json:"data"`     // base64-encoded image data
	MimeType string `json:"mimeType"` // e.g., "image/png", "image/jpeg"
}

// MarshalJSON implements json.Marshaler
func (ic *ImageContent) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"data":     ic.Data,
		"mimeType": ic.MimeType,
	}

	return marshalWithExtra(ic, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (ic *ImageContent) UnmarshalJSON(data []byte) error {
	knownFields := []string{"type", "data", "mimeType"}
	return unmarshalWithExtra(data, ic, knownFields)
}

// GetConstants implements WithConstants interface
func (ic *ImageContent) GetConstants() map[string]string {
	return map[string]string{
		"type": "image",
	}
}

// NewImageContent creates a new ImageContent with the given data and MIME type.
func NewImageContent(data, mimeType string) ImageContent {
	return ImageContent{
		Data:     data,
		MimeType: mimeType,
	}
}

// AudioContent represents audio provided to or from an LLM.
type AudioContent struct {
	WithExtra
	Data     string `json:"data"`     // base64-encoded audio data
	MimeType string `json:"mimeType"` // e.g., "audio/mp3", "audio/wav"
}

// MarshalJSON implements json.Marshaler
func (ac *AudioContent) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"data":     ac.Data,
		"mimeType": ac.MimeType,
	}

	return marshalWithExtra(ac, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (ac *AudioContent) UnmarshalJSON(data []byte) error {
	knownFields := []string{"type", "data", "mimeType"}
	return unmarshalWithExtra(data, ac, knownFields)
}

// GetConstants implements WithConstants interface
func (ac *AudioContent) GetConstants() map[string]string {
	return map[string]string{
		"type": "audio",
	}
}

// NewAudioContent creates a new AudioContent with the given data and MIME type.
func NewAudioContent(data, mimeType string) AudioContent {
	return AudioContent{
		Data:     data,
		MimeType: mimeType,
	}
}

// EmbeddedResource represents the contents of a resource embedded into a prompt or tool call result.
type EmbeddedResource struct {
	WithExtra
	Resource ResourceContentsUnion `json:"resource"`
}

// MarshalJSON implements json.Marshaler
func (er *EmbeddedResource) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"resource": er.Resource,
	}

	return marshalWithExtra(er, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (er *EmbeddedResource) UnmarshalJSON(data []byte) error {
	knownFields := []string{"resource"}
	return unmarshalWithExtra(data, er, knownFields)
}

// GetConstants implements WithConstants interface
func (er *EmbeddedResource) GetConstants() map[string]string {
	return map[string]string{
		"type": "resource",
	}
}

// ResourceContentsUnion represents a union type that can be either a TextResourceContents or a BlobResourceContents
type ResourceContentsUnion struct {
	WithExtra
	TextResourceContents *TextResourceContents `json:"-"`
	BlobResourceContents *BlobResourceContents `json:"-"`
}

// MarshalJSON implements json.Marshaler
func (rc *ResourceContentsUnion) MarshalJSON() ([]byte, error) {
	if rc.TextResourceContents != nil {
		return json.Marshal(rc.TextResourceContents)
	}
	if rc.BlobResourceContents != nil {
		return json.Marshal(rc.BlobResourceContents)
	}
	return nil, fmt.Errorf("ResourceContentsUnion: no content set")
}

// UnmarshalJSON implements json.Unmarshaler
func (rc *ResourceContentsUnion) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	if _, hasText := m["text"]; hasText {
		rc.TextResourceContents = &TextResourceContents{}
		return json.Unmarshal(data, rc.TextResourceContents)
	}

	if _, hasBlob := m["blob"]; hasBlob {
		rc.BlobResourceContents = &BlobResourceContents{}
		return json.Unmarshal(data, rc.BlobResourceContents)
	}

	return fmt.Errorf("ResourceContentsUnion: invalid resource content")
}

// ResourceTemplate represents a template description for resources available on the server.
type ResourceTemplate struct {
	WithExtra
	Name        string       `json:"name"`
	URITemplate string       `json:"uriTemplate"`
	Description string       `json:"description,omitempty"`
	MimeType    string       `json:"mimeType,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (rt *ResourceTemplate) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name":        rt.Name,
		"uriTemplate": rt.URITemplate,
	}

	if rt.Description != "" {
		fieldMap["description"] = rt.Description
	}

	if rt.MimeType != "" {
		fieldMap["mimeType"] = rt.MimeType
	}

	if rt.Annotations != nil {
		fieldMap["annotations"] = rt.Annotations
	}

	return marshalWithExtra(rt, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (rt *ResourceTemplate) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "uriTemplate", "description", "mimeType", "annotations"}
	return unmarshalWithExtra(data, rt, knownFields)
}

// TextResourceContents represents a text resource.
type TextResourceContents struct {
	WithExtra
	URI      string `json:"uri"`
	Text     string `json:"text"`
	MimeType string `json:"mimeType,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (trc *TextResourceContents) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri":  trc.URI,
		"text": trc.Text,
	}

	if trc.MimeType != "" {
		fieldMap["mimeType"] = trc.MimeType
	}

	return marshalWithExtra(trc, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (trc *TextResourceContents) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri", "text", "mimeType"}
	return unmarshalWithExtra(data, trc, knownFields)
}

// BlobResourceContents represents a binary resource.
type BlobResourceContents struct {
	WithExtra
	URI      string `json:"uri"`
	Blob     string `json:"blob"` // base64-encoded binary data
	MimeType string `json:"mimeType,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (brc *BlobResourceContents) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri":  brc.URI,
		"blob": brc.Blob,
	}

	if brc.MimeType != "" {
		fieldMap["mimeType"] = brc.MimeType
	}

	return marshalWithExtra(brc, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (brc *BlobResourceContents) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri", "blob", "mimeType"}
	return unmarshalWithExtra(data, brc, knownFields)
}

// Annotations contains optional annotations for the client.
type Annotations struct {
	WithExtra
	Audience []Role  `json:"audience,omitempty"` // intended customers of the object or data
	Priority float64 `json:"priority,omitempty"` // importance of the data (0-1)
}

// MarshalJSON implements json.Marshaler
func (a *Annotations) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if len(a.Audience) > 0 {
		fieldMap["audience"] = a.Audience
	}
	if a.Priority != 0 {
		fieldMap["priority"] = a.Priority
	}

	return marshalWithExtra(a, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (a *Annotations) UnmarshalJSON(data []byte) error {
	knownFields := []string{"audience", "priority"}
	return unmarshalWithExtra(data, a, knownFields)
}
