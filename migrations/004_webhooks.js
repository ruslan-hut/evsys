// Migration 004: Create indexes for webhook subscribers and outbox
//
// The webhook subsystem stores its recipients in webhook_subscribers and its
// pending deliveries in webhook_outbox. This migration only creates indexes:
// a unique subscriber name (the outbox references subscribers by name), the
// dispatcher's head-of-queue lookup, and a TTL that removes delivered outbox
// documents after 7 days. Failed documents have no delivered_at and are kept
// for inspection.
//
// Usage:
//   mongosh <database-name> 004_webhooks.js

print("========================================");
print("Migration 004: Webhook indexes");
print("========================================");
print("");

db.webhook_subscribers.createIndex(
    { name: 1 },
    { name: "name_unique", unique: true }
);
print("Created webhook_subscribers.name_unique");

db.webhook_outbox.createIndex(
    { subscriber: 1, status: 1, sequence: 1 },
    { name: "subscriber_status_sequence" }
);
print("Created webhook_outbox.subscriber_status_sequence");

db.webhook_outbox.createIndex(
    { delivered_at: 1 },
    { name: "delivered_at_ttl", expireAfterSeconds: 604800 }
);
print("Created webhook_outbox.delivered_at_ttl (7 days)");

// Update schema version
db.schema_version.replaceOne(
    {},
    { version: 4, updated_at: new Date() },
    { upsert: true }
);
print("");
print("Schema version updated to 4");
print("Migration 004 completed");
