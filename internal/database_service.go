package internal

import "evsys/models"

type Database interface {
	Write(table string, data Data) error
	WriteLogMessage(data Data) error
	ReadLog() (interface{}, error)
	GetChargePoints() ([]models.ChargePoint, error)
	GetConnectors() ([]models.Connector, error)
	UpdateChargePoint(chargePoint *models.ChargePoint) error
	UpdateConnector(connector *models.Connector) error
}

type Data interface {
	DataType() string
}
