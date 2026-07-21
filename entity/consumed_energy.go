package entity

// ConsumedEnergy is one group of the today-consumed-energy aggregation: the energy in Wh
// delivered by the finished transactions of one charge point since midnight UTC.
type ConsumedEnergy struct {
	ID struct {
		Location      string `json:"location" bson:"location"`
		ChargePointID string `json:"charge_point_id" bson:"charge_point_id"`
	} `json:"_id" bson:"_id"`
	Consumed int `json:"consumed" bson:"consumed"`
}
