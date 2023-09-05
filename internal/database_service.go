package internal

import "evsys/models"

type Database interface {
	Write(table string, data Data) error
	WriteLogMessage(data Data) error
	ReadLog() (interface{}, error)
	GetLastStatus() ([]models.ChargePointStatus, error)

	GetChargePoints() ([]models.ChargePoint, error)
	UpdateChargePoint(chargePoint *models.ChargePoint) error
	UpdateChargePointStatus(chargePoint *models.ChargePoint) error
	UpdateOnlineStatus(chargePointId string, isOnline bool) error
	AddChargePoint(chargePoint *models.ChargePoint) error
	GetChargePoint(id string) (*models.ChargePoint, error)

	GetConnectors() ([]*models.Connector, error)
	UpdateConnector(connector *models.Connector) error
	AddConnector(connector *models.Connector) error
	GetConnector(id int, chargePointId string) (*models.Connector, error)

	GetUserTag(idTag string) (*models.UserTag, error)
	AddUserTag(userTag *models.UserTag) error
	GetActiveUserTags(chargePointId string, listVersion int) ([]models.UserTag, error)

	GetLastTransaction() (*models.Transaction, error)
	GetTransaction(id int) (*models.Transaction, error)
	AddTransaction(transaction *models.Transaction) error
	UpdateTransaction(transaction *models.Transaction) error

	AddTransactionMeterValue(meterValue *models.TransactionMeter) error
	DeleteTransactionMeterValues(transactionId int) error

	GetSubscriptions() ([]models.UserSubscription, error)
	AddSubscription(subscription *models.UserSubscription) error
	UpdateSubscription(subscription *models.UserSubscription) error
	DeleteSubscription(subscription *models.UserSubscription) error
}

type Data interface {
	DataType() string
}
