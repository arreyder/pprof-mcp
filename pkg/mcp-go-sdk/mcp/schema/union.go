// Package schema provides a Go implementation of the Model Context Protocol (MCP).
package schema

import (
	"encoding/json"
	"fmt"
)

// ContentUnion represents a union type that can be one of: TextContent, ImageContent, or AudioContent
type ContentUnion struct {
	WithExtra
	Text  *TextContent  `json:"-"`
	Image *ImageContent `json:"-"`
	Audio *AudioContent `json:"-"`
}

// MarshalJSON implements json.Marshaler
func (cu *ContentUnion) MarshalJSON() ([]byte, error) {
	if cu.Text != nil {
		return json.Marshal(cu.Text)
	}
	if cu.Image != nil {
		return json.Marshal(cu.Image)
	}
	if cu.Audio != nil {
		return json.Marshal(cu.Audio)
	}

	return nil, fmt.Errorf("ContentUnion: no content type set")
}

// UnmarshalJSON implements json.Unmarshaler
func (cu *ContentUnion) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// The "type" field is our discriminator
	if typeStr, ok := m["type"].(string); ok {
		switch typeStr {
		case "text":
			cu.Text = &TextContent{}
			return json.Unmarshal(data, cu.Text)
		case "image":
			cu.Image = &ImageContent{}
			return json.Unmarshal(data, cu.Image)
		case "audio":
			cu.Audio = &AudioContent{}
			return json.Unmarshal(data, cu.Audio)
		default:
			return fmt.Errorf("ContentUnion: unknown type %q", typeStr)
		}
	}

	return fmt.Errorf("ContentUnion: missing 'type' field")
}

// NewContentFromText creates a new ContentUnion with TextContent
func NewContentFromText(text string) ContentUnion {
	tc := TextContent{Text: text}
	return ContentUnion{Text: &tc}
}

// NewContentFromImage creates a new ContentUnion with ImageContent
func NewContentFromImage(data, mimeType string) ContentUnion {
	ic := ImageContent{
		Data:     data,
		MimeType: mimeType,
	}
	return ContentUnion{Image: &ic}
}

// NewContentFromAudio creates a new ContentUnion with AudioContent
func NewContentFromAudio(data, mimeType string) ContentUnion {
	ac := AudioContent{
		Data:     data,
		MimeType: mimeType,
	}
	return ContentUnion{Audio: &ac}
}

// IsText returns true if this union contains TextContent
func (cu *ContentUnion) IsText() bool {
	return cu.Text != nil
}

// GetText returns the TextContent value if set, or nil
func (cu *ContentUnion) GetText() *TextContent {
	return cu.Text
}

// IsImage returns true if this union contains ImageContent
func (cu *ContentUnion) IsImage() bool {
	return cu.Image != nil
}

// GetImage returns the ImageContent value if set, or nil
func (cu *ContentUnion) GetImage() *ImageContent {
	return cu.Image
}

// IsAudio returns true if this union contains AudioContent
func (cu *ContentUnion) IsAudio() bool {
	return cu.Audio != nil
}

// GetAudio returns the AudioContent value if set, or nil
func (cu *ContentUnion) GetAudio() *AudioContent {
	return cu.Audio
}

// PromptContentUnion represents a union type that can be one of: TextContent, ImageContent, AudioContent, or EmbeddedResource
type PromptContentUnion struct {
	WithExtra
	Text     *TextContent      `json:"-"`
	Image    *ImageContent     `json:"-"`
	Audio    *AudioContent     `json:"-"`
	Resource *EmbeddedResource `json:"-"`
}

// MarshalJSON implements json.Marshaler
func (pcu *PromptContentUnion) MarshalJSON() ([]byte, error) {
	if pcu.Text != nil {
		return json.Marshal(pcu.Text)
	}
	if pcu.Image != nil {
		return json.Marshal(pcu.Image)
	}
	if pcu.Audio != nil {
		return json.Marshal(pcu.Audio)
	}
	if pcu.Resource != nil {
		return json.Marshal(pcu.Resource)
	}

	return nil, fmt.Errorf("PromptContentUnion: no content type set")
}

// UnmarshalJSON implements json.Unmarshaler
func (pcu *PromptContentUnion) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// The "type" field is our discriminator
	if typeStr, ok := m["type"].(string); ok {
		switch typeStr {
		case "text":
			pcu.Text = &TextContent{}
			return json.Unmarshal(data, pcu.Text)
		case "image":
			pcu.Image = &ImageContent{}
			return json.Unmarshal(data, pcu.Image)
		case "audio":
			pcu.Audio = &AudioContent{}
			return json.Unmarshal(data, pcu.Audio)
		case "resource":
			pcu.Resource = &EmbeddedResource{}
			return json.Unmarshal(data, pcu.Resource)
		default:
			return fmt.Errorf("PromptContentUnion: unknown type %q", typeStr)
		}
	}

	return fmt.Errorf("PromptContentUnion: missing 'type' field")
}

// NewPromptContentFromText creates a new PromptContentUnion with TextContent
func NewPromptContentFromText(text string) PromptContentUnion {
	tc := TextContent{Text: text}
	return PromptContentUnion{Text: &tc}
}

// NewPromptContentFromImage creates a new PromptContentUnion with ImageContent
func NewPromptContentFromImage(data, mimeType string) PromptContentUnion {
	ic := ImageContent{
		Data:     data,
		MimeType: mimeType,
	}
	return PromptContentUnion{Image: &ic}
}

// NewPromptContentFromAudio creates a new PromptContentUnion with AudioContent
func NewPromptContentFromAudio(data, mimeType string) PromptContentUnion {
	ac := AudioContent{
		Data:     data,
		MimeType: mimeType,
	}
	return PromptContentUnion{Audio: &ac}
}

// NewPromptContentFromResource creates a new PromptContentUnion with EmbeddedResource
func NewPromptContentFromResource(resource ResourceContentsUnion) PromptContentUnion {
	er := EmbeddedResource{
		Resource: resource,
	}
	return PromptContentUnion{Resource: &er}
}

// IsText returns true if this union contains TextContent
func (pcu *PromptContentUnion) IsText() bool {
	return pcu.Text != nil
}

// GetText returns the TextContent value if set, or nil
func (pcu *PromptContentUnion) GetText() *TextContent {
	return pcu.Text
}

// IsImage returns true if this union contains ImageContent
func (pcu *PromptContentUnion) IsImage() bool {
	return pcu.Image != nil
}

// GetImage returns the ImageContent value if set, or nil
func (pcu *PromptContentUnion) GetImage() *ImageContent {
	return pcu.Image
}

// IsAudio returns true if this union contains AudioContent
func (pcu *PromptContentUnion) IsAudio() bool {
	return pcu.Audio != nil
}

// GetAudio returns the AudioContent value if set, or nil
func (pcu *PromptContentUnion) GetAudio() *AudioContent {
	return pcu.Audio
}

// IsResource returns true if this union contains EmbeddedResource
func (pcu *PromptContentUnion) IsResource() bool {
	return pcu.Resource != nil
}

// GetResource returns the EmbeddedResource value if set, or nil
func (pcu *PromptContentUnion) GetResource() *EmbeddedResource {
	return pcu.Resource
}

// ReferenceUnion represents a union type that can be one of: PromptReference or ResourceReference
type ReferenceUnion struct {
	WithExtra
	Prompt   *PromptReference   `json:"-"`
	Resource *ResourceReference `json:"-"`
}

// MarshalJSON implements json.Marshaler
func (ru *ReferenceUnion) MarshalJSON() ([]byte, error) {
	if ru.Prompt != nil {
		return json.Marshal(ru.Prompt)
	}
	if ru.Resource != nil {
		return json.Marshal(ru.Resource)
	}

	return nil, fmt.Errorf("ReferenceUnion: no reference type set")
}

// UnmarshalJSON implements json.Unmarshaler
func (ru *ReferenceUnion) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// The "type" field is our discriminator
	if typeStr, ok := m["type"].(string); ok {
		switch typeStr {
		case "ref/prompt":
			ru.Prompt = &PromptReference{}
			return json.Unmarshal(data, ru.Prompt)
		case "ref/resource":
			ru.Resource = &ResourceReference{}
			return json.Unmarshal(data, ru.Resource)
		default:
			return fmt.Errorf("ReferenceUnion: unknown type %q", typeStr)
		}
	}

	return fmt.Errorf("ReferenceUnion: missing 'type' field")
}

// NewReferenceFromPrompt creates a new ReferenceUnion with PromptReference
func NewReferenceFromPrompt(name string) ReferenceUnion {
	pr := PromptReference{
		Name: name,
	}
	return ReferenceUnion{Prompt: &pr}
}

// NewReferenceFromResource creates a new ReferenceUnion with ResourceReference
func NewReferenceFromResource(uri string) ReferenceUnion {
	rr := ResourceReference{
		URI: uri,
	}
	return ReferenceUnion{Resource: &rr}
}

// IsPrompt returns true if this union contains PromptReference
func (ru *ReferenceUnion) IsPrompt() bool {
	return ru.Prompt != nil
}

// GetPrompt returns the PromptReference value if set, or nil
func (ru *ReferenceUnion) GetPrompt() *PromptReference {
	return ru.Prompt
}

// IsResource returns true if this union contains ResourceReference
func (ru *ReferenceUnion) IsResource() bool {
	return ru.Resource != nil
}

// GetResource returns the ResourceReference value if set, or nil
func (ru *ReferenceUnion) GetResource() *ResourceReference {
	return ru.Resource
}
