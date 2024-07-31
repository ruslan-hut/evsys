package power

import "evsys/entity"

type Repository interface {
	GetChargePoint(id string) (*entity.ChargePoint, error)
	GetLocation(locationId string) (*entity.Location, error)
	GetLocations() ([]*entity.Location, error)
	UpdateConnectorCurrentPower(connector *entity.Connector) error
}
