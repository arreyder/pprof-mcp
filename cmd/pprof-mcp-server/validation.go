package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ValidationError struct {
	Field    string
	Message  string
	Expected string
	Received string
	Hint     string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ValidateArgs enforces tool input schema server-side for agent safety.
// It supports simple union types (e.g., string|array) and applies tool-specific
// conditional requirements that JSON Schema alone cannot express here.
func ValidateArgs(tool *mcp.Tool, args map[string]any) error {
	return ValidateArgsWithName(tool, tool.Name, args)
}

// ValidateArgsWithName allows callers to validate against a canonical tool name
// that may differ from the registered name (e.g., compatibility wrappers).
func ValidateArgsWithName(tool *mcp.Tool, name string, args map[string]any) error {
	schema, ok := tool.InputSchema.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid input schema for tool %q", name)
	}
	if args == nil {
		args = map[string]any{}
	}
	if err := validateObject(args, schema, ""); err != nil {
		return err
	}
	return validateConditionals(name, args)
}

func validateObject(value map[string]any, schema map[string]any, path string) error {
	props, _ := schema["properties"].(map[string]any)
	required := stringSlice(schema["required"])
	for _, key := range required {
		val, ok := value[key]
		if !ok || val == nil {
			field := joinPath(path, key)
			return &ValidationError{
				Field:    field,
				Message:  fmt.Sprintf("missing required argument %q", field),
				Expected: "required field",
				Received: "<missing>",
				Hint:     fmt.Sprintf("Provide a value for %q.", field),
			}
		}
	}

	if additionalPropertiesDisallowed(schema) {
		for key := range value {
			if _, ok := props[key]; !ok {
				field := joinPath(path, key)
				return &ValidationError{
					Field:    field,
					Message:  fmt.Sprintf("unknown argument %q", field),
					Expected: fmt.Sprintf("one of: %s", strings.Join(sortedKeys(props), ", ")),
					Received: redactValue(field, value[key]),
					Hint:     fmt.Sprintf("Remove %q or check the tool schema.", field),
				}
			}
		}
	}

	for key, val := range value {
		propSchemaRaw, ok := props[key]
		if !ok {
			continue
		}
		propSchema, ok := propSchemaRaw.(map[string]any)
		if !ok {
			continue
		}
		field := joinPath(path, key)
		if err := validateValue(val, propSchema, field); err != nil {
			return err
		}
	}

	return nil
}

func validateValue(value any, schema map[string]any, field string) error {
	if value == nil {
		return &ValidationError{
			Field:    field,
			Message:  fmt.Sprintf("invalid argument %q: value is null", field),
			Expected: strings.Join(schemaTypes(schema), ", "),
			Received: "null",
			Hint:     fmt.Sprintf("Provide a non-null value for %q.", field),
		}
	}

	types := schemaTypes(schema)
	if len(types) == 0 {
		return &ValidationError{
			Field:    field,
			Message:  fmt.Sprintf("invalid argument %q: missing schema type", field),
			Expected: "schema type",
			Received: redactValue(field, value),
			Hint:     fmt.Sprintf("Check the schema for %q.", field),
		}
	}

	if len(types) > 1 {
		if _, ok := value.(string); ok && hasType(types, "array") {
			if minItems, ok := intValue(schema["minItems"]); ok && minItems > 1 {
				return &ValidationError{
					Field:    field,
					Message:  fmt.Sprintf("invalid argument %q: expected at least %d items, got 1", field, minItems),
					Expected: fmt.Sprintf("minimum %d items", minItems),
					Received: redactValue(field, value),
					Hint:     fmt.Sprintf("Provide at least %d values for %q.", minItems, field),
				}
			}
		}
		var firstErr error
		var firstNonTypeErr error
		for _, typ := range types {
			if err := validateValueType(value, schema, field, typ); err == nil {
				return nil
			} else {
				if firstErr == nil {
					firstErr = err
				}
				if firstNonTypeErr == nil && !isTypeMismatch(err, typ) {
					firstNonTypeErr = err
				}
			}
		}
		if firstNonTypeErr != nil {
			return firstNonTypeErr
		}
		if firstErr != nil {
			return firstErr
		}
		return &ValidationError{
			Field:    field,
			Message:  fmt.Sprintf("invalid argument %q: expected one of types %s", field, strings.Join(types, ", ")),
			Expected: fmt.Sprintf("one of: %s", strings.Join(types, ", ")),
			Received: redactValue(field, value),
			Hint:     fmt.Sprintf("Provide a value matching one of: %s.", strings.Join(types, ", ")),
		}
	}

	return validateValueType(value, schema, field, types[0])
}

func validateValueType(value any, schema map[string]any, field, typ string) error {
	switch typ {
	case "string":
		if _, ok := value.(string); !ok {
			return typeError(field, "string", value)
		}
		if err := validateEnum(value, schema, field); err != nil {
			return err
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return typeError(field, "boolean", value)
		}
	case "integer":
		if _, ok := intValue(value); !ok {
			return typeError(field, "integer", value)
		}
		if err := validateNumberBounds(value, schema, field); err != nil {
			return err
		}
	case "number":
		if _, ok := floatValue(value); !ok {
			return typeError(field, "number", value)
		}
		if err := validateNumberBounds(value, schema, field); err != nil {
			return err
		}
	case "array":
		items, ok := sliceValue(value)
		if !ok {
			return typeError(field, "array", value)
		}
		if minItems, ok := intValue(schema["minItems"]); ok {
			if len(items) < int(minItems) {
				return &ValidationError{
					Field:    field,
					Message:  fmt.Sprintf("invalid argument %q: expected at least %d items, got %d", field, minItems, len(items)),
					Expected: fmt.Sprintf("minimum %d items", minItems),
					Received: fmt.Sprintf("array(len=%d)", len(items)),
					Hint:     fmt.Sprintf("Provide at least %d values for %q.", minItems, field),
				}
			}
		}
		itemSchemaRaw, ok := schema["items"].(map[string]any)
		if ok {
			for i, item := range items {
				itemField := fmt.Sprintf("%s[%d]", field, i)
				if err := validateValue(item, itemSchemaRaw, itemField); err != nil {
					return err
				}
			}
		}
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return typeError(field, "object", value)
		}
		if err := validateObject(obj, schema, field); err != nil {
			return err
		}
	default:
		return &ValidationError{
			Field:    field,
			Message:  fmt.Sprintf("invalid argument %q: unsupported schema type %q", field, typ),
			Expected: typ,
			Hint:     fmt.Sprintf("Check the schema for %q.", field),
		}
	}

	return nil
}

func validateEnum(value any, schema map[string]any, field string) error {
	enumVals := stringSlice(schema["enum"])
	if len(enumVals) == 0 {
		return nil
	}
	val, ok := value.(string)
	if !ok {
		return typeError(field, "string", value)
	}
	for _, allowed := range enumVals {
		if val == allowed {
			return nil
		}
	}
	return &ValidationError{
		Field:    field,
		Message:  fmt.Sprintf("invalid argument %q: value %q is not allowed", field, val),
		Expected: fmt.Sprintf("one of: %s", strings.Join(enumVals, ", ")),
		Received: redactValue(field, val),
		Hint:     fmt.Sprintf("Use one of: %s.", strings.Join(enumVals, ", ")),
	}
}

func validateNumberBounds(value any, schema map[string]any, field string) error {
	val, ok := floatValue(value)
	if !ok {
		return nil
	}
	if min, ok := floatValue(schema["minimum"]); ok {
		if val < min {
			return &ValidationError{
				Field:    field,
				Message:  fmt.Sprintf("invalid argument %q: value %v is below minimum %v", field, val, min),
				Expected: fmt.Sprintf("number >= %v", min),
				Received: redactValue(field, value),
				Hint:     fmt.Sprintf("Provide a value >= %v for %q.", min, field),
			}
		}
	}
	if max, ok := floatValue(schema["maximum"]); ok {
		if val > max {
			return &ValidationError{
				Field:    field,
				Message:  fmt.Sprintf("invalid argument %q: value %v exceeds maximum %v", field, val, max),
				Expected: fmt.Sprintf("number <= %v", max),
				Received: redactValue(field, value),
				Hint:     fmt.Sprintf("Provide a value <= %v for %q.", max, field),
			}
		}
	}
	return nil
}

func typeError(field, expected string, value any) error {
	return &ValidationError{
		Field:    field,
		Message:  fmt.Sprintf("invalid argument %q: expected %s, got %s", field, expected, valueType(value)),
		Expected: expected,
		Received: redactValue(field, value),
		Hint:     fmt.Sprintf("Provide a %s value for %q.", expected, field),
	}
}

func isTypeMismatch(err error, expected string) bool {
	verr, ok := err.(*ValidationError)
	if !ok {
		return false
	}
	return strings.Contains(verr.Message, fmt.Sprintf("expected %s, got", expected))
}

func additionalPropertiesDisallowed(schema map[string]any) bool {
	val, ok := schema["additionalProperties"]
	if !ok {
		return false
	}
	allowed, ok := val.(bool)
	return ok && !allowed
}

func validateConditionals(toolName string, args map[string]any) error {
	switch toolName {
	case "profiles.download_latest_bundle":
		profileID := argString(args, "profile_id")
		eventID := argString(args, "event_id")
		if profileID != "" && eventID == "" {
			return &ValidationError{
				Field:    "event_id",
				Message:  "missing required argument \"event_id\" when \"profile_id\" is set",
				Expected: "event_id with profile_id",
				Received: "<missing>",
				Hint:     "Provide both profile_id and event_id, or omit both to download the latest profile.",
			}
		}
		if profileID == "" && eventID != "" {
			return &ValidationError{
				Field:    "profile_id",
				Message:  "missing required argument \"profile_id\" when \"event_id\" is set",
				Expected: "profile_id with event_id",
				Received: "<missing>",
				Hint:     "Provide both profile_id and event_id, or omit both to download the latest profile.",
			}
		}
	case "pprof.discover":
		profileID := argString(args, "profile_id")
		eventID := argString(args, "event_id")
		if profileID != "" && eventID == "" {
			return &ValidationError{
				Field:    "event_id",
				Message:  "missing required argument \"event_id\" when \"profile_id\" is set",
				Expected: "event_id with profile_id",
				Received: "<missing>",
				Hint:     "Provide both profile_id and event_id, or omit both to download the latest profile.",
			}
		}
		if profileID == "" && eventID != "" {
			return &ValidationError{
				Field:    "profile_id",
				Message:  "missing required argument \"profile_id\" when \"event_id\" is set",
				Expected: "profile_id with event_id",
				Received: "<missing>",
				Hint:     "Provide both profile_id and event_id, or omit both to download the latest profile.",
			}
		}
	case "datadog.profiles.pick":
		strategy := argString(args, "strategy")
		switch strategy {
		case "closest_to_ts":
			if argString(args, "target_ts") == "" {
				return &ValidationError{
					Field:    "target_ts",
					Message:  "missing required argument \"target_ts\" for strategy \"closest_to_ts\"",
					Expected: "RFC3339 timestamp",
					Received: "<missing>",
					Hint:     "Provide target_ts when using strategy closest_to_ts.",
				}
			}
		case "manual_index":
			if !argPresent(args, "index") {
				return &ValidationError{
					Field:    "index",
					Message:  "missing required argument \"index\" for strategy \"manual_index\"",
					Expected: "integer (0-based index)",
					Received: "<missing>",
					Hint:     "Provide index when using strategy manual_index.",
				}
			}
		}
	}
	return nil
}

func schemaTypes(schema map[string]any) []string {
	switch v := schema["type"].(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func hasType(types []string, want string) bool {
	for _, typ := range types {
		if typ == want {
			return true
		}
	}
	return false
}

func argString(args map[string]any, key string) string {
	val, ok := args[key]
	if !ok || val == nil {
		return ""
	}
	if str, ok := val.(string); ok {
		return strings.TrimSpace(str)
	}
	return ""
}

func argPresent(args map[string]any, key string) bool {
	val, ok := args[key]
	return ok && val != nil
}

func stringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func sliceValue(value any) ([]any, bool) {
	switch v := value.(type) {
	case []any:
		return v, true
	case []string:
		items := make([]any, 0, len(v))
		for _, item := range v {
			items = append(items, item)
		}
		return items, true
	default:
		return nil, false
	}
}

func intValue(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		if math.Mod(v, 1) == 0 {
			return int64(v), true
		}
		return 0, false
	case json.Number:
		parsed, err := v.Int64()
		return parsed, err == nil
	case *int:
		if v == nil {
			return 0, false
		}
		return int64(*v), true
	case int32:
		return int64(v), true
	case uint:
		return int64(v), true
	case uint64:
		if v > math.MaxInt64 {
			return 0, false
		}
		return int64(v), true
	default:
		return 0, false
	}
}

func floatValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		parsed, err := v.Float64()
		return parsed, err == nil
	case *float64:
		if v == nil {
			return 0, false
		}
		return *v, true
	default:
		return 0, false
	}
}

func valueType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int64, int32, uint, uint64, float32, float64, json.Number:
		return "number"
	case []any, []string:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func redactValue(field string, value any) string {
	if value == nil {
		return "null"
	}
	lower := strings.ToLower(field)
	if strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") {
		return "[REDACTED]"
	}
	switch v := value.(type) {
	case string:
		if len(v) > 200 {
			return v[:200] + "...(truncated)"
		}
		return v
	case []any:
		return fmt.Sprintf("array(len=%d)", len(v))
	case []string:
		return fmt.Sprintf("array(len=%d)", len(v))
	case map[string]any:
		return fmt.Sprintf("object(len=%d)", len(v))
	default:
		return fmt.Sprintf("%v", v)
	}
}
