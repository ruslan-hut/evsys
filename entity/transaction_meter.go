package entity

import "time"

// TransactionMeter is one meter reading of a charging session.
//
// Voltage, CurrentImport and CurrentOffered are what separate a session limited
// by the load balancer from one limited by the hardware or by the car: without
// them a power figure alone cannot say which of the three was binding.
// CurrentOffered in particular is the charge point restating the limit it is
// advertising to the vehicle, so it can be compared directly against the
// amperage the balancer asked for. All three are zero on readings from charge
// points that do not report them.
type TransactionMeter struct {
	Id              int       `json:"transaction_id" bson:"transaction_id"`
	Value           int       `json:"value" bson:"value"`
	PowerRate       int       `json:"power_rate" bson:"power_rate"`
	PowerRateWh     float64   `json:"power_rate_wh" bson:"power_rate_wh"`
	PowerActive     int       `json:"power_active" bson:"power_active"`
	Voltage         float64   `json:"voltage" bson:"voltage"`
	CurrentImport   float64   `json:"current_import" bson:"current_import"`
	CurrentOffered  float64   `json:"current_offered" bson:"current_offered"`
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
