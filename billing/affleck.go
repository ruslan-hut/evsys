package billing

import (
	"evsys/internal"
	"evsys/models"
)

type Affleck struct {
	database internal.Database
	logger   internal.LogHandler
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

func (a *Affleck) OnTransactionFinished(transaction *models.Transaction) error {

	// price in cents per kW
	pricePerKw := 30
	// consumed Watts
	consumed := transaction.MeterStop - transaction.MeterStart

	if consumed > 0 {
		// total price in cents
		price := consumed * pricePerKw / 1000
		transaction.PaymentAmount = price
	}

	return nil
}

func (a *Affleck) OnMeterValue(transaction *models.Transaction, meterValue *models.TransactionMeter) error {

	// price in cents per kW
	pricePerKw := 30
	// consumed Watts
	consumed := meterValue.Value - transaction.MeterStart

	if consumed > 0 {
		// total price in cents
		price := consumed * pricePerKw / 1000
		meterValue.Price = price
	}

	return nil
}
