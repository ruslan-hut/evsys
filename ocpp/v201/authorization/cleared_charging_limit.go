package authorization

import (
	"evsys/ocpp/common"
)

// ============================================================================
// ClearedChargingLimit - OCPP 2.0.1
// ============================================================================
// Sent by: Charging Station → CSMS
// Purpose: Inform the CSMS that the Charging Station has cleared a
//          previously set charging limit. This is sent when external
//          control (e.g., via ISO 15118) clears the limit.
// ============================================================================

const ClearedChargingLimitFeatureName = "ClearedChargingLimit"

// ChargingLimitSourceType defines the source of the charging limit
type ChargingLimitSourceType string

const (
	ChargingLimitSourceEMS   ChargingLimitSourceType = "EMS"   // Energy Management System
	ChargingLimitSourceOther ChargingLimitSourceType = "Other" // Other external source
	ChargingLimitSourceSO    ChargingLimitSourceType = "SO"    // System Operator
	ChargingLimitSourceCSO   ChargingLimitSourceType = "CSO"   // Charging Station Operator
)

// ClearedChargingLimitRequest represents the request for ClearedChargingLimit
type ClearedChargingLimitRequest struct {
	// ChargingLimitSource indicates the source of the charging limit
	ChargingLimitSource ChargingLimitSourceType `json:"chargingLimitSource" validate:"required"`

	// EvseId is the EVSE for which the limit was cleared (optional)
	EvseId *int `json:"evseId,omitempty" validate:"omitempty,min=1"`
}

// ClearedChargingLimitResponse represents the response to ClearedChargingLimit
type ClearedChargingLimitResponse struct {
	// No fields required - empty response indicates acknowledgment
}

// GetFeatureName implements common.Request interface
func (r ClearedChargingLimitRequest) GetFeatureName() string {
	return ClearedChargingLimitFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r ClearedChargingLimitRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r ClearedChargingLimitRequest) Validate() error {
	if r.ChargingLimitSource == "" {
		return &ValidationError{Field: "chargingLimitSource", Message: "required"}
	}
	if r.EvseId != nil && *r.EvseId < 1 {
		return &ValidationError{Field: "evseId", Message: "must be >= 1"}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r ClearedChargingLimitResponse) GetFeatureName() string {
	return ClearedChargingLimitFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r ClearedChargingLimitResponse) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}
