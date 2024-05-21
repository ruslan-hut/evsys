package internal

import "time"

type Event string

const (
	StatusNotification Event = "StatusNotification"
	TransactionStart   Event = "TransactionStart"
	TransactionStop    Event = "TransactionStop"
	Authorize          Event = "Authorize"
	TransactionEvent   Event = "TransactionEvent"
	Alert              Event = "Alert"
	Information        Event = "Info"
)

type EventHandler interface {
	OnStatusNotification(event *EventMessage)
	OnTransactionStart(event *EventMessage)
	OnTransactionStop(event *EventMessage)
	OnAuthorize(event *EventMessage)
	OnTransactionEvent(event *EventMessage)
	OnAlert(event *EventMessage)
	OnInfo(event *EventMessage)
}

type EventMessage struct {
	Type          string      `json:"type,omitempty" bson:"type"`
	ChargePointId string      `json:"charge_point_id" bson:"charge_point_id"`
	ConnectorId   int         `json:"connector_id" bson:"connector_id"`
	LocationId    string      `json:"location_id" bson:"location_id"`
	Evse          string      `json:"evse" bson:"evse"`
	Time          time.Time   `json:"time,omitempty" bson:"time"`
	Username      string      `json:"username,omitempty" bson:"username"`
	IdTag         string      `json:"id_tag,omitempty" bson:"id_tag"`
	TransactionId int         `json:"transaction_id,omitempty" bson:"transaction_id"`
	Status        string      `json:"status,omitempty" bson:"status"`
	Info          string      `json:"info,omitempty" bson:"info"`
	Payload       interface{} `json:"payload,omitempty" bson:"payload"`
}
