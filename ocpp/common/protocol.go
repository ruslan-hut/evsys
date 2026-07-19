package common

// ProtocolVersion represents an OCPP protocol version
type ProtocolVersion string

// OCPP protocol version constants
const (
	// OCPP16 represents OCPP 1.6 JSON protocol
	OCPP16 ProtocolVersion = "ocpp1.6"

	// OCPP201 represents OCPP 2.0.1 protocol
	OCPP201 ProtocolVersion = "ocpp2.0.1"

	// OCPP21 represents OCPP 2.1 protocol
	OCPP21 ProtocolVersion = "ocpp2.1"

	// UnknownVersion represents an unknown or unsupported protocol version
	UnknownVersion ProtocolVersion = ""
)

// String returns the string representation of the protocol version
func (p ProtocolVersion) String() string {
	return string(p)
}

// IsValid checks if the protocol version is supported
func (p ProtocolVersion) IsValid() bool {
	switch p {
	case OCPP16, OCPP201, OCPP21:
		return true
	default:
		return false
	}
}

// MajorVersion returns the major version number
// For OCPP 1.6, returns "1"
// For OCPP 2.0.1, returns "2"
// For OCPP 2.1, returns "2"
func (p ProtocolVersion) MajorVersion() string {
	switch p {
	case OCPP16:
		return "1"
	case OCPP201, OCPP21:
		return "2"
	default:
		return ""
	}
}

// ParseProtocolVersion converts a string to a ProtocolVersion
func ParseProtocolVersion(version string) ProtocolVersion {
	switch version {
	case "ocpp1.6", "1.6":
		return OCPP16
	case "ocpp2.0.1", "2.0.1":
		return OCPP201
	case "ocpp2.1", "2.1":
		return OCPP21
	default:
		return UnknownVersion
	}
}

// SupportedVersions returns all supported protocol versions
func SupportedVersions() []ProtocolVersion {
	return []ProtocolVersion{
		OCPP16,
		OCPP201,
		OCPP21,
	}
}

// DefaultVersion returns the default protocol version (backward compatibility)
func DefaultVersion() ProtocolVersion {
	return OCPP16
}
