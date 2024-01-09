package models

import "time"

type UserTag struct {
	Username       string    `json:"username" bson:"username"`
	UserId         string    `json:"user_id" bson:"user_id"`
	IdTag          string    `json:"id_tag" bson:"id_tag"`
	Source         string    `json:"source" bson:"source"`
	IsEnabled      bool      `json:"is_enabled" bson:"is_enabled"`
	Local          bool      `json:"local" bson:"local"`
	Note           string    `json:"note" bson:"note"`
	DateRegistered time.Time `json:"date_registered" bson:"date_registered"`
	LastSeen       time.Time `json:"last_seen" bson:"last_seen"`
}
