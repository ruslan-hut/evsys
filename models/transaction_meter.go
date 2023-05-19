package models

import "time"

type TransactionMeter struct {
	Id        int       `json:"transaction_id" bson:"transaction_id"`
	Value     int       `json:"value" bson:"value"`
	Time      time.Time `json:"time" bson:"time"`
	Unit      string    `json:"unit" bson:"unit"`
	Measurand string    `json:"measurand" bson:"measurand"`
}
