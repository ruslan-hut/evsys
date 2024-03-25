package power

import "evsys/models"

type Repository interface {
	GetChargePoint(id string) (*models.ChargePoint, error)
	GetLocation(locationId string) (*models.Location, error)
	UpdateConnectorCurrentPower(connector *models.Connector) error
}
