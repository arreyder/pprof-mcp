// Package schema provides a Go implementation of the Model Context Protocol (MCP).
package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// WithExtra is a base struct that can be embedded in all MCP type definitions
// to provide support for extra fields (fields not defined in the MCP specification).
type WithExtra struct {
	// Extra contains additional fields not defined in the MCP specification.
	// This is used to support forward compatibility with newer versions of the protocol.
	Extra map[string]any `json:"-"`
}

// GetExtra returns the extra fields map.
func (w *WithExtra) GetExtra() map[string]any {
	if w.Extra == nil {
		w.Extra = make(map[string]any)
	}
	return w.Extra
}

// SetExtra sets the extra fields map.
func (w *WithExtra) SetExtra(extra map[string]any) {
	w.Extra = extra
}

// WithConstants is an interface that types can implement to provide
// constant field values that should be included during JSON marshaling
// and validated during unmarshaling.
type WithConstants interface {
	// GetConstants returns a map of field names to their constant values.
	GetConstants() map[string]string
}

// marshalWithExtra is a helper function for implementing MarshalJSON methods.
// It takes a struct, the expected JSON field names, and returns a JSON representation
// that includes both the struct's fields and any extra fields.
func marshalWithExtra(v any, fieldMap map[string]any) ([]byte, error) {
	// Get the extra fields
	if we, ok := v.(interface{ GetExtra() map[string]any }); ok {
		extraMap := we.GetExtra()
		// Add extra fields to our map (but don't override existing fields)
		for k, v := range extraMap {
			if _, exists := fieldMap[k]; !exists {
				fieldMap[k] = v
			}
		}
	}

	// Handle ID field for MCPRequest types if not already set in fieldMap
	if req, ok := v.(MCPRequest); ok {
		id := req.GetRequestId()
		if !id.IsNull() && fieldMap["id"] == nil {
			fieldMap["id"] = id
		}
	}

	// Add constant fields if the type implements WithConstants
	if wc, ok := v.(WithConstants); ok {
		constants := wc.GetConstants()
		for k, v := range constants {
			// Constants take precedence over fieldMap and extraMap
			fieldMap[k] = v
		}
	}

	return json.Marshal(fieldMap)
}

// unmarshalWithExtra unmarshals the JSON data and preserves any extra fields in the Extra map
// It uses the alias-based approach for better performance and readability.
func unmarshalWithExtra(data []byte, v any, knownFields []string) error {
	// First unmarshal into a map to capture all fields
	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}

	// Create a set of known fields for faster lookup
	knownFieldsSet := make(map[string]bool, len(knownFields))
	for _, field := range knownFields {
		knownFieldsSet[field] = true
	}

	// Validate constants if the type implements WithConstants
	if wc, ok := v.(WithConstants); ok {
		constants := wc.GetConstants()
		for field, expectedValue := range constants {
			// Check if the field exists in the JSON
			rawValue, exists := allFields[field]
			if !exists {
				return fmt.Errorf("missing required constant field %q", field)
			}

			// Unmarshal the value and compare to the expected constant
			var actualValue string
			if err := json.Unmarshal(rawValue, &actualValue); err != nil {
				return fmt.Errorf("error unmarshaling constant field %q: %w", field, err)
			}

			if actualValue != expectedValue {
				return fmt.Errorf("invalid value for constant field %q: expected %q, got %q",
					field, expectedValue, actualValue)
			}

			// Mark constant fields as known
			knownFieldsSet[field] = true
		}
	}

	// Handle ID field for MCPRequest types
	if req, ok := v.(MCPRequest); ok {
		if idRaw, exists := allFields["id"]; exists {
			// Unmarshal the ID as a StringNumber
			var id StringNumber
			if err := json.Unmarshal(idRaw, &id); err == nil {
				req.SetRequestId(id)
			}
			// Mark "id" as a known field
			knownFieldsSet["id"] = true
		}
	}

	// Handle extra fields
	we, ok := v.(interface {
		GetExtra() map[string]any
		SetExtra(map[string]any)
	})
	if !ok {
		return fmt.Errorf("unmarshalWithExtra: struct does not implement GetExtra/SetExtra")
	}

	// Unmarshal known fields directly
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("unmarshalWithExtra: v must be a non-nil pointer")
	}

	// Get the struct value pointed to
	sv := rv.Elem()
	if sv.Kind() != reflect.Struct {
		return fmt.Errorf("unmarshalWithExtra: v must be a pointer to a struct")
	}

	// Get struct type
	st := sv.Type()

	// Process each field in the struct
	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Skip if field name is "WithExtra" or "Extra"
		if field.Name == "WithExtra" || field.Name == "Extra" {
			continue
		}

		// Get the JSON tag
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}

		// Extract the field name from the JSON tag
		jsonName, _ := parseJSONTag(tag)
		if jsonName == "" {
			jsonName = field.Name
		}

		// Check if we have this field in the JSON data
		rawValue, ok := allFields[jsonName]
		if !ok {
			continue
		}

		// Unmarshal the field value
		fieldValue := sv.Field(i)
		if fieldValue.CanSet() {
			if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
				fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
			}

			dest := fieldValue.Addr().Interface()
			if err := json.Unmarshal(rawValue, dest); err != nil {
				// Continue even if there's an error
				continue
			}
		}
	}

	// Process extra fields
	extra := we.GetExtra()
	if extra == nil {
		extra = make(map[string]any)
	}

	// Copy unknown fields to the Extra map
	for key, rawValue := range allFields {
		if !knownFieldsSet[key] {
			var value any
			if err := json.Unmarshal(rawValue, &value); err != nil {
				// Just skip problematic fields rather than failing
				continue
			}
			extra[key] = value
		}
	}

	we.SetExtra(extra)
	return nil
}

// parseJSONTag parses a JSON struct tag to extract the field name and whether
// the omitempty option is present.
func parseJSONTag(tag string) (name string, omitempty bool) {
	if idx := strings.Index(tag, ","); idx != -1 {
		// Get the name part
		name = tag[:idx]
		// Check if the rest contains "omitempty"
		omitempty = strings.Index(tag[idx+1:], "omitempty") == 0 && len(tag[idx+1:]) >= 9 && tag[idx+1:idx+10] == "omitempty"
	} else {
		name = tag
	}
	return name, omitempty
}

// isEmptyValue reports whether v is the zero value for its type.
// This is similar to the implementation in the encoding/json package.
func isEmptyValue(v reflect.Value) bool {
	// Handle nil interface case
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}
