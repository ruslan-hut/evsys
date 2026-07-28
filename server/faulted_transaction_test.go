package server

import (
	"sync"
	"testing"
	"time"

	"evsys/entity"
	"evsys/internal"
	"evsys/ocpp/v16/core"
	"evsys/types"
)

// faultedStubDB stubs what the Faulted-close path touches: OnStatusNotification persists the
// connector, then the goroutine it spawns loads the transaction, checks the last sample against the
// grace window, and (when quiet) runs finishAbandonedTransaction. The embedded interface is nil, so
// any other call panics. Flags are guarded by a mutex because the close runs on its own goroutine
// while the test reads them.
type faultedStubDB struct {
	internal.Database
	mu          sync.Mutex
	transaction *entity.Transaction
	lastSample  *entity.TransactionMeter
	closed      bool // UpdateTransaction was called with is_finished set
	connFreed   bool // UpdateConnector was called releasing the connector
}

func (s *faultedStubDB) GetTransaction(int) (*entity.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transaction, nil
}

func (s *faultedStubDB) ReadTransactionMeterValue(int) (*entity.TransactionMeter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSample, nil
}

func (s *faultedStubDB) ReadAllTransactionMeterValues(int) ([]entity.TransactionMeter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastSample == nil {
		return nil, nil
	}
	return []entity.TransactionMeter{*s.lastSample}, nil
}

func (s *faultedStubDB) UpdateTransaction(t *entity.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.IsFinished {
		s.closed = true
	}
	return nil
}

func (s *faultedStubDB) UpdateConnector(c *entity.Connector) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// OnStatusNotification also writes the connector while it still holds the transaction; only the
	// release in finishAbandonedTransaction sets it to -1, which is the write under test.
	if c.CurrentTransactionId == -1 {
		s.connFreed = true
	}
	return nil
}

func (s *faultedStubDB) DeleteTransactionMeterValues(int) error { return nil }
func (s *faultedStubDB) GetTodayConsumedEnergy() ([]*entity.ConsumedEnergy, error) {
	return nil, nil
}

func (s *faultedStubDB) state() (closed, connFreed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed, s.connFreed
}

// waitUntil polls cond until it holds or the deadline passes. The Faulted-close runs on a goroutine
// the handler starts, so the assertions cannot read straight after the call returns.
func waitUntil(t *testing.T, cond func() bool) bool {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func faultedNotification(now time.Time) *core.StatusNotificationRequest {
	return &core.StatusNotificationRequest{
		ConnectorId: 1,
		// this fleet reports Faulted with errorCode=NoError; the close keys off the status, not the code
		ErrorCode: core.NoError,
		Status:    core.ChargePointStatusFaulted,
		Info:      "Socket has an error. Charge is not possible",
		VendorId:  "Circutor",
		Timestamp: types.NewDateTime(now),
	}
}

// TestOnStatusNotificationFaultedClosesQuietTransaction covers the fast close: a connector reports
// Faulted while its transaction has gone quiet (no sample within the grace window), so the session
// is closed and the connector released instead of hanging until the sweep runs.
func TestOnStatusNotificationFaultedClosesQuietTransaction(t *testing.T) {
	now := time.Now().UTC()
	db := &faultedStubDB{
		transaction: &entity.Transaction{
			Id: 1, ConnectorId: 1, ChargePointId: "CP1",
			MeterStart: 1000, TimeStart: now.Add(-time.Hour),
		},
		// last reading is older than transactionReleaseGrace (2m): the session is no longer reporting
		lastSample: &entity.TransactionMeter{Id: 1, Value: 2500, Time: now.Add(-10 * time.Minute)},
	}
	h := newStopHandler(t, db, 1)
	logger := &capturingLogger{}
	h.logger = logger

	_, err := h.OnStatusNotification("CP1", faultedNotification(now))
	if err != nil {
		t.Fatalf("OnStatusNotification: %v", err)
	}

	if !waitUntil(t, func() bool { closed, freed := db.state(); return closed && freed }) {
		closed, freed := db.state()
		t.Fatalf("a quiet transaction on a faulted connector should be closed and its connector freed, got closed=%v freed=%v", closed, freed)
	}
	if !logger.has("closing open transaction #1") {
		t.Errorf("expected the close to be traced, got warns %v", logger.warns)
	}
}

// TestOnStatusNotificationFaultedLeavesLiveTransaction covers the guard: a connector can report
// Faulted while its session is still delivering energy and reporting meter values. Closing it then
// would strand the transaction, so a session reporting within the grace window is left for the
// sweep, which applies the same guard once it goes quiet.
func TestOnStatusNotificationFaultedLeavesLiveTransaction(t *testing.T) {
	now := time.Now().UTC()
	db := &faultedStubDB{
		transaction: &entity.Transaction{
			Id: 1, ConnectorId: 1, ChargePointId: "CP1",
			MeterStart: 1000, TimeStart: now.Add(-time.Hour),
		},
		// last reading is within transactionReleaseGrace (2m): the session is still live
		lastSample: &entity.TransactionMeter{Id: 1, Value: 2500, Time: now.Add(-10 * time.Second)},
	}
	h := newStopHandler(t, db, 1)
	logger := &capturingLogger{}
	h.logger = logger

	_, err := h.OnStatusNotification("CP1", faultedNotification(now))
	if err != nil {
		t.Fatalf("OnStatusNotification: %v", err)
	}

	// wait for the goroutine to reach its decision, evidenced by the left-open trace
	if !waitUntil(t, func() bool { return logger.has("still reporting, left open") }) {
		t.Fatalf("expected the still-reporting transaction to be left open with a trace, got warns %v", logger.warns)
	}
	if closed, freed := db.state(); closed || freed {
		t.Errorf("a still-reporting transaction must not be closed on Faulted, got closed=%v freed=%v", closed, freed)
	}
}
