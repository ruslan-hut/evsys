package tariff

type Restrictions struct {
	StartTime   string  `json:"start_time,omitempty" bson:"start_time,omitempty" validate:"omitempty,datetime=15:04"`
	EndTime     string  `json:"end_time,omitempty" bson:"end_time,omitempty" validate:"omitempty,datetime=15:04"`
	StartDate   string  `json:"start_date,omitempty" bson:"start_date,omitempty" validate:"omitempty,date=2006-01-02"`
	EndDate     string  `json:"end_date,omitempty" bson:"end_date,omitempty" validate:"omitempty,date=2006-01-02"`
	MinKwh      float64 `json:"min_kwh,omitempty" bson:"min_kwh,omitempty" validate:"omitempty"`
	MaxKwh      float64 `json:"max_kwh,omitempty" bson:"max_kwh,omitempty" validate:"omitempty"`
	MinPower    float64 `json:"min_power,omitempty" bson:"min_power,omitempty" validate:"omitempty"`
	MaxPower    float64 `json:"max_power,omitempty" bson:"max_power,omitempty" validate:"omitempty"`
	MinDuration int     `json:"min_duration,omitempty" bson:"min_duration,omitempty" validate:"omitempty"`
	MaxDuration int     `json:"max_duration,omitempty" bson:"max_duration,omitempty" validate:"omitempty"`
	DayOfWeek   string  `json:"day_of_week,omitempty" bson:"day_of_week,omitempty" validate:"omitempty,oneof=MONDAY TUESDAY WEDNESDAY THURSDAY FRIDAY SATURDAY SUNDAY"`
}
