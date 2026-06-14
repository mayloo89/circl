# Incident response runbook

---

## Severity levels

| Level | Definition | Target response |
|-------|-----------|-----------------|
| P1 — Critical | App completely down or data loss in progress | Immediate |
| P2 — High | Core feature broken (login, chat, contacts) | < 1 hour |
| P3 — Medium | Non-critical feature broken or degraded performance | < 4 hours |
| P4 — Low | Cosmetic issue, minor UX bug | Next deploy |

---

## Step 1 — Confirm the incident

```bash
# Check all containers are running and healthy
cd deploy
docker compose --env-file .env.prod -f docker-compose.prod.yml ps

# Check the health endpoint
curl https://your-domain.com/api/health
# Expected: {"status":"ok","db":"ok","redis":"ok","version":"..."}

# Check the frontend is reachable
curl -I https://your-domain.com
```

A degraded health response identifies which subsystem is failing:
- `"db":"error"` → Postgres issue (see below)
- `"redis":"error"` → Redis issue (see below)
- Connection refused on `/api/*` → backend is down
- Connection refused on `/*` → frontend is down

---

## Step 2 — Check logs

```bash
# All services, last 100 lines
docker compose --env-file .env.prod -f docker-compose.prod.yml logs --tail=100

# Backend only (most issues surface here)
docker compose --env-file .env.prod -f docker-compose.prod.yml logs -f backend

# Frontend only
docker compose --env-file .env.prod -f docker-compose.prod.yml logs -f frontend

# Postgres only
docker compose --env-file .env.prod -f docker-compose.prod.yml logs -f postgres
```

Logs are structured JSON. Key fields to look for:

| Field | Meaning |
|-------|---------|
| `"level":"error"` or `"level":"fatal"` | Unhandled errors |
| `"status":5xx` | HTTP 5xx responses — check `"path"` and `"error"` |
| `"event":"panic"` | A recovered panic — `"source"` is `http` or a goroutine name; `"stack"` has the trace. The request/goroutine was salvaged but this is a bug to fix. |
| `"event":"client_error"` | A browser-side error reported via `POST /client-errors` — `"client_message"`, `"client_stack"`, `"client_url"`, `"client_kind"`. |
| `"component":"worker"` | Background job failures |
| `"trace_id"` | Correlate with Grafana Tempo if observability is running |

In Grafana/Loki, find recovered panics with `{job="circl"} | json | event="panic"` and
client-side errors with `{job="circl"} | json | event="client_error"`.

---

## Step 3 — Prometheus alerts (if observability stack is running)

These alert rules fire automatically:

| Alert | Threshold | Severity |
|-------|-----------|----------|
| `BackendDown` | Backend unreachable for > 1 min | Critical |
| `DBPoolExhausted` | DB connection pool > 90% for > 2 min | Critical |
| `RecoveredPanics` | Any `circl_panics_total` increase over 5 min | Critical |
| `HighErrorRate` | HTTP 5xx rate > 1% for > 5 min | Warning |
| `HighLatencyP95` | p95 latency > 1s for > 5 min | Warning |

Check Grafana for active alerts and the HTTP RED dashboard for traffic patterns.

---

## Common scenarios

### Backend is down

```bash
# Restart the backend
docker compose --env-file .env.prod -f docker-compose.prod.yml restart backend

# Watch it come back up
docker compose --env-file .env.prod -f docker-compose.prod.yml logs -f backend
```

If the backend keeps crashing, check for:
- A failed migration on startup (`"migrations failed"` in logs) — see `db-backup-restore.md`
- A missing required env var (`"required env var ... not set"`) — check `.env.prod`
- OOM kill (`docker inspect` the container for `OOMKilled: true`)

### Postgres is down or unreachable

```bash
# Restart postgres
docker compose --env-file .env.prod -f docker-compose.prod.yml restart postgres

# Check postgres logs
docker compose --env-file .env.prod -f docker-compose.prod.yml logs postgres --tail=50

# Check disk space — the most common cause of postgres failure
df -h
```

If the volume is full, free space before restarting. Do not delete `postgres_data` — that is the database.

### Redis is down

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml restart redis

# Verify redis is accepting connections
docker compose --env-file .env.prod -f docker-compose.prod.yml exec redis redis-cli ping
# Expected: PONG
```

Redis data (presence, push subscriptions, rate-limit counters, refresh tokens) is semi-ephemeral — a restart clears in-memory state but the AOF/RDB snapshot restores the most recent state. Users may be briefly shown as offline and may need to re-authenticate if refresh tokens are lost.

### High error rate (5xx spike)

1. Check backend logs for the error pattern (`"path"`, `"error"`, `"status"` fields).
2. If a recent deploy caused it, roll back (see `deploy.md`).
3. If caused by a dependency (Postgres, Redis, moderation sidecar), fix the dependency first.
4. If caused by a specific endpoint, check whether it's safe to disable temporarily by returning 503 while you investigate.

### Disk space running low

Circl writes to two locations on the host:
- `postgres_data` Docker volume — database
- `uploads_data` Docker volume — user-uploaded files (avatars, photos, chat attachments)

```bash
# Check volume sizes
docker system df -v

# Free space: remove dangling images from old builds
docker image prune -f

# If uploads volume is large, move old files to off-site storage
```

Do not prune volumes — that deletes the database and uploaded files.

### Moderation sidecar is down

The backend fails open if `MODERATION_API_URL` is set but the sidecar is unreachable — uploads are accepted without NSFW classification until it recovers. This is a P2 incident (content safety gap).

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml restart moderation

# Wait for the model to warm up (~60s), then check
curl http://localhost:8000/healthz  # from inside the host network
```

---

## Step 4 — Rollback

If the incident was caused by a bad deploy:

```bash
git checkout <previous-tag-or-commit>
cd deploy
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

If the deploy included migrations that ran successfully, rolling back the code without rolling back the schema may cause errors. In that case, restore from the pre-deploy database backup (see `db-backup-restore.md`) before rolling back the code.

---

## Step 5 — Post-incident

After the service is restored:

1. Write a brief incident summary (what failed, when, how it was detected, how it was resolved).
2. Check whether any user data was affected — if so, notify affected users per the privacy policy.
3. Add a monitoring or alerting improvement to the backlog if the incident wasn't caught by existing alerts.
4. If CSAM or NCII content was involved, follow the legal reporting obligations in the content policy.

---

## Incident log

| Date | Severity | Summary | Resolution | Duration |
|------|----------|---------|------------|----------|
| — | — | — | — | — |
