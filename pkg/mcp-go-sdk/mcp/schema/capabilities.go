// Package schema provides a Go implementation of the Model Context Protocol (MCP).
package schema

// ClientCapabilities represents capabilities a client may support.
type ClientCapabilities struct {
	WithExtra
	Experimental map[string]map[string]interface{} `json:"experimental,omitempty"`
	Roots        *RootsCapability                  `json:"roots,omitempty"`
	Sampling     map[string]interface{}            `json:"sampling,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (c *ClientCapabilities) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if c.Experimental != nil {
		fieldMap["experimental"] = c.Experimental
	}
	if c.Roots != nil {
		fieldMap["roots"] = c.Roots
	}
	if c.Sampling != nil {
		fieldMap["sampling"] = c.Sampling
	}

	return marshalWithExtra(c, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (c *ClientCapabilities) UnmarshalJSON(data []byte) error {
	knownFields := []string{"experimental", "roots", "sampling"}
	return unmarshalWithExtra(data, c, knownFields)
}

// RootsCapability indicates client support for listing roots.
type RootsCapability struct {
	WithExtra
	ListChanged bool `json:"listChanged,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *RootsCapability) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if r.ListChanged {
		fieldMap["listChanged"] = r.ListChanged
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *RootsCapability) UnmarshalJSON(data []byte) error {
	knownFields := []string{"listChanged"}
	return unmarshalWithExtra(data, r, knownFields)
}

// ServerCapabilities represents capabilities that a server may support.
type ServerCapabilities struct {
	WithExtra
	Experimental map[string]map[string]interface{} `json:"experimental,omitempty"`
	Logging      map[string]interface{}            `json:"logging,omitempty"`
	Completions  map[string]interface{}            `json:"completions,omitempty"`
	Prompts      *PromptsCapability                `json:"prompts,omitempty"`
	Resources    *ResourcesCapability              `json:"resources,omitempty"`
	Tools        *ToolsCapability                  `json:"tools,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (s *ServerCapabilities) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if s.Experimental != nil {
		fieldMap["experimental"] = s.Experimental
	}
	if s.Logging != nil {
		fieldMap["logging"] = s.Logging
	}
	if s.Completions != nil {
		fieldMap["completions"] = s.Completions
	}
	if s.Prompts != nil {
		fieldMap["prompts"] = s.Prompts
	}
	if s.Resources != nil {
		fieldMap["resources"] = s.Resources
	}
	if s.Tools != nil {
		fieldMap["tools"] = s.Tools
	}

	return marshalWithExtra(s, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (s *ServerCapabilities) UnmarshalJSON(data []byte) error {
	knownFields := []string{"experimental", "logging", "completions", "prompts", "resources", "tools"}
	return unmarshalWithExtra(data, s, knownFields)
}

// Implementation describes the name and version of an MCP implementation.
type Implementation struct {
	WithExtra
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MarshalJSON implements json.Marshaler
func (i *Implementation) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"name":    i.Name,
		"version": i.Version,
	}

	return marshalWithExtra(i, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (i *Implementation) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name", "version"}
	return unmarshalWithExtra(data, i, knownFields)
}

// ModelPreferences represents the server's preferences for model selection during sampling.
type ModelPreferences struct {
	WithExtra
	Hints                []ModelHint `json:"hints,omitempty"`
	CostPriority         float64     `json:"costPriority,omitempty"`
	SpeedPriority        float64     `json:"speedPriority,omitempty"`
	IntelligencePriority float64     `json:"intelligencePriority,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (m *ModelPreferences) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if m.Hints != nil {
		fieldMap["hints"] = m.Hints
	}
	if m.CostPriority != 0 {
		fieldMap["costPriority"] = m.CostPriority
	}
	if m.SpeedPriority != 0 {
		fieldMap["speedPriority"] = m.SpeedPriority
	}
	if m.IntelligencePriority != 0 {
		fieldMap["intelligencePriority"] = m.IntelligencePriority
	}

	return marshalWithExtra(m, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (m *ModelPreferences) UnmarshalJSON(data []byte) error {
	knownFields := []string{"hints", "costPriority", "speedPriority", "intelligencePriority"}
	return unmarshalWithExtra(data, m, knownFields)
}

// ModelHint represents hints to use for model selection.
type ModelHint struct {
	WithExtra
	Name string `json:"name,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (m *ModelHint) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{}

	if m.Name != "" {
		fieldMap["name"] = m.Name
	}

	return marshalWithExtra(m, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (m *ModelHint) UnmarshalJSON(data []byte) error {
	knownFields := []string{"name"}
	return unmarshalWithExtra(data, m, knownFields)
}

// Root represents a root directory or file that the server can operate on.
type Root struct {
	WithExtra
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (r *Root) MarshalJSON() ([]byte, error) {
	fieldMap := map[string]interface{}{
		"uri": r.URI,
	}

	if r.Name != "" {
		fieldMap["name"] = r.Name
	}

	return marshalWithExtra(r, fieldMap)
}

// UnmarshalJSON implements json.Unmarshaler
func (r *Root) UnmarshalJSON(data []byte) error {
	knownFields := []string{"uri", "name"}
	return unmarshalWithExtra(data, r, knownFields)
}
