package internal

import (
	"context"
	"errors"
	"evsys/entity"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Integration tests for the abandoned transaction sweep. They need a real MongoDB, because the
// behaviour under test lives in an aggregation pipeline rather than in Go.
//
//	docker run -d --name evsys-test-mongo -p 27019:27017 mongo:7
//	MONGO_TEST_URI=mongodb://localhost:27019 go test -race ./internal/...

const testDatabase = "evsys_sweep_test"

func testClient(t *testing.T) *MongoDB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI is not set")
	}

	db := &MongoDB{
		ctx:           context.Background(),
		clientOptions: options.Client().ApplyURI(uri),
		database:      testDatabase,
	}

	connection, err := db.connect()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err = connection.Ping(db.ctx, nil); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if err = connection.Database(testDatabase).Drop(db.ctx); err != nil {
		t.Fatalf("drop database: %v", err)
	}
	db.disconnect(connection)

	return db
}

func withCollection(t *testing.T, db *MongoDB, name string, fn func(*mongo.Collection)) {
	t.Helper()
	connection, err := db.connect()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.disconnect(connection)
	fn(connection.Database(db.database).Collection(name))
}

func seedTransaction(t *testing.T, db *MongoDB, transaction *entity.Transaction) {
	t.Helper()
	withCollection(t, db, collectionTransactions, func(c *mongo.Collection) {
		if _, err := c.InsertOne(db.ctx, transaction); err != nil {
			t.Fatalf("seed transaction: %v", err)
		}
	})
}

func seedConnector(t *testing.T, db *MongoDB, connector *entity.Connector) {
	t.Helper()
	withCollection(t, db, collectionConnectors, func(c *mongo.Collection) {
		if _, err := c.InsertOne(db.ctx, connector); err != nil {
			t.Fatalf("seed connector: %v", err)
		}
	})
}

func seedMeterValue(t *testing.T, db *MongoDB, transactionId, value int, at time.Time) {
	t.Helper()
	withCollection(t, db, collectionMeterValues, func(c *mongo.Collection) {
		meter := entity.NewMeter(transactionId, 1, "Charging", at)
		meter.Value = value
		if _, err := c.InsertOne(db.ctx, meter); err != nil {
			t.Fatalf("seed meter value: %v", err)
		}
	})
}

func connectorPointer(t *testing.T, db *MongoDB, chargePointId string, connectorId int) int {
	t.Helper()
	var got entity.Connector
	withCollection(t, db, collectionConnectors, func(c *mongo.Collection) {
		err := c.FindOne(db.ctx, bson.M{
			"charge_point_id": chargePointId,
			"connector_id":    connectorId,
		}).Decode(&got)
		if err != nil {
			t.Fatalf("read connector: %v", err)
		}
	})
	return got.CurrentTransactionId
}

// TestGetUnfinishedTransactions covers which transactions the sweeper may and may not claim. The
// false cases matter more than the true ones: claiming a live session bills a driver for a charge
// that is still running and frees a connector with a car attached to it.
func TestGetUnfinishedTransactions(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	staleBefore := now.Add(-20 * time.Minute)
	releasedBefore := now.Add(-2 * time.Minute)

	tests := []struct {
		name string
		// pointer is what the connector's current_transaction_id holds; noConnector drops the
		// connector document entirely
		pointer     int
		noConnector bool
		finished    bool
		startedAgo  time.Duration
		// meterAgo of 0 means no meter value at all
		meterAgo time.Duration
		swept    bool
		// cause is the sweep_cause the pipeline is expected to tag a swept row with
		cause string
	}{
		{
			name:       "live session reporting meter values is left alone",
			pointer:    1,
			startedAgo: time.Hour,
			meterAgo:   20 * time.Second,
			swept:      false,
		},
		{
			name:       "session that just started is left alone",
			pointer:    1,
			startedAgo: time.Minute,
			swept:      false,
		},
		{
			name:       "pinned connector with no activity is swept",
			pointer:    1,
			startedAgo: 30 * time.Minute,
			swept:      true,
			cause:      "no activity from the charge point",
		},
		{
			name:       "pinned connector whose meter values dried up is swept",
			pointer:    1,
			startedAgo: 2 * time.Hour,
			meterAgo:   30 * time.Minute,
			swept:      true,
			cause:      "no activity from the charge point",
		},
		{
			// the regression: OnStopTransaction clears the connector and persists it before it
			// writes IsFinished, so a normal stop briefly looks exactly like an abandoned one
			name:       "stop in progress is left alone",
			pointer:    -1,
			startedAgo: time.Hour,
			meterAgo:   20 * time.Second,
			swept:      false,
		},
		{
			name:       "connector that moved on to another transaction is swept",
			pointer:    99,
			startedAgo: time.Hour,
			meterAgo:   10 * time.Minute,
			swept:      true,
			cause:      "connector released without a stop",
		},
		{
			name:        "missing connector with recent activity is left alone",
			noConnector: true,
			startedAgo:  time.Hour,
			meterAgo:    20 * time.Second,
			swept:       false,
		},
		{
			name:        "missing connector with no activity is swept",
			noConnector: true,
			startedAgo:  time.Hour,
			swept:       true,
			cause:       "no activity from the charge point",
		},
		{
			name:       "finished transaction is never returned",
			pointer:    1,
			finished:   true,
			startedAgo: 30 * time.Minute,
			swept:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := testClient(t)

			seedTransaction(t, db, &entity.Transaction{
				Id:            1,
				IsFinished:    test.finished,
				ConnectorId:   1,
				ChargePointId: "CP1",
				TimeStart:     now.Add(-test.startedAgo),
			})
			if !test.noConnector {
				seedConnector(t, db, &entity.Connector{
					Id:                   1,
					ChargePointId:        "CP1",
					CurrentTransactionId: test.pointer,
				})
			}
			if test.meterAgo != 0 {
				seedMeterValue(t, db, 1, 500, now.Add(-test.meterAgo))
			}

			got, err := db.GetUnfinishedTransactions(staleBefore, releasedBefore)
			if err != nil {
				t.Fatalf("GetUnfinishedTransactions: %v", err)
			}

			if test.swept && len(got) != 1 {
				t.Fatalf("expected the transaction to be swept, got %d", len(got))
			}
			if !test.swept && len(got) != 0 {
				t.Fatalf("expected the transaction to be left alone, got %d", len(got))
			}
			if test.swept {
				if got[0].Cause != test.cause {
					t.Errorf("sweep_cause = %q, want %q", got[0].Cause, test.cause)
				}
				if got[0].LastActivity.IsZero() {
					t.Error("last_activity not populated on swept transaction")
				}
			}
		})
	}
}

// TestGetUnfinishedTransactionsStripsJoinFields guards the $unset: the pipeline adds fields to do
// its work, and they must not survive into the decoded entity.
func TestGetUnfinishedTransactionsStripsJoinFields(t *testing.T) {
	db := testClient(t)
	now := time.Now().UTC().Truncate(time.Second)

	seedTransaction(t, db, &entity.Transaction{
		Id:            7,
		ConnectorId:   1,
		ChargePointId: "CP1",
		TimeStart:     now.Add(-time.Hour),
		MeterStart:    10,
	})
	seedConnector(t, db, &entity.Connector{Id: 1, ChargePointId: "CP1", CurrentTransactionId: 7})
	seedMeterValue(t, db, 7, 900, now.Add(-40*time.Minute))

	got, err := db.GetUnfinishedTransactions(now.Add(-20*time.Minute), now.Add(-2*time.Minute))
	if err != nil {
		t.Fatalf("GetUnfinishedTransactions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(got))
	}
	if got[0].Id != 7 || got[0].ChargePointId != "CP1" || got[0].MeterStart != 10 {
		t.Errorf("transaction decoded wrong: %+v", got[0])
	}

	// sweep_cause and last_activity are computed for the caller to log, and ride on the
	// wrapper rather than the transaction; writing the transaction back must not persist them.
	if err := db.UpdateTransaction(&got[0].Transaction); err != nil {
		t.Fatalf("UpdateTransaction: %v", err)
	}

	var raw bson.M
	withCollection(t, db, collectionTransactions, func(c *mongo.Collection) {
		if err := c.FindOne(db.ctx, bson.M{"transaction_id": 7}).Decode(&raw); err != nil {
			t.Fatalf("read back: %v", err)
		}
	})
	for _, field := range []string{"connector", "meter", "last_activity", "sweep_cause"} {
		if _, ok := raw[field]; ok {
			t.Errorf("pipeline field %q leaked into the stored document", field)
		}
	}
}

func TestGetUnfinishedTransactionsForChargePoint(t *testing.T) {
	db := testClient(t)
	now := time.Now().UTC().Truncate(time.Second)

	seedTransaction(t, db, &entity.Transaction{Id: 1, ChargePointId: "CP1", ConnectorId: 1, TimeStart: now})
	seedTransaction(t, db, &entity.Transaction{Id: 2, ChargePointId: "CP2", ConnectorId: 1, TimeStart: now})
	seedTransaction(t, db, &entity.Transaction{Id: 3, ChargePointId: "CP1", ConnectorId: 2, TimeStart: now, IsFinished: true})

	got, err := db.GetUnfinishedTransactionsForChargePoint("CP1")
	if err != nil {
		t.Fatalf("GetUnfinishedTransactionsForChargePoint: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(got))
	}
	if got[0].Id != 1 {
		t.Errorf("expected transaction 1, got %d", got[0].Id)
	}
}

// TestReadTransactionMeterValue pins the sort field. It used to sort by "timestamp", which no
// document has, so it returned an arbitrary sample - and the sweeper writes that sample's value
// into meter_stop.
func TestReadTransactionMeterValue(t *testing.T) {
	db := testClient(t)
	now := time.Now().UTC().Truncate(time.Second)

	seedMeterValue(t, db, 1, 100, now.Add(-30*time.Minute))
	seedMeterValue(t, db, 1, 300, now.Add(-10*time.Minute))
	seedMeterValue(t, db, 1, 200, now.Add(-20*time.Minute))

	got, err := db.ReadTransactionMeterValue(1)
	if err != nil {
		t.Fatalf("ReadTransactionMeterValue: %v", err)
	}
	if got.Value != 300 {
		t.Errorf("expected the newest sample (300), got %d", got.Value)
	}
}

// TestGetTransactionNotFound pins the distinction OnStartTransaction relies on to decide whether a
// connector's pointer can be trusted. If a genuine miss stopped satisfying IsNotFound, every start
// on a connector pointing at a deleted transaction would be refused instead of recovering.
func TestGetTransactionNotFound(t *testing.T) {
	db := testClient(t)

	got, err := db.GetTransaction(404)
	if got != nil {
		t.Errorf("expected no transaction, got %+v", got.Id)
	}
	if err == nil {
		t.Fatal("expected an error for a missing transaction")
	}
	if !IsNotFound(err) {
		t.Errorf("a missing transaction must satisfy IsNotFound, got %v", err)
	}

	seedTransaction(t, db, &entity.Transaction{Id: 8, ChargePointId: "CP1", ConnectorId: 1})
	found, err := db.GetTransaction(8)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if found.Id != 8 {
		t.Errorf("expected transaction 8, got %d", found.Id)
	}
}

// TestIsNotFoundRejectsOtherErrors guards the other half: a query that failed must not be mistaken
// for an absent document, or OnStartTransaction would overwrite a live session on a database blip.
func TestIsNotFoundRejectsOtherErrors(t *testing.T) {
	if IsNotFound(nil) {
		t.Error("nil is not a not-found error")
	}
	if IsNotFound(context.DeadlineExceeded) {
		t.Error("a timeout must not be reported as not-found")
	}
	if IsNotFound(errors.New("connection refused")) {
		t.Error("a connection failure must not be reported as not-found")
	}
}

func TestMigrationStuckTransactions(t *testing.T) {
	db := testClient(t)
	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-72 * time.Hour)

	// abandoned, no meter values at all: must stop at its start time so no duration reaches
	// billing, and must not acquire a payment amount
	seedTransaction(t, db, &entity.Transaction{
		Id: 1, ConnectorId: 1, ChargePointId: "CP1", TimeStart: old,
	})
	seedConnector(t, db, &entity.Connector{Id: 1, ChargePointId: "CP1", CurrentTransactionId: 1})

	// abandoned with meter values: stops at the newest sample
	seedTransaction(t, db, &entity.Transaction{
		Id: 2, ConnectorId: 2, ChargePointId: "CP1", TimeStart: old,
	})
	seedConnector(t, db, &entity.Connector{Id: 2, ChargePointId: "CP1", CurrentTransactionId: 2})
	seedMeterValue(t, db, 2, 4200, old.Add(time.Hour))

	// already carries money: must be left open for a human
	seedTransaction(t, db, &entity.Transaction{
		Id: 3, ConnectorId: 3, ChargePointId: "CP1", TimeStart: old, PaymentAmount: 1500,
	})
	seedConnector(t, db, &entity.Connector{Id: 3, ChargePointId: "CP1", CurrentTransactionId: 3})

	// live session: newer than the cutoff, must not be touched
	seedTransaction(t, db, &entity.Transaction{
		Id: 4, ConnectorId: 4, ChargePointId: "CP1", TimeStart: now.Add(-time.Hour),
	})
	seedConnector(t, db, &entity.Connector{Id: 4, ChargePointId: "CP1", CurrentTransactionId: 4})

	// the second state: transaction already finished, connector still pinned
	seedTransaction(t, db, &entity.Transaction{
		Id: 5, ConnectorId: 5, ChargePointId: "CP1", TimeStart: old, TimeStop: old, IsFinished: true,
		Reason: "stopped by system",
	})
	seedConnector(t, db, &entity.Connector{Id: 5, ChargePointId: "CP1", CurrentTransactionId: 5})

	// pointer to a transaction that does not exist: left alone, OnStartTransaction overwrites it
	seedConnector(t, db, &entity.Connector{Id: 6, ChargePointId: "CP1", CurrentTransactionId: 404})

	connection, err := db.connect()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.disconnect(connection)
	database := connection.Database(db.database)

	if err = migrationStuckTransactionsUp(db.ctx, database); err != nil {
		t.Fatalf("migration: %v", err)
	}

	read := func(id int) *entity.Transaction {
		got := &entity.Transaction{}
		if err := database.Collection(collectionTransactions).
			FindOne(db.ctx, bson.M{"transaction_id": id}).Decode(got); err != nil {
			t.Fatalf("read transaction %d: %v", id, err)
		}
		return got
	}

	aborted := read(1)
	if !aborted.IsFinished {
		t.Error("transaction 1 should be closed")
	}
	if !aborted.TimeStop.Equal(aborted.TimeStart) {
		t.Errorf("transaction 1 should stop at its start time, got %v", aborted.TimeStop)
	}
	if aborted.PaymentAmount != 0 {
		t.Errorf("transaction 1 must not acquire a payment amount, got %d", aborted.PaymentAmount)
	}
	if aborted.Reason != reasonBacklogAborted {
		t.Errorf("transaction 1 reason = %q", aborted.Reason)
	}
	if got := connectorPointer(t, db, "CP1", 1); got != -1 {
		t.Errorf("connector 1 should be released, points at %d", got)
	}

	stopped := read(2)
	if stopped.MeterStop != 4200 {
		t.Errorf("transaction 2 meter_stop = %d, want 4200", stopped.MeterStop)
	}
	if !stopped.TimeStop.Equal(old.Add(time.Hour)) {
		t.Errorf("transaction 2 should stop at its last sample, got %v", stopped.TimeStop)
	}
	if stopped.Reason != reasonBacklogStopped {
		t.Errorf("transaction 2 reason = %q", stopped.Reason)
	}

	if billed := read(3); billed.IsFinished {
		t.Error("transaction 3 carries a payment amount and must be left open")
	}
	if got := connectorPointer(t, db, "CP1", 3); got != 3 {
		t.Errorf("connector 3 should stay pinned while its transaction is open, got %d", got)
	}

	if live := read(4); live.IsFinished {
		t.Error("transaction 4 is newer than the cutoff and must be left open")
	}
	if got := connectorPointer(t, db, "CP1", 4); got != 4 {
		t.Errorf("connector 4 should stay pinned, got %d", got)
	}

	if got := connectorPointer(t, db, "CP1", 5); got != -1 {
		t.Errorf("connector 5 points at a finished transaction and should be released, got %d", got)
	}
	if orphan := read(5); orphan.Reason != "stopped by system" {
		t.Errorf("transaction 5 was already closed and must not be rewritten, reason = %q", orphan.Reason)
	}

	if got := connectorPointer(t, db, "CP1", 6); got != 404 {
		t.Errorf("connector 6 points at a missing transaction and should be left alone, got %d", got)
	}

	// re-running must be a no-op
	before := read(1)
	if err = migrationStuckTransactionsUp(db.ctx, database); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if after := read(1); !after.TimeStop.Equal(before.TimeStop) || after.Reason != before.Reason {
		t.Errorf("migration is not idempotent: time_stop %v then %v, reason %q then %q",
			before.TimeStop, after.TimeStop, before.Reason, after.Reason)
	}
}

func TestMigrationStuckTransactionsRollback(t *testing.T) {
	db := testClient(t)
	old := time.Now().UTC().Truncate(time.Second).Add(-72 * time.Hour)

	seedTransaction(t, db, &entity.Transaction{
		Id: 1, ConnectorId: 1, ChargePointId: "CP1", TimeStart: old,
	})
	seedConnector(t, db, &entity.Connector{Id: 1, ChargePointId: "CP1", CurrentTransactionId: 1})
	// closed by the runtime sweeper, not by the migration: the rollback must not touch it
	seedTransaction(t, db, &entity.Transaction{
		Id: 2, ConnectorId: 2, ChargePointId: "CP1", TimeStart: old, TimeStop: old,
		IsFinished: true, Reason: "stopped by system",
	})

	connection, err := db.connect()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.disconnect(connection)
	database := connection.Database(db.database)

	if err = migrationStuckTransactionsUp(db.ctx, database); err != nil {
		t.Fatalf("migration: %v", err)
	}
	if err = migrationStuckTransactionsDown(db.ctx, database); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	var reopened, untouched entity.Transaction
	if err = database.Collection(collectionTransactions).
		FindOne(db.ctx, bson.M{"transaction_id": 1}).Decode(&reopened); err != nil {
		t.Fatalf("read transaction 1: %v", err)
	}
	if reopened.IsFinished || reopened.Reason != "" {
		t.Errorf("transaction 1 should be reopened, got finished=%v reason=%q", reopened.IsFinished, reopened.Reason)
	}

	if err = database.Collection(collectionTransactions).
		FindOne(db.ctx, bson.M{"transaction_id": 2}).Decode(&untouched); err != nil {
		t.Fatalf("read transaction 2: %v", err)
	}
	if !untouched.IsFinished || untouched.Reason != "stopped by system" {
		t.Errorf("transaction 2 was not closed by the migration and must survive the rollback, got finished=%v reason=%q", untouched.IsFinished, untouched.Reason)
	}
}
