package entity

import "time"

type TransactionMeter struct {
	Id              int       `json:"transaction_id" bson:"transaction_id"`
	Value           int       `json:"value" bson:"value"`
	PowerRate       int       `json:"power_rate" bson:"power_rate"`
	PowerRateWh     float64   `json:"power_rate_wh" bson:"power_rate_wh"`
	PowerActive     int       `json:"power_active" bson:"power_active"`
	Price           int       `json:"price" bson:"price"`
	BatteryLevel    int       `json:"battery_level" bson:"battery_level"`
	ConsumedEnergy  int       `json:"consumed_energy" bson:"consumed_energy"`
	Time            time.Time `json:"time" bson:"time"`
	Minute          int64     `json:"minute" bson:"minute"`
	Unit            string    `json:"unit" bson:"unit"`
	Measurand       string    `json:"measurand" bson:"measurand"`
	ConnectorId     int       `json:"connector_id" bson:"connector_id"`
	ConnectorStatus string    `json:"connector_status" bson:"connector_status"`
}

func NewMeter(id, connectorId int, status string, timestamp time.Time) *TransactionMeter {
	return &TransactionMeter{
		Id:              id,
		ConnectorId:     connectorId,
		ConnectorStatus: status,
		Time:            timestamp,
		Minute:          timestamp.Unix() / 60,
	}
}
