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
	GetUserPaymentPlan(username string) (*entity.PaymentPlan, error)
	GetPaymentMethod(userId string) (*entity.PaymentMethod, error)
	GetNotBilledTransactions() ([]*entity.Transaction, error)
}

type Affleck struct {
	database Database
	logger   internal.LogHandler
	payment  PaymentService
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
func (a *Affleck) OnTransactionStart(transaction *entity.Transaction) error {
	if a.database != nil {
		if transaction.Username == "" {
			return nil
		}
		plan, _ := a.database.GetUserPaymentPlan(transaction.Username)
		if plan != nil {
			transaction.Plan = plan
		} else {
			return fmt.Errorf("no payment plan for user %s", transaction.Username)
		}
		if transaction.UserTag != nil {
			paymentMethod, _ := a.database.GetPaymentMethod(transaction.UserTag.UserId)
			transaction.PaymentMethod = paymentMethod
		}
	}
	return nil
}

func (a *Affleck) OnTransactionFinished(transaction *entity.Transaction) error {

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
	// consumed Watts
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
