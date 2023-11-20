package internal

import "evsys/models"

type BillingService interface {
	OnTransactionStart(transaction *models.Transaction) error
	OnTransactionFinished(transaction *models.Transaction) error
	OnMeterValue(transaction *models.Transaction, transactionMeter *models.TransactionMeter) error
}
