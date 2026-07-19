package provisioning

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
	"time"
)

// ============================================================================
// BootNotification - OCPP 2.0.1
// ============================================================================
// Sent by: Charging Station → CSMS
// Purpose: After each (re)boot, the Charging Station SHALL send a
//          BootNotificationRequest to the CSMS with information about its
//          configuration (e.g. version, vendor, etc.)
// ============================================================================

const BootNotificationFeatureName = "BootNotification"

// BootNotificationRequest represents the request for BootNotification
type BootNotificationRequest struct {
	// ChargingStation contains information about the charging station
	ChargingStation v201.ChargingStation `json:"chargingStation" validate:"required"`

	// Reason is the reason for sending this boot notification
	Reason v201.BootReasonType `json:"reason" validate:"required"`
}

// BootNotificationResponse represents the response to BootNotification
type BootNotificationResponse struct {
	// CurrentTime is the current time at the CSMS
	CurrentTime time.Time `json:"currentTime" validate:"required"`

	// Interval is the heartbeat interval in seconds
	Interval int `json:"interval" validate:"required,min=0"`

	// Status indicates whether the charging station has been registered
	Status v201.RegistrationStatusType `json:"status" validate:"required"`

	// StatusInfo provides additional status information
	StatusInfo *v201.StatusInfo `json:"statusInfo,omitempty"`
}

// GetFeatureName implements common.Request interface
func (r BootNotificationRequest) GetFeatureName() string {
	return BootNotificationFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r BootNotificationRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r BootNotificationRequest) Validate() error {
	if r.ChargingStation.VendorName == "" || len(r.ChargingStation.VendorName) > 50 {
		return &ValidationError{Field: "chargingStation.vendorName", Message: "required, max 50 characters"}
	}
	if r.ChargingStation.Model == "" || len(r.ChargingStation.Model) > 20 {
		return &ValidationError{Field: "chargingStation.model", Message: "required, max 20 characters"}
	}
	if r.Reason == "" {
		return &ValidationError{Field: "reason", Message: "required"}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r BootNotificationResponse) GetFeatureName() string {
	return BootNotificationFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r BootNotificationResponse) GetProtocolVersion() common.ProtocolVersion {
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
