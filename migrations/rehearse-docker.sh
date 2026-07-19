#!/usr/bin/env bash
# Rehearse migration 001 locally, in Docker, against a dump of production data.
#
# Nothing here touches a live server: it runs a throwaway mongod in a container
# and the evsys binary built from the working tree.
#
# On the server:
#   mongodump --host=localhost:27017 -u <user> -p <pass> \
#     --authenticationDatabase evsys --db evsys --out /tmp/evsys-dump
#   tar czf evsys-dump.tar.gz -C /tmp/evsys-dump .
# Then copy it here and:
#   tar xzf evsys-dump.tar.gz -C ./dump
#   ./rehearse-docker.sh run ./dump/evsys
#
#   ./rehearse-docker.sh run <dump-dir>   # full cycle: up, restore, migrate, verify
#   ./rehearse-docker.sh down             # remove the container
#
# Configure via env vars:
#   CONTAINER   container name          (default: evsys-rehearsal)
#   MONGO_PORT  host port for mongod    (default: 27018, to miss a local 27017)
#   MONGO_IMAGE mongo image tag         (default: mongo:7)
#   DB_NAME     database to restore as  (default: evsys)

set -euo pipefail

CONTAINER="${CONTAINER:-evsys-rehearsal}"
MONGO_PORT="${MONGO_PORT:-27018}"
MONGO_IMAGE="${MONGO_IMAGE:-mongo:7}"
DB_NAME="${DB_NAME:-evsys}"

# Auth is enabled so the rehearsal exercises the same path as production: the
# app authenticates against its own database, not admin (internal/mongo.go).
# Credentials are throwaway and local to the container.
ROOT_USER=root
ROOT_PASS=root
APP_USER=rehearsal
APP_PASS=rehearsal

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="${WORK_DIR:-/tmp/evsys-rehearsal-docker}"

die() { echo "error: $*" >&2; exit 1; }

# Run mongosh inside the container as root.
in_mongo() {
	docker exec "$CONTAINER" mongosh "$DB_NAME" \
		-u "$ROOT_USER" -p "$ROOT_PASS" --authenticationDatabase admin --quiet "$@"
}

cmd_down() {
	docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
	echo "removed container $CONTAINER"
}

cmd_run() {
	local dump_dir="${1:-}"
	[[ -n "$dump_dir" ]] || die "usage: $0 run <dump-dir>"
	[[ -d "$dump_dir" ]] || die "no such directory: $dump_dir"
	# A dump of a single database is a directory of .bson files.
	compgen -G "$dump_dir/*.bson" >/dev/null || die "$dump_dir contains no .bson files - point at the per-database subdirectory, e.g. ./dump/evsys"
	command -v docker >/dev/null || die "docker not found"

	mkdir -p "$WORK_DIR"

	echo "==> starting $MONGO_IMAGE as $CONTAINER on port $MONGO_PORT"
	docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
	docker run -d --name "$CONTAINER" -p "$MONGO_PORT:27017" \
		-e MONGO_INITDB_ROOT_USERNAME="$ROOT_USER" \
		-e MONGO_INITDB_ROOT_PASSWORD="$ROOT_PASS" \
		"$MONGO_IMAGE" >/dev/null

	echo -n "==> waiting for mongod"
	local ready=""
	for _ in $(seq 1 60); do
		if in_mongo --eval 'db.adminCommand({ping:1}).ok' >/dev/null 2>&1; then
			ready=1; break
		fi
		echo -n "."
		sleep 1
	done
	echo
	[[ -n "$ready" ]] || die "mongod did not become ready - check: docker logs $CONTAINER"

	echo "==> restoring dump into $DB_NAME"
	docker cp "$dump_dir" "$CONTAINER:/dump"
	docker exec "$CONTAINER" mongorestore \
		-u "$ROOT_USER" -p "$ROOT_PASS" --authenticationDatabase admin \
		--db="$DB_NAME" --drop /dump >/dev/null 2>&1 \
		|| die "mongorestore failed - rerun without output suppression to see why"

	echo "==> creating app user on $DB_NAME"
	# authSource is the app's own database, so the user must live there.
	in_mongo --eval '
		db.createUser({
			user: "'"$APP_USER"'",
			pwd: "'"$APP_PASS"'",
			roles: [{ role: "readWrite", db: db.getName() }]
		});
	' >/dev/null || die "could not create app user"

	echo "==> pre-migration state"
	local pre
	pre=$(in_mongo --eval '
		var out = { schema_version: null, counts: {} };
		var sv = db.schema_version.findOne();
		out.schema_version = sv ? sv.version : null;
		["charge_points", "connectors", "transactions"].forEach(function (c) {
			out.counts[c] = db.getCollection(c).countDocuments({});
		});
		JSON.stringify(out);
	')
	echo "    $pre"
	echo "$pre" > "$WORK_DIR/pre-state.json"

	if [[ "$pre" == *'"schema_version":1'* ]]; then
		die "dump already has schema_version 1 - migration would be skipped, so this proves nothing"
	fi

	echo "==> building evsys"
	(cd "$REPO_ROOT" && go build -o "$WORK_DIR/evsys" .) || die "build failed"

	echo "==> generating config"
	cat > "$WORK_DIR/rehearsal.yml" <<-EOF
	---
	is_debug: true
	time_zone: Europe/Madrid
	accept_unknown_tag: false
	accept_unknown_chp: false
	listen:
	  type: port
	  bind_ip: 127.0.0.1
	  port: 5200
	  tls_enabled: false
	api:
	  bind_ip: 127.0.0.1
	  port: 5201
	  tls_enabled: false
	metrics:
	  enabled: false
	mongo:
	  enabled: true
	  host: localhost
	  port: "$MONGO_PORT"
	  user: $APP_USER
	  password: $APP_PASS
	  database: $DB_NAME
	payment:
	  enabled: false
	ocpi:
	  enabled: false
	telegram:
	  enabled: false
	EOF

	echo "==> running evsys to trigger the migration"
	# The migration runs during NewCentralSystem, before the server accepts
	# connections, so a short run is enough to apply and observe it.
	set +e
	"$WORK_DIR/evsys" -conf="$WORK_DIR/rehearsal.yml" > "$WORK_DIR/evsys.log" 2>&1 &
	local pid=$!
	sleep 12
	kill "$pid" 2>/dev/null
	wait "$pid" 2>/dev/null
	set -e

	echo "--- migration log ---"
	grep -iE "migration|schema|mongodb" "$WORK_DIR/evsys.log" || echo "(no migration lines found)"
	echo "---------------------"

	if grep -q "WARNING: database migration failed" "$WORK_DIR/evsys.log"; then
		echo
		echo "migration reported failure - full log: $WORK_DIR/evsys.log"
	fi

	echo
	echo "==> verifying"
	docker cp "$REPO_ROOT/migrations/verify_001.js" "$CONTAINER:/verify_001.js" >/dev/null
	if in_mongo --eval "var PRE=$pre" --file /verify_001.js; then
		echo
		echo "full log: $WORK_DIR/evsys.log"
		echo "inspect:  docker exec -it $CONTAINER mongosh $DB_NAME"
		echo "teardown: $0 down"
	else
		echo
		echo "full log: $WORK_DIR/evsys.log"
		die "verification failed"
	fi
}

case "${1:-}" in
	run)  shift; cmd_run "$@" ;;
	down) cmd_down ;;
	*) die "usage: $0 {run <dump-dir>|down}" ;;
esac
