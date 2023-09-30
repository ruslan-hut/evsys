package billing

import (
	"evsys/internal"
	"evsys/models"
)

type Affleck struct {
	database internal.Database
	logger   internal.LogHandler
	payment  internal.PaymentService
}

func NewAffleck() *Affleck {
	return &Affleck{}
}

func (a *Affleck) SetDatabase(database internal.Database) {
	a.database = database
}

func (a *Affleck) SetLogger(logger internal.LogHandler) {
	a.logger = logger
}

func (a *Affleck) SetPayment(payment internal.PaymentService) {
	a.payment = payment
}

func (a *Affleck) OnTransactionFinished(transaction *models.Transaction) error {

	// price in cents per hour
	pricePerHour := 100
	// price in cents per kW
	pricePerKw := 30

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

func (a *Affleck) OnMeterValue(transaction *models.Transaction, meterValue *models.TransactionMeter) error {

	// price in cents per hour
	pricePerHour := 100
	// price in cents per kW
	pricePerKw := 30

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
