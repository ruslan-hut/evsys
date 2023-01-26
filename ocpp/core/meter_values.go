package core

import "evsys/types"

const MeterValuesFeatureName = "MeterValues"

type MeterValuesRequest struct {
	ConnectorId   int                `json:"connectorId" validate:"gte=0"`
	TransactionId *int               `json:"transactionId,omitempty"`
	MeterValue    []types.MeterValue `json:"meterValue" validate:"required,min=1,dive"`
}

type MeterValuesResponse struct {
}

func (r MeterValuesRequest) GetFeatureName() string {
	return MeterValuesFeatureName
}

func (c MeterValuesResponse) GetFeatureName() string {
	return MeterValuesFeatureName
}

func NewMeterValuesResponse() *MeterValuesResponse {
	return &MeterValuesResponse{}
}
