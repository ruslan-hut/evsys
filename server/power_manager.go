package server

type PowerManager interface {
	OnSystemStart()
	OnChargePointBoot(chargePointId string)
	CheckPowerLimit(chargePointId string)
}
