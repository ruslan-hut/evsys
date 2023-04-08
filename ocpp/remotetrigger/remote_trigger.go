package remotetrigger

type SystemHandler interface {
	OnTriggerMessage(chargePointId, messageTrigger string) (*TriggerMessageRequest, error)
}
