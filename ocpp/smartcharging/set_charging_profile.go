package smartcharging

import "evsys/types"

const SetChargingProfileFeatureName = "SetChargingProfile"

type SetChargingProfileRequest struct {
	ConnectorId     int                    `json:"connectorId" validate:"gte=0"`
	ChargingProfile *types.ChargingProfile `json:"csChargingProfiles" validate:"required"`
}

func NewSetChargingProfileRequest(connectorId int, chargingProfile *types.ChargingProfile) *SetChargingProfileRequest {
	return &SetChargingProfileRequest{ConnectorId: connectorId, ChargingProfile: chargingProfile}
}

func (r SetChargingProfileRequest) GetFeatureName() string {
	return SetChargingProfileFeatureName
}
