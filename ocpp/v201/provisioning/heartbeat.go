package provisioning

import (
	"evsys/ocpp/common"
	"time"
)

// ============================================================================
// Heartbeat - OCPP 2.0.1
// ============================================================================
// Sent by: Charging Station → CSMS
// Purpose: To let the CSMS know that a Charging Station is still connected.
//          The Charging Station SHALL send a HeartbeatRequest for every
//          HeartbeatInterval seconds.
// ============================================================================

const HeartbeatFeatureName = "Heartbeat"

// HeartbeatRequest represents the request for Heartbeat
type HeartbeatRequest struct {
	// No fields - empty request
}

// HeartbeatResponse represents the response to Heartbeat
type HeartbeatResponse struct {
	// CurrentTime is the current time at the CSMS
	CurrentTime time.Time `json:"currentTime" validate:"required"`
}

// GetFeatureName implements common.Request interface
func (r HeartbeatRequest) GetFeatureName() string {
	return HeartbeatFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r HeartbeatRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r HeartbeatRequest) Validate() error {
	return nil // No validation needed for empty request
}

// GetFeatureName implements common.Response interface
func (r HeartbeatResponse) GetFeatureName() string {
	return HeartbeatFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r HeartbeatResponse) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}
