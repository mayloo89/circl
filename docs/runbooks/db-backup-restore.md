# Database backup and restore runbook

---

## Overview

Postgres data lives in the `postgres_data` Docker volume. Backups use `pg_dump` inside the running container — no extra tooling required.

Recommended cadence: **daily automated backup**, **monthly restore drill**.

---

## Manual backup

```bash
cd deploy

# Dump to a timestamped file
docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U circl_user -d circl_db --no-owner --no-acl -Fc \
  > "backups/circl_$(date +%Y%m%d_%H%M%S).dump"
```

`-Fc` produces a custom-format dump (compressed, supports parallel restore). Keep dumps outside the container — on the host or copied to off-site storage.

Verify the dump is non-empty:

```bash
ls -lh backups/circl_*.dump | tail -3
```

---

## Automated daily backup (cron)

Add to the host's crontab (`crontab -e`):

```cron
# Daily DB backup at 03:00, retain 14 days
0 3 * * * cd /path/to/deploy && \
  docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U circl_user -d circl_db --no-owner --no-acl -Fc \
  > "backups/circl_$(date +\%Y\%m\%d_\%H\%M\%S).dump" && \
  find backups/ -name "circl_*.dump" -mtime +14 -delete
```

Create the backups directory first:

```bash
mkdir -p /path/to/deploy/backups
```

**Off-site copy (recommended):** pipe to `rclone` or `aws s3 cp` after the dump to push to R2 / S3.

---

## Restore

> **Warning:** restore overwrites all existing data. Take a fresh backup before restoring to a running system.

### 1. Stop the backend (to prevent writes during restore)

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml stop backend frontend
```

### 2. Drop and recreate the database

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres \
  psql -U circl_user -c "DROP DATABASE IF EXISTS circl_db;"

docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres \
  psql -U circl_user -c "CREATE DATABASE circl_db;"
```

### 3. Restore from dump

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres \
  pg_restore -U circl_user -d circl_db --no-owner --no-acl \
  < backups/circl_YYYYMMDD_HHMMSS.dump
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

Log in and confirm a user account is present. Check a recent contact or message to confirm data integrity.

---

## Restore drill

Run this monthly against a staging environment or a spare machine to confirm backups are actually valid.

```bash
# Copy latest dump to the test machine
scp deploy/backups/circl_latest.dump user@staging-host:/tmp/

# On staging: restore and verify health endpoint
# (same steps as Restore above)
```

Record the result and date below.

---

## Restore drill log

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
