// Migration 001: Add OCPP Multi-Version Support Fields
// Phase 2, Task 2.7 - Database Schema Updates
//
// This migration adds support for multiple OCPP protocol versions (1.6J, 2.0.1, 2.1)
// by adding protocol_version, evse_id, and metadata fields to relevant collections.
//
// Usage:
//   mongo <database-name> 001_ocpp_multiversion.js
//
// Or manually:
//   mongosh
//   use evsys
//   load("001_ocpp_multiversion.js")

print("========================================");
print("Migration 001: OCPP Multi-Version Support");
print("========================================");
print("");

// ============================================================================
// 1. Update charge_points collection
// ============================================================================
print("1. Updating charge_points collection...");

var chargePointsResult = db.charge_points.updateMany(
    { protocol_version: { $exists: false } },
    {
        $set: {
            protocol_version: "ocpp1.6",
            device_model: {}
        }
    }
);

print("   - Modified documents: " + chargePointsResult.modifiedCount);
print("   - Matched documents: " + chargePointsResult.matchedCount);

// Create index on protocol_version
print("   - Creating index on protocol_version...");
try {
    db.charge_points.createIndex(
        { protocol_version: 1 },
        { name: "protocol_version_1", background: true }
    );
    print("   ✓ Index created successfully");
} catch (e) {
    print("   ⚠ Index creation warning: " + e.message);
}

print("");

// ============================================================================
// 2. Update connectors collection
// ============================================================================
print("2. Updating connectors collection...");

var connectorsResult = db.connectors.updateMany(
    { evse_id: { $exists: false } },
    {
        $set: {
            evse_id: null
        }
    }
);

print("   - Modified documents: " + connectorsResult.modifiedCount);
print("   - Matched documents: " + connectorsResult.matchedCount);

// Create compound index on (charge_point_id, evse_id)
print("   - Creating compound index on (charge_point_id, evse_id)...");
try {
    db.connectors.createIndex(
        { charge_point_id: 1, evse_id: 1 },
        { name: "charge_point_evse_1", background: true }
    );
    print("   ✓ Index created successfully");
} catch (e) {
    print("   ⚠ Index creation warning: " + e.message);
}

print("");

// ============================================================================
// 3. Update transactions collection
// ============================================================================
print("3. Updating transactions collection...");

var transactionsResult = db.transactions.updateMany(
    { protocol_version: { $exists: false } },
    {
        $set: {
            protocol_version: "ocpp1.6",
            evse_id: null,
            metadata: {}
        }
    }
);

print("   - Modified documents: " + transactionsResult.modifiedCount);
print("   - Matched documents: " + transactionsResult.matchedCount);

// Create index on protocol_version
print("   - Creating index on protocol_version...");
try {
    db.transactions.createIndex(
        { protocol_version: 1 },
        { name: "protocol_version_1", background: true }
    );
    print("   ✓ Index created successfully");
} catch (e) {
    print("   ⚠ Index creation warning: " + e.message);
}

print("");

// ============================================================================
// 4. Update schema version
// ============================================================================
print("4. Updating schema version...");

db.schema_version.replaceOne(
    {},
    {
        version: 1,
        updated_at: new Date()
    },
    { upsert: true }
);

print("   ✓ Schema version set to 1");
print("");

// ============================================================================
// Summary
// ============================================================================
print("========================================");
print("Migration Summary:");
print("========================================");
print("Charge Points updated:  " + chargePointsResult.modifiedCount);
print("Connectors updated:     " + connectorsResult.modifiedCount);
print("Transactions updated:   " + transactionsResult.modifiedCount);
print("");
print("✓ Migration completed successfully");
print("========================================");
