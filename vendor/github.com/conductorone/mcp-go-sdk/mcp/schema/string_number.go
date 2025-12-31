// Package schema provides a Go implementation of the Model Context Protocol (MCP).
package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// StringNumberType represents the kind of value stored in a StringNumber.
type StringNumberType int

const (
	// NullType indicates that the StringNumber represents a null JSON value.
	// This is the zero value for StringNumberType, ensuring that the zero value
	// of StringNumber will be a null value.
	NullType StringNumberType = iota
	// StringType indicates that the StringNumber holds a string value.
	StringType
	// NumberType indicates that the StringNumber holds a number value.
	NumberType
)

// StringNumber represents a union type that can be either a string, a number, or null.
// This matches TypeScript's string | number | null type.
// It is comparable, so it can be used as a map key.
// The zero value of StringNumber represents a null value.
type StringNumber struct {
	// Type indicates whether the StringNumber holds a string, number, or null value.
	Type StringNumberType
	// StringValue holds the value if Type is StringType.
	StringValue string
	// NumberValue holds the value if Type is NumberType.
	// json.Number is used to preserve the exact number representation from the JSON.
	NumberValue json.Number
}

// NewStringNumberFromString creates a StringNumber from a string value.
func NewStringNumberFromString(s string) StringNumber {
	return StringNumber{
		Type:        StringType,
		StringValue: s,
	}
}

// NewStringNumberFromNumber creates a StringNumber from a json.Number value.
func NewStringNumberFromNumber(n json.Number) StringNumber {
	return StringNumber{
		Type:        NumberType,
		NumberValue: n,
	}
}

// NewStringNumberFromFloat creates a StringNumber from a float64 value.
func NewStringNumberFromFloat(n float64) StringNumber {
	return StringNumber{
		Type:        NumberType,
		NumberValue: json.Number(strconv.FormatFloat(n, 'f', -1, 64)),
	}
}

// NewStringNumberFromInteger creates a StringNumber from an integer value.
func NewStringNumberFromInteger(n int) StringNumber {
	return StringNumber{
		Type:        NumberType,
		NumberValue: json.Number(strconv.Itoa(n)),
	}
}

// NewStringNumberFromNull creates a StringNumber representing a null value.
// Note that the zero value of StringNumber is already a null value, so using
// the zero value directly (var sn StringNumber) is equivalent to calling this function.
func NewStringNumberFromNull() StringNumber {
	return StringNumber{
		Type: NullType,
	}
}

// MarshalJSON implements the json.Marshaler interface for StringNumber.
func (sn StringNumber) MarshalJSON() ([]byte, error) {
	switch sn.Type {
	case StringType:
		return json.Marshal(sn.StringValue)
	case NumberType:
		return []byte(sn.NumberValue), nil
	case NullType:
		return []byte("null"), nil
	default:
		return nil, fmt.Errorf("unknown StringNumber type: %d", sn.Type)
	}
}

// UnmarshalJSON implements the json.Unmarshaler interface for StringNumber.
func (sn *StringNumber) UnmarshalJSON(data []byte) error {
	// Clear existing values
	*sn = StringNumber{}

	// Check for null value
	if bytes.Equal(data, []byte("null")) {
		// Zero value is already NullType, so no need to set anything
		return nil
	}

	// Try to unmarshal as a string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		sn.Type = StringType
		sn.StringValue = s
		return nil
	}

	// If that fails, try to unmarshal as a number
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		sn.Type = NumberType
		sn.NumberValue = n
		return nil
	}

	return fmt.Errorf("value is neither a string, number, nor null: %s", string(data))
}

// String returns a string representation of the StringNumber.
func (sn StringNumber) String() string {
	switch sn.Type {
	case StringType:
		return sn.StringValue
	case NumberType:
		return string(sn.NumberValue)
	case NullType:
		return "null"
	default:
		return "<invalid>"
	}
}

// IsString returns true if the StringNumber holds a string value.
func (sn StringNumber) IsString() bool {
	return sn.Type == StringType
}

// IsNumber returns true if the StringNumber holds a number value.
func (sn StringNumber) IsNumber() bool {
	return sn.Type == NumberType
}

// IsNull returns true if the StringNumber represents a null value.
func (sn StringNumber) IsNull() bool {
	return sn.Type == NullType
}

// GetString returns the string value if StringNumber holds a string.
// If it holds a number or null, it returns an empty string.
func (sn StringNumber) GetString() string {
	if sn.Type == StringType {
		return sn.StringValue
	}
	return ""
}

// GetNumber returns the json.Number value if StringNumber holds a number.
// If it holds a string or null, it returns an empty json.Number.
func (sn StringNumber) GetNumber() json.Number {
	if sn.Type == NumberType {
		return sn.NumberValue
	}
	return ""
}

// Float64 returns the number as a float64 if StringNumber holds a number.
// If it holds a string or null, it returns 0 and an error.
func (sn StringNumber) Float64() (float64, error) {
	if sn.Type != NumberType {
		return 0, fmt.Errorf("not a number type")
	}
	return sn.NumberValue.Float64()
}

// Int64 returns the number as an int64 if StringNumber holds a number.
// If it holds a string or null, it returns 0 and an error.
func (sn StringNumber) Int64() (int64, error) {
	if sn.Type != NumberType {
		return 0, fmt.Errorf("not a number type")
	}
	return sn.NumberValue.Int64()
}

// DeepEqual compares two StringNumber values for exact equality.
// Two StringNumbers are equal if they have the same type and the exact same representation.
// For numbers, the string representation must match exactly (e.g., "42" and "42.0" are different).
func (sn StringNumber) DeepEqual(other StringNumber) bool {
	if sn.Type != other.Type {
		return false
	}
	switch sn.Type {
	case StringType:
		return sn.StringValue == other.StringValue
	case NumberType:
		// For numbers, compare the exact string representation
		return string(sn.NumberValue) == string(other.NumberValue)
	case NullType:
		return true // All null values are equal
	default:
		return false
	}
}

// Equal compares two StringNumber values semantically.
// This method considers numeric values with the same numeric value
// to be equal, even if they have different string representations (e.g., "42" and "42.0").
// This is aligned with how JavaScript/JSON would treat these values.
func (sn StringNumber) Equal(other StringNumber) bool {
	if sn.Type != other.Type {
		return false
	}

	switch sn.Type {
	case StringType:
		return sn.StringValue == other.StringValue
	case NumberType:
		// For numbers, compare the actual numeric values
		snFloat, snErr := sn.NumberValue.Float64()
		otherFloat, otherErr := other.NumberValue.Float64()

		// If both can be parsed as floats, compare them
		if snErr == nil && otherErr == nil {
			return snFloat == otherFloat
		}

		// If either cannot be parsed as a float, fall back to string comparison
		return string(sn.NumberValue) == string(other.NumberValue)
	case NullType:
		return true // All null values are equal
	default:
		return false
	}
}
