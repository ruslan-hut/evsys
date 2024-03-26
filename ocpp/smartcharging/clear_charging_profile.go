package smartcharging

import "evsys/types"

const ClearChargingProfileFeatureName = "ClearChargingProfile"

type ClearChargingProfileRequest struct {
	Id                     *int                             `json:"id,omitempty" validate:"omitempty"`
	ConnectorId            *int                             `json:"connectorId,omitempty" validate:"omitempty,gte=0"`
	ChargingProfilePurpose types.ChargingProfilePurposeType `json:"chargingProfilePurpose,omitempty" validate:"omitempty,chargingProfilePurpose"`
	StackLevel             *int                             `json:"stackLevel,omitempty" validate:"omitempty,gte=0"`
}

func (r ClearChargingProfileRequest) GetFeatureName() string {
	return ClearChargingProfileFeatureName
}

func NewClearChargingProfileRequest() *ClearChargingProfileRequest {
	return &ClearChargingProfileRequest{}
}

func NewClearDefaultChargingProfileRequest() *ClearChargingProfileRequest {
	id := 1
	stackLevel := 1
	return &ClearChargingProfileRequest{
		Id:                     &id,
		StackLevel:             &stackLevel,
		ChargingProfilePurpose: types.ChargingProfilePurposeTxDefaultProfile,
	}
}
