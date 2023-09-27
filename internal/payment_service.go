package internal

import "evsys/models"

type PaymentService interface {
	TransactionPayment(transaction *models.Transaction)
}
