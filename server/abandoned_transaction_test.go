package server

import (
	"strings"
	"sync"
	"testing"
	"time"

	"evsys/entity"
	"evsys/internal"
	"evsys/metrics/counters"

	"github.com/prometheus/client_golang/prometheus"
)

// capturingLogger records Warn messages so a test can assert on them. Warn is called synchronously
// inside finishAbandonedTransaction, so no waiting is needed; the mutex only guards -race.
type capturingLogger struct {
	mu    sync.Mutex
	warns []string
}

func (l *capturingLogger) FeatureEvent(_, _, _ string) {}
func (l *capturingLogger) RawDataEvent(_, _ string)    {}
func (l *capturingLogger) Debug(_ string)              {}
func (l *capturingLogger) Error(_ string, _ error)     {}
func (l *capturingLogger) Warn(m string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warns = append(l.warns, m)
}
func (l *capturingLogger) has(substr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, w := range l.warns {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

// abandonedStubDB stubs what finishAbandonedTransaction touches. The embedded interface is nil, so
// any other call panics. The samples disappear on DeleteTransactionMeterValues, the same way they
// do in the real collection - a read placed after the delete would come back empty.
type abandonedStubDB struct {
	internal.Database
	samples []entity.TransactionMeter
	// savedMeterValues captures transaction.MeterValues at UpdateTransaction time, which is what
	// actually lands in the document
	savedMeterValues []entity.TransactionMeter
	deleted          bool
}

func (s *abandonedStubDB) ReadTransactionMeterValue(int) (*entity.TransactionMeter, error) {
	if len(s.samples) == 0 {
		return nil, nil
	}
	last := s.samples[len(s.samples)-1]
	return &last, nil
}

func (s *abandonedStubDB) ReadAllTransactionMeterValues(int) ([]entity.TransactionMeter, error) {
	return s.samples, nil
}

func (s *abandonedStubDB) UpdateTransaction(t *entity.Transaction) error {
	s.savedMeterValues = t.MeterValues
	return nil
}

func (s *abandonedStubDB) DeleteTransactionMeterValues(int) error {
	s.deleted = true
	s.samples = nil
	return nil
}

func (s *abandonedStubDB) UpdateConnector(*entity.Connector) error { return nil }

/*
TestFinishAbandonedTransactionKeepsMeterValues pins the meter value samples onto the transaction
the sweeper closes. The sweep path used to read only the last sample and then delete the whole
set, so every system-stopped transaction lost its consumption curve permanently, while a normal
OnStopTransaction stores it in the document before deleting.
*/
func TestFinishAbandonedTransactionKeepsMeterValues(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	db := &abandonedStubDB{
		samples: []entity.TransactionMeter{
			{Id: 1, Value: 100, Time: now.Add(-40 * time.Minute)},
			{Id: 1, Value: 200, Time: now.Add(-30 * time.Minute)},
			{Id: 1, Value: 300, Time: now.Add(-25 * time.Minute)},
		},
	}
	h := newStopHandler(t, db, 1)

	transaction := &entity.Transaction{
		Id: 1, ConnectorId: 1, ChargePointId: "CP1",
		MeterStart: 0, TimeStart: now.Add(-time.Hour),
	}
	h.finishAbandonedTransaction(transaction)

	if !transaction.IsFinished || transaction.Reason != "stopped by system" {
		t.Errorf("transaction should be finished as stopped by system, got finished=%v reason=%q",
			transaction.IsFinished, transaction.Reason)
	}
	if transaction.MeterStop != 300 || !transaction.TimeStop.Equal(now.Add(-25*time.Minute)) {
		t.Errorf("meter stop should come from the newest sample, got value=%d time=%v",
			transaction.MeterStop, transaction.TimeStop)
	}
	if len(db.savedMeterValues) != 3 {
		t.Errorf("all %d samples should be saved on the transaction before deletion, got %d",
			3, len(db.savedMeterValues))
	}
	if !db.deleted {
		t.Error("meter value samples should be deleted once the transaction is saved")
	}
}

// TestFinishAbandonedTransactionTracesActiveSession asserts the trace fires when the sweep closes a
// transaction whose connector still reports an active charging status: the session is likely live on
// the charger and its connector is being freed underneath a car that is still drawing power.
func TestFinishAbandonedTransactionTracesActiveSession(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	for _, tc := range []struct {
		status    string
		wantTrace bool
	}{
		{"Charging", true},
		{"SuspendedEV", true},
		{"SuspendedEVSE", true},
		{"Available", false},
		{"Finishing", false},
		{"Preparing", false},
	} {
		t.Run(tc.status, func(t *testing.T) {
			db := &abandonedStubDB{}
			h := newStopHandler(t, db, 1)
			logger := &capturingLogger{}
			h.logger = logger
			h.chargePoints["CP1"].connectors[1].Status = tc.status

			h.finishAbandonedTransaction(&entity.Transaction{
				Id: 1, ConnectorId: 1, ChargePointId: "CP1",
				TimeStart: now.Add(-time.Hour),
			})

			got := logger.has("still reports " + tc.status)
			if got != tc.wantTrace {
				t.Errorf("status %s: trace logged = %v, want %v", tc.status, got, tc.wantTrace)
			}
		})
	}
}

func TestIsActiveChargingStatus(t *testing.T) {
	active := []string{"Charging", "SuspendedEV", "SuspendedEVSE"}
	inactive := []string{"Available", "Preparing", "Finishing", "Faulted", "Unavailable", "Reserved", ""}
	for _, s := range active {
		if !isActiveChargingStatus(s) {
			t.Errorf("isActiveChargingStatus(%q) = false, want true", s)
		}
	}
	for _, s := range inactive {
		if isActiveChargingStatus(s) {
			t.Errorf("isActiveChargingStatus(%q) = true, want false", s)
		}
	}
}

// powerRateValue reads the ocpp_current_power_rate gauge for one label set from the default
// prometheus registry, which is where promauto registers it.
func powerRateValue(t *testing.T, location, chargePointId, connectorId string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != "ocpp_current_power_rate" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, pair := range metric.GetLabel() {
				labels[pair.GetName()] = pair.GetValue()
			}
			if labels["location"] == location && labels["charge_point_id"] == chargePointId && labels["connector_id"] == connectorId {
				return metric.GetGauge().GetValue()
			}
		}
	}
	t.Fatal("ocpp_current_power_rate metric not found")
	return 0
}

// counterValue reads a counter metric for one label set from the default prometheus registry.
// A series that has never been incremented does not exist yet and reads as 0.
func counterValue(t *testing.T, name string, wantLabels map[string]string) float64 {
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
			return metric.GetCounter().GetValue()
		}
	}
	return 0
}

/*
TestFinishAbandonedTransactionCountsConsumedEnergy pins the consumed-energy counter. A session
closed by the sweep never reaches OnStopTransaction, the only other place this counter grows, so
a charge point that went silent would have its delivered energy missing from the
ocpp_consumed_today series.
*/
func TestFinishAbandonedTransactionCountsConsumedEnergy(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	labels := map[string]string{"location": "loc-counted", "charge_point_id": "CP1"}
	db := &abandonedStubDB{
		samples: []entity.TransactionMeter{{Id: 1, Value: 33975, Time: now.Add(-25 * time.Minute)}},
	}
	h := newStopHandler(t, db, 1)
	h.chargePoints["CP1"].model.LocationId = "loc-counted"

	consumedBefore := counterValue(t, "ocpp_consumed_today", labels)

	h.finishAbandonedTransaction(&entity.Transaction{
		Id: 1, ConnectorId: 1, ChargePointId: "CP1",
		MeterStart: 5000, TimeStart: now.Add(-time.Hour),
	})

	if got := counterValue(t, "ocpp_consumed_today", labels) - consumedBefore; got != 28975 {
		t.Errorf("consumed energy should grow by 28975, grew by %v", got)
	}
}

// TestFinishAbandonedTransactionWithoutEnergyIsNotCounted keeps aborted transactions out of the
// energy counter: no meter value means nothing was delivered, so there is nothing to add.
func TestFinishAbandonedTransactionWithoutEnergyIsNotCounted(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	labels := map[string]string{"location": "loc-aborted", "charge_point_id": "CP1"}
	db := &abandonedStubDB{}
	h := newStopHandler(t, db, 1)
	h.chargePoints["CP1"].model.LocationId = "loc-aborted"

	consumedBefore := counterValue(t, "ocpp_consumed_today", labels)

	h.finishAbandonedTransaction(&entity.Transaction{
		Id: 1, ConnectorId: 1, ChargePointId: "CP1",
		TimeStart: now.Add(-time.Hour),
	})

	if got := counterValue(t, "ocpp_consumed_today", labels) - consumedBefore; got != 0 {
		t.Errorf("an aborted transaction delivered nothing, consumed grew by %v", got)
	}
}

/*
TestFinishAbandonedTransactionResetsPowerRateGauge pins the gauge reset. A charge point that goes
silent mid-session sends neither the StopTransaction nor the StatusNotification that normally zero
ocpp_current_power_rate, so without a reset in the sweep path the graph stays stuck on the last
observed power until the next session on that connector.
*/
func TestFinishAbandonedTransactionResetsPowerRateGauge(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	db := &abandonedStubDB{
		samples: []entity.TransactionMeter{{Id: 1, Value: 300, Time: now.Add(-25 * time.Minute)}},
	}
	h := newStopHandler(t, db, 1)
	h.chargePoints["CP1"].model.LocationId = "loc1"

	counters.ObservePowerRate("loc1", "CP1", "1", 84000)

	h.finishAbandonedTransaction(&entity.Transaction{
		Id: 1, ConnectorId: 1, ChargePointId: "CP1",
		TimeStart: now.Add(-time.Hour),
	})

	if got := powerRateValue(t, "loc1", "CP1", "1"); got != 0 {
		t.Errorf("power rate gauge should be reset to 0, still reads %v", got)
	}
}

// TestFinishAbandonedTransactionWithoutMeterValues covers the aborted branch: no sample ever
// arrived, so nothing can be attributed and the stop lands on the start time.
func TestFinishAbandonedTransactionWithoutMeterValues(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	db := &abandonedStubDB{}
	h := newStopHandler(t, db, 1)

	transaction := &entity.Transaction{
		Id: 1, ConnectorId: 1, ChargePointId: "CP1",
		MeterStart: 0, TimeStart: now.Add(-time.Hour),
	}
	h.finishAbandonedTransaction(transaction)

	if transaction.Reason != "aborted by system" {
		t.Errorf("reason = %q, want aborted by system", transaction.Reason)
	}
	if !transaction.TimeStop.Equal(transaction.TimeStart) {
		t.Errorf("time stop should equal time start, got %v", transaction.TimeStop)
	}
	if db.savedMeterValues != nil {
		t.Errorf("no meter values should be saved, got %v", db.savedMeterValues)
	}
}
