package pprof

import (
	"strconv"
	"strings"
)

// ParsePercent converts "12.3%" or "12.3" to float64 (12.3). Returns 0 on failure.
func ParsePercent(value string) float64 {
	return parsePercent(value)
}

func parsePercent(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	value = strings.TrimSuffix(value, "%")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
