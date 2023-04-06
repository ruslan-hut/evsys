package internal

import "time"

type EventHandler interface {
	OnStatusNotification(event *EventMessage)
	OnTransactionStart(event *EventMessage)
	OnTransactionStop(event *EventMessage)
	OnAuthorize(event *EventMessage)
}

type EventMessage struct {
	Type          string      `json:"type" bson:"type"`
	ChargePointId string      `json:"charge_point_id" bson:"charge_point_id"`
	ConnectorId   int         `json:"connector_id" bson:"connector_id"`
	Time          time.Time   `json:"time" bson:"time"`
	Username      string      `json:"username" bson:"username"`
	IdTag         string      `json:"id_tag" bson:"id_tag"`
	TransactionId int         `json:"transaction_id" bson:"transaction_id"`
	Status        string      `json:"status" bson:"status"`
	Info          string      `json:"info" bson:"info"`
	Payload       interface{} `json:"payload" bson:"payload"`
}
