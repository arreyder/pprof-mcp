package main

import (
	"fmt"
	"sort"
)

// ToolRegistry stores tools by name to guarantee deterministic listing and unique names.
type ToolRegistry struct {
	tools map[string]ToolDefinition
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]ToolDefinition)}
}

func (r *ToolRegistry) Add(def ToolDefinition) error {
	if def.Tool == nil {
		return fmt.Errorf("tool definition missing tool")
	}
	name := def.Tool.Name
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("duplicate tool name %q", name)
	}
	r.tools[name] = def
	return nil
}

func (r *ToolRegistry) AddAll(defs []ToolDefinition) error {
	for _, def := range defs {
		if err := r.Add(def); err != nil {
			return err
		}
	}
	return nil
}

func (r *ToolRegistry) List() []ToolDefinition {
	keys := make([]string, 0, len(r.tools))
	for name := range r.tools {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	ordered := make([]ToolDefinition, 0, len(keys))
	for _, name := range keys {
		ordered = append(ordered, r.tools[name])
	}
	return ordered
}
