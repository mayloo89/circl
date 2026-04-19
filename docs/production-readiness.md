# Production Readiness Checklist

Everything that needs to happen before deploying Circl to production.

---

## 1. Infrastructure

### Hosting
- [ ] Backend: deploy to Fly.io, Render, or AWS (Go binary)
- [ ] Frontend: deploy to Vercel (Next.js)
- [ ] PostgreSQL: provision managed instance (Neon, RDS, Supabase)
- [ ] Redis: provision managed instance (Upstash, ElastiCache)
- [ ] S3/R2: provision bucket for file storage + CDN (CloudFront, Cloudflare)

### Docker
- [x] Create `backend/Dockerfile` (multi-stage: build + scratch/alpine) — done PR #19
- [x] Create `docker-compose.yml` for local full-stack dev (Postgres, Redis, MinIO, backend, frontend) — done PR #19
- [x] Add `.dockerignore` files — done PR #19

### Domain and TLS
- [ ] Register domain, configure DNS
- [ ] TLS certificates (automatic via platform or Let's Encrypt)
- [ ] Force HTTPS redirects
- [ ] Configure HSTS headers

---

## 2. Environment Variables

### Backend

| Variable | Dev default | Production requirement |
|----------|-------------|----------------------|
| `ENV` | `development` | Set to `production` |
| `PORT` | `8080` | Platform-assigned or custom |
| `DATABASE_URL` | local with `sslmode=disable` | Managed DB with `sslmode=require` or `sslmode=verify-full` |
| `REDIS_URL` | `localhost:6379` | Managed Redis URL with TLS |
| `JWT_SECRET` | dev value | Generate with `openssl rand -base64 64`, min 32 chars |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | `https://yourdomain.com` |
| `STORAGE_PROVIDER` | `local` | `s3` |
| `S3_ENDPOINT` | MinIO local URL | S3/R2 endpoint URL |
| `S3_REGION` | — | Bucket region |
| `S3_BUCKET` | — | Bucket name |
| `S3_ACCESS_KEY` | — | IAM access key |
| `S3_SECRET_KEY` | — | IAM secret key |
| `IMAGE_MAX_PX` | `1024` | Tune for bandwidth vs quality |
| `LOGIN_IP_LIMIT` | `20` | Tune per environment |
| `REGISTER_IP_LIMIT` | `10` | Tune per environment |
| `VAPID_PUBLIC_KEY` | — | Generate for push notifications |
| `VAPID_PRIVATE_KEY` | — | Generate for push notifications |
| `SMTP_HOST` | — | Production mail server |
| `SMTP_PORT` | — | 587 (STARTTLS) or 465 (TLS) |
| `SMTP_USER` | — | SMTP credentials |
| `SMTP_PASS` | — | SMTP credentials |
| `SMTP_FROM` | — | Sender address |

### Frontend

| Variable | Dev default | Production requirement |
|----------|-------------|----------------------|
| `NEXTAUTH_URL` | `http://localhost:3000` | `https://yourdomain.com` |
| `NEXTAUTH_SECRET` | dev value | Generate with `openssl rand -base64 32` |
| `BACKEND_URL` | `http://localhost:8080` | Internal backend URL (server-side only) |
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | `https://api.yourdomain.com` |
| `NEXT_PUBLIC_VAPID_PUBLIC_KEY` | — | Match backend `VAPID_PUBLIC_KEY` |

---

## 3. Security Fixes (Critical)

### WebSocket origin validation
**File:** `backend/internal/chat/handler.go`

The WebSocket upgrader accepts connections from any origin. WebSocket upgrades bypass CORS middleware, so this must validate origins independently.

```go
// CURRENT (insecure)
CheckOrigin: func(_ *http.Request) bool { return true }

// REQUIRED: validate against allowed origins
CheckOrigin: func(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    for _, allowed := range allowedOrigins {
        if origin == allowed { return true }
    }
    return false
}
```

### Database SSL
Ensure `DATABASE_URL` uses `sslmode=require` or `sslmode=verify-full` in production. Never use `sslmode=disable` outside of local dev.

### Rate limiting
- [x] Auth endpoint rate limiting (per-IP) — done PR #42 (`LOGIN_IP_LIMIT`, `REGISTER_IP_LIMIT`)
- [x] Report submission rate limiting — done PR #41 (10/hour per user)
- [ ] Global per-IP API rate limit middleware — planned PR #57
- [ ] WebSocket connection rate limiting — planned PR #57

### Token expiry
`main.go` hardcodes `tokenExpiry = 24 * time.Hour`. Token rotation with refresh tokens and Redis blacklist is planned in PR #58.

---

## 4. Code Changes Required

### next/image remote patterns
**File:** `frontend/next.config.ts`

Currently hardcoded to `localhost:9000` (MinIO). For production, add the S3/CDN hostname:

```ts
images: {
    remotePatterns: [
        {
            protocol: "https",
            hostname: "your-cdn.cloudfront.net", // or S3 bucket hostname
        },
    ],
},
```

### NextAuth cookie security
**File:** `frontend/lib/auth.ts`

Verify that NextAuth production defaults apply:
- `secure: true` (HTTPS only)
- `httpOnly: true`
- `sameSite: "lax"`

NextAuth handles this automatically when `NEXTAUTH_URL` uses `https://`, but worth verifying.

---

## 5. Observability

### Logging
- [ ] Switch to structured logging (zerolog) — planned PR #55
- [ ] Log request ID, user ID, latency on every request
- [ ] Ensure no PII in logs (no emails, no tokens)

### Health checks
- [x] `GET /health` exists — returns DB status and environment
- [ ] Add Redis connectivity check to health endpoint — planned PR #56
- [ ] Configure platform health check (Fly.io / Render / ALB)

### Error tracking
- [ ] Integrate Sentry or equivalent (backend + frontend)
- [ ] Capture panics in goroutines (WebSocket pumps, hub)

### Metrics
- [ ] Prometheus metrics endpoint (`/metrics`) — planned PR #56
- [ ] Key metrics: request count/latency, WebSocket active connections, Redis ops, DB pool stats
- [ ] Grafana dashboards or platform monitoring

---

## 6. CI/CD

### GitHub Actions
**File:** `.github/workflows/ci.yml`

Currently runs lint + unit tests + integration tests + E2E. Need to add:
- [ ] Coverage reporting (target: 98%+) — planned PR #59
- [ ] Docker image build — planned PR #59
- [ ] Deploy to staging on push to `develop` — planned PR #59
- [ ] Deploy to production on push to `main` — planned PR #59
- [ ] Environment secrets in GitHub Settings

### Pre-deploy checks
- [ ] Run migrations before deploy (currently auto-runs on boot — acceptable for single-instance)
- [ ] Database backup strategy before migrations
- [ ] Rollback plan (down migrations)

---

## 7. Feature Readiness

| Feature | Status | Notes |
|---------|--------|-------|
| Auth (login, register, JWT) | ✅ Done (PR #2) | bcrypt; HS256; httpOnly cookies |
| Email verification | ✅ Done (PR #50) | SHA-256 hash storage; email-enumeration-safe |
| Forgot/reset password | ✅ Done (PR #50) | Token-based; Mailpit in dev; SMTP in prod |
| Contacts (request/accept/block) | ✅ Done (PR #9–10) | Full state machine; SSE notifications |
| Real-time notifications (SSE) | ✅ Done (PR #14) | Hub; contact events; NavBar badge |
| Chat + group rooms | ✅ Done (PR #14) | WebSocket; Redis Pub/Sub; paginated history |
| Presence (online/last seen) | ✅ Done (PR #15) | Redis heartbeat with TTL; SSE events |
| S3-compatible storage (MinIO) | ✅ Done (PR #19) | `S3Storage` via `minio-go/v7`; Docker Compose dev + prod |
| Image processing | ✅ Done (PR #20) | asynq worker; EXIF strip; 1024px resize; thumbnails |
| Chat attachments | ✅ Done (PR #18, #49) | Images + videos only; lightbox; size limits |
| Ephemeral messages | ✅ Done (PR #21) | View-once; TTL 15m–24h; tombstones; cleaner worker |
| Public profiles + gallery | ✅ Done (PR #27) | `GET /profiles/{username}`; up to 6 gallery photos |
| Component design system (UI) | ✅ Done (PR #29–31) | Avatar, Badge, Button, Input, Modal, Toast, etc. |
| Frontend unit tests | ✅ Done (PR #32) | vitest + RTL + msw; 129 tests; 99.56% statements |
| Playwright E2E tests | ✅ Done (PR #33) | Auth, profile, contacts, chat flows; CI service containers |
| Backend integration tests | ✅ Done (PR #34) | Real Postgres + Redis; chat + uploads store tests |
| Expanded profiles / explore | ✅ Done (PR #35–38) | DOB, gender, location, interests; Haversine distance |
| User blocking | ✅ Done (PR #39) | Bidirectional suppression; WS filtering |
| Admin & moderation | ✅ Done (PR #41) | Reports; auto-suspend; admin panel |
| Account safety | ✅ Done (PR #42) | Login lockout; password complexity; per-IP rate limits |
| Web Push notifications | ✅ Done (PR #43) | VAPID; service worker; chat + contact events |
| Group chat management | ✅ Done (PR #45) | Create, rename, add/remove members |
| Public channels (IRC-style) | ✅ Done (PR #46) | Ephemeral membership; live sidebar; admin-only create |
| Settings page | ✅ Done (PR #47) | Push toggle; change password; delete account |
| Reversible account deletion | ✅ Done (PR #51) | 30-day grace period; reactivation endpoint; daily purge worker |
| Role-based access control | ✅ Done (PR #52) | `user`/`admin`/`super_admin`; hard delete; channel CRUD |
| Internationalisation | ✅ Done (PR #54) | next-intl; ES/EN/PT; prefix routing; locale persisted to backend |
| Structured logging (zerolog) | ⚠️ Pending | Planned PR #55 |
| Prometheus / OpenTelemetry | ⚠️ Pending | Planned PR #56 |
| Sentry error tracking | ⚠️ Pending | Planned post-launch |
| CSP / HSTS headers | ⚠️ Pending | Planned PR #57 |
| WebSocket origin validation | ⚠️ Pending | Planned PR #57 |
| Token rotation + refresh | ⚠️ Pending | Planned PR #58 |

---

## 8. Pre-Launch Checklist

- [ ] All env vars set in production platform
- [ ] Database SSL enabled
- [ ] CORS origins restricted to production domain
- [ ] WebSocket origin validation enabled
- [ ] Global API rate limiting active
- [ ] Structured logging configured
- [ ] Error tracking integrated
- [ ] Health checks configured on platform
- [ ] DNS + TLS configured
- [ ] SMTP configured (verify email / password reset flows work)
- [ ] Smoke test: register → verify email → login → profile → add contact → send message
- [ ] Load test: concurrent WebSocket connections
- [ ] Backup strategy for database
- [ ] Monitoring dashboards set up
