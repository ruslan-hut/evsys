package power

import "evsys/entity"

type Repository interface {
	GetChargePoint(id string) (*entity.ChargePoint, error)
	GetLocation(locationId string) (*entity.Location, error)
	GetLocations() ([]*entity.Location, error)
	UpdateConnectorCurrentPower(connector *entity.Connector) error
	UpdateTransactionPowerLimit(transactionId, limit int) error
	// UpdateConnectorProfileVerdict is keyed by identity rather than taking a
	// connector: the charge point's answer arrives on a goroutine that no longer
	// owns the connector the profile was built from.
	UpdateConnectorProfileVerdict(chargePointId string, connectorId int, verdict *entity.ProfileVerdict) error
}
