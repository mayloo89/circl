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
- ✅ **OpenAPI spec — full audit** ([PR #112](https://github.com/mayloo89/circl/pull/112)): 17 corrections across spec v3.1.0. `POST /ws-ticket` added; SSE + WS endpoints updated to `?ticket=`; `Profile` gains location/age/looking-for fields; preferences schema expanded to all 15 fields; `AlbumGrant` gains `expires_at`; invite grants documented as immediately active; stale view-log references removed; `album_share` message type added; `category` field on upload request.
- ✅ **UX overhaul — visual rebrand** ([PR #67](https://github.com/mayloo89/circl/pull/67)): Nunito (headings) + DM Sans (body) via `next/font/google`; Tailwind v4 brand token system (`--color-brand-*`, `--radius-card`, `--shadow-card`); `accent` Button variant (CTA orange #F97316); conversion CTAs in browse and profile migrated to `accent`
- ✅ **Light/dark mode + brand palette refresh** ([PR #113](https://github.com/mayloo89/circl/pull/113)): trans-flag blue + rose palette; blue-tinted light-surface gray scale; `ThemeContext` + `ThemeToggle` with localStorage + `prefers-color-scheme`; full light-mode contrast audit; branding assets regenerated
- ✅ **Home page redesign + UI polish** ([PR #116](https://github.com/mayloo89/circl/pull/116)): full-width hero with logo mark, ambient blobs, personalised greeting; four contextual home widgets (completeness banner, pending requests, nearby profiles, recent conversations); light-mode contrast fixes for avatar fallback circles and card ring borders; collapsed sidebar shows logo mark with dedicated expand button in nav list.
- ✅ **Operational runbooks** (`docs/runbooks/`): `deploy.md` (first-deploy checklist, release procedure, rollback), `db-backup-restore.md` (daily backups, restore, drill log, migration state), `incident-response.md` (severity levels, log guide, Prometheus alerts, common scenarios, post-incident steps), `secret-rotation.md` (JWT, VAPID, DB, S3 credentials).
- ✅ **UI audit — home, nav, and contacts** ([PR #117](https://github.com/mayloo89/circl/pull/117)): hero replaced with theme-aware logo (no decorative blobs); home widgets gain error states with retry; inline decline confirmation in `PendingRequestsWidget`; "Get started" section in completeness banner for new users; sidebar separator + dynamic push toggle label; `TopBar` route titles expanded and touch targets standardised; contacts page aligned to home card design tokens + decline `ConfirmDialog` added; `Button` `danger` variant softened to semi-transparent red; `ContactCard` accept and message buttons changed to `accent` (rose) for visual consistency.
- ✅ **React 19 hydration hardening** ([PR #115](https://github.com/mayloo89/circl/pull/115)): eliminated three classes of server/client mismatch. `ThemeContext` and `SidebarContext` rewritten with `useSyncExternalStore` (removes the `react-hooks/set-state-in-effect` lint error and the localStorage server/client divergence). Theme cookie read server-side in root layout so `<html class="dark">` is rendered correctly on first paint — no FOUC, no React 19 script-tag warning. `generateMetadata` converted to an async function so the server uses the Suspense path matching the client router, fixing the persistent `MetadataWrapper hidden` hydration error. Session pre-fetched in `LocaleLayout` and passed to `SessionProvider`. `middleware.ts` renamed to `proxy.ts` per Next.js 16 convention. Upgraded to Next.js 16.2.6, React 19.2.6, Tailwind 4.3.0.
- ✅ **UX overhaul — critical bugfixes** ([PR #66](https://github.com/mayloo89/circl/pull/66)): browse subtitle bug, unblock dialog wrong message, back buttons destroying browser history, login `blue-*`→`indigo-*` color alignment, register success emoji→SVG, WCAG contrast fix on block/report buttons, chat skeleton loading state, active locale highlighted in language switcher, contacts sorted online-first
- ✅ **UX overhaul — navigation** ([PR #68](https://github.com/mayloo89/circl/pull/68)): `BottomNav` (mobile, 5 slots, SVG icons + unread/pending badges, iOS safe-area padding) + `Sidebar` (desktop ≥1024px, same 5 items + language switcher + settings + user row) + `TopBar` (minimal mobile header with logo, active-route label, push toggle, avatar shortcut); `ProfileContext` single `/profiles/me` fetch per session; old monolithic `NavBar.tsx` removed
- ✅ **UX overhaul — home dashboard** ([PR #69](https://github.com/mayloo89/circl/pull/69)): `PendingRequestsWidget` (inline Accept/Decline, reacts to SSE contact events), `NearbyProfilesWidget` (horizontal scroll, skeleton), `RecentConversationsWidget` (avatar, last-message preview, unread badge), `ProfileCompletenessBanner` (progress bar, sessionStorage dismiss); `lib/profileCompleteness.ts` pure utility
- ✅ **UX overhaul — browse** ([PR #70](https://github.com/mayloo89/circl/pull/70)): `RangeSlider` (single-thumb filled-track) + `BottomSheet` (slide-up mobile sheet, Escape + scroll-lock) primitives; filter panel redesign with active-count badge; age and distance sliders (500 km = "Any"); "Clear filters" on empty state; backend fix to exclude profiles with unknown coordinates when distance filter is active
- ✅ **UX overhaul — public profile hero** ([PR #71](https://github.com/mayloo89/circl/pull/71)): 55vh full-bleed hero photo with gradient overlay; initials fallback; back button + ⋯ overflow menu (Block/Unblock/Report) over hero via backdrop-blur; sticky mobile action bar above bottom nav (Message/Add contact/Request sent/Accept); "Preview as visitor" button on own profile
- ✅ **UX overhaul — onboarding wizard** ([PR #72](https://github.com/mayloo89/circl/pull/72)): 4-step wizard at `/onboarding/{photo,bio,interests,location}`; step-dot progress bar; "Skip" on every step; pre-filled from existing profile data; `AppShell` redirects unonboarded users to wizard and suppresses nav; `ProfileCompletenessCard` on own profile (progress bar + per-field links to wizard); backend migration `000026` adds `onboarded_at TIMESTAMPTZ`
- ✅ **UX overhaul — chat polish + auth UX + onboarding smart steps** ([PR #74](https://github.com/mayloo89/circl/pull/74)): `PasswordField` with show/hide toggle (registration, login, settings); `DateOfBirthPicker` with three equal-width DD/MM/YYYY selects; scroll-to-bottom FAB in chat room; chat list search + 10s silent polling; onboarding smart steps skip already-complete fields and redirect to first incomplete step on entry
- ✅ **Modern Go + UX/UI audit** ([PR #75](https://github.com/mayloo89/circl/pull/75)): 19 Go modernizations (`errors.Is`, `omitzero`, `max()`, `for range n`, `strings.Cut`); frontend a11y + usability pass — `cursor-pointer` global, focus rings on all raw buttons, `prefers-reduced-motion` + `scroll-padding-top` + `scrollbar-hide` in globals.css, 5 emoji→SVG icon replacements, descriptive `alt` text on 6 images, layout-shift fixes (`scale-*` → `opacity-*`), BottomNav icon size normalization
- ✅ **Privacy hardening** ([PR #82](https://github.com/mayloo89/circl/pull/82)): public profile strips DOB + exact coordinates for non-owners (returns computed age instead); presence gated to accepted contacts; soft-deleted users excluded from presence; contact requests rate-limited 100/day
- ✅ **WebSocket ticket auth + CSP nonce** ([PR #83](https://github.com/mayloo89/circl/pull/83), [PR #84](https://github.com/mayloo89/circl/pull/84)): single-use Redis tickets (60 s TTL) replace JWT in WS query strings; per-request CSP nonce removes `'unsafe-inline'` from `script-src`
- ✅ **Mobile UX + navigation audit** ([PR #86](https://github.com/mayloo89/circl/pull/86)): iOS safe-area insets on `TopBar`/`BottomNav`/`AppShell`/`Toast`/profile sticky bar via `@utility` directives; `text-base` on inputs prevents iOS auto-zoom; auto-growing `<textarea>` chat composer; `capture` attributes on file inputs; push prompt deferred until first conversation; 44 pt touch targets audited across all interactive elements; BottomNav restructured (5 items: Home, Browse, Messages, Channels, Contacts); back buttons removed from top-level pages; profile preview mode on `/profile/[username]`; layout width consistency across all pages
- ✅ **Desktop UX + accessibility — phase 1** (PR #87): chat routes restructured around a `(messages)` route group with a layout-owned, persistent `ChatListPane` (no flicker on thread switch); channel rooms moved to dedicated `/chat/channels/[channelId]` outside the group so they never inherit the DM list — channels stay single-pane to keep leaving a conscious, confirmed action; the room body extracted into `RoomView` shared by both surfaces; skip-to-content link + `<main id="main-content">` landmark in `AppShell`; channel-leave copy hardened across EN/ES/PT to spell out irreversible message-access loss
- ✅ **Accessibility deep pass** (PR #88): centralised dialog focus management in `hooks/useFocusTrap.ts` (capture trigger, auto-focus first child, trap Tab, optional Escape, scroll-lock, restore focus on close) — `Modal`, `BottomSheet`, and `GroupMembersPanel` all delegate; `hooks/useMenuKeyboard.ts` implements WAI-ARIA menu keyboard semantics (ArrowUp/Down wrap, Home/End, Escape, focus restoration) wired into public-profile overflow menu and chat ephemeral-message menu; all 77 SVGs in the codebase now carry `aria-hidden`/`aria-label`/`role`; hardcoded `aria-label`s localized; `RangeSlider` gains `aria-label` + `aria-valuetext`
- ✅ **`PUT /profiles/me/preferences` partial-update fix** ([PR #93](https://github.com/mayloo89/circl/pull/93)): the endpoint MERGES instead of REPLACES — locale-only PUTs no longer clobber browse filters / privacy / notification toggles. `PreferencesUpdate` value type with a generic `Optional[T]` distinguishes omit / explicit null / value for the three nullable filter ints so the browse "Clear filters" affordance still works. Store builds dynamic SQL (only the columns the caller set appear in the write); empty PUT degenerates to `INSERT ... ON CONFLICT DO NOTHING`. `PrivacySection` and `NotificationsSection` simplified to PUT only `{[key]: value}`
- ✅ **Per-category notification toggles** ([PR #92](https://github.com/mayloo89/circl/pull/92)): migration `000028` adds four `notify_*` boolean columns to `profile_preferences` (default true); `cmd/api/main.go`'s `notifyUser` helper takes a category-gate closure run against `profileSvc.GetNotificationFlags` so the chat `new_message` push is gated on `ChatMessages` and the three contact events on `ContactRequests`; SSE is unsuppressed so in-app badges keep working; `channel_mentions` and `system` columns persist user intent today and will gate automatically when those push surfaces ship; new sub-section in Settings → Notifications with four toggles backed by the `Toggle` primitive and the fetch-then-PUT pattern; full EN/ES/PT i18n
- ✅ **Privacy toggles — distance / presence / read receipts / typing** ([PR #91](https://github.com/mayloo89/circl/pull/91)): migration `000027` adds four boolean columns to `profile_preferences`; backend gates the toggles on browse (distance suppressed for non-contacts when the viewed user hides), presence (symmetric — caller hides → all offline; target hides → that entry offline; SSE fanout dropped for transitioning user and skipped per-recipient), and the chat hub (symmetric `typing` and `read_receipt` frame suppression at both emit and receive). New `Toggle` UI primitive (native checkbox + `role="switch"`, 44 pt touch target, focus ring); new `PrivacySection` in `/settings` with a "Symmetric" badge on the three reciprocal toggles; full localization in EN/ES/PT
- ✅ **Privacy controls UI — manage blocked users in Settings** ([PR #90](https://github.com/mayloo89/circl/pull/90)): a dedicated "Blocked users" section in `/settings` lists everyone the user has blocked (avatar + display name + Unblock per row), with a localized empty state; the blocked-users list is removed from `/contacts` so Settings is the canonical home for account-level privacy management; the unblock confirm dialog and `DELETE /contacts/{id}/block` flow are unchanged; the public-profile overflow-menu unblock path was state-walked across all peer-profile states and confirmed correct in every state
- ✅ **Pi deploy fixes — local storage URL + CSP + image optimization** ([PR #78](https://github.com/mayloo89/circl/pull/78), [PR #79](https://github.com/mayloo89/circl/pull/79), [PR #80](https://github.com/mayloo89/circl/pull/80)): `LOCAL_STORAGE_BASE_URL` env var so file URLs resolve correctly behind nginx; CSP `script-src` and `style-src` updated to allow Next.js App Router hydration and Google Fonts; `NEXT_PUBLIC_IMAGE_UNOPTIMIZED` build arg disables `/_next/image` optimizer (appropriate for low-power Pi deployments)
- ✅ **Terms / Privacy / Community / Safety + registration consent** ([PR #101](https://github.com/mayloo89/circl/pull/101)): four MDX-driven legal pages (`/terms`, `/privacy`, `/guidelines`, `/safety`) in ES (canonical) / EN / PT with AR-specific framing — Ley 25.326, AAIP, Ley 27.736 ("Ley Olimpia"), línea 144, Buenos Aires venue; consent checkbox on registration backed by migration `000031` (`terms_accepted_at`, `privacy_accepted_at`, `accepted_policy_version`); new `Checkbox` UI primitive and `Footer` component mounted on every auth page and `/settings`; `Authenticator.Register` now takes a `RegistrationInput` struct so consent timestamps flow to the store; new `apierror.CodeTermsNotAccepted` returned on a missing or false `accept_terms` flag
- ✅ **Generic production-deploy templates** — `docs/deploy/` ships an infrastructure-agnostic starting point (compose, env example, nginx vhost, walkthrough). Captures the two non-obvious Next.js v16 fixes (`HOSTNAME=0.0.0.0` + `pgrep` healthcheck override) inline so customisers don't strip them. The obsolete root `/docker-compose.prod.yml` (MinIO, old env-var names) is removed. Each operator's actual `deploy/` folder (their domain, secrets, certificate paths, runbook quirks) stays gitignored. See **Deploying to production** below.
- ✅ **Private albums with consented per-user sharing** — owners group `album-private` uploads into named albums; access is per-album, not blanket. Three entry points feed the grant lifecycle: owner invites contact (push), viewer requests access (pull), owner shares in a DM (auto-grant via the new `MessageTypeAlbumShare` chat message). Migration `000036` adds 4 tables; the `roleFor()` predicate at the service layer is the single read gate. Photos still flow through the moderation pipeline; photo bytes stream through `GET /albums/{id}/photos/{upload_id}/file` with `Cache-Control: private, max-age=60`. Frontend: new `/albums` route + sidebar entry, `MembersPanel` with revoke confirmation that warns the viewer's browser cache can't be invalidated retroactively, share-album button in DM chat input. Watermark + NSFW-tag-mode for private uploads land in a follow-up.
- ✅ **Browse gender filter + preference seeding**: fixed `GENDER_OPTIONS` in the browse filter panel — values now match the canonical gender values stored in `profiles.gender` (`"Male"`, `"Female"`, `"Trans male"`, `"Trans female"`, `"Non-binary"`) so the `= ANY(prefs.gender_preference)` SQL filter actually works. On first visit to browse when no preferences have been saved, the user's profile "Looking for" fields (`looking_for_gender`, `looking_for_age_min`, `looking_for_age_max`) are automatically seeded as default browse preferences and persisted so results are filtered server-side from the first page load. Preferences now load before the initial browse fetch to avoid flashing a "0 active filters" badge.
- ✅ **Admin panel — mobile-responsive layout and UX**: replaced the fixed sidebar (unusable on mobile, consumed ~60% of viewport) with a slide-over drawer on small screens, triggered by a hamburger button in a sticky top bar. All six admin pages get responsive padding. Table columns hide progressively by breakpoint so identity, status, and action columns stay visible at every size. The users table collapses per-row Suspend/Ban/Reactivate/Role/Delete buttons into a single "Actions ▾" dropdown with keyboard-navigable menu (`ArrowUp`/`ArrowDown`, `Escape`, focus-restore) and full ARIA wiring.
- ✅ **Image-rejection UX + admin review surface**: closes the visible gap from the moderation pipeline. End-users see a dedicated `UploadRejectionModal` (focus-trapped, EN/ES/PT bodies routed off `moderation_code`) instead of a buried inline error; `useUpload` exposes rejections as a structured `rejection` field separate from generic `error`. Backend persists `moderation_score` + `moderation_categories` (migration `000035`) plus a per-code retention policy: NSFW + heuristic rejection files are kept for `MODERATION_REJECTED_RETENTION_DAYS` (default 30) so admins can verify false positives; hash-list matches purge immediately (CSAM / NCII posture). New admin surface `/admin/moderation` with thumbnail, score, categories, code filter and authenticated full-size lightbox via `GET /admin/moderation/{id}/image`. Daily cleanup `worker.PurgeExpiredModerationFiles`.
- ✅ **NudeNet sidecar — real NSFW detection**: `ops/moderation/` is a FastAPI + NudeNet container that exposes `POST /classify`. When the backend's `MODERATION_API_URL` env var is set (default in docker-compose: `http://localhost:8081`), the existing `NSFW` moderator in `internal/moderation` swaps the bundled `NoopClassifier` for `HTTPNSFWClassifier` — same interface, no pipeline changes. `NSFW_THRESHOLD` tunes the reject cutoff (default `0.80`). The sidecar aggregates NudeNet's per-region detections to a single `nsfw_score` so the Go side stays vendor-agnostic.
- ✅ **Image moderation pipeline**: per-upload framework (`internal/moderation`) with a `Moderator` interface + `Chain` short-circuit, three bundled detectors (`HashList` against the new `image_block_hashes` table, `Heuristic` for size/dimension/aspect bounds, `NSFW` with a pluggable classifier — `NoopClassifier` by default until NudeNet/Rekognition lands). Worker runs the chain after image decode; rejections delete the object + mark the row + short-circuit; moderator errors fail open. New `GET /admin/moderation` lists rejected uploads, `POST /admin/moderation/hashes` curates the local block list. Frontend `useUpload` polls `GET /uploads/{id}` after confirm and surfaces the rejection reason. `apierror.CodeUploadRejectedModeration`. Designed so StopNCII (hash feed) and a real NSFW classifier plug in behind the same interfaces with zero pipeline changes.
- ✅ **Habeas Data / GDPR Art. 20 data export**: users can request a zip of their data — machine-readable JSON (`data.json`) of profile, preferences, contacts, blocks, rooms, sent messages, filed reports, age attestations, and uploads, plus the bytes of every media file they own — built async via asynq, delivered via a single-use 14-day download link emailed to them. New `POST/GET /users/me/exports` + un-authenticated `GET /account/export/{token}` (the path token is the bearer credential). Migration `000033` adds the `export_requests` table with a unique partial index that lets at most one in-flight build per user; a 24h cool-down is enforced atomically inside `Create`. New `internal/exports` package + `export:user` asynq task type + `DataExportSection` in `/settings`. EN/ES/PT.
- ✅ **Expanded reports + priority queue + age-verification audit + appeals**: migration `000032` adds three new report reasons (`non_consensual_intimate_images`, `digital_gender_violence` / "Ley Olimpia", `csam`), a `priority` column on `reports` with index, the `age_verification_audit` table (preserved across hard-delete via ON DELETE SET NULL), and the `appeals` table (token hashed, 30-day TTL, one open appeal per suspension). Priority is derived from the reason server-side so callers can't downgrade CSAM or NCII reports — admin list is ordered critical → high → normal → newest-first. Register handler captures IP + User-Agent + DOB to the audit table after `Register` succeeds. New `internal/appeals` package + public `/appeal/{token}` (un-authenticated) + admin `/admin/appeals` (mounted via `admin.WithAppealsHandler`). Suspend / Ban fires the new `admin.SuspensionNotifier` async hook which mints a token, persists the appeal row, and emails the appeal link; approving an appeal reactivates the user; denying leaves the suspension in place; either way a resolution email is sent. New `Appeals` admin sidebar entry, new `apierror.CodeAppealAlreadyResolved`. Full EN/ES/PT.
- ✅ **Security hardening — phase 2** ([PR #111](https://github.com/mayloo89/circl/pull/111)): eleven backend security fixes. Trusted proxy middleware (`middleware.RealIP`) prevents rate-limit bypass via spoofed `X-Forwarded-For` — only trusted when the connection arrives from a configured CIDR (`TRUSTED_PROXIES` env var). SSE notifications auth migrated from `?token=JWT` to single-use tickets (matching the WebSocket pattern). Metrics endpoint requires `METRICS_TOKEN` (returns 403 when empty). bcrypt cost raised from 10 to 12. Refresh tokens revoked after password reset. Avatar and gallery URLs validated against the configured storage prefix to prevent external-image embedding. Rate-limit counters made atomic via a Lua script. Request body capped at 1 MiB across all JSON API routes. `Permissions-Policy` header added. Chat `?limit=` capped at 200. Forgot-password and resend-verification endpoints rate-limited per IP.
- ✅ **DM external-contact masking**: server-side detection and redaction of contact info (phones, emails, URLs, handles) in DMs between non-accepted contacts. New `internal/redact` package with two-tier normalizer, regex detection, and keyword-boosted thresholds. `Service.maybeRedact()` wired into chat send path; raw data never touches DB. `AreAcceptedContacts` on contacts store; `GetDMPeerID` on chat store; `IsExemptSender` placeholder for future service-profile logic. Migration `000042` adds `redacted` column to `messages`. Frontend: system message pill, redaction token with i18n, amber contact-sharing warning in non-accepted DMs. EN/ES/PT.
- ✅ **Message data retention**: daily background sweep hard-deletes messages past the retention window for their room type (DM: 3 months, public: 24h). Retention-driven expiry removes the row entirely (no placeholder), unlike user-set self-destruct / view-once which tombstone. `RetentionCleaner` (`worker/retention.go`) runs on a 24h ticker, queries `ListRetentionEligibleMessages` (joins messages + rooms by type, excludes tombstoned and `expires_at`-set rows), hard-deletes via `DeleteMessage`, cleans object-storage files, and broadcasts `message_deleted`. `chat.RetentionDurations` map defines per-type windows.
- ✅ **Public rooms with guest-access tier (Phase 1 / MVP)**: admin-created open chat rooms where unregistered guests can enter with a nickname + age-of-majority declaration. Guest sessions are ephemeral Redis-backed (4h TTL); guests are text-only, restricted to public rooms, rate-limited at 6 msgs/min (`GUEST_MSG_RATE`). Contact masking applies unconditionally to guest messages. 24h hard-delete retention. Migration `000044` (adds `public` room type + `visibility`; makes `messages.sender_id` nullable + adds `sender_label` for guest senders, who have no `users` row). Backend: `internal/guest` package (`POST /guest/session`, `GET /guest/rooms`); guest WS ticket flow (`POST /guest/ws-ticket`); admin CRUD (`POST/GET/DELETE /admin/public-rooms`); `SaveMessage` guest path (NULL sender + `sender_label`); `NicknameTaken` protected namespace. Frontend: `/rooms` listing, `/rooms/[roomId]` guest entry gate, `useGuestChat` hook, `GuestRoomView`; navigation links in Sidebar + BottomNav. EN/ES/PT i18n.
- ✅ **In-room moderation for public rooms**: admin-only kick/mute for any participant (guest or registered). Kick is real-time (hub `kickCmds` channel → `kicked` WS event → close connection); guests also receive a 24h IP ban (SHA-256 hash in Redis). Mute is Redis TTL-based (15 min / 1 h / 24 h), checked before each message save — sender gets `you_are_muted` event, message dropped. Nickname profanity filter at guest entry (`internal/profanity`, ~50 worst-case slurs EN/ES/PT with leet-speak normalization); IP-ban check at `POST /guest/session` when `room_id` supplied. Frontend: `isKicked` / `isMuted` state in `useGuestChat` / `useChat`; full-screen removal modal + amber muted banner in `PublicRoomShell`; admin `•••` overflow menu per roster participant with keyboard navigation. EN/ES/PT i18n. No migration needed (Redis-only state).
- ✅ **Frontend polish pass — admin i18n + legal page fixes**: full i18n pass across all 7 admin pages — ~120 new keys added to the `admin` locale namespace (EN/ES/PT); hardcoded label dictionaries removed; `toLocaleDateString()`/`toLocaleString()` calls gain explicit locale from `useLocale()`; modal backdrops changed to `bg-gray-950/80`. Legal pages: `LegalPage` gains a localized back-link; `draft` banner gated on `LEGAL_DRAFT=true` env var; blockquote side-stripe replaced with full border + tint; `<ol>` styles added; `Footer` links gain `focus-visible` ring.

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
env-var reference, and the troubleshooting section (frontend `unhealthy`
flag root cause, kernel cgroup-memory advisory, slow first build, migration
failures).

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
npm run dev            # Development server
npm run build          # Production build
npm run lint           # ESLint
npm run test           # Vitest unit tests
npm run test:coverage  # Unit tests with the coverage gate CI enforces
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
│   │   ├── profiles/ # Profile handler, service, store; preferences (locale)
│   │   ├── push/ # Web Push (VAPID) handler, service, store
│   │   ├── ratelimit/ # Redis-backed rate limiter (per-IP and per-user)
│   │   ├── redact/ # Contact-info detection + redaction (normalize, detect, redact)
│   │   ├── reports/ # User report handler, service, store
│   │   ├── server/        # Chi router, CORS, /health, /metrics endpoints
│   │   ├── storage/       # Storage interface, LocalStorage, S3Storage
│   │   ├── testutil/      # Integration test helpers (OpenDB, CreateUser, NewRedis)
│   │   ├── token/         # JWT generate/validate
│   │   ├── tracing/       # OTel SDK init, chi middleware, pgx tracer
│   │   ├── uploads/       # Upload lifecycle (request → confirm), Postgres tracking
│ │ └── worker/ # asynq tasks: image processing, ephemeral + retention cleanup, purge
│ ├── migrations/ # SQL migrations (up + down), currently at 000042
│   └── go.mod
├── ops/                   # Local observability stack (dev only)
│   ├── alloy/             # Grafana Alloy config — scrapes container stdout → Loki
│   ├── grafana/           # Provisioning YAML (datasources + dashboards-as-code)
│   ├── loki/              # Loki single-binary config, 7-day retention
│   ├── prometheus/        # prometheus.yml + alert rules
│   └── tempo/             # Tempo config (OTLP receivers, 7-day trace retention)
├── docs/
│   ├── implementation-plan.md
│   └── openapi.yaml       # OpenAPI 3.1.0 spec (~40 endpoints)
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
