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

// txProfileIdBase offsets transaction profile ids clear of the default profile,
// which uses id 1. OCPP 1.6 scopes chargingProfileId to the charge point rather
// than to the connector, so every connector needs its own id: a shared id lets
// the profile installed for one connector replace the one already installed for
// another on the same multi-connector charge point.
const txProfileIdBase = 10

// txProfileStackLevel must not exceed the charge point's reported
// ChargeProfileMaxStackLevel, or the profile is rejected outright.
const txProfileStackLevel = 10

func NewTransactionChargingProfile(connectorId, transactionId, limit int) *types.ChargingProfile {
	period := types.ChargingSchedulePeriod{
		StartPeriod: 0,
		Limit:       float64(limit),
	}
	return &types.ChargingProfile{
		ChargingProfileId:      txProfileIdBase + connectorId,
		StackLevel:             txProfileStackLevel,
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
