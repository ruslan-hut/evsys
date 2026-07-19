// Verification checks for migration 003 (abandoned transaction backlog).
//
// Run against the migrated database:
//   mongosh <uri> --file verify_003.js
//
// Exits non-zero if any check fails.

var fail = 0;

function check(label, ok, detail) {
	print((ok ? "  ok   " : "  FAIL ") + label + (detail ? " - " + detail : ""));
	if (!ok) fail++;
}

var sv = db.schema_version.findOne();
check("schema_version is at least 3", !!sv && sv.version >= 3, "got " + (sv ? sv.version : "none"));

var cutoff = new Date(Date.now() - 24 * 60 * 60 * 1000);

// The whole point: nothing older than the cutoff may still be open, except the
// rows deliberately left for manual review because they carry a payment amount.
var stillOpen = db.transactions.countDocuments({
	is_finished: false,
	time_start: { $lt: cutoff },
	payment_amount: { $lte: 0 }
});
check("no unbilled transaction left open past the cutoff", stillOpen === 0,
	stillOpen + " still open");

// A closed transaction must not leave its connector pinned, or the connector
// keeps rejecting new sessions even though the row itself looks finished.
var pinned = db.transactions.aggregate([
	{ $match: { reason: { $in: ["stopped by system (backlog)", "aborted by system (backlog)"] } } },
	{
		$lookup: {
			from: "connectors",
			let: { tid: "$transaction_id", tc: "$connector_id", tp: "$charge_point_id" },
			pipeline: [{
				$match: {
					$expr: {
						$and: [
							{ $eq: ["$charge_point_id", "$$tp"] },
							{ $eq: ["$connector_id", "$$tc"] },
							{ $eq: ["$current_transaction_id", "$$tid"] }
						]
					}
				}
			}],
			as: "pinned"
		}
	},
	{ $match: { "pinned.0": { $exists: true } } },
	{ $count: "n" }
]).toArray();
var pinnedCount = pinned.length > 0 ? pinned[0].n : 0;
check("no closed transaction still pins its connector", pinnedCount === 0,
	pinnedCount + " connector(s) still pinned");

// time_stop must never precede time_start, or duration goes negative downstream.
var negative = db.transactions.countDocuments({
	reason: { $in: ["stopped by system (backlog)", "aborted by system (backlog)"] },
	$expr: { $lt: ["$time_stop", "$time_start"] }
});
check("no closed transaction stops before it starts", negative === 0,
	negative + " with negative duration");

// Aborted rows carry no energy, so they must carry no charge either.
var billable = db.transactions.countDocuments({
	reason: "aborted by system (backlog)",
	payment_amount: { $gt: 0 }
});
check("no aborted transaction became billable", billable === 0,
	billable + " with a payment amount");

var closed = db.transactions.countDocuments({
	reason: { $in: ["stopped by system (backlog)", "aborted by system (backlog)"] }
});
print("");
print("  closed by this migration: " + closed);

var review = db.transactions.countDocuments({
	is_finished: false,
	time_start: { $lt: cutoff },
	payment_amount: { $gt: 0 }
});
if (review > 0) {
	print("  ! " + review + " transaction(s) left open for manual review");
}

print("");
if (fail > 0) {
	print("VERIFICATION FAILED: " + fail + " check(s) did not pass");
	quit(1);
}
print("migration 003 verified - abandoned transactions are closed");
