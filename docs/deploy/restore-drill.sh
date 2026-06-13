#!/usr/bin/env bash
# Restore drill — proves the latest automated backup is actually restorable.
# Spins up a throwaway Postgres container, restores the most recent dump into
# it, runs sanity checks, and tears it down. Never touches production data or
# the running stack. Run monthly; log the result in
# docs/runbooks/db-backup-restore.md.
#
# Exit status is non-zero if the backup is missing, fails to restore, or the
# sanity checks don't pass — so this is safe to wire into a cron with alerting.
set -euo pipefail

DB="${POSTGRES_DB:-circl_db}"
PG_IMAGE="postgres:17-alpine"
CONTAINER="circl-restore-drill-$$"

# The db_backups volume is created by docker compose, so its real name is
# <project>_db_backups. Resolve it without assuming the project name.
VOL="$(docker volume ls --format '{{.Name}}' | grep -E '_db_backups$' | head -1 || true)"
if [[ -z "$VOL" ]]; then
  echo "FAIL: no *_db_backups volume found — is the backup service deployed?" >&2
  exit 1
fi
echo "Using backup volume: $VOL"

cleanup() { docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --name "$CONTAINER" \
  -e POSTGRES_PASSWORD=drill \
  -v "$VOL:/backups:ro" \
  "$PG_IMAGE" >/dev/null

echo -n "Waiting for throwaway Postgres"
until docker exec "$CONTAINER" pg_isready -U postgres >/dev/null 2>&1; do
  echo -n "."; sleep 1
done
echo

# Prefer the rotation's latest pointer; fall back to the newest daily dump.
LATEST="$(docker exec "$CONTAINER" sh -c \
  "ls -1 /backups/last/${DB}-latest.sql.gz 2>/dev/null \
   || ls -1t /backups/daily/${DB}-*.sql.gz 2>/dev/null | head -1")"
if [[ -z "$LATEST" ]]; then
  echo "FAIL: no backup dump found in the volume." >&2
  exit 1
fi
echo "Restoring: $LATEST"

docker exec "$CONTAINER" createdb -U postgres circl_drill
docker exec "$CONTAINER" sh -c \
  "gunzip -c '$LATEST' | psql -U postgres -d circl_drill -v ON_ERROR_STOP=1 >/dev/null"

# Sanity checks: the schema_migrations bookkeeping table must exist and the
# users table must be queryable. Adjust the expectations to taste.
MIGRATIONS="$(docker exec "$CONTAINER" psql -U postgres -d circl_drill -tAc \
  "SELECT count(*) FROM schema_migrations;")"
USERS="$(docker exec "$CONTAINER" psql -U postgres -d circl_drill -tAc \
  "SELECT count(*) FROM users;")"

echo "schema_migrations rows: $MIGRATIONS"
echo "users rows: $USERS"
echo "PASS: backup restored and sanity checks succeeded."
