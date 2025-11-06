package remotecontrol

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
)

// ============================================================================
// RequestStartTransaction - OCPP 2.0.1
// ============================================================================
// Sent by: CSMS → Charging Station
// Purpose: Request the Charging Station to start a transaction. This replaces
//          RemoteStartTransaction from OCPP 1.6 and includes support for
//          EVSE-specific requests and charging profiles.
// ============================================================================

const RequestStartTransactionFeatureName = "RequestStartTransaction"

// RequestStartStopStatusType defines the status of a start/stop request
type RequestStartStopStatusType string

const (
	RequestStartStopStatusAccepted RequestStartStopStatusType = "Accepted" // Request accepted
	RequestStartStopStatusRejected RequestStartStopStatusType = "Rejected" // Request rejected
)

// RequestStartTransactionRequest represents the request for RequestStartTransaction
type RequestStartTransactionRequest struct {
	// IdToken is the identifier for authorization
	IdToken v201.IdToken `json:"idToken" validate:"required"`

	// RemoteStartId is the ID given by the server to this start request
	RemoteStartId int `json:"remoteStartId" validate:"required"`

	// EvseId is the EVSE to start transaction on (optional)
	EvseId *int `json:"evseId,omitempty" validate:"omitempty,min=1"`

	// ChargingProfile is the charging profile to use for this transaction
	ChargingProfile *v201.ChargingProfile `json:"chargingProfile,omitempty"`

	// GroupIdToken is the group identifier for authorization
	GroupIdToken *v201.IdToken `json:"groupIdToken,omitempty"`
}

// RequestStartTransactionResponse represents the response to RequestStartTransaction
type RequestStartTransactionResponse struct {
	// Status indicates whether the request was accepted
	Status RequestStartStopStatusType `json:"status" validate:"required"`

	// StatusInfo provides additional status information
	StatusInfo *v201.StatusInfo `json:"statusInfo,omitempty"`

	// TransactionId is the transaction identifier (if started immediately)
	TransactionId string `json:"transactionId,omitempty" validate:"omitempty,max=36"`
}

// GetFeatureName implements common.Request interface
func (r RequestStartTransactionRequest) GetFeatureName() string {
	return RequestStartTransactionFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r RequestStartTransactionRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r RequestStartTransactionRequest) Validate() error {
	if err := r.IdToken.Validate(); err != nil {
		return err
	}
	if r.RemoteStartId == 0 {
		return &ValidationError{Field: "remoteStartId", Message: "required"}
	}
	if r.EvseId != nil && *r.EvseId < 1 {
		return &ValidationError{Field: "evseId", Message: "must be >= 1"}
	}
	if r.GroupIdToken != nil {
		if err := r.GroupIdToken.Validate(); err != nil {
			return &ValidationError{Field: "groupIdToken", Message: err.Error()}
		}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r RequestStartTransactionResponse) GetFeatureName() string {
	return RequestStartTransactionFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r RequestStartTransactionResponse) GetProtocolVersion() common.ProtocolVersion {
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
