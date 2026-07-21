package entity

// ConsumedEnergy is one group of the today-consumed-energy aggregation: the transactions of one
// charge point finished since midnight UTC, with the energy in Wh they delivered. Count and
// Consumed come from the same rows, which keeps the transaction and energy metrics describing
// the same set of sessions.
type ConsumedEnergy struct {
	ID struct {
		Location      string `json:"location" bson:"location"`
		ChargePointID string `json:"charge_point_id" bson:"charge_point_id"`
	} `json:"_id" bson:"_id"`
	Consumed int `json:"consumed" bson:"consumed"`
	Count    int `json:"count" bson:"count"`
}
