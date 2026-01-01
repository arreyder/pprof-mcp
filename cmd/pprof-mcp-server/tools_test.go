package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/arreyder/pprof-mcp/internal/pprof"
)

func TestToolSchemasMarshalAndAdditionalProperties(t *testing.T) {
	for _, def := range ToolSchemas() {
		if def.Tool == nil {
			t.Fatalf("tool definition missing tool")
		}
		schema := def.Tool.InputSchema
		if schema == nil {
			t.Fatalf("tool %q missing input schema", def.Tool.Name)
		}
		if _, err := json.Marshal(schema); err != nil {
			t.Fatalf("tool %q input schema failed to marshal: %v", def.Tool.Name, err)
		}

		schemaMap, ok := schema.(map[string]any)
		if !ok {
			t.Fatalf("tool %q input schema not map", def.Tool.Name)
		}
		if schemaMap["type"] != "object" {
			t.Fatalf("tool %q input schema type not object", def.Tool.Name)
		}
		if additional, ok := schemaMap["additionalProperties"].(bool); !ok || additional {
			t.Fatalf("tool %q input schema should set additionalProperties=false", def.Tool.Name)
		}

		if def.Tool.OutputSchema != nil {
			if _, err := json.Marshal(def.Tool.OutputSchema); err != nil {
				t.Fatalf("tool %q output schema failed to marshal: %v", def.Tool.Name, err)
			}
		}
	}
}

func TestToolSchemasOrdering(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Add(ToolDefinition{Tool: &mcp.Tool{Name: "b"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := registry.Add(ToolDefinition{Tool: &mcp.Tool{Name: "a"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list := registry.List()
	names := []string{list[0].Tool.Name, list[1].Tool.Name}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("tool registry not sorted: %v", names)
	}
}

func TestToolRegistryDuplicate(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Add(ToolDefinition{Tool: &mcp.Tool{Name: "dup"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := registry.Add(ToolDefinition{Tool: &mcp.Tool{Name: "dup"}}); err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestValidateArgsMissingRequired(t *testing.T) {
	def := findTool(t, "pprof.top")
	if err := ValidateArgs(def.Tool, map[string]any{}); err == nil {
		t.Fatalf("expected validation error")
	} else if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestValidateArgsTypeMismatch(t *testing.T) {
	def := findTool(t, "pprof.traces_head")
	args := map[string]any{
		"profile": "profile.pprof",
		"lines":   "nope",
	}
	err := ValidateArgs(def.Tool, args)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	verr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !strings.Contains(verr.Message, "expected integer") {
		t.Fatalf("unexpected error message: %s", verr.Message)
	}
}

func TestValidateArgsUnknownField(t *testing.T) {
	def := findTool(t, "pprof.top")
	args := map[string]any{
		"profile": "profile.pprof",
		"bogus":   "value",
	}
	err := ValidateArgs(def.Tool, args)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "unknown argument") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestValidateArgsInvalidEnum(t *testing.T) {
	def := findTool(t, "datadog.profiles.pick")
	args := map[string]any{
		"service":  "svc",
		"env":      "prod",
		"strategy": "nope",
	}
	err := ValidateArgs(def.Tool, args)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestValidateArgsMinItems(t *testing.T) {
	def := findTool(t, "pprof.merge")
	args := map[string]any{
		"profiles":    []string{"one.pprof"},
		"output_path": "out.pprof",
	}
	err := ValidateArgs(def.Tool, args)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "expected at least") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestValidateArgsConditionalDownloadProfileEvent(t *testing.T) {
	def := findTool(t, "profiles.download_latest_bundle")
	base := map[string]any{
		"service": "svc",
		"env":     "prod",
		"out_dir": "out",
	}
	args := map[string]any{
		"service":    base["service"],
		"env":        base["env"],
		"out_dir":    base["out_dir"],
		"profile_id": "p1",
	}
	if err := ValidateArgs(def.Tool, args); err == nil {
		t.Fatalf("expected validation error")
	}

	args = map[string]any{
		"service":  base["service"],
		"env":      base["env"],
		"out_dir":  base["out_dir"],
		"event_id": "e1",
	}
	if err := ValidateArgs(def.Tool, args); err == nil {
		t.Fatalf("expected validation error")
	}

	args = map[string]any{
		"service":    base["service"],
		"env":        base["env"],
		"out_dir":    base["out_dir"],
		"profile_id": "p1",
		"event_id":   "e1",
	}
	if err := ValidateArgs(def.Tool, args); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateArgsConditionalPickStrategy(t *testing.T) {
	def := findTool(t, "datadog.profiles.pick")
	base := map[string]any{
		"service": "svc",
		"env":     "prod",
	}
	args := map[string]any{
		"service":  base["service"],
		"env":      base["env"],
		"strategy": "closest_to_ts",
	}
	if err := ValidateArgs(def.Tool, args); err == nil {
		t.Fatalf("expected validation error")
	}

	args = map[string]any{
		"service":  base["service"],
		"env":      base["env"],
		"strategy": "manual_index",
	}
	if err := ValidateArgs(def.Tool, args); err == nil {
		t.Fatalf("expected validation error")
	}

	args = map[string]any{
		"service":  base["service"],
		"env":      base["env"],
		"strategy": "manual_index",
		"index":    0,
	}
	if err := ValidateArgs(def.Tool, args); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateArgsListFieldsAcceptString(t *testing.T) {
	def := findTool(t, "pprof.list")
	args := map[string]any{
		"profile":      "profile.pprof",
		"function":     "func",
		"source_paths": "src",
	}
	if err := ValidateArgs(def.Tool, args); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	def = findTool(t, "pprof.storylines")
	args = map[string]any{
		"profile":     "profile.pprof",
		"repo_prefix": "github.com/myorg/myrepo",
	}
	if err := ValidateArgs(def.Tool, args); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateArgsProfilesStringMinItems(t *testing.T) {
	def := findTool(t, "pprof.merge")
	args := map[string]any{
		"profiles":    "only-one.pprof",
		"output_path": "out.pprof",
	}
	err := ValidateArgs(def.Tool, args)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "expected at least") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestValidateArgsNumberBelowMinimum(t *testing.T) {
	def := findTool(t, "pprof.callgraph")
	args := map[string]any{
		"profile":      "profile.pprof",
		"output_path":  "out.svg",
		"node_frac":    -0.1,
		"sample_index": "cpu",
	}
	err := ValidateArgs(def.Tool, args)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "below minimum") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestValidateArgsNumberAboveMaximum(t *testing.T) {
	def := findTool(t, "pprof.callgraph")
	args := map[string]any{
		"profile":     "profile.pprof",
		"output_path": "out.svg",
		"edge_frac":   1.5,
	}
	err := ValidateArgs(def.Tool, args)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestValidateArgsIntegerMinimum(t *testing.T) {
	def := findTool(t, "datadog.profiles.list")
	args := map[string]any{
		"service": "svc",
		"env":     "prod",
		"hours":   -1,
	}
	err := ValidateArgs(def.Tool, args)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "below minimum") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestValidateArgsIntegerMaximum(t *testing.T) {
	def := findTool(t, "pprof.traces_head")
	args := map[string]any{
		"profile": "profile.pprof",
		"lines":   maxTracesLines + 1,
	}
	err := ValidateArgs(def.Tool, args)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestValidateArgsArrayItemType(t *testing.T) {
	def := findTool(t, "pprof.storylines")
	args := map[string]any{
		"profile":     "profile.pprof",
		"repo_prefix": []any{123},
	}
	err := ValidateArgs(def.Tool, args)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "expected string") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestErrorResultFormatting(t *testing.T) {
	err := &ValidationError{
		Field:    "profile",
		Message:  "missing required argument \"profile\"",
		Expected: "required field",
		Received: "<missing>",
		Hint:     "Provide a value for \"profile\".",
	}
	res := ErrorResult(err, "")
	if !res.IsError {
		t.Fatalf("expected IsError=true")
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected one content item")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	if !strings.Contains(text.Text, "Expected:") || !strings.Contains(text.Text, "Hint:") {
		t.Fatalf("error text missing expected fields: %s", text.Text)
	}
	if res.StructuredContent == nil {
		t.Fatalf("expected structured error content")
	}
}

func TestNoMatchesResult(t *testing.T) {
	err := fmt.Errorf("%w: no matches found for regexp: Foo", pprof.ErrNoMatches)
	res := noMatchesResult("pprof.peek", map[string]any{"regex": "Foo"}, err)
	if res.IsError {
		t.Fatalf("expected IsError=false")
	}
	payload, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured payload")
	}
	if payload["matched"] != false {
		t.Fatalf("expected matched=false")
	}
	if payload["pattern"] != "Foo" {
		t.Fatalf("expected pattern Foo, got %v", payload["pattern"])
	}
	if payload["reason"] != "no_matches" {
		t.Fatalf("expected reason no_matches, got %v", payload["reason"])
	}
}

func findTool(t *testing.T, name string) ToolDefinition {
	t.Helper()
	for _, def := range ToolSchemas() {
		if def.Tool.Name == name {
			return def
		}
	}
	t.Fatalf("tool %q not found", name)
	return ToolDefinition{}
}
