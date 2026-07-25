package server

import (
	"fmt"
	"testing"
	"time"

	"evsys/entity"
	"evsys/internal"
	"evsys/metrics/counters"
	"evsys/ocpp/v16/core"
	"evsys/types"
)

// stopStubDB records the order of the writes OnStopTransaction makes. The embedded interface is
// nil: any method the handler calls that is not implemented here panics, which keeps the stub
// honest about what the handler depends on.
type stopStubDB struct {
	internal.Database
	transaction *entity.Transaction
	connector   *entity.Connector
	writes      []string
	// finishedAt records what is_finished held each time the connector was written, which is the
	// property under test rather than the call order alone
	finishedOnConnectorWrite []bool
}

func (s *stopStubDB) GetTransaction(_ int) (*entity.Transaction, error) {
	return s.transaction, nil
}

func (s *stopStubDB) UpdateTransaction(t *entity.Transaction) error {
	s.writes = append(s.writes, "transaction")
	s.transaction.IsFinished = t.IsFinished
	return nil
}

func (s *stopStubDB) UpdateConnector(c *entity.Connector) error {
	s.writes = append(s.writes, "connector")
	s.finishedOnConnectorWrite = append(s.finishedOnConnectorWrite, s.transaction.IsFinished)
	s.connector = c
	return nil
}

func (s *stopStubDB) ReadAllTransactionMeterValues(int) ([]entity.TransactionMeter, error) {
	return nil, nil
}

func (s *stopStubDB) DeleteTransactionMeterValues(int) error { return nil }
func (s *stopStubDB) GetTodayConsumedEnergy() ([]*entity.ConsumedEnergy, error) {
	return nil, nil
}
func (s *stopStubDB) GetConnector(int, string) (*entity.Connector, error) {
	return nil, nil
}
func (s *stopStubDB) AddConnector(*entity.Connector) error { return nil }
func (s *stopStubDB) SaveStopTransactionRequest(*core.StopTransactionRequest) error {
	return nil
}

type stopStubBilling struct{}

func (stopStubBilling) OnTransactionStart(*entity.Transaction) error                     { return nil }
func (stopStubBilling) OnTransactionFinished(*entity.Transaction) error                  { return nil }
func (stopStubBilling) OnMeterValue(*entity.Transaction, *entity.TransactionMeter) error { return nil }

type stopStubLogger struct{}

func (stopStubLogger) FeatureEvent(_, _, _ string) {}
func (stopStubLogger) RawDataEvent(_, _ string)    {}
func (stopStubLogger) Debug(_ string)              {}
func (stopStubLogger) Warn(_ string)               {}
func (stopStubLogger) Error(_ string, _ error)     {}

func newStopHandler(t *testing.T, db internal.Database, transactionId int) *SystemHandler {
	t.Helper()

	trigger := NewTrigger(nil, stopStubLogger{})
	// drain the trigger channels; OnStopTransaction sends on Unregister and would otherwise block
	go func() {
		for {
			select {
			case <-trigger.Register:
			case <-trigger.Unregister:
			}
		}
	}()

	h := &SystemHandler{
		chargePoints: map[string]*ChargePointState{},
		lastMeter:    map[int]*entity.TransactionMeter{},
		database:     db,
		billing:      stopStubBilling{},
		logger:       stopStubLogger{},
		trigger:      trigger,
		location:     time.UTC,
	}

	state := newChargePointState(&entity.ChargePoint{Id: "CP1"})
	connector := entity.NewConnector(1, "CP1")
	connector.CurrentTransactionId = transactionId
	state.connectors[1] = connector
	state.registerTransaction(transactionId)
	h.chargePoints["CP1"] = state

	return h
}

/*
TestOnStopTransactionWritesTransactionBeforeConnector pins the write order.

The connector used to be cleared and persisted before the transaction was marked finished. In
between, the database showed an open transaction whose connector had moved on - indistinguishable
from one abandoned by a lost StopTransaction - so a sweep landing in that gap would claim a stop
that was proceeding normally, re-run billing on it and overwrite its reason and meter readings.
*/
func TestOnStopTransactionWritesTransactionBeforeConnector(t *testing.T) {
	db := &stopStubDB{
		transaction: &entity.Transaction{
			Id: 1, ConnectorId: 1, ChargePointId: "CP1",
			MeterStart: 1000, TimeStart: time.Now().UTC().Add(-time.Hour),
		},
	}
	h := newStopHandler(t, db, 1)

	_, err := h.OnStopTransaction("CP1", &core.StopTransactionRequest{
		TransactionId: 1,
		MeterStop:     2500,
		Timestamp:     types.NewDateTime(time.Now().UTC()),
		Reason:        core.ReasonLocal,
	})
	if err != nil {
		t.Fatalf("OnStopTransaction: %v", err)
	}

	if len(db.writes) != 2 {
		t.Fatalf("expected a transaction write and a connector write, got %v", db.writes)
	}
	if db.writes[0] != "transaction" || db.writes[1] != "connector" {
		t.Errorf("write order = %v, want [transaction connector]", db.writes)
	}
	for _, finished := range db.finishedOnConnectorWrite {
		if !finished {
			t.Error("the connector was released while the transaction was still open, which is the window the sweeper mistakes for an abandoned transaction")
		}
	}
	if db.connector.CurrentTransactionId != -1 {
		t.Errorf("connector should be released, points at %d", db.connector.CurrentTransactionId)
	}
	if !db.transaction.IsFinished {
		t.Error("transaction should be finished")
	}
}

// TestOnStopTransactionTracesLateStopAfterSystemClose covers a StopTransaction arriving for a
// transaction the sweep had already aborted: the session was still live on the charger, so the
// premature close must be traced, and the real figures must still overwrite the system ones.
func TestOnStopTransactionTracesLateStopAfterSystemClose(t *testing.T) {
	db := &stopStubDB{
		transaction: &entity.Transaction{
			Id: 1, ConnectorId: 1, ChargePointId: "CP1",
			MeterStart: 0, MeterStop: 0, IsFinished: true,
			Reason:    reasonAbortedBySystem,
			TimeStart: time.Now().UTC().Add(-time.Hour),
		},
	}
	h := newStopHandler(t, db, 1)
	logger := &capturingLogger{}
	h.logger = logger

	_, err := h.OnStopTransaction("CP1", &core.StopTransactionRequest{
		TransactionId: 1,
		MeterStop:     35708,
		Timestamp:     types.NewDateTime(time.Now().UTC()),
		Reason:        core.ReasonLocal,
	})
	if err != nil {
		t.Fatalf("OnStopTransaction: %v", err)
	}

	if !logger.has("late StopTransaction for transaction #1") {
		t.Errorf("expected a late-stop trace, got warns %v", logger.warns)
	}
	// the real stop must still land: overwrite the aborted figures
	if db.transaction.MeterStop != 35708 {
		t.Errorf("meter stop = %d, want 35708 (real stop must overwrite the system close)", db.transaction.MeterStop)
	}
	if db.transaction.Reason != string(core.ReasonLocal) {
		t.Errorf("reason = %q, want %q", db.transaction.Reason, core.ReasonLocal)
	}
}

// TestOnStopTransactionDuplicateStopIsNotTracedAsLate keeps the late-stop trace specific to system
// closes: an ordinary repeated StopTransaction (the charger's own earlier stop) must not be flagged.
func TestOnStopTransactionDuplicateStopIsNotTracedAsLate(t *testing.T) {
	db := &stopStubDB{
		transaction: &entity.Transaction{
			Id: 1, ConnectorId: 1, ChargePointId: "CP1",
			MeterStart: 1000, MeterStop: 2500, IsFinished: true,
			Reason:    string(core.ReasonLocal),
			TimeStart: time.Now().UTC().Add(-time.Hour),
		},
	}
	h := newStopHandler(t, db, 1)
	logger := &capturingLogger{}
	h.logger = logger

	_, err := h.OnStopTransaction("CP1", &core.StopTransactionRequest{
		TransactionId: 1,
		MeterStop:     2500,
		Timestamp:     types.NewDateTime(time.Now().UTC()),
		Reason:        core.ReasonLocal,
	})
	if err != nil {
		t.Fatalf("OnStopTransaction: %v", err)
	}

	if logger.has("late StopTransaction") {
		t.Errorf("an ordinary duplicate stop must not be traced as late, got warns %v", logger.warns)
	}
}

// TestOnStopTransactionReleasesConnectorOnDuplicateStop covers the early return for a transaction
// that is already finished: the connector still has to come free, or a repeated StopTransaction
// would leave it pinned to a closed transaction.
func TestOnStopTransactionReleasesConnectorOnDuplicateStop(t *testing.T) {
	db := &stopStubDB{
		transaction: &entity.Transaction{
			Id: 1, ConnectorId: 1, ChargePointId: "CP1",
			MeterStart: 1000, MeterStop: 2500, IsFinished: true,
			TimeStart: time.Now().UTC().Add(-time.Hour),
		},
	}
	h := newStopHandler(t, db, 1)

	_, err := h.OnStopTransaction("CP1", &core.StopTransactionRequest{
		TransactionId: 1,
		MeterStop:     2500,
		Timestamp:     types.NewDateTime(time.Now().UTC()),
		Reason:        core.ReasonLocal,
	})
	if err != nil {
		t.Fatalf("OnStopTransaction: %v", err)
	}

	if fmt.Sprint(db.writes) != "[connector]" {
		t.Errorf("a duplicate stop should only release the connector, got %v", db.writes)
	}
	if db.connector == nil || db.connector.CurrentTransactionId != -1 {
		t.Error("a duplicate stop must still release the connector")
	}
}

/*
TestOnStopTransactionOverwritesSystemCloseWithUnchangedMeter covers the shape a power cut actually
has: the charger stops reporting before it stops charging, so the StopTransaction it flushes after
the reboot carries the very reading the system closed the transaction on. The already-finished
early return used to swallow it, leaving "stopped by system" and the last sample's timestamp on a
session the charger had reported as PowerLoss.
*/
func TestOnStopTransactionOverwritesSystemCloseWithUnchangedMeter(t *testing.T) {
	stoppedAt := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Second)
	db := &stopStubDB{
		transaction: &entity.Transaction{
			Id: 1, ConnectorId: 1, ChargePointId: "CP1",
			MeterStart: 20000, MeterStop: 31524, IsFinished: true,
			Reason:    reasonStoppedBySystem,
			TimeStart: time.Now().UTC().Add(-time.Hour),
			TimeStop:  time.Now().UTC().Add(-25 * time.Minute),
		},
	}
	h := newStopHandler(t, db, 1)
	logger := &capturingLogger{}
	h.logger = logger

	_, err := h.OnStopTransaction("CP1", &core.StopTransactionRequest{
		TransactionId: 1,
		MeterStop:     31524,
		Timestamp:     types.NewDateTime(stoppedAt),
		Reason:        core.ReasonPowerLoss,
	})
	if err != nil {
		t.Fatalf("OnStopTransaction: %v", err)
	}

	if !logger.has("late StopTransaction for transaction #1") {
		t.Errorf("expected a late-stop trace, got warns %v", logger.warns)
	}
	if logger.has("is already finished") {
		t.Errorf("a stop landing on a system close is not a duplicate, got warns %v", logger.warns)
	}
	if db.transaction.Reason != string(core.ReasonPowerLoss) {
		t.Errorf("reason = %q, want %q: the charger's reason must replace the system one",
			db.transaction.Reason, core.ReasonPowerLoss)
	}
	if !db.transaction.TimeStop.Equal(stoppedAt) {
		t.Errorf("time stop = %v, want %v: the charger's stop time must replace the last sample time",
			db.transaction.TimeStop, stoppedAt)
	}
}

/*
TestOnStopTransactionAfterSystemCloseCountsEnergyOnce pins the consumed-energy counter across the
two closes of the same session. The sweep counts what it closed on, so the late stop may only add
the difference the charger reports on top of it; adding the whole session again doubled every
sweep-then-stop session in ocpp_consumed_today.
*/
func TestOnStopTransactionAfterSystemCloseCountsEnergyOnce(t *testing.T) {
	labels := map[string]string{"location": "loc-late-stop", "charge_point_id": "CP1"}
	db := &stopStubDB{
		transaction: &entity.Transaction{
			Id: 1, ConnectorId: 1, ChargePointId: "CP1",
			MeterStart: 20000, MeterStop: 30000, IsFinished: true,
			Reason:    reasonStoppedBySystem,
			TimeStart: time.Now().UTC().Add(-time.Hour),
			TimeStop:  time.Now().UTC().Add(-25 * time.Minute),
		},
	}
	h := newStopHandler(t, db, 1)
	h.chargePoints["CP1"].model.LocationId = "loc-late-stop"

	// what the sweep already added when it closed the transaction on the last sample
	counters.CountConsumedPower("loc-late-stop", "CP1", 10000)
	before := counterValue(t, "ocpp_consumed_today", labels)

	_, err := h.OnStopTransaction("CP1", &core.StopTransactionRequest{
		TransactionId: 1,
		MeterStop:     30500,
		Timestamp:     types.NewDateTime(time.Now().UTC()),
		Reason:        core.ReasonPowerLoss,
	})
	if err != nil {
		t.Fatalf("OnStopTransaction: %v", err)
	}

	// the counter is written from a goroutine started by the handler
	var grew float64
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		grew = counterValue(t, "ocpp_consumed_today", labels) - before
		if grew != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if grew != 500 {
		t.Errorf("consumed energy should grow by the 500 the charger reports on top of the close, grew by %v", grew)
	}
}
