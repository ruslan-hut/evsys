package internal

import "evsys/models"

type Database interface {
	Write(table string, data Data) error
	WriteLogMessage(data Data) error
	ReadLog() (interface{}, error)

	GetChargePoints() ([]models.ChargePoint, error)
	UpdateChargePoint(chargePoint *models.ChargePoint) error
	AddChargePoint(chargePoint *models.ChargePoint) error
	GetChargePoint(id string) (*models.ChargePoint, error)

	GetConnectors() ([]models.Connector, error)
	UpdateConnector(connector *models.Connector) error
	AddConnector(connector *models.Connector) error
	GetConnector(id int, chargePointId string) (*models.Connector, error)

	GetUserTag(idTag string) (*models.UserTag, error)
	AddUserTag(userTag *models.UserTag) error
}

type Data interface {
	DataType() string
}
