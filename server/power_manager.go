package server

type PowerManager interface {
	OnChargePointBoot(chargePointId string) error
	BeforeNewTransaction(chargePointId string) error
	CheckPowerLimit(chargePointId string) error
}
