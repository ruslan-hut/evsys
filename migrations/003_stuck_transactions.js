// Migration 003: Close transactions abandoned before the sweeper could reach them
//
// GetUnfinishedTransactions used to exclude any transaction whose connector
// still pointed at it, but that pointer is only cleared on a normal stop. A
// lost StopTransaction therefore left both the flag and the pointer set, and
// the pair excluded itself from every sweep. Those rows keep their connector
// pinned, which makes OnStartTransaction answer new sessions with the dead
// transaction id instead of starting a real one.
//
// Only transactions idle for more than 24 hours are touched, so a session that
// is live at deploy time is left to the runtime sweeper. Rows that already
// carry a payment amount are skipped and reported: marking one finished would
// expose it to the billing worker downstream, and a months-old session must not
// be charged without a look first.
//
// Usage:
//   mongosh <database-name> 003_stuck_transactions.js

print("========================================");
print("Migration 003: Close abandoned transactions");
print("========================================");
print("");

var CUTOFF_HOURS = 24;
var cutoff = new Date(Date.now() - CUTOFF_HOURS * 60 * 60 * 1000);
print("Cutoff: " + cutoff.toISOString() + " (" + CUTOFF_HOURS + "h)");

var candidates = db.transactions.find({
    is_finished: false,
    time_start: { $lt: cutoff }
});

var closed = 0;
var skippedActive = 0;
var skippedBilled = 0;
var released = 0;

candidates.forEach(function (transaction) {
    if (transaction.payment_amount > 0) {
        print("   ! transaction " + transaction.transaction_id +
            " has payment_amount " + transaction.payment_amount +
            ", skipping for manual review");
        skippedBilled++;
        return;
    }

    var meterStop = transaction.meter_start || 0;
    var timeStop = transaction.time_start;
    var reason = "aborted by system (backlog)";

    var meterValue = db.meter_values
        .find({ transaction_id: transaction.transaction_id })
        .sort({ time: -1 })
        .limit(1)
        .toArray()[0];

    if (meterValue) {
        // a sample newer than the cutoff means the session is still delivering energy
        if (meterValue.time > cutoff) {
            skippedActive++;
            return;
        }
        meterStop = meterValue.value;
        timeStop = meterValue.time;
        reason = "stopped by system (backlog)";
    }

    db.transactions.updateOne(
        { transaction_id: transaction.transaction_id },
        {
            $set: {
                is_finished: true,
                time_stop: timeStop,
                meter_stop: meterStop,
                reason: reason
            }
        }
    );

    // release the connector this transaction was holding, matching on the id so
    // a connector that has since moved on to a live session is left alone
    var connectorResult = db.connectors.updateOne(
        {
            charge_point_id: transaction.charge_point_id,
            connector_id: transaction.connector_id,
            current_transaction_id: transaction.transaction_id
        },
        {
            $set: {
                current_transaction_id: -1,
                current_power_limit: 0
            }
        }
    );
    released += connectorResult.modifiedCount;

    closed++;
});

print("");
print("   - Closed transactions:   " + closed);
print("   - Connectors released:   " + released);
print("   - Still active, skipped: " + skippedActive);
print("   - With payment amount:   " + skippedBilled);
print("");

print("Updating schema version...");
db.schema_version.replaceOne(
    {},
    {
        version: 3,
        updated_at: new Date()
    },
    { upsert: true }
);
print("   ✓ Schema version set to 3");
print("");

print("========================================");
print("✓ Migration completed successfully");
print("Closed " + closed + " abandoned transaction(s)");
if (skippedBilled > 0) {
    print("! " + skippedBilled + " transaction(s) need manual review");
}
print("========================================");
