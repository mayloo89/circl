# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Added
- **Browse overhaul** ([PR #70](https://github.com/mayloo89/circl/pull/70)):
  - `RangeSlider` UI primitive — single-thumb slider with filled track and active-scale thumb; used for age and distance filters
  - `BottomSheet` UI primitive — slides up from bottom on mobile with backdrop + Escape key + body scroll lock; renders nothing on desktop (`lg:hidden`)
  - Mobile filter panel replaced: `<details>` removed, new "Filters" pill button with active-filter count badge opens the BottomSheet
  - Age min/max and max-distance number inputs replaced by `RangeSlider`; 500 km = "Any" (maps to `null` in preferences)
  - "Clear filters" button shown on the empty state when at least one filter is active
  - Desktop filter sidebar widened to `w-64`, made `sticky top-6`; filter labels use uppercase tracking style
  - All filter panel strings translated in EN / ES / PT

### Fixed
- **Browse distance filter** ([PR #70](https://github.com/mayloo89/circl/pull/70)): profiles with no coordinates previously bypassed the `max_distance_km` filter. The SQL `WHERE` clause now requires both the requester's and the candidate's coordinates to be present when the filter is active.

### Added
- **Home dashboard** ([PR #69](https://github.com/mayloo89/circl/pull/69)):
  - `PendingRequestsWidget` — fetches `/contacts/pending` and renders inline Accept / Decline buttons; hidden when empty; reacts to `contact_request` and `contact_removed` SSE events
  - `NearbyProfilesWidget` — fetches `/profiles/browse?limit=8`; horizontal-scroll chip row on mobile; skeleton loading state; "Browse all" link to `/browse`
  - `RecentConversationsWidget` — fetches `/chat/rooms?limit=5`; shows avatar, room name, last-message preview (text / photo / video / file), relative time, and unread badge; skeleton loading state; "See all" link to `/chat`
  - `ProfileCompletenessBanner` — sticky banner with progress bar (avatar 30 %, bio 20 %, interests 20 %, birthdate 10 %, location 20 %); lists missing fields; dismissed via `sessionStorage`; hidden once profile is complete
  - `lib/profileCompleteness.ts` — pure `computeCompleteness` function, reusable by the profile page in future PRs
  - Home page (`app/[locale]/page.tsx`) replaced: old 3-button placeholder → real dashboard with widgets
  - i18n keys added in EN / ES / PT: pending requests, nearby, recent conversations, profile completeness labels

### Added
- **Navigation overhaul — mobile bottom nav + desktop sidebar** ([PR #68](https://github.com/mayloo89/circl/pull/68)):
  - `BottomNav` — fixed 5-tab bar (Home / Browse / Messages / Contacts / Profile) for mobile (`lg:hidden`), with live unread/pending badges and iOS safe-area padding
  - `Sidebar` — fixed left sidebar for desktop (`lg:flex hidden`), same 5 items with icons + labels, language switcher, settings, and a user row with avatar + sign-out; admin link surfaced automatically for admin/super_admin roles
  - `TopBar` — minimal mobile-only header (`lg:hidden`) with logo, active-route label, push-notification toggle, and avatar shortcut to profile
  - `ProfileContext` — fetches `/profiles/me` once per authenticated session; shared by Sidebar, TopBar, and home dashboard (UX-4); eliminates per-mount profile fetches
  - Old monolithic `NavBar.tsx` removed; sign-out and locale-change logic moved into `Sidebar`

### Changed
- **UX visual rebrand foundation** ([PR #67](https://github.com/mayloo89/circl/pull/67)):
  - Fonts replaced: Geist → **Nunito** (display/headings) + **DM Sans** (body), loaded via `next/font/google` with `display: swap`
  - Tailwind v4 brand tokens added to `globals.css` via `@theme`: `--color-brand-primary` (indigo-600), `--color-brand-accent` (#F97316 orange), `--color-brand-success/danger/surface/surface-elevated`; `--radius-card` (1rem), `--radius-pill` (9999px); `--shadow-card` / `--shadow-card-hover` elevation scale
  - New `accent` Button variant (CTA orange `#F97316`) for conversion actions — distinct from `primary` (indigo, neutral) and `warning` (form-level orange)
  - `SendRequestButton` in browse and "Add contact" / "Message" buttons in `ProfileHeader` migrated to `variant="accent"`
  - Browse profile cards upgraded to `rounded-card` / `shadow-card` / `shadow-card-hover` tokens
  - All hardcoded `indigo-*` Tailwind classes replaced with `brand-*` semantic tokens across every frontend component and page — palette swaps now require a single CSS variable change in `globals.css`

### Fixed
- **UX bugfixes and quick wins** ([PR #66](https://github.com/mayloo89/circl/pull/66)):
  - Browse page subtitle always showed "No profiles found" regardless of results — replaced with `t("subtitle")` ("Discover people near you") in EN/ES/PT
  - Unblock confirmation dialog displayed the generic "Something went wrong" message instead of the user's name — replaced with `t("unblockConfirmMessage", { name })` in all locales
  - Back buttons in chat list and contacts used `router.push("/")`, destroying browser history — changed to `router.back()`; touch target enlarged with `p-2`
  - Login page used Tailwind `blue-*` colors while all other pages use `indigo-*` — all `blue-600/500/400` classes replaced with `indigo-*` equivalents
  - Register success screen used the `✉` emoji as a structural icon — replaced with an inline SVG envelope using the same visual pattern as `NavBar`
  - "Block user" and "Report" action buttons on `ProfileHeader` had `text-gray-600` (contrast ratio ~2.5:1, fails WCAG AA) — bumped to `text-gray-400`
  - Chat room loading state showed a plain `<p>` text string instead of the existing `<MessageSkeletons />` component

### Changed
- **Active locale indicator** ([PR #66](https://github.com/mayloo89/circl/pull/66)): language buttons in Settings and the NavBar dropdown now highlight the currently active locale (`bg-indigo-600 text-white`) so users know which language is selected
- **Contacts list sorted online-first** ([PR #66](https://github.com/mayloo89/circl/pull/66)): accepted contacts are sorted so online users appear at the top, using the existing `presence` map — no extra API call required

### Added
- **OpenAPI 3.1.0 spec** ([PR #63](https://github.com/mayloo89/circl/pull/63)):
  - `docs/openapi.yaml` documents all ~40 backend endpoints across 12 tag groups: System, Auth, Account, Profiles, Contacts, Chat, Notifications, Presence, Uploads, Reports, Push, Admin
  - Reusable `components/schemas`: Error, User, TokenPair, Profile, Room, Message, Contact, UploadRequest, Report, AdminUser, Channel
  - Reusable `components/responses`: Unauthorized, Forbidden, NotFound, BadRequest, TooManyRequests
  - `bearerAuth` security scheme; global `security: [{bearerAuth: []}]` with per-endpoint overrides for public routes
  - WebSocket (`GET /chat/ws`) and SSE (`GET /notifications/stream`) endpoints documented with protocol upgrade and streaming notes
  - `@redocly/cli lint` CI job added — blocks the pipeline on spec violations

- **Refresh token rotation + Redis blacklist** ([PR #62](https://github.com/mayloo89/circl/pull/62)):
  - Access token TTL reduced from 24 h to 15 min; new opaque refresh tokens (32 random bytes, hex-encoded) with 7-day TTL issued at login alongside the access token
  - `POST /auth/refresh` — validates the refresh token against Redis, deletes it (rotation), issues a new access token + refresh token pair; returns `401` on unknown/revoked tokens
  - `POST /auth/logout` — deletes the refresh token from Redis; always returns `204` to prevent enumeration
  - `PUT /users/me/password` and `DELETE /users/me` now call `RevokeAllForUser` — stores a `rt:revoked_at:<userID>` timestamp in Redis; any token issued before that timestamp is rejected on next use
  - `RedisRefreshStore` — tokens stored as `rt:<sha256(plaintext)>` with TTL; revocation checked on every `Get` call
  - `WithRefreshTokenStore` / `WithAccountRefreshStore` options keep the store injected rather than global
  - Frontend `lib/auth.ts` — JWT callback checks access-token expiry and calls `/auth/refresh` silently; stores `refreshToken` in the NextAuth session
  - `SessionGuard` — also signs out on `RefreshFailed` (expired refresh token or network error during silent refresh)
  - `NavBar` and `SignOutButton` — send `POST /auth/logout` with the refresh token before calling `signOut()`

- **Security hardening — headers, WS origin, CORS, secret scan** ([PR #61](https://github.com/mayloo89/circl/pull/61)):
  - `SecurityHeaders` middleware added to the chi middleware stack — sets `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `Cache-Control: no-store` on every response; `Strict-Transport-Security` (1 year, includeSubDomains) added in production only
  - WebSocket `CheckOrigin` in `internal/chat/handler.go` replaced `return true` with an exact-match check against `CORS_ALLOWED_ORIGINS`; when the list is empty all origins are accepted (dev convenience only); the upgrader is now created per-handler rather than as a package-level variable
  - `AllowedOrigins []string` field added to `chat.HandlerConfig`; `main.go` passes `corsOrigins` into the WS handler
  - `X-Request-ID` added to CORS `ExposedHeaders` so browser clients can read the request ID for support/debugging
  - `next.config.ts` gains a `headers()` export applying CSP, HSTS, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, and `Permissions-Policy` to all Next.js routes; `script-src` allows `unsafe-eval` in development only (Next.js HMR requirement)
  - Gitleaks secret-scanning job (`gitleaks/gitleaks-action@v2`, full-history checkout) added as the first CI job — blocks the pipeline on real leaked credentials
  - `.gitleaks.toml` allowlist added for the three known test-only secrets in `ci.yml` (integration and e2e JWT secrets, Playwright NEXTAUTH_SECRET)

- **Grafana observability stack** ([PR #60](https://github.com/mayloo89/circl/pull/60)):
  - `grafana/tempo:2.7.2` added to `docker-compose.yml` — OTLP HTTP (4318) + gRPC (4317) receivers; metrics_generator forwards service-graph and span-metrics to Prometheus via remote-write; 7-day trace retention; local filesystem storage
  - `prom/prometheus:v3.3.1` added to `docker-compose.yml` — scrapes backend at `host.docker.internal:8080/metrics`; remote-write receiver enabled for Tempo metrics_generator; `ops/prometheus/prometheus.yml` + `ops/prometheus/alerts.yml`
  - Recording rules: `job:circl_http_error_rate:rate5m`, `job:circl_http_request_rate:rate5m`, `job:circl_http_p95_latency:rate5m`
  - Alert rules: `HighErrorRate` (>1% 5xx for 5m), `HighLatencyP95` (>1s p95 for 5m), `DBPoolExhausted` (>90% pool for 2m), `BackendDown` (scrape target gone for 1m)
  - `grafana/grafana:11.5.2` added to `docker-compose.yml` on port 3001 (3000 is Next.js); anonymous access enabled for dev
  - `ops/grafana/provisioning/datasources/datasources.yaml`: Prometheus (default, exemplar→Tempo), Loki (derived field `trace_id`→Tempo), Tempo (traces-to-logs via Loki, service map, node graph)
  - `ops/grafana/provisioning/dashboards/dashboards.yaml`: file provider pointing to `/var/lib/grafana/dashboards`
  - 4 dashboards-as-code: `http-red.json` (request rate, error rate, p95/p99 latency, top routes by rate and latency), `websocket.json` (active connections over time), `db-pool.json` (utilisation gauge, connection breakdown, utilisation time series), `infrastructure.json` (goroutines, heap memory, GC pauses, CPU, open FDs)
  - `backend/.env.example`: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, `OTEL_SAMPLE_RATE`, `LOKI_URL` documented
  - `internal/logger/loki.go`: batching `lokiWriter` — groups zerolog JSON lines by `level`, flushes every 2s or at 100 entries, pushes directly to Loki's HTTP push API; no extra dependencies; activated when `LOKI_URL` is set
  - `logger.New` returns `(zerolog.Logger, func())` — flush function drains the Loki buffer on graceful shutdown; no-op when Loki is not configured
  - OTel export errors routed through zerolog at `warn` level via `otel.SetErrorHandler` — respects `LOG_LEVEL`, shows up in Loki, no more raw stdlib log spam on startup
  - `godotenv.Load()` moved to top of `main()` so `LOKI_URL`, `LOG_LEVEL`, and all other env vars from `.env` are visible before the logger (and every other component) initialises

- **Log shipping pipeline** ([PR #58](https://github.com/mayloo89/circl/pull/58)):
  - `grafana/loki:3.4.2` added to `docker-compose.yml` — single-binary mode, filesystem storage, 7-day retention via compactor, healthcheck on `/ready`
  - `grafana/alloy:v1.7.5` sidecar collects all container stdout via Docker socket (read-only mount); Alloy UI on `http://localhost:12345`
  - `ops/alloy/config.alloy`: River pipeline — `discovery.docker` → `loki.source.docker` → `loki.process` (JSON stage extracts `level`/`component` as indexed labels, adds static `env=dev`) → `loki.write`
  - `ops/loki/config.yaml`: single-binary config (inmemory ring, tsdb schema v13, filesystem chunks), `limits_config.retention_period: 168h`, compactor with `retention_enabled: true`
  - `ops/loki/logql-examples.md`: query cookbook — all-backend, error filter by label, component filter, HTTP path regex, slow requests, log-trace correlation via `trace_id`, error-rate metric, infrastructure (postgres/redis)

- **OpenTelemetry distributed tracing** ([PR #57](https://github.com/mayloo89/circl/pull/57)):
  - New `internal/tracing` package: `Init()` configures TracerProvider with OTLP HTTP exporter; falls back to a no-op exporter when `OTEL_EXPORTER_OTLP_ENDPOINT` is not set so the app starts without a collector
  - `tracing.HTTPMiddleware`: chi-compatible middleware using W3C TraceContext propagation; reads `chi.RouteContext` after `next.ServeHTTP` for low-cardinality route-pattern span names (e.g. `HTTP GET /chat/rooms/{id}`)
  - `tracing.NewPgxTracer`: implements `pgx.QueryTracer` — one span per DB query with `db.statement` and `db.rows_affected` attributes; zero new external dependencies
  - `internal/worker/otel.go`: `taskEnvelope` wraps asynq payloads with W3C trace headers; `otelMiddleware` extracts context and creates a `worker.<task>` consumer span; backward-compatible with old-format tasks
  - `chat/handler.go`: WebSocket session span (detached context via `trace.ContextWithRemoteSpanContext`) survives beyond HTTP handler return; per-message spans use inline closure pattern so `defer span.End()` fires on all exit paths
  - `middleware/logger.go`: injects `trace_id` and `span_id` from the active span into zerolog access-log entries for log-trace correlation
  - `cmd/api/main.go`: `signal.NotifyContext` replaces bare `context.Background()` for graceful shutdown; `http.Server.Shutdown(30s)` + `tp.Shutdown()` called on SIGINT/SIGTERM
  - `db/db.go`: `Option func(*pgxpool.Config)` variadic pattern allows attaching the pgx tracer at startup without changing the `Open` signature

- **Prometheus metrics + health checks** ([PR #56](https://github.com/mayloo89/circl/pull/56)):
  - `prometheus/client_golang`: Go runtime collector, HTTP handler collector (per-route latency histograms, status code counters), active WebSocket gauge, DB pool metrics
  - `/metrics` endpoint (protected by `METRICS_TOKEN` env var when set)
  - Enhanced `/health`: DB ping + Redis ping + `version` field (injected at build time via `-ldflags`)
  - `backend/Dockerfile`: `HEALTHCHECK` on `/health`; runs as non-root `USER`
  - `frontend/Dockerfile`: `HEALTHCHECK` on `/api/health`; runs as non-root `USER`

### Fixed
- **Frontend fetch hardening + auto sign-out** ([PR #59](https://github.com/mayloo89/circl/pull/59)):
  - Six fetch chains (`chat/page.tsx`, `chat/channels/page.tsx`, `chat/[roomId]/page.tsx`, `contacts/page.tsx`, `GroupMembersPanel.tsx`, `CreateGroupModal.tsx`) now check `r.ok` before calling `.json()` — prevents TypeError crashes (`rooms.map is not a function`, `channels.filter is not a function`) when the backend returns an error object instead of an array
  - `profile/page.tsx` and `GroupMembersPanel.tsx` (`loadContacts`) also hardened
  - `lib/auth.ts` jwt callback decodes the `exp` claim of the backend JWT on every session refresh; sets `token.error = "TokenExpired"` when expired
  - `SessionGuard` component (mounted in root `providers.tsx`) calls `signOut({ callbackUrl: "/login" })` when `session.error === "TokenExpired"` — eliminates the "logged-in but everything is 401" broken state
  - `types/next-auth.d.ts`: `error?: string` added to `Session` and `JWT` interfaces

### Added
- **Structured logging with zerolog** ([PR #55](https://github.com/mayloo89/circl/pull/55)):
  - `github.com/rs/zerolog` replaces the standard `log` package across the entire backend (51 call sites)
  - New `internal/logger` package: `New(env, level string) zerolog.Logger` — human-readable console output in development, JSON to stdout in production; log level configurable via `LOG_LEVEL` env var (default: `info`)
  - New `middleware.RequestLogger(log)` middleware: assigns a unique `request_id` to every request using `xid`, attaches a request-scoped logger to the context via `zerolog.Ctx`, sets the `X-Request-ID` response header, and writes one structured access-log entry per request with `method`, `path`, `status`, `latency_ms`, and `request_id`
  - `middleware.EnrichRequestLog(ctx, key, value)`: pointer-based context accumulator that lets downstream middleware contribute fields to the access-log entry — used by `RequireAuth` to add `user_id` and role to every authenticated request's log line
  - `RequireAuth` middleware now enriches the context logger with `user_id` so every log line emitted inside an authenticated handler automatically carries the user identity
  - Full traceability chain: every log line within a request shares the same `request_id`; authenticated requests additionally carry `user_id`; correlation requires only the `request_id` field
  - All background workers receive a `zerolog.Logger` at construction and tag their log lines with a `component` field: `push`, `uploads`, `ephemeral_cleaner`, `image_worker`, `purge_worker`
  - `asynqLogger` bridge: asynq's internal log output is routed through zerolog with `component=asynq`
  - No PII in logs: email addresses are never logged; user IDs (internal UUIDs) are acceptable
  - 5 new unit tests for `RequestLogger`: request_id header, access-log fields, context logger propagation, `EnrichRequestLog` accumulation, implicit 200 status

- **Internationalisation — ES / EN / PT** ([PR #54](https://github.com/mayloo89/circl/pull/54)):
  - Frontend fully translated into Spanish (default), English, and Portuguese using `next-intl` with prefix-based URL routing (`/es/`, `/en/`, `/pt/`)
  - All pages translated: auth, browse, chat list, chat room, channels, contacts, profile (own + public), settings, admin
  - All shared components translated: `ChatInput`, `MessageBubble`, `GroupMembersPanel`, `CreateGroupModal`, `ConfirmDialog`, `ReportDialog`, `PushPrompt`, `ContactCard`, `SearchBar`, `PhotoGallery`; message files cover all namespaces (auth, nav, common, chatRoom, channels, contacts, profile, publicProfile, report, pushPrompt, browse, settings, admin)
  - `app/` restructured to `app/[locale]/`; root layout reduced to a minimal shell; `[locale]/layout.tsx` owns `<html lang>`
  - `i18n/routing.ts`, `i18n/request.ts`, `i18n/navigation.ts` wiring; locale-aware `Link`, `useRouter`, `usePathname` from `@/i18n/navigation`
  - Language switcher in the user dropdown (NavBar) and a dedicated Language section in Settings — both persist the chosen locale to the backend via `PUT /profiles/me/preferences`
  - Shared `apierror` package (`backend/internal/apierror`): `Write()`, `WriteJSON()`, and ~25 stable machine-readable error code constants (`unauthorized`, `username_taken`, `invalid_token`, etc.)
  - All 10 backend handlers migrated from ad-hoc `http.Error` / `json.Encode` to `apierror.Write` / `apierror.WriteJSON`; frontend error checks updated to use the stable `code` field instead of matching English message strings
  - Migration `000025`: adds `locale TEXT NOT NULL DEFAULT 'es'` to `profile_preferences`; `GET/PUT /profiles/me/preferences` persists and returns the locale
  - Auth and intl middleware merged into a single `middleware.ts` (was split between `proxy.ts` + `middleware.ts`)

### Fixed
- **Next.js downgraded from 16.2.2 to 16.1.1**: Turbopack memory regression in 16.2.2 caused unbounded memory growth (7+ GB) in the dev server; 16.1.1 is stable

- **Admin panel: channel management, role-based access control, and hard delete** ([PR #52](https://github.com/mayloo89/circl/pull/52)):
  - Channel management UI in the admin panel: create, edit, and delete public channels
  - Role-based access control: super_admin role can promote/demote admins; admin cannot modify other admins
  - Hard delete for users: permanently removes all associated data (profile, photos, messages, uploads) from DB and S3
  - Admin panel sidebar navigation with Dashboard, Users, Reports, and Channels sections

- **UX polish** ([PR #53](https://github.com/mayloo89/circl/pull/53)):
  - Inline registration validation uses touched-state pattern — errors only shown after the user has interacted with a field
  - Error messages read from backend response body instead of generic fallback strings
  - 429 rate-limit errors differentiated from credential errors on the login page
  - Settings: change-password form collapses behind a button (expanded on click); delete-account moved to a modal with confirmation step
  - Gender options extended: Male, Female, Trans male, Trans female, Non-binary, Prefer not to say, Custom (free-text input)

- **Reversible account deletion with 30-day grace period** ([PR #51](https://github.com/mayloo89/circl/pull/51)):
  - Migration `000023`: adds `deleted_at TIMESTAMPTZ` column to `users`
  - `DELETE /users/me` now sets `status = 'deleted'` and records `deleted_at` timestamp; sends a deletion warning email async with a link to sign in and reactivate
  - Reactivation is automatic on login: `POST /auth/login` restores `status = 'active'` when valid credentials are supplied within the 30-day grace period and returns `{"reactivated": true}` in the response
  - Accounts past the 30-day window receive `401 {"error":"invalid credentials"}` — indistinguishable from wrong password
  - Login page: detects `reactivated: true` via a direct pre-signIn probe and shows a modal popup ("Account reactivated") before navigating home
  - Daily background worker (`worker.PurgeDeletedAccounts`) fully purges expired deleted accounts:
    - Retrieves all S3/MinIO upload keys for the user
    - Deletes files from object storage (originals + thumbnails)
    - Removes all associated DB records: uploads, profile photos, profiles, contacts, push subscriptions, room memberships
    - Anonymizes the `users` row in-place (email → `deleted-{id}@purged`, password hash cleared, status → `purged`) — row is kept to preserve `messages.sender_id` FK integrity
  - Settings delete-account section: updated messaging to mention the 30-day window and reversibility
  - `TEST_ENDPOINTS_ENABLED=true` guard for `POST /test/users` (E2E fixture endpoint) — never exposed in production
  - E2E fixtures: `createUser` now calls `POST /test/users` directly, bypassing email verification requirement

- **Email infrastructure: registration verification, forgot password, reset password** ([PR #50](https://github.com/mayloo89/circl/pull/50)):
  - Migration `000022`: adds `email_verified_at` to `users`; creates `password_resets` and `email_verifications` tables
  - New `email` package with `Sender` interface, `ConsoleSender` (stdout, for dev/test), and `SMTPSender` (go-mail, TLS-opportunistic)
  - Mailpit added to `docker-compose.yml` for local email capture (SMTP port 1025, web UI port 8025)
  - Email verification is **hard-enforced**: `POST /auth/login` returns `403 {"error":"email_not_verified"}` for unverified accounts; no JWT is issued
  - Registration sends a verification email async (goroutine) and returns `201 {"message":"..."}` without a JWT
  - New public endpoints: `POST /auth/forgot-password`, `POST /auth/reset-password`, `POST /auth/verify-email`, `POST /auth/resend-verification`
  - Tokens: 32-byte cryptographically random, hex-encoded; SHA-256 hash stored in DB; expiry 1h (reset) / 24h (verification)
  - `forgot-password` and `resend-verification` are email-enumeration-safe (always return 200)
  - Frontend: `/forgot-password`, `/reset-password`, `/verify-email` pages added
  - Login page: "Forgot password?" link, email-not-verified warning banner with inline resend button
  - Register page simplified to email + password + confirm only; shows "Check your email" confirmation state after submit
  - `registerEmailPasswordSchema` added to `frontend/lib/validation.ts`
  - NextAuth `authorize`: distinguishes `EmailNotVerified` (403) from invalid credentials for granular error handling

- **Chat upload restrictions + image resizing** ([PR #49](https://github.com/mayloo89/circl/pull/49)):
  - Chat attachments restricted to images and videos only — PDF, documents, and archives are now rejected by both backend and frontend
  - JPEG and PNG originals are resized to a maximum of 1024px on their longest edge before storage (configurable via `IMAGE_MAX_PX` env var)
  - Thumbnails continue to be generated at 480px
  - `ChatInput` file picker `accept` attribute updated to `image/*,video/*`

- **UX improvements** ([PR #48](https://github.com/mayloo89/circl/pull/48)):
  - Contact removal now requires confirmation via dialog — prevents accidental deletions
  - Search results: clicking a user's name or avatar navigates to their public profile
  - Registration form: per-field inline validation on blur, live password requirements checklist (8+ chars, uppercase, lowercase, number), submit button disabled until all rules are met
  - `PasswordRequirements` component extracted to `components/ui/PasswordRequirements.tsx` and shared between registration and settings pages

- **Settings page** ([PR #47](https://github.com/mayloo89/circl/pull/47)): `/settings` page with account management
  - Push notifications toggle (enable/disable)
  - Change password with live validation (8+ chars, uppercase, lowercase, number, special char)
  - Delete account with confirmation
  - `PushContext` shared between NavBar bell and settings page for sync state
  - Avatar dropdown menu replaces profile/settings links in NavBar
  - `PushPrompt` dismiss button

- **Public chat channels** ([PR #46](https://github.com/mayloo89/circl/pull/46)): IRC-style open rooms — any authenticated user can enter, chat, and leave freely
  - Migration `000020`: extends `rooms.type` CHECK to include `'channel'`; adds `rooms.description` column
  - Migration `000021`: removes existing `room_members` rows for channels (ephemeral model)
  - Channel creation restricted to admin users (`POST /chat/channels` requires `is_admin` claim)
  - `GET /chat/rooms/{id}`: new endpoint returning room data for any authenticated user; channels accessible without membership; groups/DMs require `IsMember`
  - Ephemeral membership: channel membership = active WebSocket connection; no `room_members` rows; `IsMember` returns `true` for all authenticated users on channels
  - `GET /chat/rooms/{id}/members` for channels returns live hub participants (deduplicated by `user_id`)
  - `GET /chat/rooms/{id}/messages` returns `[]` for channels — no history served; each session starts fresh
  - `participant_join` / `participant_leave` WebSocket events broadcast on channel connect/disconnect; include `username`, `display_name`, `avatar_url`
  - `RoomParticipants` hub method: request-response pattern (goroutine-safe); deduplicates multiple connections from same user
  - `GetUsername` store method queries `profiles.username`; `username` propagated through `Client`, `ClientInfo`, and WS events
  - Channel list page: shows active count ("N online now"), "Enter" button for all channels, "New channel" button visible to admins only
  - Channel room: members sidebar open by default, filter input, self excluded from list, member names link to `/profile/[username]` in new tab
  - No file attachments and no ephemeral message controls in channels
  - Leave confirmation dialog intercepts all navigation (in-page buttons, NavBar `<Link>` clicks, browser `beforeunload`) before leaving a channel
  - `GET /chat/rooms` (list) unchanged — channels excluded since they have no `room_members` rows; channel room page falls back to `GET /chat/rooms/{id}` when not found in list

- **Group chat** ([PR #45](https://github.com/mayloo89/circl/pull/45)): `PUT /chat/rooms/{id}` (rename, admin only), `GET /chat/rooms/{id}/members` (list member profiles), `POST /chat/rooms/{id}/members` (add member, admin only), `DELETE /chat/rooms/{id}/members/{userID}` (remove/self-leave)
  - Migration `000019_group_management`: adds `creator_id UUID` to `rooms` for admin-gate checks
  - `CreateGroupModal` component: contact picker with group name input; posts to `POST /chat/rooms`
  - `GroupMembersPanel` component: inline side panel for member list, admin/remove/leave actions, rename form, add-member picker
  - Chat list page: "New group" button opens group creation modal
  - Room page: group header opens members panel; group name updates reactively on rename

- **Cursor-based pagination** ([PR #44](https://github.com/mayloo89/circl/pull/44)):
  - `GET /profiles/browse` now uses cursor-based pagination (`before` param with timestamp)
  - Chat history (`GET /chat/rooms/{id}/messages`) uses cursor-based pagination
  - Infinite scroll on browse page: load more button fetches next page
  - Chat room: "Load more" at top of message list for older messages

- **Web Push Notifications** ([PR #43](https://github.com/mayloo89/circl/pull/43)): `POST /push/subscribe`, `DELETE /push/unsubscribe`, `GET /push/vapid-public-key`, migration `000018_push_subscriptions` for browser push subscriptions
  - `backend/internal/push` package: `Service` (Send, Subscribe, Unsubscribe), `Store` interface (Postgres implementation), `Handler` for HTTP endpoints
  - Push notification UI in NavBar: bell icon toggles subscription state (granted/tachada), persists across sessions
  - `usePush` hook: `Notification.requestPermission()`, service worker registration at `/sw.js`, automatic re-subscription on page load if already granted
  - Service Worker (`public/sw.js`): `push` event listener displays native notifications, `notificationclick` handler navigates to URL
  - Push notification delivery on new chat messages and contact events (contact_request, contact_accepted, contact_removed) — wired in `main.go` via `notifyUser` callback
  - VAPID key configuration via `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT` environment variables; service disabled if keys are empty
  - `.vscode/launch.json`: debug configs for Backend (Go) and Frontend (Next.js), compound launch for full app
  - `PushPrompt` component: inline banner prompting users to enable notifications (shown only when permission is "default")
  - `hub.IsConnected(userID)` method: checks if user has active SSE connection

- **Account safety & moderation** ([PR #41](https://github.com/mayloo89/circl/pull/41), [PR #42](https://github.com/mayloo89/circl/pull/42)):
  - Migration `000017_admin_moderation`: adds `role VARCHAR(20)` column on `users` ('user' | 'admin'), `reports` table
  - Admin role: `PUT /users/{id}/role` endpoint restricted to admin users
  - User moderation: `PUT /users/{id}/suspend`, `PUT /users/{id}/activate`
  - User reporting: `POST /users/{id}/report` with `reason` body; rate limited to 10 reports per hour
  - Report consequences: auto-suspend on 3+ unresolved reports in 7 days
  - Admin report management: `GET /admin/reports`, `PUT /admin/reports/{id}/resolve|dismiss`
  - Password complexity: minimum 8 chars, 1 uppercase, 1 lowercase, 1 number, 1 special char
  - Login lockout: after 5 failed attempts within 15 minutes, 15-minute lockout
  - Rate limiting: `LOGIN_IP_LIMIT`, `REGISTER_IP_LIMIT` environment variables

- **User blocking** ([PR #39](https://github.com/mayloo89/circl/pull/39)):
  - Migration `000015_create_blocks` — adds `blocks` table
  - `POST /contacts/{id}/block`, `DELETE /contacts/{id}/block`, `GET /contacts/blocked`
  - Bidirectional block check in search, contacts, chat, WebSocket
  - `ConfirmDialog` UI component
  - Frontend block/unblock UI

- **Browse/explore** ([PR #37](https://github.com/mayloo89/circl/pull/37), [PR #38](https://github.com/mayloo89/circl/pull/38)):
  - Migration `000014_browse_indexes` — adds indexes for browse ordering and Haversine
  - `GET /profiles/browse` — paginated profile discovery with age/distance/gender/interests filters
  - Interest tag filter, sort-by-distance option
  - `/browse` frontend page with card grid, infinite scroll, filter sidebar
  - Coordinates (`latitude`, `longitude`) saved from Photon/OSM location suggestions
  - `GET /profiles/available?username=` — username availability check

- **Username** ([PR #36](https://github.com/mayloo89/circl/pull/36)):
  - Mandatory username at registration
  - `GET /profiles/@{username}` support
  - Profile URLs use username

- **Expanded profiles** ([PR #35](https://github.com/mayloo89/circl/pull/35)):
  - Migration `000013_expand_profiles` — adds DOB, gender, location, interests
  - `profile_preferences` table for search preferences
  - `GET/PUT /profiles/me/preferences`
  - Profile edit page: DOB picker, gender dropdown, location autocomplete, interests tag input
  - Public profile page displays age, gender, location, interests

- **Backend integration tests** ([PR #34](https://github.com/mayloo89/circl/pull/34)):
  - `internal/testutil` package — `OpenDB`, `CreateUser`, `NewRedis` helpers
  - Integration tests for chat and uploads stores
  - CI `backend-integration` job

- **Playwright E2E testing** ([PR #33](https://github.com/mayloo89/circl/pull/33)):
  - `playwright.config.ts` with chromium, retry-on-failure
  - E2E tests: auth, profile, contacts, chat flows
  - CI `e2e` job

- **Frontend testing foundation** ([PR #32](https://github.com/mayloo89/circl/pull/32)):
  - vitest setup with jsdom, testing-library, msw
  - Unit tests for UI primitives, hooks, lib helpers

- **Route guard cleanup and form validation** ([PR #31](https://github.com/mayloo89/circl/pull/31)):
  - `lib/validation.ts` with Zod schemas
  - Removed redundant auth redirects
  - Form validation on login/register

- **Domain component library** ([PR #30](https://github.com/mayloo89/circl/pull/30)):
  - `types/chat.ts`, `lib/chatHelpers.ts`
  - `MessageBubble`, `DateSeparator`, `TypingIndicator`, `ChatInput`, `Lightbox`
  - `ContactCard`, `SearchBar`, `PhotoGallery`, `ProfileHeader`

- **UI primitive component library** ([PR #29](https://github.com/mayloo89/circl/pull/29)):
  - `Avatar`, `Badge`, `Button`, `Input`, `Skeleton`, `Modal`, `Toast`, `PresenceDot`
  - Route loading/error boundaries

- **Public profiles and photo gallery** ([PR #27](https://github.com/mayloo89/circl/pull/27)):
  - `profile_photos` table, gallery uploads (max 6)
  - `GET /profiles/{userID}` — public profile endpoint
  - `/profile/[userId]` public profile page

- **Chat UI improvements** ([PR #25](https://github.com/mayloo89/circl/pull/25), [PR #26](https://github.com/mayloo89/circl/pull/26)):
  - Message grouping, date separators, skeleton loaders
  - New-message animation, relative timestamps
  - Empty and error states

- **Read receipts** ([PR #23](https://github.com/mayloo89/circl/pull/23)):
  - ✓ / ✓✓ checkmarks on messages
  - `read_receipt` WebSocket broadcast

- **Typing indicators** ([PR #22](https://github.com/mayloo89/circl/pull/22)):
  - Real-time "X is typing…" via WebSocket

- **Image thumbnails in chat** ([PR #24](https://github.com/mayloo89/circl/pull/24)):
  - Thumbnail URL on messages, LEFT JOIN for thumbnails

- **Image processing worker** ([PR #20](https://github.com/mayloo89/circl/pull/20)):
  - asynq worker for EXIF strip and 480px thumbnails

- **Ephemeral messages** ([PR #21](https://github.com/mayloo89/circl/pull/21)):
  - View-once (tap-to-view), TTL-based (15m–24h)
  - Tombstone messages, countdown badges

- **Chat attachments** ([PR #18](https://github.com/mayloo89/circl/pull/18)):
  - Image/video/file messages, lightbox

- **S3-compatible storage** ([PR #19](https://github.com/mayloo89/circl/pull/19)):
  - `S3Storage` via minio-go
  - Docker Compose dev setup

- **Avatar upload** ([PR #17](https://github.com/mayloo89/circl/pull/17)):
  - Profile avatar upload, navbar display

- **Storage infrastructure** ([PR #16](https://github.com/mayloo89/circl/pull/16)):
  - `Storage` interface, `LocalStorage`, uploads API

- **Real-time chat** ([PR #14](https://github.com/mayloo89/circl/pull/14)):
  - WebSocket DMs and group rooms
  - Redis Pub/Sub fan-out, message history

- **Presence** ([PR #15](https://github.com/mayloo89/circl/pull/15)):
  - Online/offline dot, "Last seen X ago"

- **SSE notifications** ([PR #14](https://github.com/mayloo89/circl/pull/14)):
  - `GET /notifications/stream`, contact events

- **Contacts** ([PR #9](https://github.com/mayloo89/circl/pull/9), [PR #10](https://github.com/mayloo89/circl/pull/10)):
  - Search, send/accept/decline requests

- **Auth** ([PR #2](https://github.com/mayloo89/circl/pull/2)):
  - Registration, login, JWT, httpOnly cookies

### Changed
- `hub.Notify` now called alongside `push.Send` in `notifyUser` callback for both chat and contact events
- Contact handler uses `contactNotifier` wrapper to enrich SSE events with push notifications
- All profile queries use `COALESCE` for avatar fallback
- Modern Go 1.22–1.24 refactor: `cmp.Or`, `strings.SplitSeq`, `t.Context()`

### Security
- Blocking is strictly caller-scoped (JWT-enforced)
- No information leak: blocked users' profiles remain accessible; only interaction is suppressed

### Testing
- 98%+ test coverage across all packages
- Backend integration tests with real Postgres/Redis
- Playwright E2E tests for auth, profiles, contacts, chat flows