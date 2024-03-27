package server

type PowerManager interface {
	OnChargePointBoot(chargePointId string)
	CheckPowerLimit(chargePointId string)
}
