package core

import (
	"evsys/types"
	"fmt"
)

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

// Validate checks bounds and that at least one well-formed meter value is present.
func (r MeterValuesRequest) Validate() error {
	if r.ConnectorId < 0 {
		return fmt.Errorf("connectorId must be >= 0")
	}
	if len(r.MeterValue) == 0 {
		return fmt.Errorf("meterValue is required")
	}
	for i := range r.MeterValue {
		if err := r.MeterValue[i].Validate(); err != nil {
			return fmt.Errorf("meterValue[%d]: %w", i, err)
		}
	}
	return nil
}

func (c MeterValuesResponse) GetFeatureName() string {
	return MeterValuesFeatureName
}

func NewMeterValuesResponse() *MeterValuesResponse {
	return &MeterValuesResponse{}
}
