# Production deploy runbook

Use this checklist for first deploys and every subsequent release.

---

## Pre-deploy checklist

- [ ] All CI checks green on `develop` (lint, unit tests, build)
- [ ] `docs/deploy/.env.prod.example` reviewed — no new `CHANGE_ME` values added since last deploy
- [ ] DB backup taken (see `db-backup-restore.md`)
- [ ] If the deploy includes new migrations: read every `.up.sql` file — confirm no destructive changes run without a backup

---

## First deploy

### 1. Prerequisites on the host

- Docker Engine 24+ with `docker compose` plugin
- A reverse proxy terminating TLS (nginx, Caddy, etc.) pointing your domain at the host
- DNS resolving to the host's public IP
- An SMTP provider configured (password reset and email verification are required flows)

### 2. Clone and stage

```bash
git clone https://github.com/mayloo89/circl.git
cd circl

# Create your operator deploy folder (gitignored)
mkdir -p deploy
cp docs/deploy/.env.prod.example deploy/.env.prod
cp docs/deploy/docker-compose.prod.yml deploy/docker-compose.prod.yml
cp docs/deploy/nginx.example.conf deploy/nginx.conf
```

### 3. Fill in secrets

Edit `deploy/.env.prod`. Every `CHANGE_ME` must be replaced:

```bash
# Generate backend JWT secret
openssl rand -base64 48

# Generate frontend NextAuth secret (different value)
openssl rand -base64 48

# Generate VAPID keys for web push
npx web-push generate-vapid-keys
```

Set `DOMAIN` to your public domain (e.g. `circl.example.com`).

### 4. Configure the reverse proxy

Edit `deploy/nginx.conf` — replace `EXAMPLE_DOMAIN` and TLS cert paths. Reload nginx after.

### 5. Build and start

```bash
cd deploy
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

This will:
1. Build backend, frontend, and moderation sidecar images
2. Start postgres and redis (internal only, no exposed ports)
3. Run database migrations automatically on backend startup (`db.Migrate` in `main.go`)
4. Start the moderation sidecar (NudeNet model warms up — ~60s)
5. Start backend (waits for postgres + redis + moderation healthy)
6. Start frontend (waits for backend healthy)

### 6. Verify

```bash
# All containers healthy
docker compose --env-file .env.prod -f docker-compose.prod.yml ps

# Backend health (should return {"status":"ok","db":"ok","redis":"ok",...})
curl https://your-domain.com/api/health

# Frontend reachable
curl -I https://your-domain.com
```

### 7. Smoke test

Run these manually after every deploy:

- [ ] Register a new account → receive verification email → verify
- [ ] Log in → session persists on refresh
- [ ] Send a contact request to another account → accept → open DM → send message
- [ ] Upload a profile avatar → visible in nav and contacts list
- [ ] Sign out → redirected to login

---

## Subsequent releases

### 1. Pull and rebuild

```bash
git pull origin main   # or deploy from a tagged release

cd deploy
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

`--build` forces a rebuild of changed images. Docker Compose restarts only the containers whose image changed.

### 2. Migrations

Migrations run automatically on backend startup. The backend process exits with a fatal log entry if any migration fails — check logs immediately:

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml logs backend --tail=50
```

If a migration fails mid-deploy:
1. Do **not** restart the backend in a loop — fix the cause first.
2. Connect to postgres directly and check `schema_migrations` to see which version failed.
3. If the `.up.sql` file is idempotent, fix the data issue and restart. If it is not, restore from backup and roll back the code.

### 3. Zero-downtime note

The stack has no load balancer — there is a brief outage window (seconds) when a container restarts. Schedule deploys during low-traffic hours.

---

## Rollback

If the new version is broken after deploy:

```bash
# Roll back to the previous image (Docker keeps the last built layer)
git checkout <previous-tag-or-commit>
cd deploy
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build
```

If migrations ran and introduced irreversible schema changes, restore from the pre-deploy backup instead (see `db-backup-restore.md`).

---

## Useful commands

```bash
# Tail all logs
docker compose --env-file .env.prod -f docker-compose.prod.yml logs -f

# Tail backend only
docker compose --env-file .env.prod -f docker-compose.prod.yml logs -f backend

# Open a psql shell
docker compose --env-file .env.prod -f docker-compose.prod.yml exec postgres \
  psql -U circl_user -d circl_db

# Restart a single service
docker compose --env-file .env.prod -f docker-compose.prod.yml restart backend

# Hard stop everything
docker compose --env-file .env.prod -f docker-compose.prod.yml down
```
