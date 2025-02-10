package entity

// ErrorCounter helper struct to obtain aggregation of error counts
type ErrorCounter struct {
	Location      string `json:"location" bson:"_id.location"`
	ChargePointID string `json:"charge_point_id" bson:"_id.charge_point_id"`
	ErrorCode     string `json:"error_code" bson:"_id.error_code"`
	Count         int    `json:"count" bson:"count"`
}
