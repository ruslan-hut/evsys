package models

import "time"

type PaymentOrder struct {
	TransactionId int       `json:"transaction_id" bson:"transaction_id"`
	Order         int       `json:"order" bson:"order"`
	UserId        string    `json:"user_id" bson:"user_id"`
	UserName      string    `json:"user_name" bson:"user_name"`
	Amount        int       `json:"amount" bson:"amount"`
	Currency      string    `json:"currency" bson:"currency"`
	Description   string    `json:"description" bson:"description"`
	Identifier    string    `json:"identifier" bson:"identifier"`
	IsCompleted   bool      `json:"is_completed" bson:"is_completed"`
	Result        string    `json:"result" bson:"result"`
	Date          string    `json:"date" bson:"date"`
	TimeOpened    time.Time `json:"time_opened" bson:"time_opened"`
	TimeClosed    time.Time `json:"time_closed" bson:"time_closed"`
}
