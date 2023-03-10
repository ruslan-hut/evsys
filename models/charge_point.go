package models

type ChargePoint struct {
	Id              string `json:"charge_point_id" bson:"charge_point_id"`
	IsEnabled       bool   `json:"is_enabled" bson:"is_enabled"`
	Title           string `json:"title" bson:"title"`
	Description     string `json:"description" bson:"description"`
	Model           string `json:"model" bson:"model"`
	SerialNumber    string `json:"serial_number" bson:"serial_number"`
	Vendor          string `json:"vendor" bson:"vendor"`
	FirmwareVersion string `json:"firmware_version" bson:"firmware_version"`
	Status          string `json:"status" bson:"status"`
	ErrorCode       string `json:"error_code" bson:"error_code"`
}
