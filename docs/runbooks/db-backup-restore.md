# Database backup and restore runbook

---

## Overview

Postgres data lives in the `postgres_data` Docker volume. Backups are produced
automatically by the **`backup` service** in the production compose
(`prodrigestivill/postgres-backup-local`): it runs `pg_dump` on a schedule,
rotates daily/weekly/monthly copies, and writes gzip-compressed plain-SQL dumps
into the **`db_backups`** volume.

- **Cadence:** daily (configurable via `BACKUP_SCHEDULE`).
- **Format:** `circl_db-<timestamp>.sql.gz` (plain SQL, gzipped) under
  `/backups/{daily,weekly,monthly}/`, plus a `last/circl_db-latest.sql.gz`
  pointer to the most recent.
- **Retention:** `BACKUP_KEEP_DAYS` (14) / `BACKUP_KEEP_WEEKS` (4) /
  `BACKUP_KEEP_MONTHS` (6) by default.
- **Restore drill:** `deploy/restore-drill.sh`, monthly.
- **Off-site:** optional `backup-offsite` service (see below). Without it,
  backups share the host disk and are lost if it fails.

No host crontab to configure — the backup runs in-stack and restarts with it.

---

## Inspecting backups

```bash
cd deploy

# List what the backup service has written into the db_backups volume.
docker compose --env-file .env.prod -f docker-compose.prod.yml \
  run --rm --no-deps -v db_backups:/backups backup ls -lhR /backups/daily /backups/last
```

The most recent dump is always `last/circl_db-latest.sql.gz`.

---

## Manual backup (ad-hoc)

For an on-demand dump (e.g. right before a risky migration), matching the
automated format:

```bash
cd deploy
mkdir -p backups

docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U circl_user -d circl_db --no-owner --no-acl \
  | gzip > "backups/circl_db-$(date +%Y%m%d-%H%M%S).sql.gz"

ls -lh backups/circl_db-*.sql.gz | tail -3   # confirm non-empty
```

---

## Automated backups (the `backup` service)

Configured entirely through `.env.prod` — see `.env.prod.example`:

| Variable | Default | Meaning |
|----------|---------|---------|
| `BACKUP_SCHEDULE` | `@daily` | Cron expression or `@daily`/`@hourly`/`@weekly`. |
| `BACKUP_KEEP_DAYS` | `14` | Daily dumps to retain. |
| `BACKUP_KEEP_WEEKS` | `4` | Weekly dumps to retain. |
| `BACKUP_KEEP_MONTHS` | `6` | Monthly dumps to retain. |

A backup is also taken on container start (`BACKUP_ON_START=TRUE`) so a fresh
deploy is covered before the first scheduled run.

### Off-site copy (recommended)

The `backup-offsite` service mirrors the `db_backups` volume to any
S3-compatible bucket (AWS S3, Cloudflare R2, MinIO). It's disabled by default.
To enable, set in `.env.prod`:

```bash
COMPOSE_PROFILES=offsite
BACKUP_S3_BUCKET=circl-backups
BACKUP_S3_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com   # blank for AWS S3
BACKUP_S3_REGION=auto                                              # AWS: bucket region
BACKUP_S3_ACCESS_KEY=...
BACKUP_S3_SECRET_KEY=...
```

Then re-run `./deploy/update.sh` (or `docker compose --profile offsite up -d`).
It runs `aws s3 sync` on an interval (`BACKUP_S3_SYNC_INTERVAL`, default daily).

---

## Restore

> **Warning:** restore overwrites all existing data. Take a fresh manual backup
> before restoring to a running system.

### 1. Stop the app (to prevent writes during restore)

```bash
cd deploy
docker compose --env-file .env.prod -f docker-compose.prod.yml stop backend frontend
```

### 2. Drop and recreate the database

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres \
  psql -U circl_user -d postgres -c "DROP DATABASE IF EXISTS circl_db;"

docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres \
  psql -U circl_user -d postgres -c "CREATE DATABASE circl_db;"
```

### 3. Restore from a dump

Copy the chosen `*.sql.gz` to the host, then pipe it in:

```bash
gunzip -c circl_db-YYYYMMDD-HHMMSS.sql.gz | \
  docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres \
  psql -U circl_user -d circl_db -v ON_ERROR_STOP=1
```

### 4. Restart services

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml start backend frontend
```

### 5. Verify

```bash
curl https://your-domain.com/api/health
# {"status":"ok","db":"ok","redis":"ok",...}
```

Log in and confirm a user account is present. Check a recent contact or message
to confirm data integrity.

---

## Restore drill

Proves the latest backup is actually restorable — run **monthly**. The script
spins up a throwaway Postgres container, restores `last/circl_db-latest.sql.gz`
into it, runs sanity checks, and tears it down. It never touches production or
the running stack, and exits non-zero on any failure (safe to alert on).

```bash
./deploy/restore-drill.sh
```

Record the result below.

### Restore drill log

| Date | Dump used | Outcome | Performed by |
|------|-----------|---------|--------------|
| — | — | — | — |

---

## Migration state

The `schema_migrations` table tracks which migrations have been applied:

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml exec postgres \
  psql -U circl_user -d circl_db -c \
  "SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 5;"
```

`dirty = true` means a migration failed mid-run. Fix the cause, then reset:

```bash
# Only if you are certain the partial migration is safe to re-run
docker compose --env-file .env.prod -f docker-compose.prod.yml exec postgres \
  psql -U circl_user -d circl_db -c \
  "UPDATE schema_migrations SET dirty = false WHERE version = <failed_version>;"
```

If not safe to re-run, restore from backup.
