package common

// EnergyMix This type is used to specify the energy mix and environmental impact of the supplied energy at a location or in a tariff.
type EnergyMix struct {
	IsGreenEnergy       bool                 `json:"is_green_energy" bson:"is_green_energy" validate:"required"`
	EnergySources       []*EnergySource      `json:"energy_sources,omitempty" bson:"energy_sources,omitempty" validate:"omitempty,dive"`
	EnvironmentalImpact *EnvironmentalImpact `json:"environ_impact,omitempty" bson:"environ_impact,omitempty" validate:"omitempty"`
	SupplierName        string               `json:"supplier_name,omitempty" bson:"supplier_name,omitempty" validate:"omitempty,max=64"`
	EnergyProductName   string               `json:"energy_product_name,omitempty" bson:"energy_product_name,omitempty" validate:"omitempty,max=64"`
}

// EnergySource Key-value pairs (enum + percentage) of energy sources. All given values should add up to 100 percent per category.
type EnergySource struct {
	Source     EnergySourceCategory `json:"source" bson:"source" validate:"required"`
	Percentage int                  `json:"percentage" bson:"percentage" validate:"required,min=0,max=100"`
}

// EnergySourceCategory Categories of energy sources.
// NUCLEAR	Nuclear power sources.
// GENERAL_FOSSIL	All kinds of fossil power sources.
// COAL	Fossil power from coal.
// GAS	Fossil power from gas.
// GENERAL_GREEN	All kinds of regenerative power sources.
// SOLAR	Regenerative power from PV.
// WIND	Regenerative power from wind turbines.
// WATER	Regenerative power from water turbines.
type EnergySourceCategory string

const (
	NUCLEAR        EnergySourceCategory = "NUCLEAR"
	GENERAL_FOSSIL EnergySourceCategory = "GENERAL_FOSSIL"
	COAL           EnergySourceCategory = "COAL"
	GAS            EnergySourceCategory = "GAS"
	GENERAL_GREEN  EnergySourceCategory = "GENERAL_GREEN"
	SOLAR          EnergySourceCategory = "SOLAR"
	WIND           EnergySourceCategory = "WIND"
	WATER          EnergySourceCategory = "WATER"
)

// EnvironmentalImpact Key-value pairs (enum + amount) of waste and carbon dioxide emission in g/kWh.
type EnvironmentalImpact struct {
	Category EnvironmentalImpactCategory `json:"category" bson:"category" validate:"required"`
	Amount   int                         `json:"amount" bson:"amount" validate:"required,min=0"`
}

// EnvironmentalImpactCategory Categories of environmental impact values.
// NUCLEAR_WASTE	Produced nuclear waste in grams per kilowatt-hour.
// CARBON_DIOXIDE	Exhausted carbon dioxide in grams per kilowatt-hour.
type EnvironmentalImpactCategory string

const (
	NUCLEAR_WASTE  EnvironmentalImpactCategory = "NUCLEAR_WASTE"
	CARBON_DIOXIDE EnvironmentalImpactCategory = "CARBON_DIOXIDE"
)
