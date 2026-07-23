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

// NewGetCompositeScheduleRequestInUnit asks for the schedule in a specific unit.
// A charge point answering in watts cannot be compared against an amperage limit
// without knowing the voltage, so a diagnostic read asks for amperes explicitly.
func NewGetCompositeScheduleRequestInUnit(connectorId, duration int, unit types.ChargingRateUnitType) *GetCompositeScheduleRequest {
	return &GetCompositeScheduleRequest{
		ConnectorId:      connectorId,
		Duration:         duration,
		ChargingRateUnit: unit,
	}
}
