package core

import "evsys/types"

const StartTransactionFeatureName = "StartTransaction"

type StartTransactionRequest struct {
	ConnectorId   int             `json:"connectorId" validate:"gt=0"`
	IdTag         string          `json:"idTag" validate:"required,max=20"`
	MeterStart    int             `json:"meterStart" validate:"gte=0"`
	ReservationId *int            `json:"reservationId,omitempty" validate:"omitempty"`
	Timestamp     *types.DateTime `json:"timestamp" validate:"required"`
}

type StartTransactionResponse struct {
	IdTagInfo     *types.IdTagInfo `json:"idTagInfo" validate:"required"`
	TransactionId int              `json:"transactionId"`
}

func (req StartTransactionRequest) GetFeatureName() string {
	return StartTransactionFeatureName
}

func (res StartTransactionResponse) GetFeatureName() string {
	return StartTransactionFeatureName
}

func NewStartTransactionResponse(idTagInfo *types.IdTagInfo, transactionId int) *StartTransactionResponse {
	return &StartTransactionResponse{IdTagInfo: idTagInfo, TransactionId: transactionId}
}
