package core

import "evsys/types"

const RemoteStartTransactionFeatureName = "RemoteStartTransaction"

type RemoteStartTransactionRequest struct {
	ConnectorId     *int                   `json:"connectorId,omitempty" validate:"omitempty,gt=0"`
	IdTag           string                 `json:"idTag" validate:"required,max=20"`
	ChargingProfile *types.ChargingProfile `json:"chargingProfile,omitempty"`
}

type RemoteStartTransactionResponse struct {
	Status types.RemoteStartStopStatus `json:"status" validate:"required,remoteStartStopStatus"`
}

func (r RemoteStartTransactionRequest) GetFeatureName() string {
	return RemoteStartTransactionFeatureName
}

func NewRemoteStartTransactionRequest(idTag string) *RemoteStartTransactionRequest {
	return &RemoteStartTransactionRequest{IdTag: idTag}
}
