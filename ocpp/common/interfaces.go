package common

import (
	"net"
	"time"
)

// Request represents a version-agnostic OCPP request
// All version-specific request types should implement this interface
type Request interface {
	// GetFeatureName returns the OCPP feature/action name
	GetFeatureName() string

	// GetProtocolVersion returns the protocol version this request belongs to
	GetProtocolVersion() ProtocolVersion

	// Validate performs validation on the request fields
	// Returns error if validation fails
	Validate() error
}

// Response represents a version-agnostic OCPP response
// All version-specific response types should implement this interface
type Response interface {
	// GetFeatureName returns the OCPP feature/action name this response is for
	GetFeatureName() string

	// GetProtocolVersion returns the protocol version this response belongs to
	GetProtocolVersion() ProtocolVersion
}

// MessageHandler provides version-specific message handling
// Each OCPP protocol version should have its own implementation
type MessageHandler interface {
	// HandleRequest processes an incoming request from a charge point
	// Returns the response to send back or an error
	HandleRequest(ws VersionedWebSocket, action string, payload []byte) (Response, error)

	// CreateRequest creates an outgoing request to send to a charge point
	// This is used for Central System initiated operations (e.g., RemoteStartTransaction)
	CreateRequest(action string, payload interface{}) (Request, error)

	// GetVersion returns the protocol version this handler supports
	GetVersion() ProtocolVersion

	// SupportsFeature checks if a specific feature is supported by this handler
	SupportsFeature(action string) bool
}

// VersionedWebSocket extends the basic WebSocket interface with protocol version tracking
// This allows the system to route messages to the correct version-specific handler
type VersionedWebSocket interface {
	// ID returns the unique identifier for this WebSocket connection
	// Typically the charge point ID
	ID() string

	// GetProtocol returns the negotiated OCPP protocol version
	GetProtocol() ProtocolVersion

	// SetProtocol sets the protocol version for this connection
	// Called after WebSocket subprotocol negotiation
	SetProtocol(ProtocolVersion)

	// RemoteAddr returns the remote network address
	RemoteAddr() net.Addr

	// IsClosed checks if the WebSocket connection is closed
	IsClosed() bool

	// Close closes the WebSocket connection
	Close() error

	// WriteMessage sends a message through the WebSocket
	WriteMessage(data []byte) error

	// SetCloseHandler sets the handler for close messages
	SetCloseHandler(func(int, string) error)

	// SetPongHandler sets the handler for pong messages
	SetPongHandler(func(string) error)

	// SetReadDeadline sets the read deadline
	SetReadDeadline(time.Time) error

	// SetWriteDeadline sets the write deadline
	SetWriteDeadline(time.Time) error

	// SetUniqueId sets the unique ID for the current message transaction
	// Used for correlating requests and responses
	SetUniqueId(string)

	// GetUniqueId returns the unique ID for the current message transaction
	GetUniqueId() string
}

// Feature represents a complete OCPP feature with request and response types
// This is used for feature registration in the registry
type Feature interface {
	// GetFeatureName returns the name of the feature/action
	GetFeatureName() string

	// GetVersion returns the protocol version this feature belongs to
	GetVersion() ProtocolVersion

	// GetRequestExample returns an example instance of the request type
	// Used for type registration
	GetRequestExample() Request

	// GetResponseExample returns an example instance of the response type
	// Used for type registration
	GetResponseExample() Response
}

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string // The field that failed validation
	Message string // The error message
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
