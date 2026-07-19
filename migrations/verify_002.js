// Verification checks for migration 002 (trigger_message backfill).
//
// Run against the migrated database:
//   mongosh <uri> --file verify_002.js
//
// Exits non-zero if any check fails.

var fail = 0;

function check(label, ok, detail) {
	print((ok ? "  ok   " : "  FAIL ") + label + (detail ? " - " + detail : ""));
	if (!ok) fail++;
}

var sv = db.schema_version.findOne();
check("schema_version is at least 2", !!sv && sv.version >= 2, "got " + (sv ? sv.version : "none"));

// The whole point: no charge point may be left without the field, since a
// missing field decodes as false and disables meter value triggering.
var missing = db.charge_points.countDocuments({ trigger_message: { $exists: false } });
check("no charge point missing trigger_message", missing === 0, missing + " still missing");

var total = db.charge_points.countDocuments({});
var enabled = db.charge_points.countDocuments({ trigger_message: true });
check("at least one charge point has triggering enabled", enabled > 0,
	enabled + " of " + total + " enabled");

print("");
if (fail > 0) {
	print("VERIFICATION FAILED: " + fail + " check(s) did not pass");
	quit(1);
}
print("migration 002 verified - meter value triggering is enabled");
