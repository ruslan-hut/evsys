package smartcharging

import (
	"evsys/types"
	"time"
)

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

func NewDefaultChargingProfile(limit int) *types.ChargingProfile {
	duration := 86400
	period := types.ChargingSchedulePeriod{
		StartPeriod: 0,
		Limit:       float64(limit),
	}
	return &types.ChargingProfile{
		ChargingProfileId:      1,
		StackLevel:             1,
		ChargingProfilePurpose: types.ChargingProfilePurposeTxDefaultProfile,
		ChargingProfileKind:    types.ChargingProfileKindRecurring,
		RecurrencyKind:         types.RecurrencyKindDaily,
		ChargingSchedule: &types.ChargingSchedule{
			StartSchedule:    types.NewDateTime(time.Now()),
			Duration:         &duration,
			ChargingRateUnit: types.ChargingRateUnitAmperes,
			ChargingSchedulePeriod: []types.ChargingSchedulePeriod{
				period,
			},
		},
	}
}

func NewTransactionChargingProfile(transactionId, limit int) *types.ChargingProfile {
	period := types.ChargingSchedulePeriod{
		StartPeriod: 0,
		Limit:       float64(limit),
	}
	return &types.ChargingProfile{
		ChargingProfileId:      10,
		StackLevel:             10,
		TransactionId:          transactionId,
		ChargingProfilePurpose: types.ChargingProfilePurposeTxProfile,
		ChargingProfileKind:    types.ChargingProfileKindRelative,
		ChargingSchedule: &types.ChargingSchedule{
			ChargingRateUnit: types.ChargingRateUnitAmperes,
			ChargingSchedulePeriod: []types.ChargingSchedulePeriod{
				period,
			},
		},
	}
}
