package models

import "time"

type ChargePointStatus struct {
	ChargePointID string            `json:"charge_point_id" bson:"charge_point_id"`
	ConnectorID   int               `json:"connector_id" bson:"connector_id"`
	Status        string            `json:"status" bson:"status"`
	StatusTime    time.Time         `json:"status_time" bson:"status_time"`
	IsOnline      bool              `json:"is_online" bson:"is_online"`
	EventTime     time.Time         `json:"event_time" bson:"event_time"`
	Time          string            `json:"time" bson:"time"`
	TransactionId int               `json:"transaction_id" bson:"transaction_id"`
	Connectors    []ConnectorStatus `json:"connectors" bson:"connectors"`
}

type ConnectorStatus struct {
	ConnectorID   int       `json:"connector_id" bson:"connector_id"`
	Status        string    `json:"status" bson:"status"`
	StatusTime    time.Time `json:"status_time" bson:"status_time"`
	Time          string    `json:"time" bson:"time"`
	TransactionId int       `json:"current_transaction_id" bson:"current_transaction_id"`
	Info          string    `json:"info" bson:"info"`
}
