package entity

import (
	"time"
)

type ErrorData struct {
	Location        string    `json:"location" bson:"location"`
	ChargePointID   string    `json:"charge_point_id" bson:"charge_point_id"`
	ConnectorID     int       `json:"connector_id" bson:"connector_id"`
	ErrorCode       string    `json:"error_code" bson:"error_code"`
	Info            string    `json:"info,omitempty" bson:"info"`
	Status          string    `json:"status" bson:"status"`
	Timestamp       time.Time `json:"timestamp,omitempty" bson:"timestamp"`
	VendorId        string    `json:"vendor_id,omitempty" bson:"vendor_id"`
	VendorErrorCode string    `json:"vendor_error_code,omitempty" bson:"vendor_error_code"`
}
