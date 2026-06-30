package core

import (
	"evsys/types"
	"fmt"
	"time"
)

const StopTransactionFeatureName = "StopTransaction"

type Reason string

const (
	ReasonDeAuthorized   Reason = "DeAuthorized"
	ReasonEmergencyStop  Reason = "EmergencyStop"
	ReasonEVDisconnected Reason = "EVDisconnected"
	ReasonHardReset      Reason = "HardReset"
	ReasonLocal          Reason = "Local"
	ReasonOther          Reason = "Other"
	ReasonPowerLoss      Reason = "PowerLoss"
	ReasonReboot         Reason = "Reboot"
	ReasonRemote         Reason = "Remote"
	ReasonSoftReset      Reason = "SoftReset"
	ReasonUnlockCommand  Reason = "UnlockCommand"
)

type StopTransactionRequest struct {
	IdTag           string             `json:"idTag,omitempty" bson:"id_tag" validate:"max=20"`
	MeterStop       int                `json:"meterStop" bson:"meter_stop"`
	Timestamp       *types.DateTime    `json:"timestamp" bson:"timestamp" validate:"required"`
	TransactionId   int                `json:"transactionId" bson:"transaction_id"`
	Reason          Reason             `json:"reason,omitempty" bson:"reason" validate:"omitempty,reason"`
	TransactionData []types.MeterValue `json:"transactionData,omitempty" bson:"transaction_data" validate:"omitempty,dive"`
}

type StopTransactionResponse struct {
	IdTagInfo *types.IdTagInfo `json:"idTagInfo,omitempty" validate:"omitempty"`
}

func (r StopTransactionRequest) GetFeatureName() string {
	return StopTransactionFeatureName
}

// GetTimestamp returns the request timestamp, or time.Now() when omitted.
func (r StopTransactionRequest) GetTimestamp() time.Time {
	if r.Timestamp == nil {
		return time.Now()
	}
	return r.Timestamp.Time
}

// IsValid reports whether the reason is a known OCPP 1.6 stop reason.
func (r Reason) IsValid() bool {
	switch r {
	case ReasonDeAuthorized, ReasonEmergencyStop, ReasonEVDisconnected, ReasonHardReset,
		ReasonLocal, ReasonOther, ReasonPowerLoss, ReasonReboot, ReasonRemote,
		ReasonSoftReset, ReasonUnlockCommand:
		return true
	default:
		return false
	}
}

// Validate checks optional-field constraints. Timestamp is handled defensively
// via GetTimestamp and is therefore not required here.
func (r StopTransactionRequest) Validate() error {
	if len(r.IdTag) > 20 {
		return fmt.Errorf("idTag exceeds 20 characters")
	}
	if r.Reason != "" && !r.Reason.IsValid() {
		return fmt.Errorf("invalid reason: %q", r.Reason)
	}
	for i := range r.TransactionData {
		if err := r.TransactionData[i].Validate(); err != nil {
			return fmt.Errorf("transactionData[%d]: %w", i, err)
		}
	}
	return nil
}

func (c StopTransactionResponse) GetFeatureName() string {
	return StopTransactionFeatureName
}

func NewStopTransactionResponse() *StopTransactionResponse {
	return &StopTransactionResponse{}
}
