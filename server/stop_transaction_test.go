package server

import (
	"fmt"
	"testing"
	"time"

	"evsys/entity"
	"evsys/internal"
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
