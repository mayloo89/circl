# Circl — Private contact platform with secure chat

Circl is a privacy-first platform for private profiles and real-time chat.
Only authenticated users can view, search, and message each other; public
"guest" chat rooms lower the barrier for newcomers without exposing the
private network. It is built as a best-practices showcase — a Go backend and a
Next.js frontend, fully internationalized (Spanish, English, Portuguese), with
a security and accessibility posture held to production standards.

> **Status:** feature-complete MVP, in pre-v1 hardening. The living roadmap,
> PR-by-PR history, and production env-var reference are in
> [`docs/implementation-plan.md`](docs/implementation-plan.md); every change is
> logged in [`CHANGELOG.md`](CHANGELOG.md).

## Stack

- **Frontend**: Next.js 16 (App Router) + React 19 + TypeScript + Tailwind CSS 4
  + next-intl (prefix-routed `/es/`, `/en/`, `/pt/`) + NextAuth v5 (credentials,
  JWT, httpOnly cookies). Vitest + Testing Library + MSW for unit; Playwright for E2E.
- **Backend**: Go 1.25 (chi router) + pgx/v5 + gorilla/websocket. Postgres for
  persistence, Redis for presence, Pub/Sub fan-out, WS-ticket auth, and rate limits.
  asynq worker for image processing, ephemeral cleanup, and account purge.
- **Storage**: pluggable `Storage` interface — `LocalStorage` for dev, `S3Storage`
  (MinIO / S3 / R2 via minio-go) for production.
- **Observability**: zerolog → Loki (logs), prometheus/client_golang → Prometheus
  (metrics), OpenTelemetry → Tempo (traces), unified in Grafana; `trace_id`
  correlated across all three.
- **CI/CD**: GitHub Actions — secret-scan (gitleaks), golangci-lint, govulncheck,
  `npm audit`, unit + integration + E2E, OpenAPI lint, and pull-based image deploy.

## Features

- **Accounts & profiles** — email/password auth with mandatory email
  verification, 15-minute access JWTs plus opaque refresh tokens, login lockout,
  and reversible (soft) account deletion with a 30-day grace period and a
  background purge. Rich profiles: username, age (18+ enforced), gender,
  geolocated discovery, interests, and a reorderable photo gallery.
- **Discovery** — a browse feed ranked by shared interests, activity recency, and
  proximity, with a pause-discovery toggle and a profile-completeness gate.
  Redacted public profiles for non-contacts; soft-deleted users are filtered out.
- **Contacts & chat** — full contact-request lifecycle, real-time DMs and group
  chat over WebSockets with Redis Pub/Sub fan-out, presence, typing indicators,
  read receipts, image/video attachments, and view-once / TTL ephemeral messages.
- **Public rooms & guest tier** — IRC-style open rooms a guest can join with just
  a nickname (Turnstile-gated), with in-room admin moderation (kick / mute) — the
  top-of-funnel that avoids the cold-start problem.
- **Trust & safety** — blocking, reporting with auto-suspension, a full admin
  moderation panel, an appeals flow, a fail-closed image-moderation pipeline
  (ready for a CSAM/NCII vendor to be plugged in), DM contact-info masking between
  non-contacts, message-retention sweeps, and GDPR-style data export.
- **Notifications** — in-app SSE stream plus Web Push (VAPID) for chat and
  contact events.
- **Private albums** — share photo sets per-recipient with a consent trail and
  a per-viewer watermark; access is revocable.

See [`CHANGELOG.md`](CHANGELOG.md) for the full, dated history and
[`docs/implementation-plan.md`](docs/implementation-plan.md) for the roadmap and
open backlog.

## Architecture highlights

The decisions that make this a showcase rather than a CRUD app:

- **WebSocket auth never carries a JWT in the URL** — clients exchange a
  short-lived, single-use Redis ticket (`POST /ws-ticket`) at connect time.
- **CSP uses a per-request nonce** minted in the middleware; `'unsafe-inline'` is
  never allowed in `script-src`.
- **Fail-closed by default** — the auth middleware treats an unresolvable session
  as logged-out, and legal-floor image moderation holds an upload as `pending`
  (never served) if a detector errors, retrying with backoff.
- **Privacy is enforced server-side** — contact-info masking rewrites DMs *before
  persistence*, public profiles are redacted for non-contacts, and presence is
  gated to accepted contacts.
- **Fully internationalized** — every user-facing string (including `aria-label`s)
  flows through next-intl in all three locales; no hardcoded copy in components.
- **Accessibility as an invariant** — focus-trapped dialogs, keyboard-navigable
  menus, and `aria-invalid`/`aria-describedby` on form errors.

For the visual and product design language, see [`DESIGN.md`](DESIGN.md).

## Local setup

### Requirements
- Node.js 20+
- Go 1.25+
- PostgreSQL 17+ (e.g. [Postgres.app](https://postgresapp.com) on macOS)
- Redis 7+ (e.g. `brew install redis && brew services start redis` on macOS)

### 1. Database

Create the user and database (run once):

```bash
psql postgres -c "CREATE USER circl_user WITH PASSWORD 'circl_password';"
psql postgres -c "CREATE DATABASE circl_db OWNER circl_user;"
```

Migrations run automatically when the backend starts — no manual step needed.

### 2. Backend

```bash
cd backend
cp .env.example .env
# DATABASE_URL is already set for local development in .env.example
go run ./cmd/api
```

API available at [http://localhost:8080](http://localhost:8080).

```bash
curl http://localhost:8080/health
# → {"status":"ok","env":"development","db":"ok","redis":"ok","version":"dev"}
```

### 3. Seed a test user

```bash
# Insert a user with password "password"
psql postgres://circl_user:circl_password@localhost:5432/circl_db -c "
  INSERT INTO users (email, password_hash, provider, status)
  VALUES (
    'test@example.com',
    '\$2a\$10\$SOWOqkV.1kjNlizSgHM4ZuV6MvEFpGByzUqYA8plJtbEL8/Q4jlF.',
    'local',
    'active'
  ) ON CONFLICT (email) DO NOTHING;"
```

Or generate your own bcrypt hash:

```bash
# macOS / Linux
htpasswd -bnBC 10 "" yourpassword | tr -d ':\n' | cut -c2-
```

### 4. Frontend

```bash
cd frontend
npm install
cp .env.example .env.local
# NEXTAUTH_SECRET: generate with → openssl rand -base64 32
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

**Test credentials**: `test@example.com` / `password`

### Full stack via Docker

```bash
docker compose up   # API, Postgres, Redis, MinIO, Mailpit, and the observability suite
```

## Deploying to production

Generic templates and a walkthrough live in [`docs/deploy/`](docs/deploy/README.md)
— five services (Postgres, Redis, the NudeNet moderation sidecar, the Go
backend, the Next.js frontend), one Docker network, one host-bound entry
point behind your own reverse proxy / TLS terminator. The TL;DR:

```bash
git clone https://github.com/mayloo89/circl.git /opt/circl
cd /opt/circl

# Stage your local, gitignored deploy folder from the generic templates.
mkdir -p deploy
cp docs/deploy/docker-compose.prod.yml deploy/
cp docs/deploy/update.sh               deploy/
cp docs/deploy/restore-drill.sh        deploy/
cp docs/deploy/.env.prod.example       deploy/.env.prod
cp docs/deploy/nginx.example.conf      deploy/nginx.conf

# Fill in secrets and customise.
chmod 600 deploy/.env.prod && nano deploy/.env.prod
sed -i 's/EXAMPLE_DOMAIN/your.domain/g' deploy/nginx.conf
sudo cp deploy/nginx.conf /etc/nginx/conf.d/your.domain.conf
sudo nginx -t && sudo systemctl reload nginx

# Pull the published linux/arm64 images from GHCR and start. CI publishes
# them on every green push to develop/main; subsequent updates are the same
# one-liner. (Forks / custom domains: ./deploy/update.sh --source builds
# from source instead.)
./deploy/update.sh
```

See [`docs/deploy/README.md`](docs/deploy/README.md) for the full walkthrough,
env-var reference, and the troubleshooting section.

## Environment variables

Each folder has a `.env.example` — copy it and fill in the values. These files are gitignored and never committed. The production env-var reference lives in [`docs/implementation-plan.md`](docs/implementation-plan.md).

| File | Copy to |
|------|---------|
| `backend/.env.example` | `backend/.env` |
| `frontend/.env.example` | `frontend/.env.local` |

## Scripts

### Frontend
```bash
cd frontend
npm run dev            # Development server
npm run build          # Production build
npm run lint           # ESLint
npm run test           # Vitest unit tests
npm run test:coverage  # Unit tests with the coverage gate CI enforces
npx playwright test    # E2E tests
```

### Backend
```bash
cd backend
go run ./cmd/api           # Development server
go build ./cmd/api         # Build binary
go test ./...              # Run tests
go test ./... -cover       # Run tests with coverage
go vet ./...               # Static analysis
golangci-lint run          # Linter (config in .golangci.yml)
```

### Full local CI
```bash
./run-ci-local.sh          # Spins up Postgres + Redis in Docker and runs the whole pipeline
```

### Load test (WebSocket)
```bash
# k6 — drives concurrent guest WS connections to a public room.
# Target backend must run with per-IP limits disabled (single source IP):
#   GLOBAL_IP_LIMIT=0 GUEST_IP_RATE=0 WS_IP_CONN_LIMIT=0 ./api
PEAK_VUS=300 k6 run loadtest/ws-load-test.js   # see loadtest/README.md
```

## API reference

The full API reference is in [`docs/openapi.yaml`](docs/openapi.yaml) (OpenAPI 3.1.0, ~40 endpoints).

All protected routes require `Authorization: Bearer <token>`. Short-lived access tokens (15 min) are refreshed silently via `POST /auth/refresh` using the opaque refresh token returned at login.

## Repository structure

```
circl/
├── frontend/                 # Next.js app
│   ├── app/[locale]/         # All pages under a locale prefix (/es/, /en/, /pt/)
│   │   ├── admin/            # Admin panel (users, reports, channels, appeals, moderation)
│   │   ├── browse/           # Profile discovery
│   │   ├── chat/             # Chat list, DM/group rooms, and (separate) channels
│   │   ├── contacts/         # Contacts page
│   │   ├── profile/          # Own profile (edit) + [username] public view
│   │   ├── rooms/            # Public guest rooms
│   │   └── settings/         # Notifications, language, password, delete account
│   ├── components/           # Shared UI (nav, chat, ui primitives, admin, landing)
│   ├── contexts/             # React contexts (Profile, Notifications, Push, Theme)
│   ├── hooks/                # useChat, usePresence, useUpload, useFocusTrap, …
│   ├── i18n/                 # next-intl config (routing, request, navigation)
│   ├── lib/                  # NextAuth config + client helpers
│   ├── messages/             # Translation catalogs: en.json, es.json, pt.json
│   ├── proxy.ts              # Middleware: auth guard + intl routing + per-request CSP nonce
│   └── e2e/                  # Playwright specs
├── backend/                  # Go API
│   ├── cmd/api/              # Server entry point (main.go)
│   ├── internal/             # Business logic (handler / service / store per package)
│   │   ├── auth/             # Register / login / refresh
│   │   ├── admin/            # Admin moderation
│   │   ├── albums/           # Private albums + consent grants + watermarking
│   │   ├── chat/             # Rooms, Hub (WebSocket fan-out), WS-ticket auth
│   │   ├── contacts/ · profiles/ · presence/ · reports/ · uploads/
│   │   ├── guest/            # Guest sessions + public-room entry
│   │   ├── moderation/       # Image-moderation pipeline + admin queue
│   │   ├── notifications/    # SSE hub + Notifier
│   │   ├── push/             # Web Push (VAPID)
│   │   ├── ratelimit/        # Redis-backed per-IP / per-user limiters
│   │   ├── redact/           # DM contact-info detection + redaction
│   │   ├── storage/          # Storage interface, LocalStorage, S3Storage
│   │   ├── worker/           # asynq tasks: image processing, cleanup, purge
│   │   └── logger/ · metrics/ · middleware/ · tracing/ · token/ · apierror/
│   ├── migrations/           # Numbered SQL (up + down) via golang-migrate
│   └── go.mod
├── ops/                      # Local observability stack (Loki, Alloy, Tempo, Prometheus, Grafana)
├── loadtest/                 # k6 WebSocket load test
├── docs/                     # implementation-plan.md, openapi.yaml, deploy/, runbooks/
├── .github/workflows/        # CI pipeline
├── docker-compose.yml        # Full dev stack
├── CONTRIBUTING.md · SECURITY.md · LICENSE
├── CHANGELOG.md
└── README.md
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the workflow, commit conventions,
quality gates, and the architectural invariants to respect. Security issues go
through [SECURITY.md](SECURITY.md) — never a public issue.

## License

[GNU AGPL-3.0](LICENSE). If you run a modified version of Circl as a network
service, you must make your modified source available to its users.
