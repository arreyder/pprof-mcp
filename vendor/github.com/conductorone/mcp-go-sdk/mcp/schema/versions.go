package schema

// Protocol version constants for Model Context Protocol (MCP).
// These represent known protocol versions supported by this SDK.
const (
	// ProtocolVersion2024_11_05 represents the 2024-11-05 version of MCP
	ProtocolVersion2024_11_05 = "2024-11-05"

	// ProtocolVersion2025_03_26 represents the 2025-03-26 version of MCP
	ProtocolVersion2025_03_26 = "2025-03-26"
)

// DefaultSupportedProtocolVersions is the default list of protocol versions
// supported by this SDK, ordered by preference (latest/most preferred first).
var DefaultSupportedProtocolVersions = []string{
	ProtocolVersion2025_03_26,
	ProtocolVersion2024_11_05,
}
