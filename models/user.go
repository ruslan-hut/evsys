package models

import "time"

type User struct {
	Username       string    `json:"username" bson:"username"`
	Password       string    `json:"password" bson:"password"`
	Name           string    `json:"name" bson:"name"`
	Role           string    `json:"role" bson:"role"`
	PaymentPlan    string    `json:"payment_plan" bson:"payment_plan"`
	Token          string    `json:"token" bson:"token"`
	UserId         string    `json:"user_id" bson:"user_id"`
	DateRegistered time.Time `json:"date_registered" bson:"date_registered"`
	LastSeen       time.Time `json:"last_seen" bson:"last_seen"`
}
