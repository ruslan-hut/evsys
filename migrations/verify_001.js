// Verification checks for migration 001 (OCPP multi-version support).
//
// Run against the migrated database. Optionally define PRE beforehand to also
// assert document counts are unchanged:
//
//   mongosh <uri> --eval "var PRE=$(cat pre-state.json)" --file verify_001.js
//
// Exits non-zero if any check fails.

var fail = 0;

function check(label, ok, detail) {
	print((ok ? "  ok   " : "  FAIL ") + label + (detail ? " - " + detail : ""));
	if (!ok) fail++;
}

// At least 1: later migrations move the version on, and these checks must
// still hold once they have.
var sv = db.schema_version.findOne();
check("schema_version is at least 1", !!sv && sv.version >= 1, "got " + (sv ? sv.version : "none"));

// Document counts must be untouched: migration 001 is additive only.
if (typeof PRE !== "undefined" && PRE && PRE.counts) {
	Object.keys(PRE.counts).forEach(function (c) {
		var now = db.getCollection(c).countDocuments({});
		check(c + " count unchanged", now === PRE.counts[c], PRE.counts[c] + " -> " + now);
	});
} else {
	print("  skip  document count comparison (no pre-migration state supplied)");
}

// Every document must have picked up the new fields.
check("charge_points.protocol_version backfilled",
	db.charge_points.countDocuments({ protocol_version: { $exists: false } }) === 0);
check("charge_points.device_model backfilled",
	db.charge_points.countDocuments({ device_model: { $exists: false } }) === 0);
check("connectors.evse_id backfilled",
	db.connectors.countDocuments({ evse_id: { $exists: false } }) === 0);
check("transactions.protocol_version backfilled",
	db.transactions.countDocuments({ protocol_version: { $exists: false } }) === 0);
check("transactions.metadata backfilled",
	db.transactions.countDocuments({ metadata: { $exists: false } }) === 0);

// Existing rows must be tagged as 1.6, not left blank or mislabelled.
var wrong = db.charge_points.countDocuments({ protocol_version: { $ne: "ocpp1.6" } });
check("all existing charge_points are ocpp1.6", wrong === 0, wrong + " tagged otherwise");

function hasIndex(coll, name) {
	return db.getCollection(coll).getIndexes().some(function (i) { return i.name === name; });
}
check("charge_points.protocol_version_1 index", hasIndex("charge_points", "protocol_version_1"));
check("transactions.protocol_version_1 index", hasIndex("transactions", "protocol_version_1"));
check("connectors.charge_point_evse_1 index", hasIndex("connectors", "charge_point_evse_1"));

print("");
if (fail > 0) {
	print("REHEARSAL FAILED: " + fail + " check(s) did not pass");
	quit(1);
}
print("rehearsal passed - migration 001 is clean on production data");
