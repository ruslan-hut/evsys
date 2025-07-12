package tariff

import "evsys/entity/common"

type Tariff struct {
	Id          string              `json:"id" bson:"id" validate:"required,max=36"`
	Currency    string              `json:"currency" bson:"currency" validate:"required,len=3"`
	AltText     *common.DisplayText `json:"tariff_alt_text,omitempty" bson:"tariff_alt_text,omitempty" validate:"omitempty"`
	AltUrl      string              `json:"tariff_alt_url,omitempty" bson:"tariff_alt_url,omitempty" validate:"omitempty,url"`
	Elements    []*Element          `json:"elements" bson:"elements" validate:"required,dive"`
	EnergyMix   *common.EnergyMix   `json:"energy_mix,omitempty" bson:"energy_mix,omitempty" validate:"omitempty"`
	LastUpdated string              `json:"last_updated,omitempty" bson:"last_updated,omitempty" validate:"omitempty,datetime=2006-01-02T15:04:05Z"`
}

type Element struct {
	PriceComponents []*PriceComponent `json:"price_components" bson:"price_components" validate:"required,dive"`
	Restrictions    *Restrictions     `json:"restrictions,omitempty" bson:"restrictions,omitempty" validate:"omitempty"`
}

func (t *Tariff) PricePerKwh() float64 {
	var total float64
	for _, element := range t.Elements {
		for _, priceComponent := range element.PriceComponents {
			if priceComponent.IsEnergy() {
				total += priceComponent.Price
			}
		}
	}
	return total
}

func Empty() *Tariff {
	return &Tariff{
		Elements: []*Element{{PriceComponents: []*PriceComponent{NewPriceEnergy()}}},
	}
}
