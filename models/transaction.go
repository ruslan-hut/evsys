package models

type Transaction struct {
	Id            string `json:"transaction_id" bson:"transaction_id"`
	ConnectorId   string `json:"connector_id" bson:"connector_id"`
	ChargePointId string `json:"charge_point_id" bson:"charge_point_id"`
	IdTag         string `json:"id_tag" bson:"id_tag"`
	ReservationId int    `json:"reservation_id" bson:"reservation_id"`
	MeterStart    int    `json:"meter_start" bson:"meter_start"`
	MeterStop     int    `json:"meter_stop" bson:"meter_stop"`
	TimeStart     string `json:"time_start" bson:"time_start"`
	TimeStop      string `json:"time_stop" bson:"time_stop"`
	Reason        string `json:"reason" bson:"reason"`
}
