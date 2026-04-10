# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Added
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