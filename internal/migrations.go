package internal

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Migration represents a database schema migration
type Migration struct {
	Version     int
	Description string
	Up          func(ctx context.Context, db *mongo.Database) error
	Down        func(ctx context.Context, db *mongo.Database) error
}

const (
	collectionSchema = "schema_version"

	// Migration version constants
	MigrationOCPPMultiVersion  = 1 // OCPP multi-version support (Phase 2, Task 2.7)
	MigrationTriggerMessage    = 2 // Enable meter value triggering on existing charge points
	MigrationStuckTransactions = 3 // Close transactions abandoned before the sweeper was fixed

	// stuckTransactionCutoff is how far back a transaction must have been idle to count as
	// backlog. The runtime sweeper handles anything more recent, so this only has to be long
	// enough that a session live at deploy time is never touched.
	stuckTransactionCutoff = 24 * time.Hour

	// Reasons written by the backlog migration. They double as the marker the rollback matches
	// on, so they must stay distinct from the reasons the runtime sweeper writes.
	reasonBacklogStopped = "stopped by system (backlog)"
	reasonBacklogAborted = "aborted by system (backlog)"
)

// SchemaVersion tracks the current database schema version
type SchemaVersion struct {
	Version   int       `bson:"version"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// GetMigrations returns all available migrations in order
func GetMigrations() []Migration {
	return []Migration{
		{
			Version:     MigrationOCPPMultiVersion,
			Description: "Add OCPP multi-version support fields (protocol_version, evse_id, metadata)",
			Up:          migrationOCPPMultiVersionUp,
			Down:        migrationOCPPMultiVersionDown,
		},
		{
			Version:     MigrationTriggerMessage,
			Description: "Enable trigger_message on charge points that predate the flag",
			Up:          migrationTriggerMessageUp,
			Down:        migrationTriggerMessageDown,
		},
		{
			Version:     MigrationStuckTransactions,
			Description: "Close transactions abandoned while the sweeper could not reach them",
			Up:          migrationStuckTransactionsUp,
			Down:        migrationStuckTransactionsDown,
		},
	}
}

// migrationOCPPMultiVersionUp adds fields for OCPP multi-version support
func migrationOCPPMultiVersionUp(ctx context.Context, db *mongo.Database) error {
	log.Println("Running migration: Add OCPP multi-version support fields")

	// 1. Add protocol_version to charge_points (default to "ocpp1.6" for existing)
	chargePointsCollection := db.Collection("charge_points")
	updateResult, err := chargePointsCollection.UpdateMany(
		ctx,
		bson.M{"protocol_version": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{
			"protocol_version": "ocpp1.6",
			"device_model":     bson.M{},
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update charge_points: %w", err)
	}
	log.Printf("Updated %d charge points with protocol_version", updateResult.ModifiedCount)

	// 2. Add evse_id to connectors (set to null for OCPP 1.6 compatibility)
	connectorsCollection := db.Collection("connectors")
	updateResult, err = connectorsCollection.UpdateMany(
		ctx,
		bson.M{"evse_id": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{
			"evse_id": nil,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update connectors: %w", err)
	}
	log.Printf("Updated %d connectors with evse_id field", updateResult.ModifiedCount)

	// 3. Add protocol_version, evse_id, and metadata to transactions
	transactionsCollection := db.Collection("transactions")
	updateResult, err = transactionsCollection.UpdateMany(
		ctx,
		bson.M{"protocol_version": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{
			"protocol_version": "ocpp1.6",
			"evse_id":          nil,
			"metadata":         bson.M{},
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update transactions: %w", err)
	}
	log.Printf("Updated %d transactions with multi-version fields", updateResult.ModifiedCount)

	// 4. Create indexes for better query performance
	log.Println("Creating indexes for new fields...")

	// Index on charge_points.protocol_version
	_, err = chargePointsCollection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "protocol_version", Value: 1}},
			Options: options.Index().SetName("protocol_version_1").SetBackground(true),
		},
	)
	if err != nil {
		log.Printf("Warning: failed to create index on charge_points.protocol_version: %v", err)
		// Don't fail migration for index creation errors
	}

	// Index on transactions.protocol_version
	_, err = transactionsCollection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "protocol_version", Value: 1}},
			Options: options.Index().SetName("protocol_version_1").SetBackground(true),
		},
	)
	if err != nil {
		log.Printf("Warning: failed to create index on transactions.protocol_version: %v", err)
	}

	// Compound index on connectors (charge_point_id, evse_id)
	_, err = connectorsCollection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{Key: "charge_point_id", Value: 1},
				{Key: "evse_id", Value: 1},
			},
			Options: options.Index().SetName("charge_point_evse_1").SetBackground(true),
		},
	)
	if err != nil {
		log.Printf("Warning: failed to create compound index on connectors: %v", err)
	}

	log.Println("Migration completed successfully")
	return nil
}

// migrationOCPPMultiVersionDown removes OCPP multi-version support fields
func migrationOCPPMultiVersionDown(ctx context.Context, db *mongo.Database) error {
	log.Println("Rolling back migration: Remove OCPP multi-version support fields")

	// Remove fields from charge_points
	chargePointsCollection := db.Collection("charge_points")
	_, err := chargePointsCollection.UpdateMany(
		ctx,
		bson.M{},
		bson.M{"$unset": bson.M{
			"protocol_version": "",
			"device_model":     "",
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to rollback charge_points: %w", err)
	}

	// Remove fields from connectors
	connectorsCollection := db.Collection("connectors")
	_, err = connectorsCollection.UpdateMany(
		ctx,
		bson.M{},
		bson.M{"$unset": bson.M{
			"evse_id": "",
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to rollback connectors: %w", err)
	}

	// Remove fields from transactions
	transactionsCollection := db.Collection("transactions")
	_, err = transactionsCollection.UpdateMany(
		ctx,
		bson.M{},
		bson.M{"$unset": bson.M{
			"protocol_version": "",
			"evse_id":          "",
			"metadata":         "",
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to rollback transactions: %w", err)
	}

	// Drop indexes
	_, _ = chargePointsCollection.Indexes().DropOne(ctx, "protocol_version_1")
	_, _ = transactionsCollection.Indexes().DropOne(ctx, "protocol_version_1")
	_, _ = connectorsCollection.Indexes().DropOne(ctx, "charge_point_evse_1")

	log.Println("Rollback completed successfully")
	return nil
}

// migrationTriggerMessageUp enables meter value triggering on charge points
// that predate the trigger_message flag.
//
// Before the flag existed the server always triggered MeterValues during a
// transaction. Documents written before it decode as false, which silently
// stops checkListenTransaction from registering the connector, so no samples
// are collected and finished transactions end up with no meter values.
// Backfilling true restores the original behaviour; the flag stays available
// as a per-charge-point opt-out.
func migrationTriggerMessageUp(ctx context.Context, db *mongo.Database) error {
	log.Println("Running migration: Enable trigger_message on existing charge points")

	// Only touch documents missing the field, so points explicitly set to
	// false since the flag shipped keep their setting.
	result, err := db.Collection("charge_points").UpdateMany(
		ctx,
		bson.M{"trigger_message": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"trigger_message": true}},
	)
	if err != nil {
		return fmt.Errorf("failed to update charge_points: %w", err)
	}
	log.Printf("Enabled trigger_message on %d charge points", result.ModifiedCount)

	return nil
}

/*
migrationStuckTransactionsUp closes transactions that were abandoned before the sweeper could
reach them.

GetUnfinishedTransactions used to exclude any transaction whose connector still pointed at it, but
that pointer is only cleared on a normal stop. A lost StopTransaction therefore left both the flag
and the pointer set, and the pair excluded itself from every sweep. Those rows keep their connector
pinned, which makes OnStartTransaction answer new sessions with the dead transaction id.

Only transactions idle for longer than stuckTransactionCutoff are touched, so a session that is
live at deploy time is left to the runtime sweeper. Rows that already carry a payment amount are
skipped and reported: marking one finished would expose it to the billing worker downstream, and a
months-old session must not be charged without a look first.
*/
func migrationStuckTransactionsUp(ctx context.Context, db *mongo.Database) error {
	log.Println("Running migration: Close abandoned transactions")

	cutoff := time.Now().Add(-stuckTransactionCutoff)
	transactions := db.Collection("transactions")

	cursor, err := transactions.Find(ctx, bson.M{
		"is_finished": false,
		"time_start":  bson.M{"$lt": cutoff},
	})
	if err != nil {
		return fmt.Errorf("failed to read unfinished transactions: %w", err)
	}

	// read the candidates out before writing, so the updates below cannot disturb the cursor
	var candidates []struct {
		Id            int       `bson:"transaction_id"`
		ChargePointId string    `bson:"charge_point_id"`
		ConnectorId   int       `bson:"connector_id"`
		MeterStart    int       `bson:"meter_start"`
		TimeStart     time.Time `bson:"time_start"`
		PaymentAmount int       `bson:"payment_amount"`
	}
	if err = cursor.All(ctx, &candidates); err != nil {
		return fmt.Errorf("failed to read unfinished transactions: %w", err)
	}
	log.Printf("Found %d transaction(s) idle since before %s", len(candidates), cutoff.Format(time.RFC3339))

	var closed, skippedActive, skippedBilled int
	for _, transaction := range candidates {
		if transaction.PaymentAmount > 0 {
			log.Printf("Transaction %d has payment_amount %d, skipping for manual review",
				transaction.Id, transaction.PaymentAmount)
			skippedBilled++
			continue
		}

		meterStop := transaction.MeterStart
		timeStop := transaction.TimeStart
		reason := reasonBacklogAborted

		var meterValue struct {
			Value int       `bson:"value"`
			Time  time.Time `bson:"time"`
		}
		err = db.Collection("meter_values").FindOne(
			ctx,
			bson.M{"transaction_id": transaction.Id},
			options.FindOne().SetSort(bson.D{{Key: "time", Value: -1}}),
		).Decode(&meterValue)
		if err != nil && err != mongo.ErrNoDocuments {
			return fmt.Errorf("failed to read meter values of transaction %d: %w", transaction.Id, err)
		}
		if err == nil {
			// a sample newer than the cutoff means the session is still delivering energy
			if meterValue.Time.After(cutoff) {
				skippedActive++
				continue
			}
			meterStop = meterValue.Value
			timeStop = meterValue.Time
			reason = reasonBacklogStopped
		}

		_, err = transactions.UpdateOne(
			ctx,
			bson.M{"transaction_id": transaction.Id},
			bson.M{"$set": bson.M{
				"is_finished": true,
				"time_stop":   timeStop,
				"meter_stop":  meterStop,
				"reason":      reason,
			}},
		)
		if err != nil {
			return fmt.Errorf("failed to close transaction %d: %w", transaction.Id, err)
		}

		// release the connector this transaction was holding, matching on the id so a
		// connector that has since moved on to a live session is left alone
		_, err = db.Collection("connectors").UpdateOne(
			ctx,
			bson.M{
				"charge_point_id":        transaction.ChargePointId,
				"connector_id":           transaction.ConnectorId,
				"current_transaction_id": transaction.Id,
			},
			bson.M{"$set": bson.M{
				"current_transaction_id": -1,
				"current_power_limit":    0,
			}},
		)
		if err != nil {
			return fmt.Errorf("failed to release connector of transaction %d: %w", transaction.Id, err)
		}

		closed++
	}

	log.Printf("Closed %d abandoned transactions (%d still active, %d skipped with a payment amount)",
		closed, skippedActive, skippedBilled)

	orphaned, err := releaseOrphanedConnectors(ctx, db)
	if err != nil {
		return err
	}
	log.Printf("Released %d connector(s) pointing at a finished transaction", orphaned)

	return nil
}

/*
releaseOrphanedConnectors clears connector pointers that reference an already finished transaction.

These are the residue of the sweeper as it behaved before it released connectors: it marked the
transaction finished and left the pointer set. The pass above cannot reach them, because it starts
from transactions that are still open, and neither can anything at runtime except the connector
itself being used again. Until the pointer is cleared, OnStartTransaction answers every session on
that connector with ConcurrentTx.

Pointers to a transaction that does not exist at all are left alone: OnStartTransaction overwrites
those on the next start, and clearing them here would also swallow a pointer left dangling by a
failed lookup.
*/
func releaseOrphanedConnectors(ctx context.Context, db *mongo.Database) (int, error) {
	connectors := db.Collection("connectors")

	cursor, err := connectors.Find(ctx, bson.M{"current_transaction_id": bson.M{"$gte": 0}})
	if err != nil {
		return 0, fmt.Errorf("failed to read connectors: %w", err)
	}

	var pinned []struct {
		Id            int    `bson:"connector_id"`
		ChargePointId string `bson:"charge_point_id"`
		TransactionId int    `bson:"current_transaction_id"`
	}
	if err = cursor.All(ctx, &pinned); err != nil {
		return 0, fmt.Errorf("failed to read connectors: %w", err)
	}

	released := 0
	for _, connector := range pinned {
		count, err := db.Collection("transactions").CountDocuments(ctx, bson.M{
			"transaction_id": connector.TransactionId,
			"is_finished":    true,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to read transaction %d: %w", connector.TransactionId, err)
		}
		if count == 0 {
			continue
		}

		log.Printf("Connector %d of %s points at finished transaction %d, releasing",
			connector.Id, connector.ChargePointId, connector.TransactionId)

		_, err = connectors.UpdateOne(
			ctx,
			bson.M{
				"charge_point_id":        connector.ChargePointId,
				"connector_id":           connector.Id,
				"current_transaction_id": connector.TransactionId,
			},
			bson.M{"$set": bson.M{
				"current_transaction_id": -1,
				"current_power_limit":    0,
			}},
		)
		if err != nil {
			return 0, fmt.Errorf("failed to release connector %d of %s: %w",
				connector.Id, connector.ChargePointId, err)
		}
		released++
	}

	return released, nil
}

// migrationStuckTransactionsDown reopens the transactions this migration closed, matching on the
// reasons it wrote. It cannot restore the connector pointers it cleared: by the time a rollback
// runs those connectors may have started real sessions, and repinning them to a dead transaction
// is the failure this migration exists to undo.
func migrationStuckTransactionsDown(ctx context.Context, db *mongo.Database) error {
	log.Println("Rolling back migration: Reopen transactions closed as abandoned")

	result, err := db.Collection("transactions").UpdateMany(
		ctx,
		bson.M{"reason": bson.M{"$in": bson.A{reasonBacklogStopped, reasonBacklogAborted}}},
		bson.M{"$set": bson.M{
			"is_finished": false,
			"time_stop":   time.Time{},
			"reason":      "",
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to rollback transactions: %w", err)
	}
	log.Printf("Reopened %d transactions; connector pointers were not restored", result.ModifiedCount)

	return nil
}

// migrationTriggerMessageDown removes the trigger_message field again. It can
// only match on the value, so a charge point enabled by hand after the
// migration ran is indistinguishable from one this migration set.
func migrationTriggerMessageDown(ctx context.Context, db *mongo.Database) error {
	log.Println("Rolling back migration: Remove trigger_message from charge points")

	result, err := db.Collection("charge_points").UpdateMany(
		ctx,
		bson.M{"trigger_message": true},
		bson.M{"$unset": bson.M{"trigger_message": ""}},
	)
	if err != nil {
		return fmt.Errorf("failed to rollback charge_points: %w", err)
	}
	log.Printf("Removed trigger_message from %d charge points", result.ModifiedCount)

	return nil
}
