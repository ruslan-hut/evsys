package availability

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
	"time"
)

// ============================================================================
// StatusNotification - OCPP 2.0.1
// ============================================================================
// Sent by: Charging Station → CSMS
// Purpose: Notify the CSMS about the status of a connector. This is similar
//          to OCPP 1.6 StatusNotification but uses the EVSE/Connector
//          hierarchical model and has different status values.
// ============================================================================

const StatusNotificationFeatureName = "StatusNotification"

// StatusNotificationRequest represents the request for StatusNotification
type StatusNotificationRequest struct {
	// Timestamp is when the status change occurred
	Timestamp time.Time `json:"timestamp" validate:"required"`

	// ConnectorStatus is the current status of the connector
	ConnectorStatus v201.ConnectorStatusType `json:"connectorStatus" validate:"required"`

	// EvseId is the EVSE identifier (0 for charging station itself)
	EvseId int `json:"evseId" validate:"required,min=0"`

	// ConnectorId is the connector identifier within the EVSE
	ConnectorId int `json:"connectorId" validate:"required,min=0"`
}

// StatusNotificationResponse represents the response to StatusNotification
type StatusNotificationResponse struct {
	// No fields required - empty response indicates acknowledgment
}

// GetFeatureName implements common.Request interface
func (r StatusNotificationRequest) GetFeatureName() string {
	return StatusNotificationFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r StatusNotificationRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r StatusNotificationRequest) Validate() error {
	if r.ConnectorStatus == "" {
		return &ValidationError{Field: "connectorStatus", Message: "required"}
	}
	if r.EvseId < 0 {
		return &ValidationError{Field: "evseId", Message: "must be >= 0"}
	}
	if r.ConnectorId < 0 {
		return &ValidationError{Field: "connectorId", Message: "must be >= 0"}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r StatusNotificationResponse) GetFeatureName() string {
	return StatusNotificationFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r StatusNotificationResponse) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
