package models

type UserSubscription struct {
	UserID           int    `json:"user_id" bson:"user_id"`
	User             string `json:"user" bson:"user"`
	SubscriptionType string `json:"subscription_type" bson:"subscription_type"`
}
