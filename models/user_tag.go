package models

type UserTag struct {
	Username  string `json:"username" bson:"username"`
	UserId    string `json:"user_id" bson:"user_id"`
	IdTag     string `json:"id_tag" bson:"id_tag"`
	IsEnabled bool   `json:"is_enabled" bson:"is_enabled"`
	Note      string `json:"note" bson:"note"`
}
