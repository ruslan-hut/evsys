// Rollback for migration 004: Drop the webhook indexes
//
// The collections and their data are left in place: subscribers are operator
// configuration, and outbox history may still be wanted for inspection.
//
// Usage:
//   mongosh <database-name> 004_rollback.js

print("========================================");
print("Rollback 004: Drop webhook indexes");
print("========================================");
print("");

try { db.webhook_subscribers.dropIndex("name_unique"); print("Dropped webhook_subscribers.name_unique"); }
catch (e) { print("webhook_subscribers.name_unique not present: " + e.message); }

try { db.webhook_outbox.dropIndex("subscriber_status_sequence"); print("Dropped webhook_outbox.subscriber_status_sequence"); }
catch (e) { print("webhook_outbox.subscriber_status_sequence not present: " + e.message); }

try { db.webhook_outbox.dropIndex("delivered_at_ttl"); print("Dropped webhook_outbox.delivered_at_ttl"); }
catch (e) { print("webhook_outbox.delivered_at_ttl not present: " + e.message); }

// Update schema version
db.schema_version.replaceOne(
    {},
    { version: 3, updated_at: new Date() },
    { upsert: true }
);
print("");
print("Schema version set back to 3");
print("Rollback 004 completed");
