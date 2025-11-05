package provisioning

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
)

// ============================================================================
// Reset - OCPP 2.0.1
// ============================================================================
// Sent by: CSMS → Charging Station
// Purpose: Request the Charging Station to reboot. The Charging Station
//          SHALL respond with a ResetResponse and then reboot if accepted.
// ============================================================================

const ResetFeatureName = "Reset"

// ResetType defines the type of reset
type ResetType string

const (
	ResetTypeImmediate ResetType = "Immediate" // Immediate reset
	ResetTypeOnIdle    ResetType = "OnIdle"    // Reset when idle
)

// ResetRequest represents the request for Reset
type ResetRequest struct {
	// Type is the type of reset
	Type ResetType `json:"type" validate:"required"`

	// EvseId is the EVSE to reset (optional, if omitted reset entire station)
	EvseId *int `json:"evseId,omitempty" validate:"omitempty,min=1"`
}

// ResetStatusType defines the status of a reset request
type ResetStatusType string

const (
	ResetStatusAccepted  ResetStatusType = "Accepted"  // Reset accepted
	ResetStatusRejected  ResetStatusType = "Rejected"  // Reset rejected
	ResetStatusScheduled ResetStatusType = "Scheduled" // Reset scheduled (for OnIdle type)
)

// ResetResponse represents the response to Reset
type ResetResponse struct {
	// Status indicates whether the reset was accepted
	Status ResetStatusType `json:"status" validate:"required"`

	// StatusInfo provides additional status information
	StatusInfo *v201.StatusInfo `json:"statusInfo,omitempty"`
}

// GetFeatureName implements common.Request interface
func (r ResetRequest) GetFeatureName() string {
	return ResetFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r ResetRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r ResetRequest) Validate() error {
	if r.Type == "" {
		return &ValidationError{Field: "type", Message: "required"}
	}
	if r.EvseId != nil && *r.EvseId < 1 {
		return &ValidationError{Field: "evseId", Message: "must be >= 1"}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r ResetResponse) GetFeatureName() string {
	return ResetFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r ResetResponse) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}
