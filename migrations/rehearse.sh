#!/usr/bin/env bash
# Rehearse migration 001 against a copy of the production database.
#
# Never writes to the source database: it is only ever read by mongodump.
#
#   ./rehearse.sh prepare              # dump prod -> restore as rehearsal copy -> record pre-state
#   <start evsys-m against the copy>   # the app applies migration 001 on boot
#   ./rehearse.sh verify               # compare post-state against pre-state
#   ./rehearse.sh cleanup              # drop the rehearsal copy and the dump
#
# Configure via env vars:
#   PROD_DB     name of the production database to copy   (required)
#   COPY_DB     name of the rehearsal copy                (default: <PROD_DB>_rehearsal)
#   MONGO_HOST  host:port of the mongod                   (default: localhost:27017)
#   MONGO_USER  username                                  (optional; omit for unauthenticated)
#   MONGO_PASS  password                                  (required when MONGO_USER is set)
#   AUTH_DB     authentication database                   (default: PROD_DB)
#   WORK_DIR    scratch space for the dump                (default: /tmp/evsys-rehearsal)
#
# AUTH_DB defaults to PROD_DB because the application authenticates against its
# own database, not admin (internal/mongo.go, AuthSource: conf.Mongo.Database).
# For the same reason the copy needs its own user - see the note printed by
# 'prepare', otherwise the binary cannot connect to the rehearsal database.

set -euo pipefail

PROD_DB="${PROD_DB:-}"
COPY_DB="${COPY_DB:-${PROD_DB}_rehearsal}"
MONGO_HOST="${MONGO_HOST:-localhost:27017}"
MONGO_USER="${MONGO_USER:-}"
MONGO_PASS="${MONGO_PASS:-}"
AUTH_DB="${AUTH_DB:-$PROD_DB}"
WORK_DIR="${WORK_DIR:-/tmp/evsys-rehearsal}"
STATE_FILE="$WORK_DIR/pre-state.json"

COLLECTIONS=(charge_points connectors transactions)

die() { echo "error: $*" >&2; exit 1; }

[[ -n "$PROD_DB" ]] || die "PROD_DB is not set"
[[ "$COPY_DB" != "$PROD_DB" ]] || die "COPY_DB must differ from PROD_DB - refusing to touch production"
if [[ -n "$MONGO_USER" && -z "$MONGO_PASS" ]]; then
	die "MONGO_USER is set but MONGO_PASS is empty"
fi

# Credentials as argv, not embedded in a URI, so passwords with reserved
# characters need no percent-encoding.
AUTH_ARGS=()
if [[ -n "$MONGO_USER" ]]; then
	AUTH_ARGS=(-u "$MONGO_USER" -p "$MONGO_PASS" --authenticationDatabase "$AUTH_DB")
fi

# Run a mongosh snippet against a database and print its output.
db_eval() {
	local target="$1"; shift
	mongosh "mongodb://$MONGO_HOST/$target" "${AUTH_ARGS[@]}" --quiet --eval "$1"
}

copy_eval() { db_eval "$COPY_DB" "$1"; }

# Quote a value as a JS string literal, escaping backslashes and double quotes.
js_string() {
	local s=${1//\\/\\\\}
	printf '"%s"' "${s//\"/\\\"}"
}

# Emit {collection: count} plus the current schema version, as JSON.
snapshot() {
	copy_eval '
		var out = { schema_version: null, counts: {} };
		var sv = db.schema_version.findOne();
		out.schema_version = sv ? sv.version : null;
		['"$(printf '"%s",' "${COLLECTIONS[@]}")"'].forEach(function (c) {
			out.counts[c] = db.getCollection(c).countDocuments({});
		});
		JSON.stringify(out);
	'
}

cmd_prepare() {
	command -v mongodump >/dev/null || die "mongodump not found"
	command -v mongorestore >/dev/null || die "mongorestore not found"
	command -v mongosh >/dev/null || die "mongosh not found"

	echo "==> dumping $PROD_DB (read-only)"
	rm -rf "$WORK_DIR"
	mkdir -p "$WORK_DIR"
	mongodump --host="$MONGO_HOST" "${AUTH_ARGS[@]}" \
		--db="$PROD_DB" --out="$WORK_DIR/dump"

	echo "==> restoring into $COPY_DB"
	mongorestore --host="$MONGO_HOST" "${AUTH_ARGS[@]}" \
		--nsFrom="$PROD_DB.*" --nsTo="$COPY_DB.*" \
		--drop "$WORK_DIR/dump"

	echo "==> recording pre-migration state"
	snapshot > "$STATE_FILE"
	cat "$STATE_FILE"

	local version
	version=$(copy_eval 'var s = db.schema_version.findOne(); print(s ? s.version : "none")')
	if [[ "$version" != "none" && "$version" != "0" ]]; then
		echo
		echo "warning: copy already reports schema_version=$version"
		echo "         migration 001 will be skipped as already applied - check PROD_DB is correct"
	fi

	if [[ -n "$MONGO_USER" ]]; then
		echo
		echo "==> granting $MONGO_USER access to $COPY_DB"
		# The app authenticates with authSource set to its own database, so the
		# copy needs a user of its own - the prod one is scoped to $AUTH_DB.
		copy_eval '
			var user = '"$(js_string "$MONGO_USER")"';
			var pass = '"$(js_string "$MONGO_PASS")"';
			if (db.getUser(user)) {
				print("  user already exists on this database");
			} else {
				db.createUser({ user: user, pwd: pass, roles: [{ role: "readWrite", db: db.getName() }] });
				print("  created " + user + " with readWrite on " + db.getName());
			}
		' || {
			echo "  warning: could not create the user (needs userAdmin on $COPY_DB)."
			echo "  The binary will fail to authenticate against $COPY_DB until this exists."
		}
	fi

	cat <<-EOF

	Next: point a config at database "$COPY_DB" and start the binary, e.g.

	    sed 's/^  database:.*/  database: $COPY_DB/' /etc/conf/evsys-m.yml > /etc/conf/evsys-rehearsal.yml
	    /usr/local/bin/evsys-m -conf=/etc/conf/evsys-rehearsal.yml

	Watch for "Running migration 1" and "Migration 1 completed successfully".
	A "WARNING: database migration failed" line means it failed but the app kept
	running - treat that as a failed rehearsal.

	Then: $0 verify
	EOF
}

cmd_verify() {
	[[ -f "$STATE_FILE" ]] || die "no pre-state recorded - run '$0 prepare' first"

	local pre post
	pre=$(cat "$STATE_FILE")
	post=$(snapshot)

	echo "pre : $pre"
	echo "post: $post"
	echo

	mongosh "mongodb://$MONGO_HOST/$COPY_DB" "${AUTH_ARGS[@]}" --quiet \
		--eval "var PRE=$pre" --file "$(dirname "${BASH_SOURCE[0]}")/verify_001.js"
}

cmd_cleanup() {
	echo "==> dropping $COPY_DB"
	copy_eval 'db.dropDatabase()'
	rm -rf "$WORK_DIR"
	echo "done"
}

case "${1:-}" in
	prepare) cmd_prepare ;;
	verify)  cmd_verify ;;
	cleanup) cmd_cleanup ;;
	*) die "usage: $0 {prepare|verify|cleanup}" ;;
esac
