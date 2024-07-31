package entity

type PaymentPlan struct {
	PlanId       string `json:"plan_id" bson:"plan_id"`
	IsDefault    bool   `json:"is_default" bson:"is_default"`
	IsActive     bool   `json:"is_active" bson:"is_active"`
	PricePerKwh  int    `json:"price_per_kwh" bson:"price_per_kwh"`
	PricePerHour int    `json:"price_per_hour" bson:"price_per_hour"`
}
