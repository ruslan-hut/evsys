package entity

import "time"

type PaymentPlan struct {
	PlanId       string `json:"plan_id" bson:"plan_id"`
	IsDefault    bool   `json:"is_default" bson:"is_default"` // global default, for all users
	IsActive     bool   `json:"is_active" bson:"is_active"`
	PricePerKwh  int    `json:"price_per_kwh" bson:"price_per_kwh"`
	PricePerHour int    `json:"price_per_hour" bson:"price_per_hour"`
	StartTime    string `json:"start_time" bson:"start_time"`
	EndTime      string `json:"end_time" bson:"end_time"`
}

// IsCurrentTimeRange determines if the current time falls within the specified start and end time of the PaymentPlan.
func (p *PaymentPlan) IsCurrentTimeRange() bool {
	// if start and end time are not specified, then the plan is always active
	if p.StartTime == "" && p.EndTime == "" {
		return true
	}
	// if one of the times is not specified, then the plan is not valid
	if p.StartTime == "" || p.EndTime == "" {
		return false
	}
	now := time.Now()
	current := now.Hour()*60 + now.Minute()

	start, e1 := time.Parse("15:04", p.StartTime)
	end, e2 := time.Parse("15:04", p.EndTime)
	if e1 != nil || e2 != nil {
		return false
	}

	startMinute := start.Hour()*60 + start.Minute()
	endMinute := end.Hour()*60 + end.Minute()

	if startMinute <= endMinute {
		return startMinute <= current && current <= endMinute
	}
	// range is after midnight
	return startMinute <= current || current <= endMinute
}
