package handlers

type CommandHandler interface {
	TriggerMessage(chargePointId, messageType string)
}
