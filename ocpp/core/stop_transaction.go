package core

import "evsys/types"

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
	IdTag           string             `json:"idTag,omitempty" validate:"max=20"`
	MeterStop       int                `json:"meterStop"`
	Timestamp       *types.DateTime    `json:"timestamp" validate:"required"`
	TransactionId   int                `json:"transactionId"`
	Reason          Reason             `json:"reason,omitempty" validate:"omitempty,reason"`
	TransactionData []types.MeterValue `json:"transactionData,omitempty" validate:"omitempty,dive"`
}

type StopTransactionResponse struct {
	IdTagInfo *types.IdTagInfo `json:"idTagInfo,omitempty" validate:"omitempty"`
}

func (r StopTransactionRequest) GetFeatureName() string {
	return StopTransactionFeatureName
}

func (c StopTransactionResponse) GetFeatureName() string {
	return StopTransactionFeatureName
}

func NewStopTransactionResponse() *StopTransactionResponse {
	return &StopTransactionResponse{}
}
