// Rollback for Migration 002: remove trigger_message from charge points
//
// This can only match on the value, so a charge point enabled by hand after
// the migration ran is indistinguishable from one the migration set.
//
// Usage:
//   mongosh <database-name> 002_rollback.js

print("========================================");
print("Rollback 002: Remove trigger_message");
print("========================================");
print("");

var result = db.charge_points.updateMany(
    { trigger_message: true },
    { $unset: { trigger_message: "" } }
);

print("   - Modified documents: " + result.modifiedCount);
print("");

print("Resetting schema version...");
db.schema_version.replaceOne(
    {},
    {
        version: 1,
        updated_at: new Date()
    },
    { upsert: true }
);
print("   ✓ Schema version reset to 1");
print("");

print("========================================");
print("✓ Rollback completed successfully");
print("========================================");
