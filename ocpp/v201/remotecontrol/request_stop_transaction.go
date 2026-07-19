package remotecontrol

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
)

// ============================================================================
// RequestStopTransaction - OCPP 2.0.1
// ============================================================================
// Sent by: CSMS → Charging Station
// Purpose: Request the Charging Station to stop a transaction. This replaces
//          RemoteStopTransaction from OCPP 1.6 and uses transaction ID
//          instead of just the transaction number.
// ============================================================================

const RequestStopTransactionFeatureName = "RequestStopTransaction"

// RequestStopTransactionRequest represents the request for RequestStopTransaction
type RequestStopTransactionRequest struct {
	// TransactionId is the identifier of the transaction to stop
	TransactionId string `json:"transactionId" validate:"required,max=36"`
}

// RequestStopTransactionResponse represents the response to RequestStopTransaction
type RequestStopTransactionResponse struct {
	// Status indicates whether the request was accepted
	Status RequestStartStopStatusType `json:"status" validate:"required"`

	// StatusInfo provides additional status information
	StatusInfo *v201.StatusInfo `json:"statusInfo,omitempty"`
}

// GetFeatureName implements common.Request interface
func (r RequestStopTransactionRequest) GetFeatureName() string {
	return RequestStopTransactionFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r RequestStopTransactionRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r RequestStopTransactionRequest) Validate() error {
	if r.TransactionId == "" {
		return &ValidationError{Field: "transactionId", Message: "required"}
	}
	if len(r.TransactionId) > 36 {
		return &ValidationError{Field: "transactionId", Message: "max 36 characters"}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r RequestStopTransactionResponse) GetFeatureName() string {
	return RequestStopTransactionFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r RequestStopTransactionResponse) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}
