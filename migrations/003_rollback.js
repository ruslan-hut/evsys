// Rollback for Migration 003: reopen transactions closed as abandoned
//
// Matches on the reasons the migration wrote, which the runtime sweeper never
// produces. It cannot restore the connector pointers the migration cleared: by
// the time a rollback runs those connectors may have started real sessions, and
// repinning them to a dead transaction is the failure the migration undoes.
//
// Usage:
//   mongosh <database-name> 003_rollback.js

print("========================================");
print("Rollback 003: Reopen abandoned transactions");
print("========================================");
print("");

var result = db.transactions.updateMany(
    { reason: { $in: ["stopped by system (backlog)", "aborted by system (backlog)"] } },
    {
        $set: {
            is_finished: false,
            time_stop: new Date("0001-01-01T00:00:00Z"),
            reason: ""
        }
    }
);

print("   - Modified documents: " + result.modifiedCount);
print("   ! Connector pointers were not restored");
print("");

print("Resetting schema version...");
db.schema_version.replaceOne(
    {},
    {
        version: 2,
        updated_at: new Date()
    },
    { upsert: true }
);
print("   ✓ Schema version reset to 2");
print("");

print("========================================");
print("✓ Rollback completed successfully");
print("========================================");
