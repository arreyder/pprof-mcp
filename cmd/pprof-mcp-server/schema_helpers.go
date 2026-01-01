package main

func NewObjectSchema(props map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func NewObjectSchemaWithAdditional(props map[string]any, additional any, required ...string) map[string]any {
	schema := NewObjectSchema(props, required...)
	schema["additionalProperties"] = additional
	return schema
}

func prop(typ, desc string) map[string]any {
	return map[string]any{
		"type":        typ,
		"description": desc,
	}
}

func enumProp(typ, desc string, values []string) map[string]any {
	p := prop(typ, desc)
	p["enum"] = values
	return p
}

func arrayProp(itemType, desc string) map[string]any {
	return arrayPropSchema(prop(itemType, "Item"), desc)
}

func arrayPropSchema(item map[string]any, desc string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": desc,
		"items":       item,
	}
}

func arrayOrStringPropSchema(item map[string]any, desc string) map[string]any {
	return map[string]any{
		"type":        []string{"array", "string"},
		"description": desc,
		"items":       item,
	}
}

func arrayPropMin(item map[string]any, desc string, minItems int) map[string]any {
	p := arrayPropSchema(item, desc)
	p["minItems"] = minItems
	return p
}

func arrayOrStringPropMin(item map[string]any, desc string, minItems int) map[string]any {
	p := arrayOrStringPropSchema(item, desc)
	p["minItems"] = minItems
	return p
}

func numberProp(desc string, min, max *float64) map[string]any {
	p := prop("number", desc)
	if min != nil {
		p["minimum"] = *min
	}
	if max != nil {
		p["maximum"] = *max
	}
	return p
}

func integerProp(desc string, min, max *int) map[string]any {
	p := prop("integer", desc)
	if min != nil {
		p["minimum"] = *min
	}
	if max != nil {
		p["maximum"] = *max
	}
	return p
}

func intPtr(v int) *int {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}

func ProfilePath() map[string]any {
	return prop("string", "Path to the pprof profile file (required). Accepts handle IDs like handle:abc123 from profiles.download_latest_bundle.")
}

func BinaryPathOptional() map[string]any {
	return prop("string", "Path to the binary for symbol resolution")
}

func bundleInputSchema() map[string]any {
	return map[string]any{
		"type":        []string{"string", "array"},
		"description": "Bundle handle (any profile handle) or list of bundle file handles",
		"items": NewObjectSchema(map[string]any{
			"type":   prop("string", "Profile type (cpu, heap, mutex, block, goroutines)"),
			"handle": prop("string", "Profile handle from profiles.download_latest_bundle"),
			"bytes":  prop("integer", "Profile size in bytes"),
		}, "handle"),
	}
}
