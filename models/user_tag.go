package models

import "time"

type UserTag struct {
	Username       string    `json:"username" bson:"username"`
	UserId         string    `json:"user_id" bson:"user_id"`
	IdTag          string    `json:"id_tag" bson:"id_tag"`
	IsEnabled      bool      `json:"is_enabled" bson:"is_enabled"`
	Note           string    `json:"note" bson:"note"`
	DateRegistered time.Time `json:"date_registered" bson:"date_registered"`
	LastSeen       time.Time `json:"last_seen" bson:"last_seen"`
}
