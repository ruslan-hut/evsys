// Migration 002: Enable trigger_message on existing charge points
//
// Before the trigger_message flag existed the server always triggered
// MeterValues during a transaction. Charge point documents written before it
// have no such field, which decodes as false and silently stops the server
// from registering the connector with the trigger service - so no meter
// values are collected and finished transactions have an empty meter_values
// array. Backfilling true restores the original behaviour; the flag remains
// available as a per-charge-point opt-out.
//
// Usage:
//   mongosh <database-name> 002_trigger_message.js

print("========================================");
print("Migration 002: Enable trigger_message");
print("========================================");
print("");

// Only touch documents missing the field, so charge points explicitly set to
// false since the flag shipped keep their setting.
var missing = db.charge_points.countDocuments({ trigger_message: { $exists: false } });
print("Charge points without trigger_message: " + missing);

var result = db.charge_points.updateMany(
    { trigger_message: { $exists: false } },
    { $set: { trigger_message: true } }
);

print("   - Modified documents: " + result.modifiedCount);
print("   - Matched documents:  " + result.matchedCount);
print("");

print("Updating schema version...");
db.schema_version.replaceOne(
    {},
    {
        version: 2,
        updated_at: new Date()
    },
    { upsert: true }
);
print("   ✓ Schema version set to 2");
print("");

print("========================================");
print("✓ Migration completed successfully");
print("Enabled on " + result.modifiedCount + " charge point(s)");
print("========================================");
