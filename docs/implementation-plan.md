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
- [x] **Refresh token rotation + Redis blacklist** (PR #62): opaque 32-byte refresh tokens stored hashed in Redis with 7-day TTL; `POST /auth/refresh` validates, rotates (old token deleted before new one issued), and returns new access + refresh token pair; `POST /auth/logout` deletes the refresh token; password change and account deletion call `RevokeAllForUser` (timestamp-based invalidation via `rt:revoked_at:<userID>` key); access token TTL reduced from 24h to 15min; frontend `auth.ts` auto-refreshes silently on expiry; `SessionGuard` signs out on `RefreshFailed`; logout from navbar/settings also invalidates the refresh token server-side.
- [x] **OpenAPI spec** (PR #63): OpenAPI 3.1.0 spec for all ~40 endpoints under `docs/openapi.yaml`; covers 12 tag groups (System, Auth, Account, Profiles, Contacts, Chat, Notifications, Presence, Uploads, Reports, Push, Admin); reusable components (schemas, responses, securitySchemes); `@redocly/cli lint` job added to CI.
- [x] **UX overhaul — critical bugfixes** (PR #66): browse subtitle always showed "No profiles found" → `t("subtitle")`; unblock dialog showed generic error → `t("unblockConfirmMessage", { name })`; back buttons used `router.push("/")` → `router.back()`; login used `blue-*` colors → `indigo-*`; register success screen used `✉` emoji → inline SVG; "Block/Report" buttons had contrast ~2.5:1 (WCAG fail) → `text-gray-400`; chat room loading showed text → `<MessageSkeletons />`; active locale highlighted in Settings and NavBar; accepted contacts sorted online-first.

- [x] **UX overhaul — visual rebrand** (PR #67): `next/font/google` Nunito (display) + DM Sans (body); Tailwind v4 brand tokens via `@theme` (`--color-brand-*`, `--radius-card`, `--radius-pill`, `--shadow-card/card-hover`); CSS custom properties in `globals.css`; `accent` Button variant for CTA orange (#F97316); browse cards use `rounded-card`/`shadow-card`; conversion CTAs (`SendRequestButton`, "Add contact", "Message") migrated to `variant="accent"`.

- [x] **UX overhaul — navigation** (PR #68): `BottomNav` (mobile, 5 slots SVG icons + badges) + `Sidebar` (desktop ≥1024px) + `TopBar` (minimal mobile header); `ProfileContext` (single `/profiles/me` fetch per session); layout shell per breakpoint.

- [x] **UX overhaul — home dashboard** (PR #69): replaced 3-button placeholder with `PendingRequestsWidget` + `NearbyProfilesWidget` (horizontal scroll) + `RecentConversationsWidget`; `ProfileCompletenessBanner` with progress bar (sessionStorage dismiss); `lib/profileCompleteness.ts` pure utility.

- [x] **UX overhaul — browse** (PR #70): `RangeSlider` (single-thumb, fill track) + `BottomSheet` (slide-up mobile sheet, inline on desktop) primitives; `<details>` filter replaced by BottomSheet with active-filter badge; age min/max and distance inputs replaced by RangeSliders (500 km = "Any"); "Clear filters" CTA on empty state; i18n for all filter labels in EN/ES/PT; profiles with unknown distance excluded when max_distance_km is set (backend fix).

- [x] **UX overhaul — public profile hero** (PR #71): `<h1>` shows person's name; 55vh hero photo with gradient overlay + overflow menu (Block / Report / Unblock); sticky mobile action bar above bottom nav; inline desktop actions; "Preview as visitor" button on own profile page; Block/Report moved from inline links to overflow menu.

- [x] **UX overhaul — onboarding wizard** (PR #72): `/onboarding/{photo,bio,interests,location}` 4-step wizard; step-dot progress indicator + "Skip all"; `ProfileCompletenessCard` on own profile with progress bar + per-field links; `onboarded_at TIMESTAMPTZ` backend migration; `mark_onboarded` flag in `PUT /profiles/me`; `AppShell` redirects unonboarded users to wizard and suppresses nav on onboarding pages.

- [x] **UX overhaul — chat polish + auth UX + onboarding smart steps** ([PR #74](https://github.com/mayloo89/circl/pull/74)): `PasswordField` (show/hide toggle) on registration, login, settings, and reset-password pages; `DateOfBirthPicker` (three equal-width numeric selects DD/MM/YYYY, local state preserves partial selections); scroll-to-bottom FAB in chat room; chat list search (client-side filter) + 10s silent polling; onboarding smart steps via `lib/onboardingSteps.ts` (`incompleteSteps`, `nextStepAfter`) — skips already-complete steps and redirects to first incomplete on entry; E2E tests updated for new date picker.

- [x] **Modern Go + UX/UI audit fixes** ([PR #75](https://github.com/mayloo89/circl/pull/75)): 19 Go modernizations — `errors.Is()` (6 sites), `omitzero` (8 sites), `max()` builtin (2 sites), `for range n` (1 site), `strings.Cut` (1 site); frontend accessibility/usability pass — `cursor-pointer` global in `Button.tsx` + 20+ raw buttons; `prefers-reduced-motion` + `scroll-padding-top` + `scrollbar-hide` in `globals.css`; focus rings on all raw `<button>` elements; 5 emoji icons → accessible SVGs; descriptive `alt` text on 6 images; layout shift fixes (`scale-*` → `opacity-*`); BottomNav icon size unified to `h-5 w-5`.

- [x] **Pi deploy — configurable local storage URL** ([PR #78](https://github.com/mayloo89/circl/pull/78)): `LOCAL_STORAGE_BASE_URL` env var replaces hardcoded `http://localhost:<port>/uploads/files`; backend logs `base_url` instead of `path`; `deploy/` directory added to `.gitignore`; backend `Dockerfile` creates `/app/data` with correct ownership before `USER circl`.

- [x] **Pi deploy — CSP fix for Next.js App Router** ([PR #79](https://github.com/mayloo89/circl/pull/79)): `script-src` gains `'unsafe-inline'` (required for App Router bootstrap hydration); `style-src` adds `https://fonts.googleapis.com`; `font-src` adds `https://fonts.gstatic.com`; `connect-src` adds `wss:`; `img-src` broadened to `https:`.

- [x] **Privacy hardening** ([PR #82](https://github.com/mayloo89/circl/pull/82)): S-1 — `GET /profiles/{ref}` returns public subset for non-owners (age computed, no DOB/coords); S-2 — `GetPresence` filters `status = 'active'`; S-4 — presence gated to accepted contacts (`online: false` for non-contacts); S-7 — contact requests rate-limited at 100/day per user.

- [x] **WebSocket ticket auth** ([PR #83](https://github.com/mayloo89/circl/pull/83)): S-3 — `POST /ws-ticket` issues single-use UUID tickets (60 s TTL) stored in Redis; WS auth reads `?ticket=` and consumes via `GETDEL`; `useChat` hook exchanges Bearer JWT for a ticket before each WebSocket upgrade.

- [x] **CSP nonce** ([PR #84](https://github.com/mayloo89/circl/pull/84)): S-6 — per-request nonce in `middleware.ts` injected into request headers (`x-nonce`, `Content-Security-Policy`) so Next.js stamps its bootstrap scripts; `'unsafe-inline'` removed from `script-src`.

- [x] **Accessibility deep pass** (PR #88): centralised dialog focus management in a new `hooks/useFocusTrap.ts` (capture trigger, auto-focus first child, trap Tab, optional Escape, scroll-lock, restore focus on close); `Modal` and `BottomSheet` refactored to delegate (BottomSheet previously had no trap and no auto-focus); `GroupMembersPanel` gains `role="dialog"` + localized `aria-label` (channel/group settings) and joins the trap. New `hooks/useMenuKeyboard.ts` implements WAI-ARIA menu keyboard semantics (auto-focus first menuitem, ArrowUp/Down wrap, Home/End, Escape closes, Tab passes through, focus restored to trigger); wired into the public profile overflow menu and the chat input ephemeral-message menu, both of which gain `role="menu"` / `role="menuitem"` / `aria-haspopup` / `aria-expanded`. Decorative SVG sweep — 17 icons across 8 files patched, all 77 SVGs in the codebase now carry one of `aria-hidden`/`aria-label`/`role`. Hardcoded `aria-label` strings localized (close buttons, conversations landmark, profile avatar). `RangeSlider` gains `aria-label` + `aria-valuetext` (announces formatted value, e.g. "Any distance"). Manual VoiceOver / iOS Safari pass deferred to a follow-up — structural a11y is CI-verified, hands-on screen-reader testing requires a real device session.

- [x] **Desktop UX + accessibility — phase 1** (PR #87): "Nuevo chat" entry point on `/chat` — `NewChatModal` lists accepted contacts with search and creates a DM via `POST /chat/rooms/dm`; `ChatListPane` fetches `/contacts` count once and disables both compose buttons (chat + group) with localized `title` tooltips when `contacts.length === 0`. W-2 — desktop two-column DM layout via a Next.js layout-owned, persistent side rail. Chat routes restructured around a `(messages)` route group: `app/[locale]/chat/(messages)/layout.tsx` owns the `ChatListPane` (mounted once) so navigating between `/chat` and `/chat/<roomId>` and between threads no longer flickers or refetches. Channel rooms moved out of `/chat/[roomId]` to a dedicated `/chat/channels/[channelId]` route that lives outside the (messages) group — channels never inherit the DM list pane because navigating between channels risks accidental membership/message loss. The 750-line room body extracted into `components/chat/RoomView.tsx` (accepts `surface: "messages" | "channels"` to control back/leave destination); both new room routes render `<RoomView />`. Legacy URLs redirect transparently (channel id under `/chat/<id>` → `/chat/channels/<id>`, and the inverse). `ChatListPane` (`components/chat/ChatListPane.tsx`, `variant="page" | "pane"`) marks the selected row with `aria-current="page"` + brand-tinted background. W-7 — skip-to-content link in `AppShell` (`sr-only focus:not-sr-only`) jumps to `<main id="main-content">`; EN/ES/PT copy in `nav.skipToContent`. Channel-leave copy (`chatRoom.leaveChannelMessage`) rewritten across locales to make the irreversible message-access loss explicit. W-1 was already shipped in PR #68 (sidebar `isActive` + `aria-current`); no further work needed. W-3/4/5/6 (keyboard nav, ARIA landmarks, focus traps, VoiceOver) deferred to PR #88.

- [x] **Mobile UX + navigation audit** ([PR #86](https://github.com/mayloo89/circl/pull/86)): M-1 — iOS safe-area insets via `@utility pt-topbar` / `@utility pb-bottomnav` in `globals.css` applied to `TopBar`, `BottomNav`, `AppShell`, `Toast`, public profile sticky bar; M-2 — `text-base` (16 px) on `Input` and `ChatInput` textarea prevents iOS auto-zoom; M-3 — `ChatInput` migrated from `<input>` to auto-growing `<textarea>` (Enter sends, Shift+Enter newline, max-h-40 + scroll); M-4 — `capture="user"` on onboarding photo input (avatar selfie), `capture="environment"` on chat file input; M-5 — `PushPrompt` defers until user has ≥1 chat room (falls back to 30 s); M-6 — full touch-target audit: bell buttons, BottomSheet close, ProfileCompletenessBanner dismiss, chat room back button (added `p-3`), public profile hero/overflow buttons (`h-11 w-11`), own profile mobile Settings/Logout (`p-3`); mobile settings/logout added to profile page header (`lg:hidden`); collapsed sidebar logout moved to always-visible dedicated row above user/avatar row; BottomNav restructured to 5 items (Home, Browse, Messages, Channels, Contacts — Profile removed, accessed via TopBar avatar `p-1.5`); Sidebar Profile nav item removed (avatar row is sole desktop entry); back buttons removed from Contacts, own profile, `/chat`, and `/chat/channels` (all top-level pages); profile preview mode on `/profile/[username]` — sticky banner + hidden action buttons instead of redirect; layout consistency: Contacts + Profile edit `max-w-lg` → `max-w-2xl`, public profile content `mx-auto max-w-2xl`.

- [x] **Pi deploy — disable Next.js image optimizer** ([PR #80](https://github.com/mayloo89/circl/pull/80)): `NEXT_PUBLIC_IMAGE_UNOPTIMIZED` build arg (`false` by default); when `"true"` sets `images.unoptimized: true` in `next.config.ts`, bypassing `/_next/image` and its `remotePatterns` check; eliminates CPU-intensive resize/WebP conversion on low-power Pi hardware; `frontend/Dockerfile` wires the arg through.

### Trust & Safety / legal (pre-launch blocker)

- [ ] **Terms / Privacy / Community Guidelines pages** — add `/terms`, `/privacy`, `/guidelines`, `/safety` MDX-driven routes in all three locales; "I agree to Terms + Privacy" checkbox on registration; footer links from login + Settings.
- [ ] **GDPR Art. 20 / CCPA data export** — `POST /account/export` enqueues an asynq job → generates a JSON + media zip in S3 → emails a one-time signed URL with 14-day retention.
- [ ] **Image moderation in the upload worker** — NSFW classifier + CSAM detection (open-source / AWS Rekognition / Cloudflare Images / PhotoDNA / Thorn Safer); rejected uploads return `code=upload_rejected_moderation`; admin notified.
- [ ] **Expanded report categories** — `underage`, `threat_violence`, `csam`, `impersonation`. CSAM auto-pages on-call and bypasses the normal queue.
- [ ] **Age-verification audit log + policy doc** — log "claims 18+" with timestamp + IP at registration; document the policy under `/safety`. Identity verification (Stripe Identity / Veriff) deferred to a paid tier or report-flagged users.
- [ ] **Secret rotation verification** — confirm `JWT_SECRET` and VAPID keys have been rotated since the original commit removal; document rotation steps in a runbook (deferred to Phase 4).
- [ ] **Appeals process for suspended/banned users** — on suspension, send email with reason + an unauthenticated `/appeal/{token}` link; admin queue gains an "Appeals" tab.

### Privacy controls UI

- [ ] **Block / unblock surfaced in the public profile overflow menu** — block exists today; verify unblock is reachable for every state.
- [ ] **"Don't show my distance to non-contacts" toggle** in `/settings`; backend already filters via `profile_preferences`.
- [ ] **Hide presence / last-seen toggle** in `/settings` — symmetric: turning off your visibility also hides others' presence from you.
- [ ] **Read-receipts and typing-indicator opt-out** — symmetric (you don't see others' reads if you've turned yours off); backend suppresses both emit and receive on the WS frames.
- [ ] **Per-category notification toggles** in `/settings` — chat messages, contact requests, channel mentions, system. Persist in `profile_preferences`.
- [ ] **Manage blocked users list** in `/settings` — avatar + name + Unblock action.

### Discovery & retention

- [ ] **Browse ranking** — score = shared interests + recency of activity + distance, with a deterministic shuffle per session.
- [ ] **Pause-discovery toggle** — single boolean on `profile_preferences` ("don't show me to others").
- [ ] **Primary-photo selector** in the profile photo gallery.
- [ ] **Profile-completeness gating** — blur browse cards under 40% completeness with a CTA on the user's own card.

### Communication features

- [ ] **Message reactions, reply-to threading, in-room message search**.
- [ ] **Link previews in chat** — server-fetched OG metadata, cached.
- [ ] **Lightbox swipe-to-close + pinch-to-zoom** (`components/chat/Lightbox.tsx`).

### Group / channel admin

- [ ] **Group enhancements** — avatar, description, per-room mute, invite links, admin transfer.
- [ ] **Channel enhancements** — slowmode, per-channel kick (separate from platform ban), pinned announcements.

### Web / desktop polish

- [ ] **Browse filter state synced to URL** `searchParams` so filters survive refresh and URLs are shareable; restored on mount.
- [ ] **Form-error a11y** — `Input.tsx` lacks `aria-invalid` and `aria-describedby`; on submit error, move focus to the first invalid field. Should pass axe-core CI for form pages.
- [ ] **`app/robots.ts`** with `Disallow: /` (or selectively indexable); `metadata.robots: { index: false }` on profile/chat/contacts/admin layouts.
- [ ] **Color-contrast audit** — any remaining `text-gray-500/600` body text on `bg-gray-900` lifted to `text-gray-400` minimum (most fixed in earlier UX passes; sweep remaining call sites).
- [ ] **Browse card double-action cleanup** — card is a `<Link>` and the contact button blocks navigation via `e.preventDefault()`; replace with explicit two-action layout to remove the gestural ambiguity on mobile.
- [ ] **Contacts search results separation** — currently mixed with the established-contacts sections; render a dedicated search-results view above the lists or as a switch.
- [ ] **Manual screen-reader pass** — VoiceOver on iOS Safari + macOS Safari across every authenticated route; fix labels, redundant announcements, role/link semantics. Deferred from PR #88 because it requires a hands-on device session.

### i18n cleanup

- [ ] **Hardcoded `"en"` in `chatHelpers.ts`** `toLocaleDateString` calls — replace with `useLocale()`.
- [ ] **Hardcoded English distance strings** in browse ("km away", "< 1 km away") — move to `messages/*.json`.
- [ ] **BottomNav label wrap test** — verify ES/PT labels don't wrap at 360px viewport width.

### Performance

- [ ] **Server/client split** — page-level `"use client"` everywhere; split each route into a server shell + client island where viable.
- [ ] **Lazy-load heavy components** — `PhotoGallery`, `Lightbox`, `CreateGroupModal`, `NewChatModal` via `next/dynamic`.
- [ ] **Dynamic Type compatibility** — hardcoded `text-[10px]`, `text-[11px]`, `text-[12px]` in chat break iOS Dynamic Type; replace with Tailwind tokens.
- [ ] **Dedupe `/chat/rooms` fetch** — `ChatListPane` and `RoomView` both fetch room metadata; share via context provider so the right pane stops re-fetching when the layout already has it.

### Polish

- [ ] **Branded `not-found.tsx`** per locale.
- [ ] **`app/manifest.ts`** for PWA add-to-home.
- [ ] **Pull-to-refresh** on chat list and browse.
- [ ] **Offline banner** driven by `navigator.onLine`.
- [ ] **In-app notification inbox** — persistent log of past SSE events.
- [ ] **Email digest** for dormant users (>14 days inactive).
- [ ] **Status page or in-app degraded-service banner** driven by `/health`.
- [ ] **Feature-flag system** — env-driven minimum, `unleash` long-term.
- [ ] **Maintenance-mode flag** in config.
- [ ] **Marketing landing page** for unauthenticated visitors.

### Phase 4 — Production deployment & operations

- [ ] **Hosting decisions** — frontend on Vercel; backend on Fly.io / Render / AWS; PostgreSQL on Neon / RDS / Supabase; Redis on Upstash / ElastiCache; object storage on S3 / R2 with CloudFront / Cloudflare CDN. Domain registration and DNS.
- [ ] **TLS** — automatic via the platform or Let's Encrypt; force HTTPS redirect (HSTS already enabled in `SecurityHeaders` for production).
- [ ] **CI/CD pipeline** — staging deploy on push to `develop`, production deploy on push to `main`; coverage reporting gate (98%+ on handlers/services); Docker image build + registry push; environment secrets in GitHub Actions Settings.
- [ ] **Pre-deploy checks** — run migrations before deploy; database backup before destructive migrations; tested rollback plan with down migrations; platform health-check + readiness gates.
- [ ] **DB ops** — automated daily backups; periodic restore drill; `sslmode=require` (or `verify-full`) in production `DATABASE_URL`.
- [ ] **Secrets management** — `JWT_SECRET`, VAPID keys, `NEXTAUTH_SECRET`, SMTP credentials in vault/KMS (not env files); key-rotation runbook documented under `docs/runbooks/`.
- [ ] **Observability hosting decision** — Grafana Cloud (managed) vs. self-hosted Loki + Prometheus + Tempo + Grafana stack from PR #60.
- [ ] **Sentry integration** — frontend + backend error tracking; capture panics in goroutines (WebSocket pumps, hub).
- [ ] **Global per-IP API rate limit middleware** — beyond the existing per-endpoint limiters (`LOGIN_IP_LIMIT`, `REGISTER_IP_LIMIT`, contact-request 100/day, reports 10/hour).
- [ ] **WebSocket connection rate limit** — cap concurrent WS upgrades per IP.
- [ ] **`next/image` remote patterns** — replace dev `localhost:9000` MinIO entry with the production CDN hostname.
- [ ] **NextAuth cookie verification** — confirm `secure: true` / `httpOnly: true` / `sameSite: "lax"` are applied (automatic when `NEXTAUTH_URL` is `https://`, but worth confirming on first deploy).

### Phase 5 — Final polish & launch

- [ ] **WCAG 2.1 AA audit** — axe-core CI gate; manual VoiceOver run; keyboard-only walkthrough.
- [ ] **Operational runbooks** under `docs/runbooks/` — incident playbooks (DB outage, Redis outage, message-delivery degraded), on-call rotation, alert response procedures, secret rotation.
- [ ] **Final docs** — README polish, `CONTRIBUTING.md`, architecture diagram (component + data flow), `SECURITY.md` (responsible disclosure).
- [ ] **Pre-launch smoke test** — register → verify email → login → complete profile → add contact → send DM → join channel → block + unblock → delete account → reactivate.
- [ ] **Load test** concurrent WebSocket connections at expected peak.

> Observability PRs (#56–#60) follow the OTel convention: logs via Loki, metrics via Prometheus, traces via Tempo, all correlated by `trace_id` and unified in Grafana. Production hosting (managed vs. self-hosted) is decided in Phase 4.

## 9. Testing strategy
- Unit: handlers and services (auth, chat, profiles, contacts).
- Integration: real Postgres and Redis (skipped when env vars not set).
- E2E: Playwright (login, profiles, search, chat, presence flows).
- Security: rate limiting, headers (CSP/HSTS), payload size limits.

## 10. Operations
- SLIs: HTTP/WS latency, message delivery rate, 5xx error rate, heartbeat expiry.
- Alerts: WS drops, queue backlogs, worker errors, disk space, DB connections.
- Encrypted and tested backups; periodic key rotation.

## 11. Production environment reference

### Backend

| Variable | Dev default | Production requirement |
|----------|-------------|----------------------|
| `ENV` | `development` | `production` |
| `PORT` | `8080` | Platform-assigned or custom |
| `DATABASE_URL` | local with `sslmode=disable` | Managed DB with `sslmode=require` (or `verify-full`) |
| `REDIS_URL` | `localhost:6379` | Managed Redis URL with TLS |
| `JWT_SECRET` | dev value | `openssl rand -base64 64`, min 32 chars |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | `https://yourdomain.com` |
| `STORAGE_PROVIDER` | `local` | `s3` |
| `LOCAL_STORAGE_BASE_URL` | `http://localhost:8080/uploads/files` | Public URL for local storage if used in prod (Pi deploy) |
| `S3_ENDPOINT` | MinIO local URL | S3 / R2 endpoint |
| `S3_REGION` | — | Bucket region |
| `S3_BUCKET` | — | Bucket name |
| `S3_ACCESS_KEY` | — | IAM access key |
| `S3_SECRET_KEY` | — | IAM secret key |
| `IMAGE_MAX_PX` | `1024` | Tune for bandwidth vs. quality |
| `LOGIN_IP_LIMIT` | `20` | Tune per environment |
| `REGISTER_IP_LIMIT` | `10` | Tune per environment |
| `VAPID_PUBLIC_KEY` | — | Generate for Web Push |
| `VAPID_PRIVATE_KEY` | — | Generate for Web Push |
| `SMTP_HOST` | — | Production mail server |
| `SMTP_PORT` | — | 587 (STARTTLS) or 465 (TLS) |
| `SMTP_USER` | — | SMTP credentials |
| `SMTP_PASS` | — | SMTP credentials |
| `SMTP_FROM` | — | Sender address |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | unset (no-op exporter) | OTLP collector URL (e.g. Grafana Cloud or self-hosted Tempo) |
| `OTEL_SERVICE_NAME` | `circl-backend` | Same |
| `OTEL_SAMPLE_RATE` | `0.1` | Tune for trace volume |
| `LOG_LEVEL` | `debug` | `info` |
| `TEST_ENDPOINTS_ENABLED` | `true` (E2E only) | unset |

### Frontend

| Variable | Dev default | Production requirement |
|----------|-------------|----------------------|
| `NEXTAUTH_URL` | `http://localhost:3000` | `https://yourdomain.com` |
| `NEXTAUTH_SECRET` | dev value | `openssl rand -base64 32` |
| `BACKEND_URL` | `http://localhost:8080` | Internal backend URL (server-side only) |
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | `https://api.yourdomain.com` |
| `NEXT_PUBLIC_VAPID_PUBLIC_KEY` | — | Match backend `VAPID_PUBLIC_KEY` |
| `NEXT_PUBLIC_IMAGE_UNOPTIMIZED` | `false` | `true` only on low-power deploys (Pi) where `/_next/image` is too expensive |
