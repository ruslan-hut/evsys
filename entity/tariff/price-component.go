package tariff

type DimensionType string

const (
	Energy DimensionType = "ENERGY"
	//Flat        DimensionType = "FLAT"
	//ParkingTime DimensionType = "PARKING_TIME"
	//Time        DimensionType = "TIME"
)

type PriceComponent struct {
	Type     DimensionType `json:"type" bson:"type" validate:"required,oneof=ENERGY FLAT PARKING_TIME TIME"`
	Price    float64       `json:"price" bson:"price" validate:"required"`
	StepSize int           `json:"step_size" bson:"step_size" validate:"required"`
}

func (p *PriceComponent) IsEnergy() bool {
	return p.Type == Energy
}

func NewPriceEnergy() *PriceComponent {
	return &PriceComponent{
		Type: Energy,
	}
}
