package core

import (
	"evsys/types"
	"fmt"
	"time"
)

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

// GetTimestamp returns the request timestamp, or time.Now() when omitted.
func (req StartTransactionRequest) GetTimestamp() time.Time {
	if req.Timestamp == nil {
		return time.Now()
	}
	return req.Timestamp.Time
}

// Validate checks required fields and bounds. Timestamp is handled defensively
// via GetTimestamp and is therefore not required here.
func (req StartTransactionRequest) Validate() error {
	if req.ConnectorId <= 0 {
		return fmt.Errorf("connectorId must be > 0")
	}
	if req.IdTag == "" {
		return fmt.Errorf("idTag is required")
	}
	if len(req.IdTag) > 20 {
		return fmt.Errorf("idTag exceeds 20 characters")
	}
	if req.MeterStart < 0 {
		return fmt.Errorf("meterStart must be >= 0")
	}
	return nil
}

func (res StartTransactionResponse) GetFeatureName() string {
	return StartTransactionFeatureName
}

func NewStartTransactionResponse(idTagInfo *types.IdTagInfo, transactionId int) *StartTransactionResponse {
	return &StartTransactionResponse{IdTagInfo: idTagInfo, TransactionId: transactionId}
}
