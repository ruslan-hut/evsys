package internal

import "evsys/models"

type BillingService interface {
	OnTransactionFinished(transaction *models.Transaction) error
	OnMeterValue(transaction *models.Transaction, transactionMeter *models.TransactionMeter) error
}
