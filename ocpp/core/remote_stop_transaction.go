package core

import "evsys/types"

const RemoteStopTransactionFeatureName = "RemoteStopTransaction"

type RemoteStopTransactionRequest struct {
	TransactionId int `json:"transactionId"`
}

type RemoteStopTransactionResponse struct {
	Status types.RemoteStartStopStatus `json:"status" validate:"required,remoteStartStopStatus"`
}

func (r RemoteStopTransactionRequest) GetFeatureName() string {
	return RemoteStopTransactionFeatureName
}

func (c RemoteStopTransactionResponse) GetFeatureName() string {
	return RemoteStopTransactionFeatureName
}

func NewRemoteStopTransactionRequest(transactionId int) *RemoteStopTransactionRequest {
	return &RemoteStopTransactionRequest{TransactionId: transactionId}
}
