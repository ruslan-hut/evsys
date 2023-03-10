package models

type Connector struct {
	Id            int    `json:"connector_id" bson:"connector_id"`
	ChargePointId string `json:"charge_point_id" bson:"charge_point_id"`
	IsEnabled     bool   `json:"is_enabled" bson:"is_enabled"`
	Status        string `json:"status" bson:"status"`
}
