package internal

import (
	"evsys/entity"
	"evsys/ocpp/core"
)

type Database interface {
	Write(table string, data Data) error
	WriteLogMessage(data Data) error
	ReadLog() (interface{}, error)
	GetLastStatus() ([]entity.ChargePointStatus, error)
	OnlineCounter() (map[string]int, error)

	GetChargePoints() ([]*entity.ChargePoint, error)
	UpdateChargePoint(chargePoint *entity.ChargePoint) error
	UpdateChargePointStatus(chargePoint *entity.ChargePoint) error
	UpdateOnlineStatus(chargePointId string, isOnline bool) error
	ResetOnlineStatus() error
	AddChargePoint(chargePoint *entity.ChargePoint) error
	GetChargePoint(id string) (*entity.ChargePoint, error)

	GetConnectors() ([]*entity.Connector, error)
	UpdateConnector(connector *entity.Connector) error
	AddConnector(connector *entity.Connector) error
	GetConnector(id int, chargePointId string) (*entity.Connector, error)

	GetUserTag(idTag string) (*entity.UserTag, error)
	AddUserTag(userTag *entity.UserTag) error
	UpdateTag(userTag *entity.UserTag) error
	UpdateTagLastSeen(userTag *entity.UserTag) error
	GetActiveUserTags(chargePointId string, listVersion int) ([]entity.UserTag, error)

	GetPaymentMethod(userId string) (*entity.PaymentMethod, error)
	GetUserPaymentPlan(username string) (*entity.PaymentPlan, error)

	GetPaymentOrderByTransaction(transactionId int) (*entity.PaymentOrder, error)
	GetLastOrder() (*entity.PaymentOrder, error)
	SavePaymentOrder(order *entity.PaymentOrder) error

	GetLastTransaction() (*entity.Transaction, error)
	GetTransaction(id int) (*entity.Transaction, error)
	AddTransaction(transaction *entity.Transaction) error
	UpdateTransaction(transaction *entity.Transaction) error
	GetUnfinishedTransactions() ([]*entity.Transaction, error)
	SaveStopTransactionRequest(stopTransaction *core.StopTransactionRequest) error

	AddTransactionMeterValue(meterValue *entity.TransactionMeter) error
	AddSampleMeterValue(meterValue *entity.TransactionMeter) error
	ReadTransactionMeterValue(transactionId int) (*entity.TransactionMeter, error)
	ReadAllTransactionMeterValues(transactionId int) ([]entity.TransactionMeter, error)
	DeleteTransactionMeterValues(transactionId int) error
	ReadLastMeterValues() ([]*entity.TransactionMeter, error)

	GetSubscriptions() ([]entity.UserSubscription, error)
	AddSubscription(subscription *entity.UserSubscription) error
	UpdateSubscription(subscription *entity.UserSubscription) error
	DeleteSubscription(subscription *entity.UserSubscription) error
}

type Data interface {
	DataType() string
}
