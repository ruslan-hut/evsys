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
	MigrationOCPPMultiVersion = 1 // OCPP multi-version support (Phase 2, Task 2.7)
	MigrationTriggerMessage   = 2 // Enable meter value triggering on existing charge points
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
