# Circl — Private contact platform with secure chat

Private profiles and real-time chat. Only authenticated users can view, search, and message other users.

## Stack
- **Frontend**: Next.js 16+ (App Router) + React 19 + TypeScript + Tailwind CSS 4
- **Backend**: Go 1.25+ (chi router) + WebSockets
- **Auth**: NextAuth.js (Auth.js) v5 — JWT + httpOnly cookies
- **DB**: PostgreSQL 17+, migrations via golang-migrate
- **Cache / real-time**: Redis 7+ (presence, Pub/Sub, rate limits)
- **Queues**: asynq (image processing, maintenance tasks)
- **Storage**: S3/R2 + CDN
- **Observability**: zerolog → Loki (logs); Prometheus (metrics); OpenTelemetry → Tempo (traces); Grafana (unified dashboards + alerts)
- **CI/CD**: GitHub Actions (secret-scan, lint, unit, integration, e2e)

## Documentation
- [Implementation plan](docs/implementation-plan.md)
- [API reference (OpenAPI 3.1.0)](docs/openapi.yaml)
- [Production readiness checklist](docs/production-readiness.md)

## Project status
- ✅ **Foundation** ([PR #1](https://github.com/mayloo89/circl/pull/1)): repo structure, linters, CI/CD
- ✅ **Auth** ([PR #2](https://github.com/mayloo89/circl/pull/2)): registration, login, JWT tokens, NextAuth.js session
- ✅ **Private profiles** ([PR #2](https://github.com/mayloo89/circl/pull/2)): display name, bio — `GET /profiles/me`, `PUT /profiles/me`
- ✅ **Contacts** ([PR #9](https://github.com/mayloo89/circl/pull/9), [PR #10](https://github.com/mayloo89/circl/pull/10)): search, send/accept/decline/remove requests — full contacts lifecycle
- ✅ **Real-time notifications** ([PR #14](https://github.com/mayloo89/circl/pull/14)): SSE (`GET /notifications/stream`), global nav badge, contact request/accepted/removed events
- ✅ **Chat and rooms** ([PR #14](https://github.com/mayloo89/circl/pull/14)): WebSocket DMs and group rooms, Redis Pub/Sub fan-out, message history, unread counts
- ✅ **Presence** ([PR #15](https://github.com/mayloo89/circl/pull/15)): online/offline dot on contacts list, "Online" / "Last seen X ago" in DM chat header, instant updates via SSE
- ✅ **Storage infrastructure** ([PR #16](https://github.com/mayloo89/circl/pull/16)): Storage interface abstraction, LocalStorage (dev), uploads API (request → confirm lifecycle)
- ✅ **Profile avatars** ([PR #17](https://github.com/mayloo89/circl/pull/17)): upload from profile page, displayed in navbar, contacts list, chat list, chat room header, and message bubbles
- ✅ **Chat attachments** ([PR #18](https://github.com/mayloo89/circl/pull/18)): images, videos, and files in chat; ephemeral (view-once + TTL) messages
- ✅ **S3-compatible storage** ([PR #19](https://github.com/mayloo89/circl/pull/19)): MinIO backend with pre-signed PUT URLs; Docker Compose dev and prod setup
- ✅ **Image processing** ([PR #20](https://github.com/mayloo89/circl/pull/20)): asynq background worker — EXIF strip and 480px thumbnail generation for JPEG/PNG uploads
- ✅ **Typing indicators** ([PR #22](https://github.com/mayloo89/circl/pull/22)): real-time "X is typing…" via WebSocket with 2s server-side debounce
- ✅ **Read receipts** ([PR #23](https://github.com/mayloo89/circl/pull/23)): ✓ / ✓✓ on sent messages; updates in real time via WebSocket
- ✅ **Image thumbnails in chat** ([PR #24](https://github.com/mayloo89/circl/pull/24)): thumbnails served from storage instead of full-res URLs in message list
- ✅ **Chat UI** ([PR #25](https://github.com/mayloo89/circl/pull/25), [PR #26](https://github.com/mayloo89/circl/pull/26)): message grouping, date separators, skeleton loaders, new-message animation, relative timestamps, attachment type previews
- ✅ **Public profiles + gallery** ([PR #27](https://github.com/mayloo89/circl/pull/27)): public profile view, photo gallery (up to 6 photos), profile navigation from contacts and chat header
- ✅ **Usernames** ([PR #36](https://github.com/mayloo89/circl/pull/36)): unique handles (`[a-z0-9_]`, 3–30 chars), immutable once set, used in all profile URLs (`/profile/[username]`)
- ✅ **Extended profiles** ([PR #35](https://github.com/mayloo89/circl/pull/35)): date of birth (18+ enforced), gender, location (autocomplete via Photon/OSM), interests tags
- ✅ **Registration with profile seeding** ([PR #36](https://github.com/mayloo89/circl/pull/36)): username + DOB collected at signup, profile seeded immediately after account creation
- ✅ **User blocking** ([PR #39](https://github.com/mayloo89/circl/pull/39)): block/unblock users, bidirectional suppression in browse/search/contacts/chat, WebSocket message filtering, performance-optimized batch queries
- ✅ **User reporting** ([PR #40](https://github.com/mayloo89/circl/pull/40)): report users with reason, rate limited (10/hour), auto-suspend after 3+ reports in 7 days
- ✅ **Admin moderation** ([PR #41](https://github.com/mayloo89/circl/pull/41)): admin role, user suspension/activation, report management (resolve/dismiss with notes)
- ✅ **Account safety** ([PR #42](https://github.com/mayloo89/circl/pull/42)): login lockout (5 failed attempts = 15 min lockout), password complexity (8+ chars, upper/lower/number/special), rate limiting (configurable per IP)
- ✅ **Web Push Notifications** ([PR #43](https://github.com/mayloo89/circl/pull/43)): subscribe/unsubscribe, service worker, push delivery on chat/contact events
- ✅ **Cursor-based pagination** ([PR #44](https://github.com/mayloo89/circl/pull/44)): cursor-based for browse and chat history, infinite scroll in chat room
- ✅ **Group chat** ([PR #45](https://github.com/mayloo89/circl/pull/45)): create groups, rename (admin), add/remove members, member panel UI
- ✅ **Public chat channels** ([PR #46](https://github.com/mayloo89/circl/pull/46)): IRC-style open rooms — browse, enter, chat; ephemeral membership (WS connection = presence); no message history; live participant sidebar with filter; admin-only channel creation; leave confirmation guard
- ✅ **Settings page** ([PR #47](https://github.com/mayloo89/circl/pull/47)): push notifications toggle, change password with live validation, delete account, avatar dropdown menu
- ✅ **UX improvements** ([PR #48](https://github.com/mayloo89/circl/pull/48)): contact removal confirmation dialog, clickable profile from search results, registration inline validation with live password checklist
- ✅ **Chat upload restrictions + image resizing** ([PR #49](https://github.com/mayloo89/circl/pull/49)): chat attachments restricted to images and videos; JPEG/PNG originals resized to max 1024px (configurable); thumbnails remain at 480px
- ✅ **Email verification + forgot/reset password** ([PR #50](https://github.com/mayloo89/circl/pull/50)): hard email enforcement (login blocked until verified); forgot/reset password flow; Mailpit for local email dev; `ConsoleSender` for testing; `SMTPSender` for production
- ✅ **Reversible account deletion** ([PR #51](https://github.com/mayloo89/circl/pull/51)): soft delete with 30-day grace period; login automatically reactivates account within the grace period and shows a confirmation modal; deletion warning email sent on delete; daily background worker fully purges expired accounts — deletes S3 files (originals + thumbnails), removes all DB records, anonymizes the users row
- ✅ **Admin panel: channel management, RBAC, hard delete** ([PR #52](https://github.com/mayloo89/circl/pull/52)): create/edit/delete public channels from admin UI; super_admin can promote/demote admins; hard-delete permanently removes all user data from DB and S3
- ✅ **UX polish** ([PR #53](https://github.com/mayloo89/circl/pull/53)): touched-state inline validation, backend error codes surfaced in UI, 429 differentiated on login, change-password collapsible, delete-account modal, extended gender options (trans male/female, non-binary, custom)
- ✅ **Internationalisation — ES / EN / PT** ([PR #54](https://github.com/mayloo89/circl/pull/54)): next-intl with prefix-based routing (`/es/`, `/en/`, `/pt/`); all pages and components translated (ChatInput, MessageBubble, GroupMembersPanel, CreateGroupModal, ConfirmDialog, ReportDialog, PushPrompt, ContactCard, SearchBar, PhotoGallery); language switcher in NavBar and Settings persists preference to backend; shared `apierror` package with stable machine-readable error codes across all handlers; migration `000025` adds `locale` to `profile_preferences`
- ✅ **Structured logging** ([PR #55](https://github.com/mayloo89/circl/pull/55)): zerolog replaces stdlib `log` across the entire backend; `RequestLogger` middleware generates a `request_id` per request (xid), attaches it to context, sets `X-Request-ID` response header, and writes one access-log entry with `method`, `path`, `status`, `latency_ms`; `RequireAuth` enriches the context logger with `user_id` so every authenticated log line carries full traceability; all background workers tagged with `component`; asynq internal logs routed through zerolog; no PII in logs; `LOG_LEVEL` env var (default: `info`); dev: colored console, prod: JSON
- ✅ **Prometheus metrics + health checks** ([PR #56](https://github.com/mayloo89/circl/pull/56)): `prometheus/client_golang`; Go runtime, HTTP handler, WebSocket, and DB pool collectors; `/metrics` endpoint; enhanced `/health` with DB + Redis ping and version field; `HEALTHCHECK` in backend and frontend Dockerfiles; non-root `USER` in both images
- ✅ **OpenTelemetry distributed tracing** ([PR #57](https://github.com/mayloo89/circl/pull/57)): OTel SDK with OTLP HTTP exporter (no-op fallback when endpoint unset); chi HTTP middleware with route-pattern span names; pgx QueryTracer for per-query DB spans; asynq trace context propagation via `taskEnvelope`; WebSocket session and per-message spans; `trace_id`/`span_id` injected into zerolog for log-trace correlation; graceful shutdown via `signal.NotifyContext` + `http.Server.Shutdown`
- ✅ **Log shipping pipeline** ([PR #58](https://github.com/mayloo89/circl/pull/58)): Loki 3.4.2 + Grafana Alloy v1.7.5 in `docker-compose.yml`; Alloy collects all container stdout via Docker socket; JSON stage indexes `level` and `component` as Loki labels; 7-day retention; `ops/loki/logql-examples.md` query cookbook
- ✅ **Frontend fetch hardening + auto sign-out** ([PR #59](https://github.com/mayloo89/circl/pull/59)): all API fetch chains check `r.ok` before `.json()` — prevents TypeError crashes when the backend returns an error object; `SessionGuard` detects expired backend JWT via `exp` claim and calls `signOut()` automatically
- ✅ **Grafana observability stack** ([PR #60](https://github.com/mayloo89/circl/pull/60)): Tempo 2.7.2, Prometheus v3.3.1, and Grafana 11.5.2 added to `docker-compose.yml`; Grafana auto-provisioned with Prometheus + Loki + Tempo datasources (cross-datasource exemplar/trace-to-log links); 4 dashboards-as-code (HTTP RED, WebSocket, DB pool, Go runtime); Prometheus recording rules + 4 alert rules (HighErrorRate, HighLatencyP95, DBPoolExhausted, BackendDown); `internal/logger/loki.go` batching writer ships backend logs directly to Loki over HTTP (no file tailing); OTel export errors routed through zerolog at warn level
- ✅ **Security hardening** ([PR #61](https://github.com/mayloo89/circl/pull/61)): `SecurityHeaders` middleware sets `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Cache-Control`, and HSTS (production only) on every response; WebSocket `CheckOrigin` validates against `CORS_ALLOWED_ORIGINS` instead of accepting all origins; `X-Request-ID` added to CORS exposed headers; `next.config.ts` applies CSP, HSTS, and `Permissions-Policy` via Next.js `headers()`; gitleaks secret-scanning job added to CI
- ✅ **Refresh token rotation + Redis blacklist** ([PR #62](https://github.com/mayloo89/circl/pull/62)): access tokens reduced to 15-min TTL; opaque 7-day refresh tokens stored hashed in Redis; `POST /auth/refresh` rotates (delete-before-issue); `POST /auth/logout` invalidates server-side; password change and account deletion revoke all tokens via timestamp-based `rt:revoked_at:<userID>` key; frontend silently refreshes on expiry via NextAuth JWT callback
- ✅ **OpenAPI spec** ([PR #63](https://github.com/mayloo89/circl/pull/63)): `docs/openapi.yaml` — OpenAPI 3.1.0 spec covering all ~40 endpoints across 12 tag groups; reusable schemas, responses, and `bearerAuth` security scheme; `@redocly/cli lint` CI job
- ✅ **UX overhaul — visual rebrand** ([PR #67](https://github.com/mayloo89/circl/pull/67)): Nunito (headings) + DM Sans (body) via `next/font/google`; Tailwind v4 brand token system (`--color-brand-*`, `--radius-card`, `--shadow-card`); `accent` Button variant (CTA orange #F97316); conversion CTAs in browse and profile migrated to `accent`
- ✅ **UX overhaul — critical bugfixes** ([PR #66](https://github.com/mayloo89/circl/pull/66)): browse subtitle bug, unblock dialog wrong message, back buttons destroying browser history, login `blue-*`→`indigo-*` color alignment, register success emoji→SVG, WCAG contrast fix on block/report buttons, chat skeleton loading state, active locale highlighted in language switcher, contacts sorted online-first
- ✅ **UX overhaul — navigation** ([PR #68](https://github.com/mayloo89/circl/pull/68)): `BottomNav` (mobile, 5 slots, SVG icons + unread/pending badges, iOS safe-area padding) + `Sidebar` (desktop ≥1024px, same 5 items + language switcher + settings + user row) + `TopBar` (minimal mobile header with logo, active-route label, push toggle, avatar shortcut); `ProfileContext` single `/profiles/me` fetch per session; old monolithic `NavBar.tsx` removed
- ✅ **UX overhaul — home dashboard** ([PR #69](https://github.com/mayloo89/circl/pull/69)): `PendingRequestsWidget` (inline Accept/Decline, reacts to SSE contact events), `NearbyProfilesWidget` (horizontal scroll, skeleton), `RecentConversationsWidget` (avatar, last-message preview, unread badge), `ProfileCompletenessBanner` (progress bar, sessionStorage dismiss); `lib/profileCompleteness.ts` pure utility
- ✅ **UX overhaul — browse** ([PR #70](https://github.com/mayloo89/circl/pull/70)): `RangeSlider` (single-thumb filled-track) + `BottomSheet` (slide-up mobile sheet, Escape + scroll-lock) primitives; filter panel redesign with active-count badge; age and distance sliders (500 km = "Any"); "Clear filters" on empty state; backend fix to exclude profiles with unknown coordinates when distance filter is active
- ✅ **UX overhaul — public profile hero** ([PR #71](https://github.com/mayloo89/circl/pull/71)): 55vh full-bleed hero photo with gradient overlay; initials fallback; back button + ⋯ overflow menu (Block/Unblock/Report) over hero via backdrop-blur; sticky mobile action bar above bottom nav (Message/Add contact/Request sent/Accept); "Preview as visitor" button on own profile
- ✅ **UX overhaul — onboarding wizard** ([PR #72](https://github.com/mayloo89/circl/pull/72)): 4-step wizard at `/onboarding/{photo,bio,interests,location}`; step-dot progress bar; "Skip" on every step; pre-filled from existing profile data; `AppShell` redirects unonboarded users to wizard and suppresses nav; `ProfileCompletenessCard` on own profile (progress bar + per-field links to wizard); backend migration `000026` adds `onboarded_at TIMESTAMPTZ`
- ✅ **UX overhaul — chat polish + auth UX + onboarding smart steps** ([PR #74](https://github.com/mayloo89/circl/pull/74)): `PasswordField` with show/hide toggle (registration, login, settings); `DateOfBirthPicker` with three equal-width DD/MM/YYYY selects; scroll-to-bottom FAB in chat room; chat list search + 10s silent polling; onboarding smart steps skip already-complete fields and redirect to first incomplete step on entry

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

## Environment variables

Each folder has a `.env.example` — copy it and fill in the values. These files are gitignored and never committed.

| File | Copy to |
|------|---------|
| `backend/.env.example` | `backend/.env` |
| `frontend/.env.example` | `frontend/.env.local` |

## Scripts

### Frontend
```bash
cd frontend
npm run dev      # Development server
npm run build    # Production build
npm run lint     # ESLint
```

### Backend
```bash
cd backend
go run ./cmd/api           # Development server
go build ./cmd/api         # Build binary
go test ./...              # Run tests
go test ./... -cover       # Run tests with coverage
go vet ./...               # Static analysis
```

## API reference

The full API reference is in [`docs/openapi.yaml`](docs/openapi.yaml) (OpenAPI 3.1.0, ~40 endpoints).

All protected routes require `Authorization: Bearer <token>`. Short-lived access tokens (15 min) are refreshed silently via `POST /auth/refresh` using the opaque refresh token returned at login.

## Repository structure

```
circl/
├── frontend/               # Next.js app
│   ├── app/
│   │   ├── [locale]/      # All pages under locale prefix (/es/, /en/, /pt/)
│   │   │   ├── admin/     # Admin panel (dashboard, users, reports, channels)
│   │   │   ├── browse/    # Profile discovery
│   │   │   ├── chat/      # Chat list, room, and channels pages
│   │   │   ├── contacts/  # Contacts page
│   │   │   ├── login/     # Login page
│   │   │   ├── profile/   # Own profile (edit) + [username] public view
│   │   │   ├── settings/  # Settings (notifications, language, password, delete)
│   │   │   └── layout.tsx # Locale layout: <html lang>, NextIntlClientProvider
│   │   ├── layout.tsx     # Minimal root shell (no html/body)
│   │   └── page.tsx       # Redirects → /es
│   ├── i18n/              # next-intl config (routing, request, navigation)
│   ├── messages/          # Translation files: en.json, es.json, pt.json
│   ├── middleware.ts       # Auth guard + intl locale routing (merged)
│   ├── components/        # Shared UI components (NavBar, ui/*, profile/*, chat/*)
│   ├── contexts/          # React contexts (NotificationsContext, PushContext)
│   ├── hooks/             # Custom hooks (useChat, usePresence, useUpload, …)
│   ├── lib/               # Auth config (NextAuth.js)
│   ├── types/             # next-auth type augmentation
│   └── package.json
├── backend/               # Go API
│   ├── cmd/api/           # Server entry point (main.go)
│   ├── internal/          # Business logic (clean architecture)
│   │   ├── apierror/      # Shared error writer + stable error code constants
│   │   ├── auth/          # Register/login/refresh handler, service, store
│   │   ├── admin/         # Admin moderation handler, service, store
│   │   ├── chat/          # Chat rooms, Hub (WebSocket fan-out), store, handler
│   │   ├── config/        # Env helpers
│   │   ├── contacts/      # Contacts handler, service, store
│   │   ├── db/            # Connection pool, migrations runner
│   │   ├── email/         # Sender interface, ConsoleSender, SMTPSender
│   │   ├── logger/        # zerolog setup (dev: console, prod: JSON) + Loki writer
│   │   ├── metrics/       # Prometheus collectors (HTTP, WS, DB pool, runtime)
│   │   ├── middleware/    # RequireAuth, RequireAdmin, SecurityHeaders, RequestLogger
│   │   ├── notifications/ # SSE Hub, Notifier interface, stream handler
│   │   ├── presence/      # Redis heartbeat, offline, batch presence query
│   │   ├── profiles/      # Profile handler, service, store; preferences (locale)
│   │   ├── push/          # Web Push (VAPID) handler, service, store
│   │   ├── ratelimit/     # Redis-backed rate limiter (per-IP and per-user)
│   │   ├── reports/       # User report handler, service, store
│   │   ├── server/        # Chi router, CORS, /health, /metrics endpoints
│   │   ├── storage/       # Storage interface, LocalStorage, S3Storage
│   │   ├── testutil/      # Integration test helpers (OpenDB, CreateUser, NewRedis)
│   │   ├── token/         # JWT generate/validate
│   │   ├── tracing/       # OTel SDK init, chi middleware, pgx tracer
│   │   ├── uploads/       # Upload lifecycle (request → confirm), Postgres tracking
│   │   └── worker/        # asynq tasks: image processing, ephemeral cleanup, purge
│   ├── migrations/        # SQL migrations (up + down), currently at 000025
│   └── go.mod
├── ops/                   # Local observability stack (dev only)
│   ├── alloy/             # Grafana Alloy config — scrapes container stdout → Loki
│   ├── grafana/           # Provisioning YAML (datasources + dashboards-as-code)
│   ├── loki/              # Loki single-binary config, 7-day retention
│   ├── prometheus/        # prometheus.yml + alert rules
│   └── tempo/             # Tempo config (OTLP receivers, 7-day trace retention)
├── docs/
│   ├── implementation-plan.md
│   ├── openapi.yaml       # OpenAPI 3.1.0 spec (~40 endpoints)
│   └── production-readiness.md
├── .github/workflows/     # CI: secret-scan, frontend, backend, backend-integration, e2e, openapi-lint
├── .gitleaks.toml         # Gitleaks allowlist for known test-only secrets
├── docker-compose.yml     # Full dev stack (API, Postgres, Redis, MinIO, Mailpit, observability)
├── CHANGELOG.md
└── README.md
```

## Contributing
1. Branch off `develop`: `git checkout -b feature/your-feature`
2. Make changes and write tests (target: 98%+ coverage)
3. Commit with descriptive messages in English
4. Push and open a PR targeting `develop`

## License
MIT
