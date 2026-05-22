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

**Content & age policy (decided 2026-05-10).** Circl is an adults-only (18+) platform, AR-first (ES canonical). Three-tier content model:

- **Public surfaces** (profile avatar, photo gallery, channels): non-explicit content allowed (lingerie / swimwear); explicit nudity blocked by NSFW classifier on upload.
- **DMs** (only opens between mutually-accepted contacts): explicit nudity allowed between consenting adults.
- **Private albums** with per-user grants: the recommended surface for sharing explicit content — the grant log creates an auditable consent trail and is the platform's primary legal cover under **Ley 27.736 ("Ley Olimpia")**.

**Hard floors that override consent everywhere**: CSAM detection (PhotoDNA / Cloudflare CSAM Scanning Tool) and NCII matching (StopNCII.org). No "consenting adults" exception exists in law for these. Allowing nudity also rules out Vercel hosting (AUP), Stripe payments (AUP), and Apple App Store / Google Play distribution — Circl stays web-only with adult-compatible infra (Cloudflare Pages / self-hosted; CCBill / Segpay if/when monetised). Decision logged in `memory/project_content_policy.md`.

- [x] **Terms / Privacy / Community Guidelines / Safety pages** ([PR #101](https://github.com/mayloo89/circl/pull/101)) — four MDX-driven routes (`/terms`, `/privacy`, `/guidelines`, `/safety`) under `app/[locale]/` in ES (canonical) + EN + PT, rendered through a shared `LegalPage` chrome (header, last-updated stamp, `Borrador — requiere revisión legal antes del lanzamiento` banner, `Footer`). MDX wired via `@next/mdx` + a `legal-prose` utility in `globals.css` so the `.mdx` files stay class-free. `/privacy` references **Ley 25.326** + the AAIP-registration commitment (filing deferred to Phase 4); `/safety` references **Ley Olimpia** + the sextortion playbook (línea 144); `/guidelines` spells out the three-tier content model in user-facing language; `/terms` picks the City of Buenos Aires as venue. Registration consent: migration `000031` adds nullable `terms_accepted_at`, `privacy_accepted_at`, `accepted_policy_version` to `users`; `Authenticator.Register` now takes a `RegistrationInput` struct (similarly `Store.CreateUser` takes `CreateUserInput`) so the consent timestamps and current `auth.CurrentPolicyVersion = "v1"` flow through to the store; new `apierror.CodeTermsNotAccepted` returned when the register payload is missing `accept_terms: true`. New `Checkbox` UI primitive (44×44 touch target, `aria-invalid` + `aria-describedby` wiring, ReactNode label so the rich consent label can embed `Link`s to the legal pages); new `Footer` component (flush-left links / flush-right copyright on `sm+`, stacked-centered below) mounted on every auth page and `/settings`. `middleware.ts` gains a `PUBLIC_PAGES` allowlist so the four legal routes are reachable without authentication and don't bounce logged-in users away. Public contact email throughout: `info.circl.ar@gmail.com`. Existing accounts left as NULL on the consent columns (no backfill).
- [x] **Expanded report categories + priority queue + age-verification audit + appeals process** — second PR of the Trust & Safety block, backing the safety-page promises from PR #101. Migration `000032` adds three new report reasons (`non_consensual_intimate_images`, `digital_gender_violence` for Ley 27.736 / "Ley Olimpia", and `csam`), a `priority` column on `reports` (`normal` / `high` / `critical`) with index, the `age_verification_audit` table (user_id nullable + ON DELETE SET NULL so the row survives account hard-delete; captures email, attested_age=18, IP, user_agent, DOB, policy_version), and the `appeals` table (token hashed, 30-day TTL, unique partial index for one open appeal per suspension, status open → submitted → approved/denied/expired). Priority is derived from the reason server-side (`csam` + `non_consensual_intimate_images` → `critical`; `digital_gender_violence` → `high`; rest → `normal`) so callers can't downgrade. `GET /reports` accepts `?priority=` and returns rows ordered critical → high → normal then newest-first. New `auth.AgeAuditStore` interface + `NewAgeAuditStore` keep the audit method off the main `Store` interface so test fakes don't have to implement it; the register handler captures IP via the existing `clientIP()` + `User-Agent` + supplied DOB after `Register` succeeds and after the profile seed — audit failure is logged but does not block registration. New `internal/appeals` package, public `/appeal/{token}` (un-authenticated — added to `middleware.ts` `PUBLIC_PAGES`; locked-out user can reach it) with GET (fetch) and POST (submit body, 4096-char max); admin `/admin/appeals` (list + resolve), mounted inside the admin router via the new `admin.WithAppealsHandler` option to avoid chi mount collisions. New `admin.SuspensionNotifier` hook fires asynchronously after Suspend or Ban: mints a 32-byte hex token, persists it hashed, emails the user the appeal link plus the suspension reason; the suspension itself never blocks on the email. Approving an appeal calls back into `admin.Service` (the `Reactivator`) to flip status to `active`; denying leaves the suspension in place. Resolution emails the user the decision + admin note in either case. New email templates `AppealMessage` + `AppealResolutionMessage`. Admin sidebar gains an "Appeals" entry; new public `/[locale]/appeal/[token]` page; new admin `/[locale]/admin/appeals` page with status tabs + decision modal; ReportDialog updated with new categories + rose-tinted priority-queue notice; admin reports page renders a Priority column + filter. New `apierror.CodeAppealAlreadyResolved` is the only new error code. Full EN/ES/PT.
- [x] **Image moderation pipeline (framework + bundled detectors)** — `internal/moderation` ships a `Moderator` interface + `Chain` short-circuit + three concrete detectors: `HashList` (SHA-256 lookup against the new `image_block_hashes` table — extensible to StopNCII / PhotoDNA feeds by inserting rows with the right `source` value), `Heuristic` (file-size + dimension + aspect-ratio bounds), `NSFW` (wraps a pluggable `NSFWClassifier` — bundled `NoopClassifier` until a real model adapter lands). Worker hooks the chain after image decode and before thumbnail; rejections delete the storage object, mark the row (`moderation_status='rejected'` + reason), and short-circuit; moderator errors fail open. Admin queue at `/admin/moderation`; admin curates the local hash list at `POST /admin/moderation/hashes`. Frontend `useUpload` polls `/uploads/{id}` after confirm and surfaces the rejection reason. Migration `000034`. New `apierror.CodeUploadRejectedModeration`.
- [x] **Image-rejection UX + admin review** — closes the visible gap from the moderation pipeline: end-users now see a dedicated `UploadRejectionModal` (focus-trapped, EN/ES/PT bodies routed off `moderation_code` so the message stays meaningful across reason rotations) instead of a buried inline error; `useUpload` exposes rejections as a structured `rejection` field separate from generic `error`. Backend persists `moderation_score` + `moderation_categories` (migration `000035`) so admins see what the classifier actually saw, not just "above threshold". A per-code retention policy (`retainFileFor` in `internal/worker`) keeps NSFW + heuristic rejection files for `MODERATION_REJECTED_RETENTION_DAYS` (default 30) so admins can verify false positives; hash-list matches are still purged immediately (CSAM/NCII legal posture). New admin surface `/admin/moderation` lists rejected uploads with thumbnail, score, categories, code filter, and a lightbox preview backed by an authenticated `GET /admin/moderation/{id}/image` stream (`AuthedImage` component handles the Bearer-header → blob URL dance). New cleanup task `worker.PurgeExpiredModerationFiles` runs daily alongside `PurgeDeletedAccounts`.
- [ ] **Vendor integrations to plug into the moderation pipeline** (no code change required to the orchestration layer once these land):
  - **CSAM** — Cloudflare CSAM Scanning Tool (free, easy onboarding) or PhotoDNA (Microsoft, free but requires NCMEC application). A new `Moderator` impl that calls the vendor API; insertion point is the existing `Chain` in `cmd/api/main.go`.
  - **NCII** — apply to **StopNCII.org** as a participating platform; periodically fetch their hash feed and insert into `image_block_hashes` with `source='stopncii'`. The existing `HashList` detector already rejects on match.
  - [x] **NSFW classifier — NudeNet sidecar** ✅ Python service in `ops/moderation/` exposing `POST /classify`; `HTTPNSFWClassifier` (`internal/moderation/nudenet.go`) implements `NSFWClassifier` and is wired in `main.go` when `MODERATION_API_URL` is set. Threshold via `NSFW_THRESHOLD` (default 0.80). Rejects explicit nudity from public surfaces. `album-private` uploads use tag-not-block (see NSFW.Check: `ContextPrivate` returns `Allow()`).
- [x] **Habeas Data / GDPR Art. 20 data export** — `POST /users/me/exports` enqueues an asynq `export:user` task → builds a versioned `data.json` (account, profile, preferences, contacts, blocks, rooms, sent messages, filed reports, age attestations, uploads) plus the bytes of every media file the user owns (avatar, gallery, chat attachments) → zips it → uploads to `exports/<user_id>/<request_id>.zip` → emails a single-use 14-day download link. Public download mounted at `/account/export/{token}` — the path token is the bearer credential because the user may have lost their session by the time the email arrives. Migration `000033` adds an `export_requests` table with a unique partial index that allows at most one in-flight build per user; a 24h cool-down is enforced atomically inside `Create` via `WHERE NOT EXISTS`. Missing media items are best-effort (logged + skipped) so a single dead key cannot abort the entire export. Private-album grants log will be added in PR #5 when that table exists.
- [x] **Private albums with consented sharing — core**: owners group `album-private` uploads into named albums and grant access to specific contacts. Migration `000036` adds `private_albums`, `private_album_photos`, `private_album_grants` (partial unique index keeps at most one open `(album, grantee)` grant while preserving history), and the `private_album_views` access log. New `internal/albums` package exposes CRUD + grant lifecycle (`invite` push, `request` pull, `chat` auto-grant via DM share). The `roleFor()` predicate at the service layer is the single read gate. Photo bytes stream through `GET /albums/{id}/photos/{upload_id}/file` with `Cache-Control: private, max-age=60`. Chat integration: new `MessageTypeAlbumShare` rendered via the existing `new_message` envelope; an adapter in `cmd/api/main.go` bridges `albums.ChatBridge` to `chat.Store.SaveMessage` + `chat.Hub.Publish` so the albums package never imports the chat package directly. Frontend `/albums` route + detail page + sidebar entry + chat share affordance, all EN/ES/PT. NSFW classifier is **tag-not-block** for `album-private` context (hash-list and heuristic still hard-block); error logging added to the 500 paths in the albums and uploads handlers.
- [x] **Private albums — time-limited grants, immediate invite access, owner attribution**: four improvements shipped in the same PR as the core albums work. Migration `000040` adds `expires_at TIMESTAMPTZ NULL` to `private_album_grants`; every read path gates on `expires_at IS NULL OR expires_at > NOW()`; a daily goroutine in `main.go` calls `albumsStore.ExpireAlbumGrants` to formally revoke expired grants for the audit trail. Both `POST /albums/{id}/grants/invite` and `POST /albums/{id}/share-in-chat` accept `expires_in` preset (`24h` | `7d` | `30d` | `none`). `invite`-source grants are now created with `status=active` immediately — the prior `pending` flow was broken (no notification, no accept UI); `AcceptGrant`/`DenyGrant` now only handle `source=request`. `ListAlbumsSharedWith` LEFT JOINs `profiles` so shared album cards carry owner display name + avatar URL in one round-trip. `AlbumCard` renders the owner row; the decorative mountain-glyph SVG was removed. Frontend: `MembersPanel` and `ShareAlbumDialog` both expose a four-chip expiry picker; `MembersPanel` shows remaining access time per grant row; the album detail viewer notice shows the expiry date when set. EN/ES/PT i18n throughout.
- [x] **Private albums — hardening**: (1) deterrence watermark (`compositeWatermark` in `internal/albums/watermark.go`) burns the viewer's full UUID + UTC timestamp into a semi-transparent pill (goregular font, 3× supersampled then CatmullRom-downscaled for smooth rendering) at the bottom-right of every photo served to a non-owner; JPEG and PNG handled natively, WebP decoded and re-encoded as JPEG; owners receive raw bytes; (2) 20-invitations/day rate limit on `POST /albums/{id}/grants/invite` via `ratelimit.RedisLimiter`; errors fail open; (3) `private_album_views` dropped (migration `000041`) — access is covered by grants, identity by the watermark, eliminating unbounded row growth; (4) `← Back` button added for viewer role on album detail page. 5 new backend tests.
- [x] **Security hardening — phase 2** ([PR #111](https://github.com/mayloo89/circl/pull/111)): eleven backend security fixes across the full stack. (1) **Trusted proxy / real IP middleware** — new `middleware.RealIP(cidrs)` reads `X-Forwarded-For`/`X-Real-IP` only when the connection arrives from a configured CIDR (`TRUSTED_PROXIES` env var); falls back to `r.RemoteAddr` otherwise, preventing rate-limit bypass via spoofed headers; `middleware.ClientIP(r)` is now the canonical IP accessor across all five rate-limited endpoints. (2) **SSE notifications auth** — `GET /notifications/stream` migrated from `?token=JWT` to `?ticket=` using the same single-use 60-second Redis ticket mechanism as the WebSocket; `useNotifications.ts` now calls `POST /ws-ticket` before each SSE connection. (3) **Metrics auth hardened** — `metrics.Handler` returns 403 when `METRICS_TOKEN` is empty instead of serving unauthenticated. (4) **bcrypt work factor** raised from cost 10 to 12 in `Register`, `ChangePassword`, and `ResetPassword`. (5) **Refresh token revocation on password reset** — `POST /auth/reset-password` calls `RevokeAllForUser` on success, closing the gap left when `ChangePassword` already had this protection. `EmailFlowService.ResetPassword` now returns `(userID string, err error)`. (6) **Storage URL prefix validation** — `UpdateAvatar` and `AddPhoto` reject URLs that do not start with the configured storage public URL, preventing external-image embedding; controlled via `Service.SetStoragePublicURL` wired in `main.go`. (7) **Atomic rate-limit counters** — Redis `INCR`+`EXPIRE` replaced by a Lua script that sets the TTL only on the first increment, eliminating the race window where a key could survive indefinitely. (8) **Request body size cap** — `LimitRequestBody` (1 MiB) wraps all JSON API routes in a chi group; upload paths apply their own per-category caps and are excluded. (9) **`Permissions-Policy` header** added to the security headers middleware. (10) **`?limit=` capped at 200** in `GET /chat/rooms/{id}/messages`. (11) **IP rate limiting on forgot-password and resend-verification** endpoints.
- [ ] **Private albums — mutual unlock (follow-up PR)**: Feeld-style independent opt-in where both parties must share an album with each other before either can view the other's. Design and UX semantics deferred.
- [ ] **Secret rotation verification** — confirm `JWT_SECRET` and VAPID keys have been rotated since the original commit removal; document rotation steps in a runbook (deferred to Phase 4).

### Privacy controls UI

- [x] **Block / unblock surfaced in the public profile overflow menu** ([PR #90](https://github.com/mayloo89/circl/pull/90)) — verified across all peer-profile states (not contacts, sent, incoming, accepted, blocked); the overflow menu and sticky/desktop action bar render the correct CTA in every state, no code change required.
- [x] **Manage blocked users list** in `/settings` ([PR #90](https://github.com/mayloo89/circl/pull/90)) — new `BlockedUsersSection` in the Settings page renders avatar + display name + Unblock per row with an empty state ("You haven't blocked anyone."); owns its own `GET /contacts/blocked` fetch and calls `DELETE /contacts/{id}/block` behind a confirm dialog. The blocked list was removed from `/contacts` so Settings is the canonical home for account-level privacy management.
- [x] **"Don't show my distance to non-contacts" toggle** in `/settings` ([PR #91](https://github.com/mayloo89/circl/pull/91)) — `Service.Browse` now batch-loads each browsed profile's `hide_distance_from_non_contacts` flag plus the caller's accepted contacts and nulls `distance_km` for non-contact viewers when the flag is set. Migration `000027` introduced the column. Public profile (`GET /profiles/{ref}`) already strips coordinates so no extra gate was needed.
- [x] **Hide presence / last-seen toggle** in `/settings` ([PR #91](https://github.com/mayloo89/circl/pull/91)) — symmetric. `presence.NewHandler` takes a `PrivacyLookup`; `getPresenceHandler` forces `online=false` + drops `last_seen_at` when the viewed user hides, and for every entry when the caller hides. `heartbeatHandler` / `offlineHandler` fan out SSE via `fanoutPresence` which drops the event entirely if the transitioning user hides and skips per-recipient when a contact hides.
- [x] **Read-receipts and typing-indicator opt-out** ([PR #91](https://github.com/mayloo89/circl/pull/91)) — symmetric. `chat.HandlerConfig` gained a `PrivacyResolver`; `hub.deliver` parses each frame's `event` and skips `typing` / `read_receipt` for clients with the matching flag. `readPump` gates the `typing` emit on `c.hideTyping`; `NotifyRoomRead` gates the `read_receipt` publish on the marking user's `HideReadReceipts`. Mid-session toggles take full effect on the next WS reconnect (Client struct holds the snapshot taken at connect time).
- [x] **Per-category notification toggles** in `/settings` ([PR #92](https://github.com/mayloo89/circl/pull/92)) — migration `000028` added `notify_chat_messages`, `notify_contact_requests`, `notify_channel_mentions`, `notify_system` (default `TRUE`). `cmd/api/main.go`'s `notifyUser` now takes a category gate closure that runs against `profileSvc.GetNotificationFlags`; the chat `new_message` push is gated on `ChatMessages` and the contact events on `ContactRequests`. SSE remains unsuppressed so in-app badges still work. The `channel_mentions` and `system` columns persist intent today and will gate automatically when those push surfaces ship. UI lives under Notifications as a sub-section ("Send me notifications for") with four toggles using the existing `Toggle` primitive and the same fetch-then-PUT pattern as `PrivacySection`.
- [x] **Latent merge bug — `PUT /profiles/me/preferences`** ([PR #93](https://github.com/mayloo89/circl/pull/93)) — fixed. New `PreferencesUpdate` value type carries each field as `nil-pointer-or-Optional[int]`; the store builds dynamic SQL so only the columns the caller actually set appear in the write. `Optional[int]` distinguishes the three states the browse "Clear filters" affordance depends on (omit / explicit null / value). `PrivacySection` and `NotificationsSection` simplified to PUT only `{[key]: value}` now that the backend honors partial bodies; `LanguageSection` and `Sidebar` locale-only PUTs no longer clobber filters or toggles.

### Discovery & retention

- [ ] **Browse ranking** — score = shared interests + recency of activity + distance, with a deterministic shuffle per session.
- [ ] **Pause-discovery toggle** — single boolean on `profile_preferences` ("don't show me to others").
- [ ] **Primary-photo selector** in the profile photo gallery.
- [ ] **Profile-completeness gating** — blur browse cards under 40% completeness with a CTA on the user's own card.
- [x] **"Hide profiles without a profile photo" filter in browse** ([PR #99](https://github.com/mayloo89/circl/pull/99)) — migration `000029` adds `require_photo BOOLEAN NOT NULL DEFAULT FALSE` to `profile_preferences`. The browse SQL gates on `prefs.require_photo IS NOT TRUE OR (p.avatar_url IS NOT NULL AND p.avatar_url <> '')`, so empty-string avatars are treated as missing too. UI is a checkbox under the existing distance / gender / interests filters in the browse `FilterPanel`, persisted on toggle via the same partial-update PUT to `/profiles/me/preferences` that the existing filters use; the active-filter count and "Clear filters" handler now include `require_photo`.
- [x] **"Looking for" public-profile fields** ([PR #100](https://github.com/mayloo89/circl/pull/100)) — migration `000030` adds three columns to `profiles` (`looking_for_gender TEXT[]`, `looking_for_age_min INT`, `looking_for_age_max INT`). Gender set matches the self-id list (Male, Female, Trans male, Trans female, Non-binary, Prefer not to say) so users can express interest in any identity the platform supports; labels reuse the existing `profile.gender*` translation keys. Backend rejects unknown genders, ages outside 18–120, and inverted min > max. Both owner and non-owner profile responses include the fields. Profile edit gains a "Looking for" card with two rows (gender chips, min/max age inputs); public profile renders a chip block when at least one field has a value. Also fixed the pre-existing i18n bug on the profile edit page — labels (username / display name / bio / DOB / gender / location / interests) and gender dropdown options now use `t(...)` instead of hardcoded English. EN/ES/PT i18n.

### Communication features

- [ ] **Message reactions, reply-to threading, in-room message search**.
- [ ] **Link previews in chat** — server-fetched OG metadata, cached.
- [ ] **Lightbox swipe-to-close + pinch-to-zoom** (`components/chat/Lightbox.tsx`).
- [x] **Channel-leave double-confirmation** ([PR #96](https://github.com/mayloo89/circl/pull/96)) — in-app `ConfirmDialog` followed by browser `beforeunload` dialog when navigating away from a channel via the in-app interceptor. Suppressed by a `bypassBeforeUnloadRef` set inside `confirmLeave` so the listener short-circuits once the user has already confirmed.

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
- [x] **Distance "Cualquiera/Any/Qualquer" label → `∞ km`** ([PR #96](https://github.com/mayloo89/circl/pull/96)) — at the slider's max value, the localized "Any" label was visually long in ES/PT and read as a word rather than a quantity. Replaced by the `∞ km` glyph across all three locales (the symbol carries enough meaning that no per-locale word is needed).
- [x] **Camera capture on profile avatar + gallery file inputs** ([PR #96](https://github.com/mayloo89/circl/pull/96)) — onboarding photo and chat composer already had `capture` attributes; the profile edit page's avatar and showcase-photo `<input type="file">`s were missing them. Added `capture="user"` (front camera) on both since these are typically self-photos.
- [x] **Contacts page pending-requests section** ([PR #96](https://github.com/mayloo89/circl/pull/96)) — verified that `pending.length > 0` already gates the section so it disappears at zero. No code change; recorded here so the audit isn't lost.
- [x] **`RangeSlider` value column wrap** ([PR #96](https://github.com/mayloo89/circl/pull/96)) — fixed `w-12` (48px) was narrower than `"XXX km"` at `text-sm` with `tabular-nums` (~50px), so distance values intermittently wrapped (`362` / `km`). Bumped to `w-14` and added `whitespace-nowrap` so any future overflow surfaces visibly in QA instead of silently wrapping.

### i18n cleanup

- [ ] **Hardcoded `"en"` in `chatHelpers.ts`** `toLocaleDateString` calls — replace with `useLocale()`.
- [ ] **Hardcoded English distance strings** in browse ("km away", "< 1 km away") — move to `messages/*.json`.
- [ ] **BottomNav label wrap test** — verify ES/PT labels don't wrap at 360px viewport width.
- [x] **Locale picker on unauthenticated pages** ([PR #97](https://github.com/mayloo89/circl/pull/97)) — new `AuthLocalePicker` (`components/AuthLocalePicker.tsx`) renders fixed top-right ES / EN / PT pills on every unauthenticated route. Mounted in `Providers.AppShell` only when `status === "unauthenticated"` so it doesn't appear during the loading flicker or on authenticated pages. Switching pills uses next-intl's `router.replace(pathname, { locale })`, which updates the URL prefix and the `NEXT_LOCALE` cookie so the choice persists into the session and across reconnects. We deliberately don't persist to `profile_preferences.locale` at register time because the registration endpoint is unauthenticated and creating a parallel "set locale before signin" path would be over-engineered for one field; the cookie + URL prefix already carry the choice forward, and the user can refine via Settings → Language after login.
- [x] **Locale-aware `DateOfBirthPicker` field order** ([PR #97](https://github.com/mayloo89/circl/pull/97)) — picker reads `useLocale()` and renders MM/DD/YYYY for `en` (US/UK convention) and DD/MM/YYYY for `es`/`pt`. Year stays last in both orderings. Three vitest cases cover the locale → ordering mapping by introspecting `select` aria-labels.

### Performance

- [ ] **Server/client split** — page-level `"use client"` everywhere; split each route into a server shell + client island where viable.
- [ ] **Lazy-load heavy components** — `PhotoGallery`, `Lightbox`, `CreateGroupModal`, `NewChatModal` via `next/dynamic`.
- [ ] **Dynamic Type compatibility** — hardcoded `text-[10px]`, `text-[11px]`, `text-[12px]` in chat break iOS Dynamic Type; replace with Tailwind tokens.
- [ ] **Dedupe `/chat/rooms` fetch** — `ChatListPane` and `RoomView` both fetch room metadata; share via context provider so the right pane stops re-fetching when the layout already has it.

### Polish

- [x] **Admin user list shows real presence** ([PR #98](https://github.com/mayloo89/circl/pull/98)) — added an "Activity" column to `/admin/users` separate from the moderation `status` column. Backend handler enriches each `UserRecord` with `online` + `last_seen_at` via a new `PresenceLookupFunc` injected into `admin.NewHandler`; main.go provides the adapter on top of `presence.Store.GetPresence` so admin stays decoupled from the presence package. UI shows a green dot + "Online" when present, a gray dot + relative "X ago" using the existing `formatLastSeen` helper otherwise (with the absolute timestamp on hover via `title`), and "Never" for users who have never connected. Three handler tests cover the overlay, the Redis-down propagation as a 500, and the legacy nil-lookup fallback.
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
| `MODERATION_API_URL` | unset (NoopClassifier) | NudeNet sidecar URL, e.g. `http://moderation:8000` |
| `NSFW_THRESHOLD` | `0.80` | Score above which the NSFW moderator rejects |
| `MODERATION_REJECTED_RETENTION_DAYS` | `30` | Days to keep rejected NSFW / heuristic upload files for admin review before purge. Hash-list matches are purged immediately regardless. |
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
