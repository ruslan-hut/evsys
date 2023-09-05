package models

import "time"

type ChargePoint struct {
	Id               string    `json:"charge_point_id" bson:"charge_point_id"`
	IsEnabled        bool      `json:"is_enabled" bson:"is_enabled"`
	Title            string    `json:"title" bson:"title"`
	Description      string    `json:"description" bson:"description"`
	Model            string    `json:"model" bson:"model"`
	SerialNumber     string    `json:"serial_number" bson:"serial_number"`
	Vendor           string    `json:"vendor" bson:"vendor"`
	FirmwareVersion  string    `json:"firmware_version" bson:"firmware_version"`
	LocalAuthVersion int       `json:"local_auth_version" bson:"local_auth_version"`
	Status           string    `json:"status" bson:"status"`
	StatusTime       time.Time `json:"status_time" bson:"status_time"`
	Info             string    `json:"info" bson:"info"`
	Address          string    `json:"address" bson:"address"`
	Location         Location  `json:"location" bson:"location"`
	ErrorCode        string    `json:"error_code" bson:"error_code"`
	IsOnline         bool      `json:"is_online" bson:"is_online"`
	EventTime        time.Time `json:"event_time" bson:"event_time"`
}
