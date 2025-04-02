package entity

// ErrorCounter helper struct to obtain aggregation of error counts
type ErrorCounter struct {
	ID struct {
		Location      string `json:"location" bson:"location"`
		ChargePointID string `json:"charge_point_id" bson:"charge_point_id"`
		ErrorCode     string `json:"error_code" bson:"error_code"`
	} `json:"_id" bson:"_id"`
	Count int `json:"count" bson:"count"`
}
