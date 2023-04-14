package remotetrigger

type SystemHandler interface {
	OnTriggerMessage(chargePointId string, connectorId int, messageTrigger string) (*TriggerMessageRequest, error)
}
