package models

type UserTag struct {
	Username  string `json:"username" bson:"username"`
	IdTag     string `json:"id_tag" bson:"id_tag"`
	IsEnabled bool   `json:"is_enabled" bson:"is_enabled"`
}