// Verification for migration 004: webhook indexes
//
// Usage:
//   mongosh <database-name> verify_004.js

print("========================================");
print("Verify 004: Webhook indexes");
print("========================================");
print("");

let failures = 0;

function expectIndex(collection, indexName, expected) {
    const found = db.getCollection(collection).getIndexes().some(i => i.name === indexName);
    if (found === expected) {
        print("OK   " + collection + "." + indexName + (expected ? " present" : " absent"));
    } else {
        print("FAIL " + collection + "." + indexName + " expected " + (expected ? "present" : "absent"));
        failures++;
    }
}

expectIndex("webhook_subscribers", "name_unique", true);
expectIndex("webhook_outbox", "subscriber_status_sequence", true);
expectIndex("webhook_outbox", "delivered_at_ttl", true);

const ttl = db.webhook_outbox.getIndexes().find(i => i.name === "delivered_at_ttl");
if (ttl && ttl.expireAfterSeconds === 604800) {
    print("OK   webhook_outbox.delivered_at_ttl expires after 7 days");
} else if (ttl) {
    print("FAIL webhook_outbox.delivered_at_ttl expireAfterSeconds = " + ttl.expireAfterSeconds + ", expected 604800");
    failures++;
}

const version = db.schema_version.findOne();
if (version && version.version >= 4) {
    print("OK   schema version is " + version.version);
} else {
    print("FAIL schema version is " + (version ? version.version : "missing") + ", expected >= 4");
    failures++;
}

print("");
if (failures === 0) {
    print("Verification passed");
} else {
    print("Verification FAILED: " + failures + " problem(s)");
}
