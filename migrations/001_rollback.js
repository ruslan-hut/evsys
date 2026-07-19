// Migration 001 Rollback: Remove OCPP Multi-Version Support Fields
//
// This script rolls back the changes made by migration 001.
// Use with caution as this will remove data!
//
// Usage:
//   mongo <database-name> 001_rollback.js
//
// Or manually:
//   mongosh
//   use evsys
//   load("001_rollback.js")

print("========================================");
print("Migration 001 ROLLBACK: Remove OCPP Multi-Version Support");
print("========================================");
print("");
print("⚠ WARNING: This will remove protocol_version, evse_id, and metadata fields!");
print("");

// ============================================================================
// 1. Remove fields from charge_points
// ============================================================================
print("1. Removing fields from charge_points...");

var chargePointsResult = db.charge_points.updateMany(
    {},
    {
        $unset: {
            protocol_version: "",
            device_model: ""
        }
    }
);

print("   - Modified documents: " + chargePointsResult.modifiedCount);

// Drop index
print("   - Dropping protocol_version index...");
try {
    db.charge_points.dropIndex("protocol_version_1");
    print("   ✓ Index dropped");
} catch (e) {
    print("   ⚠ Index drop warning: " + e.message);
}

print("");

// ============================================================================
// 2. Remove fields from connectors
// ============================================================================
print("2. Removing fields from connectors...");

var connectorsResult = db.connectors.updateMany(
    {},
    {
        $unset: {
            evse_id: ""
        }
    }
);

print("   - Modified documents: " + connectorsResult.modifiedCount);

// Drop index
print("   - Dropping compound index...");
try {
    db.connectors.dropIndex("charge_point_evse_1");
    print("   ✓ Index dropped");
} catch (e) {
    print("   ⚠ Index drop warning: " + e.message);
}

print("");

// ============================================================================
// 3. Remove fields from transactions
// ============================================================================
print("3. Removing fields from transactions...");

var transactionsResult = db.transactions.updateMany(
    {},
    {
        $unset: {
            protocol_version: "",
            evse_id: "",
            metadata: ""
        }
    }
);

print("   - Modified documents: " + transactionsResult.modifiedCount);

// Drop index
print("   - Dropping protocol_version index...");
try {
    db.transactions.dropIndex("protocol_version_1");
    print("   ✓ Index dropped");
} catch (e) {
    print("   ⚠ Index drop warning: " + e.message);
}

print("");

// ============================================================================
// 4. Update schema version
// ============================================================================
print("4. Resetting schema version...");

db.schema_version.replaceOne(
    {},
    {
        version: 0,
        updated_at: new Date()
    },
    { upsert: true }
);

print("   ✓ Schema version reset to 0");
print("");

// ============================================================================
// Summary
// ============================================================================
print("========================================");
print("Rollback Summary:");
print("========================================");
print("Charge Points modified:  " + chargePointsResult.modifiedCount);
print("Connectors modified:     " + connectorsResult.modifiedCount);
print("Transactions modified:   " + transactionsResult.modifiedCount);
print("");
print("✓ Rollback completed successfully");
print("========================================");
