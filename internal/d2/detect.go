package d2

import (
	"os"
	"strings"
)

// IsD2Environment checks if we're running in a d2 development environment
func IsD2Environment() bool {
	// Check for d2 environment variable (lowercase)
	d2Env := os.Getenv("d2")
	if strings.ToLower(d2Env) == "true" || d2Env == "1" {
		return true
	}

	// Also check uppercase for backwards compatibility
	d2EnvUpper := os.Getenv("D2")
	if strings.ToLower(d2EnvUpper) == "true" || d2EnvUpper == "1" {
		return true
	}

	// Additional checks for d2 indicators
	// Check if kubectl context suggests local dev
	// (Could add more sophisticated detection here)

	return false
}

// GetEnvironmentInfo returns information about the current environment
func GetEnvironmentInfo() map[string]string {
	info := make(map[string]string)

	info["d2"] = os.Getenv("d2")
	info["D2"] = os.Getenv("D2")
	info["is_d2"] = "false"
	if IsD2Environment() {
		info["is_d2"] = "true"
	}

	// Add other useful environment indicators
	info["dd_site"] = os.Getenv("DD_SITE")
	info["dd_env"] = os.Getenv("DD_ENV")

	return info
}
