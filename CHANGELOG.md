# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Changed

- **Privacy toggles — distance / presence / read receipts / typing**: a new "Privacy" section in `/settings` exposes four toggles whose state is persisted on `profile_preferences` (migration `000027` adds `hide_distance_from_non_contacts`, `hide_presence`, `hide_read_receipts`, `hide_typing_indicator` as `BOOLEAN NOT NULL DEFAULT FALSE`). Each toggle ships with a localized label + description in EN/ES/PT; the three reciprocal ones (presence, read receipts, typing) carry a "Symmetric" badge to make the two-way semantics explicit. The Privacy section fetches the full preferences object on mount and PUTs the full object back on every change so toggling one flag never clobbers browse filters or the locale. Backend gates the toggles on the right surfaces:
  - **Distance** — `Service.Browse` batch-loads each browsed profile's `hide_distance_from_non_contacts` flag plus the caller's accepted-contact set and sets `distance_km = nil` when the viewed user has the flag and is not a contact. Public profile (`GET /profiles/{ref}`) already strips coordinates; no extra gate needed.
  - **Presence** — `presence.NewHandler` now takes a `PrivacyLookup`. `getPresenceHandler` forces `online=false` and strips `last_seen_at` for any entry whose viewed user has `hide_presence`, and for every entry when the caller has `hide_presence` (symmetric on read). `heartbeatHandler` / `offlineHandler` route SSE fanout through `fanoutPresence`, which drops the event entirely if the transitioning user has `hide_presence` and skips per-recipient when a contact has it (symmetric on push).
  - **Read receipts** — `chat.HandlerConfig` gains a `PrivacyResolver`. The WS `Client` records `hideReadReceipts` at connect; `hub.deliver` parses the frame's `event` field and skips `read_receipt` frames for clients with the flag. `cmd/api/main.go`'s `NotifyRoomRead` callback drops the publish entirely when the marking user has the flag (sender-side gate).
  - **Typing** — same shape as read receipts: `Client.hideTyping` is the receive gate inside `hub.deliver`; `readPump` checks `c.hideTyping` before publishing the typing frame (sender-side gate). The 2 s server-side debounce is unchanged.
  - **Mid-session caveat** — read-receipt and typing flags are read at WS connect and live on the `Client` struct, so a toggle takes full effect on next reconnect; the read-receipt sender gate is fresh on every PUT/read because it's resolved per-call in `NotifyRoomRead`.
  Tests cover: profiles `Browse` distance suppression with mixed contact/flag combinations; presence handler symmetric `getPresenceHandler` (caller hides → all offline; target hides → that entry offline) + heartbeat fanout (transitioning user hides → no events; recipient hides → skipped); chat hub `deliver` suppression for `typing` and `read_receipt` while `new_message` continues to be delivered to the same hider.

- **Privacy controls UI — manage blocked users in Settings** ([PR #90](https://github.com/mayloo89/circl/pull/90)): a new "Blocked users" section in `/settings` lists everyone the user has blocked, with avatar + display name and an Unblock button per row, plus an empty state ("You haven't blocked anyone."). The section owns its own fetch of `GET /contacts/blocked` and calls `DELETE /contacts/{id}/block`; an Unblock confirm dialog mirrors the previous flow ("They will be able to message you again."). The blocked list is removed from `/contacts` since Settings is the canonical home for account-level privacy management. Related i18n keys (`settings.blockedUsers` / `blockedUsersLabel` / `blockedUsersDesc` / `blockedEmpty` / `unblock` / `unblockConfirmMessage` / `unblockFailed`) added in EN/ES/PT; orphan `contacts.{blockedUsers,unblock,unblockConfirmMessage}` removed. The unblock affordance in the public profile overflow menu was state-walked across own profile / blocked / not contacts / sent / incoming / accepted — already correct in every state, no code change.

- **Documentation consolidation**: `docs/implementation-plan.md` is now the single living source of truth. The dated audit snapshots — `docs/expert-review-plan.md` (2026-05-02), `docs/ux-audit.md` (2026-04-25), `docs/production-readiness.md` — have been deleted; their genuinely-still-open items folded into the implementation plan as a categorized backlog (Trust & Safety / legal, Privacy controls UI, Discovery & retention, Communication features, Group / channel admin, Web / desktop polish, i18n cleanup, Performance, Polish), and Phase 4 / Phase 5 expanded with the deployment + observability + pre-launch items they previously hosted. The production env-var dev→prod reference table moved into a new §11. README updated to drop the deleted-doc link.

- **Accessibility deep pass** (PR #88):
  - **Centralised dialog focus management**: new `hooks/useFocusTrap.ts` is now the single source of truth for dialog overlays — captures the trigger, auto-focuses the first focusable child, traps Tab/Shift-Tab inside the container, optionally handles Escape, locks body scroll, and restores focus to the trigger on close. `Modal` and `BottomSheet` refactored to delegate (`BottomSheet` previously had no trap and no auto-focus). `GroupMembersPanel` gains `role="dialog"` + `aria-modal` + localized `aria-label` (channel/group settings) and joins the trap. Added a regression test for trigger-restore-on-close.
  - **Menu keyboard semantics**: new `hooks/useMenuKeyboard.ts` implements WAI-ARIA menu keyboard navigation — first menuitem auto-focused on open, ArrowUp/Down wrap, Home/End jump to ends, Escape closes, Tab/Shift-Tab close (focus continues past, not trapped), focus restored to trigger. Wired into the public profile overflow menu (Block / Report / Unblock) and the chat input's ephemeral-message menu, both of which now also expose `role="menu"` + `role="menuitem"` + `aria-haspopup="menu"` + `aria-expanded`.
  - **Decorative SVGs marked `aria-hidden`**: 17 icons across `MessageBubble`, `ChatInput`, `Lightbox`, `ChatListPane`, `PhotoGallery`, `chat/channels/page`, and `login/page` patched. Verified via Python sweep that all 77 SVGs in the codebase now carry one of `aria-hidden`/`aria-label`/`role`.
  - **Localized hardcoded `aria-label` strings**: close buttons in `BottomSheet`, `Lightbox`, `CreateGroupModal`, `CreateChannelModal`, `GroupMembersPanel` now use `common.close`; `profile/page.tsx` avatar trigger uses new `profile.changeAvatar` (EN/ES/PT). Added `chat.conversationsList` for the desktop DM-list `<aside>` landmark.
  - **`RangeSlider` accessibility**: native `<input type="range">` already had keyboard support; added `aria-label` and `aria-valuetext` props so screen readers announce the slider's purpose and the formatted value (e.g. "Any distance" / "30 km") instead of the raw number. Wired through the Browse filter sliders.
  - **Manual screen-reader pass not in scope** for this PR — the structural a11y work above can be CI-verified, but VoiceOver/iOS Safari behaviour requires hands-on testing and will land in a follow-up.

- **Desktop UX + accessibility — phase 1** (PR #87):
  - **Start-a-chat from `/chat`**: new `NewChatModal` (`components/chat/NewChatModal.tsx`) lists the user's accepted contacts with search and `POST`s to `/chat/rooms/dm` on selection. Triggered from a new "Nuevo chat" button in the chat header next to the existing "Nuevo grupo" button. `ChatListPane` fetches `/contacts` once on mount and uses the contact count to **disable both compose buttons when the user has zero contacts**, with localized `title` tooltips explaining the prerequisite (EN/ES/PT). Contacts are sorted alphabetically (case-insensitive `localeCompare`); a contact count is shown above the list; a footer note clarifies that "only accepted contacts can be messaged" with a "Manage contacts" link to `/contacts` for resolving pending/sent requests. Eliminates the "I have no DMs and no obvious way to start one" dead-end.
  - **W-2 Desktop two-column DM layout (layout-owned, persistent)**: chat routes restructured around a `(messages)` route group with its own `layout.tsx` that owns the `ChatListPane` side rail. The list is mounted once and persists across navigation between `/chat` and `/chat/<roomId>` and between threads — no flicker, no refetch, no scroll-position loss when switching conversations. Channel rooms moved to a dedicated `/chat/channels/[channelId]` route that lives outside the (messages) group, so the DM list pane never appears next to a channel — accidental channel-switching could erase membership and message access, so channels stay deliberately single-pane. The 750-line room body extracted into `components/chat/RoomView.tsx` with a `surface: "messages" | "channels"` prop that controls the back/leave destination; both new room routes render `<RoomView />`. Legacy URLs are handled — `/chat/<channel-id>` redirects to `/chat/channels/<id>` (and vice versa) once the room type is known. `ChatListPane` (`components/chat/ChatListPane.tsx`) supports `variant="page" | "pane"` and marks the active row with `aria-current="page"` + brand-tinted background.
  - **W-7 Skip-to-content link**: `AppShell` exposes a `sr-only focus:not-sr-only` skip link as the first focusable element when authenticated; jumps to the new `<main id="main-content">` wrapper. EN/ES/PT copy in `nav.skipToContent`.
  - **Channel leave copy hardening**: `chatRoom.leaveChannelMessage` rewritten in EN/ES/PT to spell out the irreversible nature of the action — leaving a channel permanently removes access to past messages.
  - **Note on W-1**: Sidebar already highlights the current route (`Sidebar.tsx` `isActive` + `aria-current="page"`); no work needed.

- **Mobile UX + navigation audit** ([PR #86](https://github.com/mayloo89/circl/pull/86)):
  - **M-1 Safe area insets**: `--safe-top` / `--safe-bottom` CSS variables in `:root`; `TopBar` wraps content in an inner `h-14` div and adds `paddingTop: env(safe-area-inset-top)` to its fixed header so the background colour fills the notch/Dynamic Island area; `BottomNav` uses `height: calc(4rem + env(safe-area-inset-bottom))` + matching `paddingBottom` so nav items stay in the 4 rem zone; `AppShell` padding via `@utility pt-topbar` / `@utility pb-bottomnav` with `lg:pt-0 lg:pb-0` overrides; `Toast` and the public profile sticky action bar account for the bottom safe area.
  - **M-2 Input auto-zoom**: `Input` component adds `text-base` (16 px) to its `<input>` element; `ChatInput` textarea also uses `text-base` — both prevent iOS Safari's 300 ms auto-zoom on focus.
  - **M-3 Auto-growing textarea**: `ChatInput` replaces its single-line `<input type="text">` with a `<textarea rows={1} class="max-h-40 resize-none overflow-y-auto">` that expands as the user types (`scrollHeight` approach); Enter sends, Shift+Enter inserts a newline; height resets on send; `inputRef` prop type updated to `HTMLTextAreaElement`.
  - **M-4 Camera capture**: onboarding photo `<input>` adds `capture="user"` (front camera for avatar selfies); chat attachment `<input>` adds `capture="environment"` (rear camera for media messages).
  - **M-5 Push prompt timing**: `PushPrompt` defers the notification permission request until the user has at least one conversation — it fetches `GET /chat/rooms` on mount and only sets `eligible` when the list is non-empty; falls back to showing after 30 s if the request fails.
  - **M-6 Touch targets**: Bell buttons in `TopBar` (`p-1.5` → `p-3`), `BottomSheet` close (`p-1.5 h-4 w-4` → `p-3 h-5 w-5`), `ProfileCompletenessBanner` dismiss (`p-1 h-3.5 w-3.5` → `p-3.5 h-4 w-4`), chat room back button (added `p-3`, was bare icon), public profile hero and overflow buttons (`h-10 w-10` → `h-11 w-11`), own profile mobile Settings + Logout buttons (`p-2` → `p-3`) — all now meet the 44 pt minimum.
  - **Mobile settings & logout**: Added `lg:hidden` gear-icon link (→ `/settings`) and logout button to the own profile page header so mobile users can reach both without the sidebar.
  - **Collapsed sidebar logout**: Sign-out button was hidden behind `{!collapsed && ...}`; moved to a dedicated always-visible row above the user/avatar row.
  - **Navigation cleanup**: Spurious back buttons removed from `/chat` and `/chat/channels` (top-level pages); back button removed from Contacts page (top-level) and own profile page (reached via persistent TopBar/Sidebar avatar).
  - **BottomNav restructure**: Profile tab removed; Channels restored — final 5 items: Home, Browse, Messages, Channels, Contacts. Profile on mobile is accessed via the TopBar avatar (now with `p-1.5` touch target).
  - **Sidebar cleanup**: Profile removed from `navItems`; the avatar/name row at the bottom of the sidebar is the sole desktop profile entry point, eliminating the duplicate link.
  - **Profile preview mode**: Visiting `/profile/[username]` as the profile owner now renders a sticky preview banner ("This is how your profile appears to others" + "Exit preview" pill → `/profile`) instead of silently redirecting; action buttons (contact/message/block/report) are hidden in preview mode; i18n keys added in EN/ES/PT.
  - **Layout consistency**: Contacts and own profile edit pages `max-w-lg` → `max-w-2xl`; public profile content section capped at `mx-auto max-w-2xl` — all list/form pages now share the same width as chat list and settings.

### Security

- **CSP nonce — remove `unsafe-inline` from `script-src` (S-6)** ([PR #84](https://github.com/mayloo89/circl/pull/84)): A per-request nonce (base64 UUID) is generated in `middleware.ts` for every production request. The nonce is injected into both the request headers (`x-nonce`, `Content-Security-Policy`) so Next.js stamps its own bootstrap scripts, and the response `Content-Security-Policy` header so the browser enforces it. `'unsafe-inline'` is removed from `script-src`; the static CSP block in `next.config.ts` is removed since middleware now owns it exclusively.

- **WebSocket ticket auth (S-3)** ([PR #83](https://github.com/mayloo89/circl/pull/83)): `POST /ws-ticket` (behind `RequireAuth`) issues a single-use UUID ticket stored in Redis with a 60 s TTL; `GET /chat/rooms/{id}/ws` now authenticates via `?ticket=` instead of `?token=`; tickets are consumed atomically via `GETDEL` so reuse returns 401; JWT no longer appears in WebSocket upgrade query strings or proxy access logs. New `internal/wsticket` package with `Store`, `Issuer`, and `Redeemer` interfaces.

- **Privacy hardening — profile fields, presence gating, contact rate limit** ([PR #82](https://github.com/mayloo89/circl/pull/82)):
  - **S-1 Profile privacy:** `GET /profiles/{ref}` now returns a public subset for non-owners (computed age, no DOB, no lat/lng); owner view unchanged; `computeAge` helper added.
  - **S-2 Soft-delete filter:** `GetPresence` query adds `AND status = 'active'` so deleted/banned users no longer return `last_seen_at` data.
  - **S-4 Presence gating:** `GET /presence?ids=...` returns real `online` status to any authenticated user (discovery use case); `last_seen_at` is only included for the caller's accepted contacts and themselves; absent from JSON entirely (`omitzero`) for non-contacts. `PresenceStore` interface extracted so handlers depend on an abstraction; nil-DB guards removed, replaced with proper stubs in tests.
  - **S-7 Contact request rate limit:** `POST /contacts` enforces 100 requests/day per user via `contacts.WithLimiter`; wired in `main.go` alongside the existing reports limiter.

### Fixed

- **Pi deploy — disable Next.js image optimizer** ([PR #80](https://github.com/mayloo89/circl/pull/80)): `NEXT_PUBLIC_IMAGE_UNOPTIMIZED` build arg (default `false`); when set to `"true"` in `deploy/docker-compose.prod.yml`, `images.unoptimized: true` is baked into the Next.js bundle at build time, bypassing `/_next/image` and its `remotePatterns` check; eliminates CPU-intensive WebP conversion on Raspberry Pi; `frontend/Dockerfile` wires the new ARG/ENV.

- **Pi deploy — CSP fix for Next.js App Router** ([PR #79](https://github.com/mayloo89/circl/pull/79)): `script-src` gains `'unsafe-inline'` (required for App Router bootstrap hydration scripts); `style-src` adds `https://fonts.googleapis.com`; `font-src` adds `https://fonts.gstatic.com`; `connect-src` adds `wss:`; `img-src` broadened to `https:`.

- **Pi deploy — configurable local storage URL** ([PR #78](https://github.com/mayloo89/circl/pull/78)): `LOCAL_STORAGE_BASE_URL` env var replaces hardcoded `http://localhost:<port>/uploads/files` so file URLs resolve correctly behind an nginx reverse proxy; backend logs `base_url` on startup; `backend/Dockerfile` adds `RUN mkdir -p /app/data && chown circl:circl /app/data` so the uploads volume mount point exists before the process starts; `deploy/` and `.env.prod` added to `.gitignore`.

### Added

- **Modern Go + UX/UI audit fixes** ([PR #75](https://github.com/mayloo89/circl/pull/75)):
  - **Go backend — 19 modernizations:** `errors.Is()` replaces `==` / `!=` comparisons on sentinel errors in `presence.go`, `db.go`, and `refresh_store_test.go` (6 sites); `omitzero` replaces `omitempty` on all `*time.Time` and map fields across `chat/handler.go`, `chat/chat.go`, `uploads/uploads.go`, `reports/reports.go`, and `worker/otel.go` (8 sites); `max()` builtin replaces explicit clamp blocks in `worker/image_task.go` (2 sites); `for range n` replaces `for i := 0; i < n; i++` in `ratelimit/redis_test.go`; `strings.Cut` replaces `[:strings.Index()]` slice in `cmd/api/main.go`
  - **Frontend — cursor-pointer global:** `cursor-pointer` added to `Button.tsx` base class, fixing all 23+ `<Button>` instances; `cursor-pointer` also added to every raw `<button>` across `ContactCard`, `PushPrompt`, `ChatInput`, `PhotoGallery`, `MessageBubble`, `chat/[roomId]/page`, `admin/reports`, `channels/page`, and `browse/page`
  - **Focus rings on raw `<button>` elements:** `focus:outline-none focus:ring-2 focus:ring-brand-hover` (or `focus:ring-red-500` for destructive actions) added to every interactive `<button>` not using the `<Button>` component (20+ sites)
  - **`prefers-reduced-motion` global rule:** added to `globals.css` — `*::before/after` rule sets all `transition-duration` and `animation-duration` to `0.01ms !important`, covering the 59 component-level transitions that were previously un-reduced
  - **`scroll-padding-top`:** `html { scroll-padding-top: 3.5rem }` added to `globals.css` for correct anchor navigation with fixed header
  - **`scrollbar-hide` utility:** defined in `globals.css` (`scrollbar-width: none` + `::-webkit-scrollbar { display: none }`) for `NearbyProfilesWidget`
  - **Emojis → SVG icons:** replaced 5 emoji icons with accessible SVGs — username availability indicators (✓ → checkmark SVG, ✗ → X SVG) in `register/page.tsx`, close button (✕ → X SVG) in `channels/page.tsx`, back arrow (← → chevron-left SVG) in `chat/[roomId]/page.tsx`, paperclip (📎 → paperclip SVG) in `MessageBubble.tsx`
  - **Alt text:** descriptive `alt` attributes added to 6 previously-empty images — `Avatar.tsx` (`alt={name ?? "User avatar"}`), `PhotoGallery.tsx` (`alt="Profile photo"` × 2), `Lightbox.tsx` (`alt="Full size image"`), `onboarding/photo/page.tsx` (`alt="Profile photo preview"`), `profile/page.tsx` (`alt="Your avatar"`), `NearbyProfilesWidget.tsx` (`alt={profile.display_name ?? "Nearby user"}`)
  - **Layout shift fixes:** `active:scale-95` → `active:opacity-70` on view-once and media buttons in `MessageBubble.tsx`; `group-hover:scale-105` → `group-hover:opacity-80` on profile avatar images in `browse/page.tsx` and `NearbyProfilesWidget.tsx`; `active:scale-95` → `hover:opacity-90 active:opacity-70` on photo grid in `PhotoGallery.tsx`
  - **BottomNav icon consistency:** all 6 SVG icon functions changed from hardcoded `width="22" height="22"` to `className="h-5 w-5"` (aligns with the rest of the icon system)
  - **`aria-label` on unlabeled inputs:** added `aria-label` to the interests search input in `browse/page.tsx`; `transition-colors` / `transition-opacity` added alongside hover states that were missing transitions (7 sites)

- **Chat polish + auth UX + onboarding smart steps** ([PR #74](https://github.com/mayloo89/circl/pull/74)):
  - `PasswordField` component with show/hide toggle; replaces `<input type="password">` on registration, login, settings, and reset-password pages
  - `DateOfBirthPicker` component with three equal-width numeric selects (DD / MM / YYYY); replaces `<input type="date">`; uses local state so partial selections are preserved across React re-renders
  - Scroll-to-bottom FAB in chat room: auto-hides when user is at the bottom, appears on new messages when scrolled up
  - Chat list search: full-text client-side filter across room names and participants; empty-state message when no match
  - 10-second polling on chat list: silently refreshes unread counts and last-message previews without disrupting the search UI
  - Onboarding smart steps: `incompleteSteps(profile)` / `nextStepAfter` skip already-complete fields; `OnboardingRedirect` redirects directly to the first incomplete step instead of always starting at photo
  - `lib/onboardingSteps.ts`: `ONBOARDING_STEPS`, `incompleteSteps`, `firstIncompleteStep`, `nextStepAfter`
  - E2E tests updated to use `selectOption` on `DateOfBirthPicker` selects
  - All new strings added to EN / ES / PT locale files

- **Onboarding wizard** ([PR #72](https://github.com/mayloo89/circl/pull/72)):
  - 4-step wizard at `/onboarding/{photo,bio,interests,location}` — minimal shell (no nav), step-dot progress bar, "Skip" button on every step, "Skip all" header link
  - Photo step: avatar upload with live preview reusing the existing 3-step upload flow
  - Bio step: textarea with character counter; pre-filled from existing profile
  - Interests step: search-as-you-type chip selector; pre-filled from existing profile
  - Location step: city search (Photon/OSM) + "Use my location" geolocation button; last step marks profile as onboarded via `mark_onboarded: true`
  - `AppShell` redirects authenticated users without `onboarded_at` to `/onboarding/photo` (skips own-profile and onboarding pages); suppresses Sidebar / TopBar / BottomNav on onboarding pages
  - `ProfileCompletenessCard` on own profile page (`/profile`): progress bar, percentage, per-field links to the relevant onboarding step
  - Backend migration `000026`: `onboarded_at TIMESTAMPTZ NULL` column on `profiles`; `mark_onboarded: true` in `PUT /profiles/me` sets it once (write-once via `COALESCE`)
  - `onboarded_at` included in `ProfileContext.MyProfile` and in all three locale files (EN / ES / PT)

- **Public profile hero redesign** ([PR #71](https://github.com/mayloo89/circl/pull/71)):
  - 55 vh hero section: avatar photo as full-bleed `<Image>` with `object-cover`; gradient-fade overlay blending into the page background
  - Initials fallback when no avatar: brand-gradient background with a large translucent initial
  - Back button and ⋯ overflow menu float over the hero via absolute positioning with `backdrop-blur`
  - Overflow menu houses Block / Unblock / Report; removed inline "Block user" and "Report" text links
  - `<h1>` now displays the person's display name (previously showed generic "Profile" string)
  - **Sticky mobile action bar** fixed above the bottom nav (`bottom-16`), containing context-aware CTA (Message / Add contact / Request sent / Accept request)
  - **Desktop inline actions** shown below the interests section (`hidden lg:block`)
  - **"Preview as visitor"** button on own profile page (`/profile`) opens `/profile/{username}`
  - All new strings translated in EN / ES / PT

- **Browse overhaul** ([PR #70](https://github.com/mayloo89/circl/pull/70)):
  - `RangeSlider` UI primitive — single-thumb slider with filled track and active-scale thumb; used for age and distance filters
  - `BottomSheet` UI primitive — slides up from bottom on mobile with backdrop + Escape key + body scroll lock; renders nothing on desktop (`lg:hidden`)
  - Mobile filter panel replaced: `<details>` removed, new "Filters" pill button with active-filter count badge opens the BottomSheet
  - Age min/max and max-distance number inputs replaced by `RangeSlider`; 500 km = "Any" (maps to `null` in preferences)
  - "Clear filters" button shown on the empty state when at least one filter is active
  - Desktop filter sidebar widened to `w-64`, made `sticky top-6`; filter labels use uppercase tracking style
  - All filter panel strings translated in EN / ES / PT

- **Home dashboard** ([PR #69](https://github.com/mayloo89/circl/pull/69)):
  - `PendingRequestsWidget` — fetches `/contacts/pending` and renders inline Accept / Decline buttons; hidden when empty; reacts to `contact_request` and `contact_removed` SSE events
  - `NearbyProfilesWidget` — fetches `/profiles/browse?limit=8`; horizontal-scroll chip row on mobile; skeleton loading state; "Browse all" link to `/browse`
  - `RecentConversationsWidget` — fetches `/chat/rooms?limit=5`; shows avatar, room name, last-message preview (text / photo / video / file), relative time, and unread badge; skeleton loading state; "See all" link to `/chat`
  - `ProfileCompletenessBanner` — sticky banner with progress bar (avatar 30 %, bio 20 %, interests 20 %, birthdate 10 %, location 20 %); lists missing fields; dismissed via `sessionStorage`; hidden once profile is complete
  - `lib/profileCompleteness.ts` — pure `computeCompleteness` function, reusable by the profile page in future PRs
  - Home page (`app/[locale]/page.tsx`) replaced: old 3-button placeholder → real dashboard with widgets
  - i18n keys added in EN / ES / PT: pending requests, nearby, recent conversations, profile completeness labels

- **Navigation overhaul — mobile bottom nav + desktop sidebar** ([PR #68](https://github.com/mayloo89/circl/pull/68)):
  - `BottomNav` — fixed 5-tab bar (Home / Browse / Messages / Contacts / Profile) for mobile (`lg:hidden`), with live unread/pending badges and iOS safe-area padding
  - `Sidebar` — fixed left sidebar for desktop (`lg:flex hidden`), same 5 items with icons + labels, language switcher, settings, and a user row with avatar + sign-out; admin link surfaced automatically for admin/super_admin roles
  - `TopBar` — minimal mobile-only header (`lg:hidden`) with logo, active-route label, push-notification toggle, and avatar shortcut to profile
  - `ProfileContext` — fetches `/profiles/me` once per authenticated session; shared by Sidebar, TopBar, and home dashboard (UX-4); eliminates per-mount profile fetches
  - Old monolithic `NavBar.tsx` removed; sign-out and locale-change logic moved into `Sidebar`

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
  - Frontend `lib/auth.ts` — JWT callback checks access-token expiry and calls `/auth/refresh` silently; stores `refreshToken` in the NextAuth session
  - `SessionGuard` — also signs out on `RefreshFailed` (expired refresh token or network error during silent refresh)
  - `NavBar` and `SignOutButton` — send `POST /auth/logout` with the refresh token before calling `signOut()`

- **Grafana observability stack** ([PR #60](https://github.com/mayloo89/circl/pull/60)):
  - `grafana/tempo:2.7.2` added to `docker-compose.yml` — OTLP HTTP (4318) + gRPC (4317) receivers; metrics_generator forwards service-graph and span-metrics to Prometheus via remote-write; 7-day trace retention; local filesystem storage
  - `prom/prometheus:v3.3.1` added to `docker-compose.yml` — scrapes backend at `host.docker.internal:8080/metrics`; remote-write receiver enabled for Tempo metrics_generator; `ops/prometheus/prometheus.yml` + `ops/prometheus/alerts.yml`
  - Recording rules: `job:circl_http_error_rate:rate5m`, `job:circl_http_request_rate:rate5m`, `job:circl_http_p95_latency:rate5m`
  - Alert rules: `HighErrorRate` (>1% 5xx for 5m), `HighLatencyP95` (>1s p95 for 5m), `DBPoolExhausted` (>90% pool for 2m), `BackendDown` (scrape target gone for 1m)
  - `grafana/grafana:11.5.2` added to `docker-compose.yml` on port 3001 (3000 is Next.js); anonymous access enabled for dev
  - `ops/grafana/provisioning/datasources/datasources.yaml`: Prometheus (default, exemplar→Tempo), Loki (derived field `trace_id`→Tempo), Tempo (traces-to-logs via Loki, service map, node graph)
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

- **Structured logging with zerolog** ([PR #55](https://github.com/mayloo89/circl/pull/55)):
  - `github.com/rs/zerolog` replaces the standard `log` package across the entire backend (51 call sites)
  - New `internal/logger` package: `New(env, level string) zerolog.Logger` — human-readable console output in development, JSON to stdout in production; log level configurable via `LOG_LEVEL` env var (default: `info`)
  - New `middleware.RequestLogger(log)` middleware: assigns a unique `request_id` to every request using `xid`, attaches a request-scoped logger to the context via `zerolog.Ctx`, sets the `X-Request-ID` response header, and writes one structured access-log entry per request with `method`, `path`, `status`, `latency_ms`, and `request_id`
  - `middleware.EnrichRequestLog(ctx, key, value)`: pointer-based context accumulator that lets downstream middleware contribute fields to the access-log entry — used by `RequireAuth` to add `user_id` and role to every authenticated request's log line
  - `RequireAuth` middleware now enriches the context logger with `user_id` so every log line emitted inside an authenticated handler automatically carries the user identity
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

- **Admin panel: channel management, role-based access control, and hard delete** ([PR #52](https://github.com/mayloo89/circl/pull/52)):
  - Channel management UI in the admin panel: create, edit, and delete public channels
  - Role-based access control: super_admin role can promote/demote admins; admin cannot modify other admins
  - Hard delete for users: permanently removes all associated data (profile, photos, messages, uploads) from DB and S3
  - Admin panel sidebar navigation with Dashboard, Users, Reports, and Channels sections

- **Reversible account deletion with 30-day grace period** ([PR #51](https://github.com/mayloo89/circl/pull/51)):
  - Migration `000023`: adds `deleted_at TIMESTAMPTZ` column to `users`
  - `DELETE /users/me` now sets `status = 'deleted'` and records `deleted_at` timestamp; sends a deletion warning email async with a link to sign in and reactivate
  - Reactivation is automatic on login: `POST /auth/login` restores `status = 'active'` when valid credentials are supplied within the 30-day grace period and returns `{"reactivated": true}` in the response
  - Accounts past the 30-day window receive `401 {"error":"invalid credentials"}` — indistinguishable from wrong password
  - Login page: detects `reactivated: true` via a direct pre-signIn probe and shows a modal popup ("Account reactivated") before navigating home
  - Daily background worker (`worker.PurgeDeletedAccounts`) fully purges expired deleted accounts: retrieves all S3/MinIO upload keys, deletes files from object storage (originals + thumbnails), removes all associated DB records, anonymizes the `users` row in-place
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
  - Frontend: `/forgot-password`, `/reset-password`, `/verify-email` pages added; login page gains "Forgot password?" link and email-not-verified warning banner with inline resend button

- **Settings page** ([PR #47](https://github.com/mayloo89/circl/pull/47)): `/settings` page with push notifications toggle, change password with live validation, delete account with confirmation; `PushContext` shared between NavBar and settings; avatar dropdown menu; `PushPrompt` dismiss button

- **Public chat channels** ([PR #46](https://github.com/mayloo89/circl/pull/46)): IRC-style open rooms — any authenticated user can enter, chat, and leave freely; ephemeral membership (WS connection = presence, no `room_members` rows); no message history per session; live participant sidebar with filter; admin-only channel creation; leave confirmation guard; migrations 000020–000021

- **Group chat** ([PR #45](https://github.com/mayloo89/circl/pull/45)): `PUT /chat/rooms/{id}` (rename, admin only), `GET /chat/rooms/{id}/members`, `POST /chat/rooms/{id}/members` (add, admin only), `DELETE /chat/rooms/{id}/members/{userID}` (remove/self-leave); `CreateGroupModal`; `GroupMembersPanel`; migration `000019` adds `creator_id` to `rooms`

- **Cursor-based pagination** ([PR #44](https://github.com/mayloo89/circl/pull/44)): `GET /profiles/browse` and chat history (`GET /chat/rooms/{id}/messages`) use cursor-based pagination (`before` timestamp); infinite scroll on browse page; "Load more" at top of chat room for older messages

- **Web Push Notifications** ([PR #43](https://github.com/mayloo89/circl/pull/43)): `POST /push/subscribe`, `DELETE /push/unsubscribe`, `GET /push/vapid-public-key`; migration `000018_push_subscriptions`; `backend/internal/push` package (`Service`, `Store`, `Handler`); push delivery on new chat messages and contact events; `usePush` hook; service worker (`public/sw.js`); `PushPrompt` banner; VAPID configuration via env vars

- **Account safety** ([PR #42](https://github.com/mayloo89/circl/pull/42)): login lockout after 5 failed attempts (15-minute window); password complexity requirements (8+ chars, uppercase, lowercase, number, special char); rate limiting via `LOGIN_IP_LIMIT` / `REGISTER_IP_LIMIT` env vars (per IP per hour)

- **Admin & moderation** ([PR #41](https://github.com/mayloo89/circl/pull/41)): migration `000017_admin_moderation` adds `role VARCHAR(20)` to `users` and creates `reports` table; `PUT /users/{id}/role`; `PUT /users/{id}/suspend`, `PUT /users/{id}/activate`; `POST /users/{id}/report` (rate limited, 10/hour); auto-suspend on 3+ unresolved reports in 7 days; `GET /admin/reports`, `PUT /admin/reports/{id}/resolve|dismiss`

- **User blocking** ([PR #39](https://github.com/mayloo89/circl/pull/39)): migration `000015_create_blocks`; `POST /contacts/{id}/block`, `DELETE /contacts/{id}/block`, `GET /contacts/blocked`; bidirectional suppression in search/contacts/chat/WebSocket; `ConfirmDialog` UI component

- **Browse/explore** ([PR #37](https://github.com/mayloo89/circl/pull/37), [PR #38](https://github.com/mayloo89/circl/pull/38)): migration `000014_browse_indexes`; `GET /profiles/browse` with age/distance/gender/interests filters; Haversine distance in SQL; coordinates saved from Photon/OSM location picker; sort-by-distance option; `/browse` frontend page with card grid, infinite scroll, filter sidebar; `GET /profiles/available?username=` availability check

- **Expanded profiles** ([PR #35](https://github.com/mayloo89/circl/pull/35)): migration `000013_expand_profiles` — adds DOB, gender, location, interests; `profile_preferences` table; `GET/PUT /profiles/me/preferences`; profile edit page: DOB picker, gender dropdown, location autocomplete (Photon/OSM), interests tag input; public profile page displays age, gender, location, interests

- **Usernames** ([PR #36](https://github.com/mayloo89/circl/pull/36)): mandatory unique username at registration; `GET /profiles/@{username}`; profile URLs use username; availability check endpoint

- **Backend integration tests** ([PR #34](https://github.com/mayloo89/circl/pull/34)): `internal/testutil` package (`OpenDB`, `CreateUser`, `NewRedis` helpers); integration tests for chat and uploads stores; CI `backend-integration` job

- **Playwright E2E testing** ([PR #33](https://github.com/mayloo89/circl/pull/33)): `playwright.config.ts` with chromium, retry-on-failure; E2E tests for auth, profile, contacts, and chat flows; CI `e2e` job

- **Frontend testing foundation** ([PR #32](https://github.com/mayloo89/circl/pull/32)): vitest + @testing-library/react + msw; unit tests for UI primitives, hooks, lib helpers

- **Route guard cleanup and form validation** ([PR #31](https://github.com/mayloo89/circl/pull/31)): `lib/validation.ts` with Zod schemas; removed redundant per-page auth redirects; form validation on login/register

- **Domain component library** ([PR #30](https://github.com/mayloo89/circl/pull/30)): `MessageBubble`, `DateSeparator`, `TypingIndicator`, `ChatInput`, `Lightbox`; `ContactCard`, `SearchBar`; `PhotoGallery`, `ProfileHeader`; shared `types/chat.ts` and `lib/chatHelpers.ts`

- **UI primitive component library** ([PR #29](https://github.com/mayloo89/circl/pull/29)): `Avatar`, `Badge`, `Button`, `Input`, `Skeleton`, `Modal`, `Toast`, `PresenceDot`; route `loading.tsx`/`error.tsx` for all authenticated segments

- **Public profiles and photo gallery** ([PR #27](https://github.com/mayloo89/circl/pull/27)): `profile_photos` table, gallery uploads (max 6); `GET /profiles/{userID}`; `/profile/[userId]` public profile page

- **Chat UI improvements** ([PR #25](https://github.com/mayloo89/circl/pull/25), [PR #26](https://github.com/mayloo89/circl/pull/26)): message grouping, date separators, skeleton loaders, new-message animation, relative timestamps, empty and error states, attachment type previews

- **Read receipts** ([PR #23](https://github.com/mayloo89/circl/pull/23)): ✓ / ✓✓ checkmarks on messages; `read_receipt` WebSocket broadcast; real-time updates

- **Typing indicators** ([PR #22](https://github.com/mayloo89/circl/pull/22)): real-time "X is typing…" via WebSocket with 2s server-side debounce

- **Image thumbnails in chat** ([PR #24](https://github.com/mayloo89/circl/pull/24)): `thumbnail_url` on messages; `LEFT JOIN uploads` in `ListMessages`; thumbnail display with lightbox fallback

- **Image processing worker** ([PR #20](https://github.com/mayloo89/circl/pull/20)): asynq worker for EXIF strip and 480px JPEG thumbnails; `thumbnail_key` on uploads

- **Ephemeral messages** ([PR #21](https://github.com/mayloo89/circl/pull/21)): view-once (tap-to-view); TTL options (15m–24h); tombstone messages; TTL countdown badge; cleaner worker

- **Chat attachments** ([PR #18](https://github.com/mayloo89/circl/pull/18)): image/video/file messages; lightbox

- **S3-compatible storage** ([PR #19](https://github.com/mayloo89/circl/pull/19)): `S3Storage` via minio-go; Docker Compose dev/prod setup

- **Avatar upload** ([PR #17](https://github.com/mayloo89/circl/pull/17)): profile avatar upload; displayed in navbar, contacts list, chat list, chat room header, and message bubbles

- **Storage infrastructure** ([PR #16](https://github.com/mayloo89/circl/pull/16)): `Storage` interface; `LocalStorage`; uploads API (request → confirm lifecycle); `useUpload` hook

- **Presence** ([PR #15](https://github.com/mayloo89/circl/pull/15)): Redis heartbeat with TTL; online/offline dot on contacts; "Last seen X ago" in DM header; `presence_online`/`presence_offline` SSE events

- **Real-time chat + SSE notifications** ([PR #14](https://github.com/mayloo89/circl/pull/14)): WebSocket DMs and group rooms; Redis Pub/Sub fan-out; Postgres message persistence; paginated history; unread counts; `GET /notifications/stream`; `contact_request`, `contact_accepted`, `contact_removed` SSE events; NavBar badge; `NotificationsContext`

- **Contacts** ([PR #9](https://github.com/mayloo89/circl/pull/9), [PR #10](https://github.com/mayloo89/circl/pull/10)): user search; send/accept/decline/cancel requests; `DELETE /contacts/{id}`; `pending → accepted` state machine; real-time SSE notifications

- **Auth** ([PR #2](https://github.com/mayloo89/circl/pull/2)): registration and login with bcrypt; JWT HS256; `RequireAuth` middleware; NextAuth.js credentials provider; httpOnly cookies

### Changed

- **UX visual rebrand foundation** ([PR #67](https://github.com/mayloo89/circl/pull/67)):
  - Fonts replaced: Geist → **Nunito** (display/headings) + **DM Sans** (body), loaded via `next/font/google` with `display: swap`
  - Tailwind v4 brand tokens added to `globals.css` via `@theme`: `--color-brand-primary` (indigo-600), `--color-brand-accent` (#F97316 orange), `--color-brand-success/danger/surface/surface-elevated`; `--radius-card` (1rem), `--radius-pill` (9999px); `--shadow-card` / `--shadow-card-hover` elevation scale
  - New `accent` Button variant (CTA orange `#F97316`) for conversion actions — distinct from `primary` (indigo, neutral) and `warning` (form-level orange)
  - `SendRequestButton` in browse and "Add contact" / "Message" buttons in `ProfileHeader` migrated to `variant="accent"`
  - Browse profile cards upgraded to `rounded-card` / `shadow-card` / `shadow-card-hover` tokens
  - All hardcoded `indigo-*` Tailwind classes replaced with `brand-*` semantic tokens across every frontend component and page
- **Active locale indicator** ([PR #66](https://github.com/mayloo89/circl/pull/66)): language buttons in Settings and the NavBar dropdown now highlight the currently active locale (`bg-indigo-600 text-white`) so users know which language is selected
- **Contacts list sorted online-first** ([PR #66](https://github.com/mayloo89/circl/pull/66)): accepted contacts are sorted so online users appear at the top, using the existing `presence` map — no extra API call required
- **UX polish** ([PR #53](https://github.com/mayloo89/circl/pull/53)): touched-state inline validation on registration — errors only shown after the user has interacted with a field; error messages read from backend response body; 429 rate-limit differentiated from credential errors on login; change-password form collapses behind a button; delete-account moved to a modal; gender options extended (trans male/female, non-binary, custom free-text)
- **Chat upload restrictions + image resizing** ([PR #49](https://github.com/mayloo89/circl/pull/49)): chat attachments restricted to images and videos only (PDF/documents/archives rejected by backend and frontend); JPEG and PNG originals resized to a maximum of 1024px (configurable via `IMAGE_MAX_PX`); thumbnails remain at 480px; `ChatInput` `accept` attribute updated
- **UX improvements** ([PR #48](https://github.com/mayloo89/circl/pull/48)): contact removal requires confirmation dialog; clicking a user's name or avatar in search results navigates to their public profile; registration form per-field inline validation on blur; live `PasswordRequirements` checklist (8+ chars, uppercase, lowercase, number)
- `hub.Notify` now called alongside `push.Send` in `notifyUser` callback for both chat and contact events
- Contact handler uses `contactNotifier` wrapper to enrich SSE events with push notifications
- All profile queries use `COALESCE` for avatar fallback
- Modern Go 1.22–1.25 refactor throughout: `cmp.Or`, `strings.SplitSeq`, `t.Context()`, `wg.Go`, `slices`/`maps` packages

### Fixed

- **Browse distance filter** ([PR #70](https://github.com/mayloo89/circl/pull/70)): profiles with no coordinates previously bypassed the `max_distance_km` filter — the SQL `WHERE` clause now requires both the requester's and the candidate's coordinates to be present when the filter is active
- **UX bugfixes** ([PR #66](https://github.com/mayloo89/circl/pull/66)):
  - Browse page subtitle always showed "No profiles found" regardless of results — replaced with `t("subtitle")` ("Discover people near you") in EN/ES/PT
  - Unblock confirmation dialog displayed the generic "Something went wrong" message instead of the user's name
  - Back buttons in chat list and contacts used `router.push("/")`, destroying browser history — changed to `router.back()`
  - Login page used Tailwind `blue-*` colors while all other pages use `indigo-*`
  - Register success screen used the `✉` emoji as a structural icon — replaced with an inline SVG envelope
  - "Block user" and "Report" action buttons on `ProfileHeader` had `text-gray-600` (contrast ratio ~2.5:1, fails WCAG AA) — bumped to `text-gray-400`
  - Chat room loading state showed a plain `<p>` text string instead of the existing `<MessageSkeletons />` component
- **Frontend fetch hardening + auto sign-out** ([PR #59](https://github.com/mayloo89/circl/pull/59)): six fetch chains now check `r.ok` before calling `.json()` — prevents TypeError crashes when the backend returns an error object instead of an array; `SessionGuard` detects expired backend JWT via `exp` claim and calls `signOut()` automatically
- **Next.js pinned to 16.1.1**: Turbopack memory regression in 16.2.2 caused unbounded memory growth (7+ GB) in the dev server

### Security

- **Security hardening — headers, WS origin, CORS, secret scan** ([PR #61](https://github.com/mayloo89/circl/pull/61)):
  - `SecurityHeaders` middleware: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `Cache-Control: no-store` on every response; HSTS (1 year, includeSubDomains) in production only
  - WebSocket `CheckOrigin` replaced `return true` with exact-match check against `CORS_ALLOWED_ORIGINS`
  - `X-Request-ID` added to CORS `ExposedHeaders`
  - `next.config.ts` gains a `headers()` export applying CSP, HSTS, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, and `Permissions-Policy` to all Next.js routes; `script-src` allows `unsafe-eval` in development only
  - Gitleaks secret-scanning job added as the first CI job — blocks the pipeline on real leaked credentials
  - `.gitleaks.toml` allowlist added for the three known test-only secrets in `ci.yml`
- Blocking is strictly caller-scoped (JWT-enforced); no information leak — blocked users' profiles remain accessible, only interaction is suppressed
