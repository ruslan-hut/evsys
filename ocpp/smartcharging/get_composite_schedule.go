package smartcharging

import "evsys/types"

const GetCompositeScheduleFeatureName = "GetCompositeSchedule"

type GetCompositeScheduleRequest struct {
	ConnectorId      int                        `json:"connectorId" validate:"gte=0"`
	Duration         int                        `json:"duration" validate:"gte=0"`
	ChargingRateUnit types.ChargingRateUnitType `json:"chargingRateUnit,omitempty" validate:"omitempty,chargingRateUnit"`
}

func (r GetCompositeScheduleRequest) GetFeatureName() string {
	return GetCompositeScheduleFeatureName
}

func NewGetCompositeScheduleRequest(connectorId int, duration int) *GetCompositeScheduleRequest {
	return &GetCompositeScheduleRequest{ConnectorId: connectorId, Duration: duration}
}
