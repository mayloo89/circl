# Secret rotation runbook

Run this after a suspected compromise, on a scheduled basis (recommended: every 90 days), or before the first production deployment.

---

## Secrets inventory

| Secret | Where set | Effect of rotation |
|--------|-----------|-------------------|
| `JWT_SECRET` | backend `.env` | All access tokens invalid — users signed out within 15 min (next API call gets 401, `SessionGuard` triggers `signOut()`) |
| `AUTH_SECRET` | frontend `.env` | All NextAuth session cookies invalid — users signed out immediately on next page load |
| `VAPID_PUBLIC_KEY` + `VAPID_PRIVATE_KEY` | backend `.env` | All push subscriptions in DB become invalid — push silently stops until each user re-opens the app (service worker auto-re-registers) |
| `POSTGRES_PASSWORD` | backend + DB `.env` | Backend can't connect to DB until both are updated simultaneously |
| `S3_SECRET_KEY` | backend `.env` | File uploads and reads fail until updated |
| `REDIS_URL` password (if set) | backend `.env` | All real-time features (presence, chat, push, rate limits) fail until updated |
| `METRICS_TOKEN` | backend `.env` | `/metrics` endpoint changes access requirement |

---

## 1. JWT_SECRET

Rotating this logs out all users within the 15-minute access token TTL.

```bash
# Generate a new secret (min 32 bytes)
openssl rand -base64 48
```

1. Replace `JWT_SECRET` in backend `.env` / secrets manager.
2. Restart the backend: `docker compose restart backend` (or equivalent).
3. Verify: `curl https://api.example.com/health` returns `{"status":"ok"}`.
4. Watch logs for `401` errors in the first few minutes — expected as tokens expire.

**Note:** Refresh tokens are opaque and stored hashed in Redis; they are independent of `JWT_SECRET`. Rotating `JWT_SECRET` does not invalidate refresh tokens. If you need a hard logout of all users (e.g. compromise), also flush all `rt:*` keys in Redis:

```bash
redis-cli -u "$REDIS_URL" --scan --pattern 'rt:*' | xargs redis-cli -u "$REDIS_URL" del
```

---

## 2. AUTH_SECRET

Rotating this invalidates all NextAuth session cookies immediately.

```bash
openssl rand -base64 48
```

1. Replace `AUTH_SECRET` in frontend `.env` / secrets manager.
2. Rebuild and restart the frontend: `docker compose up --build frontend`.
3. Verify: open the app — unauthenticated users should be redirected to `/login`.

---

## 3. VAPID keys (Web Push)

Rotating VAPID keys silently breaks push for all users until they re-open the app.

```bash
npx web-push generate-vapid-keys
```

1. Replace `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` in backend `.env`.
2. Replace `NEXT_PUBLIC_VAPID_KEY` in frontend `.env` with the new public key.
3. **Clear stale subscriptions from the DB** — old endpoint+auth records will all bounce:

```sql
DELETE FROM push_subscriptions;
```

4. Rebuild frontend (public key is baked in at build time): `docker compose up --build frontend`.
5. Restart backend: `docker compose restart backend`.
6. Verify: open the app on a device with push enabled — the browser prompts to re-subscribe automatically when the service worker detects the key mismatch. Check backend logs for successful `201` on the subscribe endpoint.

---

## 4. Database password

Must update DB and backend config atomically — there is a brief outage window.

```bash
openssl rand -base64 32 | tr -d '/+=' | head -c 32
```

1. Set the new password in Postgres:

```sql
ALTER USER circl_user PASSWORD 'new-password-here';
```

2. Immediately update `DATABASE_URL` in backend `.env`.
3. Restart the backend: `docker compose restart backend`.
4. Verify: `curl https://api.example.com/health` — `db` field should be `"ok"`.

**Production note:** With connection pooling (PgBouncer), drain existing connections first.

---

## 5. S3 / R2 credentials

1. Rotate the access key in MinIO / Cloudflare R2 / AWS IAM.
2. Update `S3_ACCESS_KEY` and `S3_SECRET_KEY` in backend `.env`.
3. Restart backend: `docker compose restart backend`.
4. Verify: upload a test avatar through the app — check it loads correctly.

---

## 6. Post-rotation checklist

- [ ] `GET /health` returns `{"status":"ok","db":"ok","redis":"ok"}`
- [ ] Login flow works end-to-end
- [ ] File upload (avatar) works
- [ ] WebSocket chat connects and sends a message
- [ ] Push notification received on a subscribed device (VAPID rotation only)
- [ ] No `500` errors in logs within 5 minutes of restart
- [ ] Update `last_rotated` dates below

---

## Rotation log

Record each rotation here so the team knows when secrets are due for renewal.

| Secret | Last rotated | Rotated by |
|--------|-------------|------------|
| JWT_SECRET | — | — |
| AUTH_SECRET | — | — |
| VAPID keys | — | — |
| POSTGRES_PASSWORD | — | — |
| S3_SECRET_KEY | — | — |
