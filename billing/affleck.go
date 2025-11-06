package billing

import (
	"evsys/entity"
	"evsys/internal"
	"fmt"
)

type PaymentService interface {
	TransactionPayment(transaction *entity.Transaction)
}

type Database interface {
	GetDefaultPaymentPlan() (*entity.PaymentPlan, error)
	GetUserPaymentPlan(username string) (*entity.PaymentPlan, error)
	GetPaymentMethod(userId string) (*entity.PaymentMethod, error)
	GetNotBilledTransactions() ([]*entity.Transaction, error)
}

type Affleck struct {
	database    Database
	logger      internal.LogHandler
	payment     PaymentService
	defaultPlan *entity.PaymentPlan
}

func NewAffleck() *Affleck {
	return &Affleck{}
}

func (a *Affleck) SetDatabase(database Database) {
	a.database = database
}

func (a *Affleck) SetLogger(logger internal.LogHandler) {
	a.logger = logger
}

func (a *Affleck) SetPayment(payment PaymentService) {
	a.payment = payment
}

// OnTransactionStart set payment plan for transaction
// Works with both OCPP 1.6J and 2.0.1 transactions
func (a *Affleck) OnTransactionStart(transaction *entity.Transaction) error {
	return a.choosePaymentPlan(transaction)
}

// OnTransactionFinished calculates final payment amount for transaction
// Handles both OCPP 1.6J and 2.0.1 protocol versions
func (a *Affleck) OnTransactionFinished(transaction *entity.Transaction) error {
	if transaction.Plan == nil {
		return nil
	}

	// price in cents per hour
	pricePerHour := transaction.Plan.PricePerHour
	// price in cents per kW
	pricePerKw := transaction.Plan.PricePerKwh

	// consumed minutes, minus 1 hour for the first hour
	duration := transaction.TimeStop.Sub(transaction.TimeStart).Minutes() - 60
	// consumed Watts
	consumed := transaction.MeterStop - transaction.MeterStart

	price := 0
	if duration > 0 {
		price = int(duration) * pricePerHour / 60
	}
	if consumed > 0 {
		price = price + consumed*pricePerKw/1000
	}

	transaction.PaymentAmount = price

	return nil
}

// OnMeterValue calculates running price for a meter value sample
// Handles both OCPP 1.6J and 2.0.1 meter value formats
func (a *Affleck) OnMeterValue(transaction *entity.Transaction, meterValue *entity.TransactionMeter) error {
	if transaction.Plan == nil {
		return nil
	}

	// price in cents per hour
	pricePerHour := transaction.Plan.PricePerHour
	// price in cents per kW
	pricePerKw := transaction.Plan.PricePerKwh

	// consumed minutes, minus 1 hour for the first hour
	duration := meterValue.Time.Sub(transaction.TimeStart).Minutes() - 60
	// consumed Watts (meter value format is consistent across versions after adapter conversion)
	consumed := meterValue.Value - transaction.MeterStart

	price := 0
	if duration > 0 {
		price = int(duration) * pricePerHour / 60
	}
	if consumed > 0 {
		price = price + consumed*pricePerKw/1000
	}

	meterValue.Price = price
	return nil
}

func (a *Affleck) choosePaymentPlan(transaction *entity.Transaction) error {
	if a.database == nil {
		return nil
	}
	if transaction == nil {
		return nil
	}
	if transaction.Username == "" {
		return nil
	}
	plan, _ := a.database.GetUserPaymentPlan(transaction.Username)
	if plan != nil && plan.IsCurrentTimeRange() {
		transaction.Plan = plan
		return nil
	}
	if a.defaultPlan == nil {
		a.defaultPlan, _ = a.database.GetDefaultPaymentPlan()
	}
	transaction.Plan = a.defaultPlan
	if transaction.Plan == nil {
		return fmt.Errorf("no payment plan for %s", transaction.Username)
	}
	return nil
}
