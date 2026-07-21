package server

import (
	"testing"

	"evsys/entity"
	"evsys/internal"

	"github.com/prometheus/client_golang/prometheus"
)

type consumedStubDB struct {
	internal.Database
	result []*entity.ConsumedEnergy
}

func (s *consumedStubDB) GetTodayConsumedEnergy() ([]*entity.ConsumedEnergy, error) {
	return s.result, nil
}

func consumedGroup(location, chargePointId string, consumed int) *entity.ConsumedEnergy {
	c := &entity.ConsumedEnergy{Consumed: consumed}
	c.ID.Location = location
	c.ID.ChargePointID = chargePointId
	return c
}

// gaugeValue reads a gauge metric for one label set from the default prometheus registry.
// A series that was never set does not exist yet and reads as 0.
func gaugeValue(t *testing.T, name string, wantLabels map[string]string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
	metric:
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, pair := range metric.GetLabel() {
				labels[pair.GetName()] = pair.GetValue()
			}
			for k, v := range wantLabels {
				if labels[k] != v {
					continue metric
				}
			}
			return metric.GetGauge().GetValue()
		}
	}
	return 0
}

/*
TestObserveConsumedPowerZeroesDroppedSeries pins the midnight behaviour of ocpp_consumed_power.
The aggregation only returns groups with transactions finished today, so right after midnight a
charge point that charged yesterday is simply absent from the result - and a gauge that is not
written keeps its old value. The refresh has to zero the series it stops receiving, or every
morning starts with yesterday's totals on the graph.
*/
func TestObserveConsumedPowerZeroesDroppedSeries(t *testing.T) {
	db := &consumedStubDB{result: []*entity.ConsumedEnergy{
		consumedGroup("loc-cons", "CP1", 33975),
		consumedGroup("loc-cons", "CP2", 1200),
	}}
	h := &SystemHandler{database: db, logger: stopStubLogger{}}

	h.observeConsumedPower()

	if got := gaugeValue(t, "ocpp_consumed_power", map[string]string{"location": "loc-cons", "charge_point_id": "CP1"}); got != 33975 {
		t.Errorf("CP1 gauge = %v, want 33975", got)
	}
	if got := gaugeValue(t, "ocpp_consumed_power", map[string]string{"location": "loc-cons", "charge_point_id": "CP2"}); got != 1200 {
		t.Errorf("CP2 gauge = %v, want 1200", got)
	}

	// the day rolls over: CP2 has no transactions finished today
	db.result = []*entity.ConsumedEnergy{consumedGroup("loc-cons", "CP1", 500)}
	h.observeConsumedPower()

	if got := gaugeValue(t, "ocpp_consumed_power", map[string]string{"location": "loc-cons", "charge_point_id": "CP1"}); got != 500 {
		t.Errorf("CP1 gauge after rollover = %v, want 500", got)
	}
	if got := gaugeValue(t, "ocpp_consumed_power", map[string]string{"location": "loc-cons", "charge_point_id": "CP2"}); got != 0 {
		t.Errorf("CP2 gauge should be zeroed once it drops out of the aggregation, still reads %v", got)
	}
}
