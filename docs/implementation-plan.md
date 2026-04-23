# Implementation Plan — Private contact platform with secure chat (Go + Next.js)

## 1. Functional scope
- Private profiles: only authenticated users can view and search other profiles.
- Data: display name, bio, photos, search preferences (avoid exposing sensitive PII).
- Internal search: basic filters, pagination; no public enumeration.
- 1:1 chat and group rooms; persistent message history.
- Online/offline presence with "last seen".
- Photo and file uploads with CDN delivery.
- Multi-language UI: Spanish (default), English, Portuguese — user-selectable, persisted to backend.

## 2. Stack
- Frontend: Next.js (App Router) + React + TypeScript + Tailwind. Playwright for e2e.
- i18n: next-intl — prefix-based locale routing (`/es/`, `/en/`, `/pt/`), `useTranslations` / `getTranslations`, `createNavigation`.
- Backend: Go (chi) + middlewares; WebSockets (gorilla/websocket).
- Auth: NextAuth.js (Auth.js) v5 with credentials provider; httpOnly cookies; JWT HS256.
- DB: PostgreSQL + golang-migrate for schema migrations; pgx/v5 connection pool.
- Cache / real-time: Redis (presence, Pub/Sub for chat fan-out, rate limits).
- Queues / worker: asynq for image processing and maintenance tasks.
- Storage: S3/R2 + pre-signed URLs; image processing (EXIF strip, JPEG thumbnails) in asynq worker.
- Infra: Frontend on Vercel; backend on Fly.io/Render/AWS; Postgres (Neon/RDS), Redis (Upstash/ElastiCache).
- Quality: ESLint/Prettier, golangci-lint, Go tests, CI via GitHub Actions.
- Observability: zerolog (structured JSON logs) → Loki; Prometheus (metrics) with client_golang; OpenTelemetry traces → Tempo; Grafana Alloy / OTel Collector as shipper; Grafana as unified viz layer; trace_id correlated across logs/metrics/traces; Sentry for frontend error tracking (post-launch).

## 3. Logical architecture
- Frontend: middleware-protected routes; CSR for chat; SSR only with a valid session.
- API:
  - Auth: login/logout, token rotation, revocation (Redis blacklist).
  - Profiles: private CRUD, pagination, no direct contact exposure without consent.
  - Search: requires session; Postgres indexes; generic responses to prevent enumeration.
  - Chat: authenticated WebSockets; 1:1 and group rooms; Postgres persistence; fan-out via Redis Pub/Sub.
  - Presence: Redis heartbeats with TTL; online/offline/last seen computation.
  - Media: pre-signed S3 uploads; post-process hooks; orphan cleanup.
- Worker: image processing, revoked session expiry, presence cleanup.

## 4. Security
- TLS end-to-end; HSTS; strict CSP (no unsafe-inline, no eval).
- httpOnly, Secure, SameSite=Lax/Strict cookies; refresh token rotation.
- CSRF protection (if cookies); restricted CORS; rate limiting per IP and user on login/search/chat.
- Server-side validation and sanitisation; payload and file size limits.
- Enumeration protection: generic responses on login/reset; search requires authentication.
- Logs without PII; hashed IPs if required; encrypted backups; secrets in vault/KMS.
- Least privilege: user/admin roles; ownership checks on all resources (profiles, chats).

## 5. Data model (base)
- users: id, email (unique), password hash, status, created_at, last_login_at/ip_hash.
- profiles: user_id (FK), display_name, bio, searchable fields, avatar_url, privacy settings.
- contacts: id, requester_id, addressee_id, status (pending/accepted/blocked), created_at.
- rooms: id, type (dm|group), name, dm_key (unique for DMs), created_at, updated_at.
- room_members: room_id, user_id, last_read_at.
- messages: id, room_id, sender_id, type, content, media_url?, expires_at, view_once, created_at.
- message_views: message_id, user_id, viewed_at (for view-once ephemeral messages).
- Indexes: profile search, messages(room_id, created_at) for pagination; FK constraints.

## 6. Real-time flow
- WS handshake with session token via `?token=` query param (browser WS API has no custom header support).
- Per-room Redis Pub/Sub channel: `chat:room:{roomID}`; hub subscribes on first client, unsubscribes on last.
- Presence: heartbeat every N seconds → Redis key with TTL; TTL expiry marks offline.
- Fan-out: client sends message → saved to Postgres → published to Redis Pub/Sub → forwarded via WS to all subscribers.
- SSE bus: single `EventSource` connection per session for out-of-band notifications (contact events, new message badges).

## 7. Media handling
- Frontend requests a pre-signed URL → uploads directly to S3.
- Worker processes (resize, strip metadata) → saves variants → updates URL in DB.
- Delivery via CDN; anti-hotlink policies.

## 8. Roadmap

### Completed

- [x] **Foundation** (PR #1): repo, CI/CD (GitHub Actions), linters, dev/prod environments, secrets.
- [x] **Auth** (PR #2): registration and login with bcrypt; JWT HS256; `RequireAuth` middleware; NextAuth.js credentials provider; httpOnly cookies.
- [x] **Private profiles** (PR #2): `GET/PUT /profiles/me`; lazy profile creation; profile page in frontend.
- [x] **Contacts** (PR #9–10): user search; send/accept/decline/cancel requests; `DELETE /contacts/{id}`; `pending → accepted` state machine; real-time SSE notifications.
- [x] **Modern Go refactor** (PR #11): `cmp.Or`, `strings.SplitSeq`, `t.Context()` throughout.
- [x] **Real-time notifications (SSE)** (PR #14): `notifications.Hub`; `GET /notifications/stream?token=`; `contact_request`, `contact_accepted`, `contact_removed` events; NavBar badge; `NotificationsContext`.
- [x] **Chat and rooms** (PR #14): WebSocket (`GET /chat/rooms/{id}/ws?token=`); Redis Pub/Sub fan-out; DMs and group rooms; Postgres persistence; paginated history; unread counts.
- [x] **Presence** (PR #15): Redis heartbeat with TTL; batch `GET /presence?ids=`; `presence_online`/`presence_offline` SSE; green dot on contacts; last seen in DM header; `formatLastSeen`.
- [x] **Media — storage infra** (PR #16): `Storage` interface; `LocalStorage`; `uploads` table; request/confirm lifecycle; `useUpload` hook.
- [x] **Media — avatar upload** (PR #17): profile avatar UI; `PUT /profiles/me/avatar`; avatar in navbar/contacts/chat.
- [x] **Media — chat attachments** (PR #18): attachment button; image/file/video messages; lightbox.
- [x] **Media — S3 provider** (PR #19): `S3Storage` via `minio-go/v7`; Docker Compose dev/prod; Dockerfiles for backend and frontend.
- [x] **Media — image processing** (PR #20): asynq worker; EXIF strip; 480px JPEG thumbnails; `thumbnail_key` on uploads.
- [x] **Ephemeral messages** (PR #21): view-once tap-to-view; TTL options (15m–24h); tombstones; TTL countdown badge; cleaner worker.
- [x] **Typing indicators** (PR #22): `typing` WS frame; server-side debounce; "X is typing…" UI with auto-clear.
- [x] **Read receipts** (PR #23): `read_receipt` WS broadcast; ✓/✓✓ checkmarks; `peer_last_read_at` seeded from API.
- [x] **Image thumbnails in chat** (PR #24): `thumbnail_url` on messages; `LEFT JOIN uploads` in `ListMessages`; thumbnail display with lightbox fallback.
- [x] **Chat UI improvements** (PR #25): message grouping; date separators; skeleton loaders; new-message animation; relative timestamps on room list; empty and error states.
- [x] **Chat UX fixes** (PR #26): additional UX polish and bug fixes.
- [x] **Public profiles and photo gallery** (PR #27): `profile_photos` table; `GET /profiles/{userID}`; gallery upload/delete; public profile page with contact action button; DB trigger for auto profile creation.
- [x] **Documentation and memory cleanup** (PR #28): roadmap sync, stale memory files updated, dead env var and unused dependency removed.
- [x] **UI primitive component library** (PR #29): `Avatar`, `Badge`, `Button`, `Input`, `Skeleton`, `Modal`, `Toast`, `PresenceDot` in `components/ui/`; route `loading.tsx`/`error.tsx` for all authenticated segments; all callers updated.
- [x] **Domain component library** (PR #30): `MessageBubble`, `DateSeparator`, `TypingIndicator`, `ChatInput`, `Lightbox` in `components/chat/`; `ContactCard`, `SearchBar` in `components/contacts/`; `PhotoGallery`, `ProfileHeader` in `components/profile/`; shared `types/chat.ts` and `lib/chatHelpers.ts`; `chat/[roomId]/page.tsx` reduced from ~800 to ~210 lines.
- [x] **Route guard cleanup and form validation** (PR #31): removed redundant per-page `useEffect` auth redirects (superseded by existing `proxy.ts`); removed server-side redirect from `app/page.tsx`; `lib/validation.ts` with Zod schemas for login and register forms.

### Upcoming — see full roadmap in development plan

- [x] **Frontend testing foundation** (PR #32): vitest + @testing-library/react + msw; 129 tests across all `components/ui/*`, `hooks/useUpload`, `hooks/useHeartbeat`, `hooks/usePresence`, `lib/chatHelpers`, `lib/validation`; 99.56% statements, 100% branches and functions.
- [x] **Playwright E2E testing** (PR #33): `@playwright/test`; `playwright.config.ts`; fixtures with API-level user creation; auth, profile, contacts, and real-time chat flows; CI job with Postgres + Redis service containers and backend auto-start.
- [x] **Backend integration tests** (PR #34): `internal/testutil` package (OpenDB, CreateUser, NewRedis); chat store integration tests (ViewOnceMessage, DeleteMessage, TombstoneMessage, ListExpiredMessages); uploads pgStore integration tests; CI `backend-integration` job with Postgres + Redis service containers.
- [x] **Expanded profile schema** (PR #35): `date_of_birth`, `gender`, `location_text`, `latitude`, `longitude`, `interests[]`; `profile_preferences` table; `GET/PUT /profiles/me/preferences`; gender dropdown with free-text "Other"; Photon/OSM location autocomplete; interests tag input; public profile displays age/gender/location/interests.
- [x] **Username** (PR #36): mandatory username; `GET /profiles/@{username}`; profile URLs use username; availability check endpoint.
- [x] **Browse/explore endpoint** (PR #37): `GET /profiles/explore` with age/distance/gender filters; explore page with card UI.
- [x] **Geolocation** (PR #38): Haversine distance in SQL; coordinates saved from Photon/OSM location picker; sort-by-distance option; distance displayed on explore cards; interest tag filter.
- [x] **User blocking** (PR #39): block/unblock users; bidirectional suppression in browse/search/contacts/chat; WebSocket message filtering; batch query performance optimization; comprehensive test coverage.
- [x] **Account safety** (PR #42): login lockout (5 failed attempts = 15 min lockout); password complexity validation (8+ chars, upper/lower/number/special); rate limiting via Redis (LOGIN_IP_LIMIT, REGISTER_IP_LIMIT per hour per IP)
- [x] **Admin & moderation** (PR #41): admin role (PUT /users/{id}/role); user suspension/activation; reports table; report rate limiting (10/hour); auto-suspend on 3+ reports in 7 days; admin report management (resolve/dismiss)
- [x] **Web Push notifications** (PR #43): push subscription endpoint (/push/subscribe, /push/unsubscribe); VAPID key flow; service worker (sw.js); usePush hook; push delivery on chat messages and contact events
- [x] **Cursor-based pagination** (PR #44): cursor-based pagination for browse and chat history; "Load more" button for infinite scroll
- [x] **Group chat** (PR #45): create groups, rename (admin only), add/remove members, member panel UI; migration 000019 adds creator_id to rooms
- [x] **Public chat channels** (PR #46): IRC-style open rooms; ephemeral membership (WS connection = presence, no room_members rows); no message history per session; live participant sidebar with username links and filter; admin-only creation; leave confirmation guard; migrations 000020–000021
- [x] **Settings page** (PR #47): `/settings` page with push notifications toggle, change password with live validation, delete account; PushContext shared between NavBar and settings; avatar dropdown menu; PushPrompt dismiss button
- [x] **UX improvements** (PR #48): contact removal confirmation dialog; clickable name/avatar in search results navigates to public profile; registration form per-field inline validation on blur, live `PasswordRequirements` checklist; shared `PasswordRequirements` component
- [x] **Chat upload restrictions + image resizing** (PR #49): `CategoryChatAttachment` restricted to images + videos (PDF removed); JPEG/PNG originals resized to `IMAGE_MAX_PX` (default 1024px) before EXIF strip; configurable via env var; `ChatInput` accept updated
- [x] **Email infrastructure: verification + forgot/reset password** (PR #50): migration 000022 (`email_verified_at`, `password_resets`, `email_verifications`); `email` package (`Sender` interface, `ConsoleSender`, `SMTPSender` via go-mail); Mailpit in docker-compose; hard email enforcement (login blocked until verified); 4 new auth endpoints; token generation with SHA-256 hash storage; email-enumeration-safe responses; register page simplified to email+password; `/forgot-password`, `/reset-password`, `/verify-email` frontend pages
- [x] **Reversible account deletion** (PR #51): migration 000023 (`deleted_at`); `DELETE /users/me` soft-deletes with grace period; `POST /auth/reactivate` endpoint; login returns `403 account_deleted` within 30-day window; daily `PurgeDeletedAccounts` worker anonymizes expired accounts; login page reactivation banner; settings page updated messaging; `POST /test/users` endpoint behind `TEST_ENDPOINTS_ENABLED` guard for E2E fixtures
- [x] **Admin panel + role-based access control** (PR #52): migration 000024 replaces `is_admin boolean` with `role TEXT` (`user`/`admin`/`super_admin`); `token.RoleUser/RoleAdmin/RoleSuperAdmin` constants; `RequireSuperAdmin` middleware (super_admin only) alongside existing `RequireAdmin` (admin or super_admin); `HardDeleteUser` purges all user data across 9 tables + anonymizes the user row immediately; `SetUserRole` with role validation; super-admin-only routes (`DELETE /admin/users/{id}`, `PUT /admin/users/{id}/role`); channel CRUD (`POST/PUT/DELETE /admin/channels/{id}`); admin frontend: dashboard, user list with suspend/ban/reactivate/hard-delete/role modals, reports review, channel management with create and edit modals; `session.role` string replaces `session.isAdmin` boolean throughout frontend; profile links from user list
- [x] **UX polish** (PR #53): touched-state inline validation on registration; error messages read from backend response body; 429 rate-limit differentiated from credential errors on login; change-password collapses behind button; delete-account moved to modal; gender options extended (trans male/female, non-binary, custom free-text)
- [x] **Internationalisation** (PR #54): next-intl with prefix-based URL routing (`/es/`, `/en/`, `/pt/`); all pages and components translated into ES/EN/PT (`ChatInput`, `MessageBubble`, `GroupMembersPanel`, `CreateGroupModal`, `ConfirmDialog`, `ReportDialog`, `PushPrompt`, `ContactCard`, `SearchBar`, `PhotoGallery`); `app/[locale]/` restructure; language switcher in NavBar and Settings persists to backend; shared `apierror` package with stable machine-readable error codes across all 10 handlers; migration 000025 adds `locale` to `profile_preferences`; auth + intl middleware merged into single `middleware.ts`; Next.js pinned to 16.1.1 (16.2.2 had Turbopack memory regression)

- [x] **Structured logging** (PR #55): zerolog; `internal/logger` package (dev: ConsoleWriter, prod: JSON; `LOG_LEVEL` env var); `RequestLogger` middleware (request_id via xid, `X-Request-ID` header, access log with method/path/status/latency_ms); `EnrichRequestLog` accumulator — `RequireAuth` injects `user_id` into the access log; all 51 `log.*` call sites replaced; background workers (`push`, `uploads`, `ephemeral_cleaner`, `image_worker`, `purge_worker`) receive a logger at construction; `asynqLogger` bridge routes asynq internals through zerolog; no PII in logs.
- [x] **Metrics + health checks** (PR #56): `prometheus/client_golang`; Go runtime + HTTP handler + WS + DB pool collectors; `/metrics` endpoint (optionally protected); enhanced `/health` (DB ping + Redis ping + version); `backend/Dockerfile` `HEALTHCHECK`; non-root USER; `frontend/Dockerfile` `HEALTHCHECK`.
- [x] **Distributed tracing with OpenTelemetry** (PR #57): OTel SDK init (OTLP HTTP exporter, no-op fallback when `OTEL_EXPORTER_OTLP_ENDPOINT` unset); custom chi HTTP middleware with route-pattern span names; pgx QueryTracer; asynq trace context propagation via `taskEnvelope`; WS session + per-message spans (detached context, IIFE pattern); `trace_id`/`span_id` injected into zerolog for log-trace correlation; graceful shutdown via `signal.NotifyContext` + `http.Server.Shutdown` + `tp.Shutdown`.
- [x] **Log shipping pipeline** (PR #58): Loki 3.4.2 + Grafana Alloy v1.7.5 added to `docker-compose.yml`; Alloy scrapes all container stdout via Docker socket; JSON stage extracts `level`/`component` from zerolog as indexed Loki labels; static `env=dev` label; 7-day retention via Loki compactor; `ops/loki/logql-examples.md` with query cookbook (errors, slow requests, log-trace correlation, component filter).
- [x] **Frontend fetch hardening + auto sign-out** (PR #59, hotfix): all `fetch` chains in chat/contacts/profile/components now check `r.ok` before calling `.json()` — prevents non-array state on 4xx responses; `SessionGuard` component detects expired backend JWT via `exp` claim and calls `signOut()` automatically, eliminating the "broken app full of 401s" state.
- [x] **Grafana stack + dashboards + alert rules** (PR #60): Tempo 2.7.2, Prometheus v3.3.1, Grafana 11.5.2 added to `docker-compose.yml`; Grafana provisioned via YAML (datasources: Prometheus + Loki + Tempo with cross-datasource linking; dashboards-as-code under `ops/grafana/dashboards/`); 4 core dashboards (HTTP RED, WebSocket active connections, DB pool utilisation, Go runtime/infrastructure); Prometheus recording rules (error rate, request rate, p95 latency) + 4 alert rules (HighErrorRate, HighLatencyP95, DBPoolExhausted, BackendDown); `ops/prometheus/alerts.yml` + `ops/tempo/config.yaml`; `OTEL_EXPORTER_OTLP_ENDPOINT` / `OTEL_SERVICE_NAME` / `OTEL_SAMPLE_RATE` added to `.env.example`.
- [x] **Security hardening — headers, WS origin, CORS, secret scan** (PR #61): `SecurityHeaders` middleware (X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Cache-Control; HSTS in production only); WebSocket `CheckOrigin` validates against `CORS_ALLOWED_ORIGINS` instead of accepting all origins; `X-Request-ID` added to CORS `ExposedHeaders`; CSP + HSTS + Permissions-Policy via Next.js `headers()` in `next.config.ts`; gitleaks secret-scan job added to CI; `.gitleaks.toml` allowlists known test-only secrets.
- [ ] **Refresh token rotation + Redis blacklist** (PR #62): sliding-window refresh tokens; Redis blacklist on logout/rotation; revocation on password change and account deletion.
- [ ] **OpenAPI spec + generated types** (PR #63): OpenAPI 3.1 spec for all ~30 endpoints under `docs/openapi.yaml`; spec-first validation in CI; optional generated TypeScript client for frontend.
- [ ] **Phase 4 — Deployment + observability hosting** (PR #64–65): CI deploy workflow; production hosting (Fly.io + Vercel + Neon + Upstash + S3/R2); secrets via vault/KMS; **observability hosting decision (Grafana Cloud managed vs self-hosted)**; DB backups (automated + tested restore drill); key rotation runbook.
- [ ] **Phase 5 — Polish & launch** (PR #66–68): onboarding wizard; landing page for unauthenticated users; accessibility audit (WCAG 2.1 AA); runbooks (`docs/runbooks/`) for common incidents; final docs (README, CONTRIBUTING, architecture diagram).

> Observability PRs (#56–#60) follow the OTel convention: logs via Loki, metrics via Prometheus, traces via Tempo, all correlated by trace_id and unified in Grafana. Production hosting (managed vs self-hosted) is decided in Phase 4.

## 9. Testing strategy
- Unit: handlers and services (auth, chat, profiles, contacts).
- Integration: real Postgres and Redis (skipped when env vars not set).
- E2E: Playwright (login, profiles, search, chat, presence flows).
- Security: rate limiting, headers (CSP/HSTS), payload size limits.

## 10. Operations
- SLIs: HTTP/WS latency, message delivery rate, 5xx error rate, heartbeat expiry.
- Alerts: WS drops, queue backlogs, worker errors, disk space, DB connections.
- Encrypted and tested backups; periodic key rotation.
