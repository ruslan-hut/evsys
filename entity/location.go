package entity

import "evsys/entity/common"

type Location struct {
	Id          string             `json:"id" bson:"id" validate:"required,max=39"`
	Roaming     bool               `json:"roaming" bson:"roaming" validate:"required"`
	Name        string             `json:"name,omitempty" bson:"name,omitempty" validate:"omitempty,max=255"`
	Address     string             `json:"address" bson:"address" validate:"required,max=45"`
	City        string             `json:"city" bson:"city" validate:"required,max=45"`
	PostalCode  string             `json:"postal_code" bson:"postal_code" validate:"required,max=10"`
	Country     string             `json:"country" bson:"country" validate:"required,iso3166_1_alpha3"`
	Coordinates common.GeoLocation `json:"coordinates" bson:"coordinates" validate:"required"`
	// PowerLimit is the site's rated capacity, recorded for operators to read.
	// Nothing branches on it: the load balancer assigns from its own fixed slots,
	// and whether a charge point is balanced at all is ChargePoint.SmartCharging.
	// It was once the balancer's on/off switch, which meant a location that had
	// simply never had the figure filled in ran its chargers unlimited.
	PowerLimit int `json:"power_limit" bson:"power_limit" validate:"required"`
	// DefaultPowerLimit, unlike PowerLimit, is acted on: it is the amperage of
	// the TxDefaultProfile installed on a charge point when it boots, and zero
	// means the default profile is cleared instead.
	DefaultPowerLimit int            `json:"default_power_limit" bson:"default_power_limit" validate:"required"`
	Evses             []*ChargePoint `json:"evses,omitempty" bson:"evses,omitempty" validate:"omitempty"`
}
