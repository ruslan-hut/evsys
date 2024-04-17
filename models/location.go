package models

type Location struct {
	Id                string         `json:"id" bson:"id" validate:"required,max=39"`
	Roaming           bool           `json:"roaming" bson:"roaming" validate:"required"`
	Name              string         `json:"name,omitempty" bson:"name,omitempty" validate:"omitempty,max=255"`
	Address           string         `json:"address" bson:"address" validate:"required,max=45"`
	City              string         `json:"city" bson:"city" validate:"required,max=45"`
	PostalCode        string         `json:"postal_code" bson:"postal_code" validate:"required,max=10"`
	Country           string         `json:"country" bson:"country" validate:"required,iso3166_1_alpha3"`
	Coordinates       GeoLocation    `json:"coordinates" bson:"coordinates" validate:"required"`
	PowerLimit        int            `json:"power_limit" bson:"power_limit" validate:"required"`
	DefaultPowerLimit int            `json:"default_power_limit" bson:"default_power_limit" validate:"required"`
	Evses             []*ChargePoint `json:"evses,omitempty" bson:"evses,omitempty" validate:"omitempty"`
}
