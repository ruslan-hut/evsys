package models

type ChargePointStatus struct {
	ChargePointID string `json:"charge_point_id" bson:"charge_point_id"`
	ConnectorID   int    `json:"connector_id" bson:"connector_id"`
	Status        string `json:"status" bson:"status"`
	Time          string `json:"time" bson:"time"`
}
