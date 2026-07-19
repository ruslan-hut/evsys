package transactions

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
	"time"
)

// ============================================================================
// TransactionEvent - OCPP 2.0.1
// ============================================================================
// Sent by: Charging Station → CSMS
// Purpose: Report transaction-related events. This is the unified message
//          that replaces StartTransaction, StopTransaction, and MeterValues
//          from OCPP 1.6. All transaction lifecycle events are reported using
//          this single message type with different eventType values.
// ============================================================================

const TransactionEventFeatureName = "TransactionEvent"

// TransactionEventRequest represents the request for TransactionEvent
type TransactionEventRequest struct {
	// EventType indicates the type of event (Started, Updated, Ended)
	EventType v201.TransactionEventType `json:"eventType" validate:"required"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp" validate:"required"`

	// TriggerReason indicates why the event was triggered
	TriggerReason v201.TriggerReasonType `json:"triggerReason" validate:"required"`

	// SeqNo is the sequence number of this transaction event (incremental, 0-based)
	SeqNo int `json:"seqNo" validate:"required,min=0"`

	// TransactionInfo contains transaction-related information
	TransactionInfo v201.Transaction `json:"transactionInfo" validate:"required"`

	// Offline indicates if the Charging Station was offline when the event occurred
	Offline *bool `json:"offline,omitempty"`

	// NumberOfPhasesUsed indicates the number of electrical phases used
	NumberOfPhasesUsed *int `json:"numberOfPhasesUsed,omitempty" validate:"omitempty,min=1,max=3"`

	// CableMaxCurrent is the maximum current of the connected cable in Amps
	CableMaxCurrent *float64 `json:"cableMaxCurrent,omitempty" validate:"omitempty,min=0"`

	// ReservationId is the ID of the reservation that terminated as a result of this transaction
	ReservationId *int `json:"reservationId,omitempty"`

	// IdToken is the identifier used to start/stop the transaction
	IdToken *v201.IdToken `json:"idToken,omitempty"`

	// Evse identifies the EVSE where the transaction took place
	Evse *v201.EVSE `json:"evse,omitempty"`

	// MeterValue contains meter values sampled during the transaction
	MeterValue []v201.MeterValue `json:"meterValue,omitempty" validate:"omitempty,dive"`

	// PreconditioningStatus indicates if preconditioning is supported/active
	PreconditioningStatus PreconditioningStatusType `json:"preconditioningStatus,omitempty"`
}

// PreconditioningStatusType defines preconditioning status
type PreconditioningStatusType string

const (
	PreconditioningStatusReady        PreconditioningStatusType = "Ready"        // Ready for preconditioning
	PreconditioningStatusNotReady     PreconditioningStatusType = "NotReady"     // Not ready
	PreconditioningStatusInProgress   PreconditioningStatusType = "InProgress"   // Preconditioning in progress
	PreconditioningStatusNotSupported PreconditioningStatusType = "NotSupported" // Not supported
)

// TransactionEventResponse represents the response to TransactionEvent
type TransactionEventResponse struct {
	// TotalCost is the total cost of the transaction (optional)
	TotalCost *float64 `json:"totalCost,omitempty"`

	// ChargingPriority is the priority for this transaction (range -9 to 9)
	ChargingPriority *int `json:"chargingPriority,omitempty" validate:"omitempty,min=-9,max=9"`

	// IdTokenInfo contains updated authorization information
	IdTokenInfo *v201.IdTokenInfo `json:"idTokenInfo,omitempty"`

	// UpdatedPersonalMessage is an updated personal message to display
	UpdatedPersonalMessage *v201.MessageContent `json:"updatedPersonalMessage,omitempty"`
}

// GetFeatureName implements common.Request interface
func (r TransactionEventRequest) GetFeatureName() string {
	return TransactionEventFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r TransactionEventRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r TransactionEventRequest) Validate() error {
	if r.EventType == "" {
		return &ValidationError{Field: "eventType", Message: "required"}
	}
	if r.TriggerReason == "" {
		return &ValidationError{Field: "triggerReason", Message: "required"}
	}
	if r.SeqNo < 0 {
		return &ValidationError{Field: "seqNo", Message: "must be >= 0"}
	}
	if r.TransactionInfo.TransactionId == "" {
		return &ValidationError{Field: "transactionInfo.transactionId", Message: "required"}
	}

	// Validate IdToken if present
	if r.IdToken != nil {
		if err := r.IdToken.Validate(); err != nil {
			return &ValidationError{Field: "idToken", Message: err.Error()}
		}
	}

	// Validate number of phases
	if r.NumberOfPhasesUsed != nil && (*r.NumberOfPhasesUsed < 1 || *r.NumberOfPhasesUsed > 3) {
		return &ValidationError{Field: "numberOfPhasesUsed", Message: "must be 1, 2, or 3"}
	}

	// Validate cable max current
	if r.CableMaxCurrent != nil && *r.CableMaxCurrent < 0 {
		return &ValidationError{Field: "cableMaxCurrent", Message: "must be >= 0"}
	}

	return nil
}

// GetFeatureName implements common.Response interface
func (r TransactionEventResponse) GetFeatureName() string {
	return TransactionEventFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r TransactionEventResponse) GetProtocolVersion() common.ProtocolVersion {
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
