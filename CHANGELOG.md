# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

## [2.2.0] - 2026-03-29 — Backend integration tests

### Added
- `internal/testutil` package — `OpenDB`, `CreateUser`, and `NewRedis` helpers shared across all integration test files; skips gracefully when `TEST_DATABASE_URL` is not set
- `internal/chat/store_test.go` — integration tests for the remaining `pgStore` methods: `ViewOnceMessage` (sender rejection, non-view-once rejection, unknown message, successful tombstone), `DeleteMessage` (success + `ErrNotFound` on second delete), `TombstoneMessage` (converts expired message to tombstone, verifies `ListMessages` shows the tombstone, `ErrNotFound` for unknown ID), `ListExpiredMessages`
- `internal/uploads/store_test.go` — integration tests for `pgStore`: `Create` (ID and `CreatedAt` populated), `GetByID` (found + `ErrNotFound`), `Commit` (success, `ErrNotPending` on re-commit), `SetThumbnailKey` (success, `ErrNotFound`)
- CI `backend-integration` job with Postgres 17 + Redis 7 service containers; builds the API binary to run migrations, then runs the full test suite with `TEST_DATABASE_URL`

## [2.1.0] - 2026-03-29 — Playwright E2E testing

### Added
- `playwright.config.ts` — Playwright setup: chromium project, sequential workers, retry-on-failure in CI, `npm run dev` webServer with `reuseExistingServer` locally
- `e2e/fixtures.ts` — base test fixture with `createUser` (API-level user creation) and `loginAs` helpers; `authenticatedPage` fixture for single-user tests
- `e2e/auth.spec.ts` — register new account, login with valid credentials, wrong password error, duplicate email error
- `e2e/profile.spec.ts` — view My Profile page, update display name, navigate home
- `e2e/contacts.spec.ts` — search by email, send contact request, accept request, open DM from contacts
- `e2e/chat.spec.ts` — full real-time flow: two users become contacts, both open the same DM room, user A sends a message, both see it
- `npm run test:e2e`, `npm run test:e2e:ui`, `npm run test:e2e:debug` scripts
- `@playwright/test` devDependency
- CI `e2e` job with Postgres 17 + Redis 7 service containers, Go backend auto-start (`STORAGE_PROVIDER=local`), and artifact upload on failure

## [2.0.0] - 2026-03-28 — Frontend testing foundation

### Added
- `vitest.config.ts` — vitest setup with jsdom, `@vitejs/plugin-react`, path alias (`@/`), and module-level mocks for `next/image` and `next/navigation`
- `test/setup.ts` — global test setup: `@testing-library/jest-dom` matchers + MSW server lifecycle
- `test/msw-server.ts` — shared MSW `setupServer` instance for handler overrides per test
- `test/__mocks__/next-image.tsx` — `next/image` mock rendering a plain `<img>` tag
- `test/__mocks__/next-navigation.ts` — `next/navigation` hook mocks (`useRouter`, `useParams`, etc.)
- `components/ui/__tests__/` — full test suite for all 8 UI primitives: `Avatar`, `Badge`, `Button`, `Input`, `Modal`, `PresenceDot`, `Skeleton`, `Toast`
- `hooks/__tests__/useUpload.test.ts` — 3-step upload flow, file size/type validation, all error paths, network failure
- `hooks/__tests__/useHeartbeat.test.ts` — initial beat, interval setup, visibility-change behavior, cleanup
- `hooks/__tests__/usePresence.test.ts` — presence fetch, polling interval, SSE online/offline updates, `formatLastSeen` helper
- `lib/__tests__/chatHelpers.test.ts` — full coverage of `sameCalendarDay`, `formatDaySeparator`, `isFirstInGroup`, `isLastInGroup`, `formatExpiry`, `expiryColorClass`
- `lib/__tests__/validation.test.ts` — `loginSchema` and `registerSchema` happy path and all rejection cases
- `npm run test`, `npm run test:watch`, `npm run test:coverage` scripts
- vitest, @vitejs/plugin-react, @testing-library/react, @testing-library/user-event, @testing-library/jest-dom, jsdom, msw, @vitest/coverage-v8 devDependencies

### Changed
- `hooks/useUpload.ts` — added `/* v8 ignore next 2 */` on the unreachable KB branch of `formatBytes` (all category limits are ≥ 1 MB)

## [1.9.0] - 2026-03-28 — Route guard cleanup and form validation

### Added
- `lib/validation.ts` — Zod schemas (`loginSchema`, `registerSchema`) for auth forms with typed `LoginInput` / `RegisterInput` exports
- `zod` dependency for schema-based validation

### Changed
- `app/login/page.tsx` — validates email and password with `loginSchema` before calling `signIn`
- `app/register/page.tsx` — validates email, password length, and password confirmation with `registerSchema`; replaces manual `password !== confirm` check
- `app/page.tsx` — removed redundant server-side auth redirect (`proxy.ts` already handles all route protection)
- `app/chat/page.tsx`, `app/chat/[roomId]/page.tsx`, `app/contacts/page.tsx`, `app/profile/page.tsx`, `app/profile/[userId]/page.tsx` — removed per-page `useEffect` auth redirects (redundant with `proxy.ts`)

## [1.8.0] - 2026-03-28 — Domain component library

### Added
- `types/chat.ts` — shared chat types: `HistoryMessage`, `AnyMessage`, `EphemeralMode`, `EPHEMERAL_LABELS`
- `lib/chatHelpers.ts` — message grouping utilities: `isFirstInGroup`, `isLastInGroup`, `sameCalendarDay`, `formatDaySeparator`, `formatExpiry`, `expiryColorClass`
- `components/chat/Lightbox` — unified image/video lightbox; wraps `Modal`; replaces duplicated inline lightboxes in chat room and public profile
- `components/chat/MessageBubble` — full message rendering: tombstone, view-once (own/other), image, video, file, text; expiry countdown; read receipt; animation support
- `components/chat/DateSeparator` — horizontal rule with "Today / Yesterday / weekday" label
- `components/chat/TypingIndicator` — "X is typing…" bar; renders nothing when no typers
- `components/chat/ChatInput` — complete input bar: attach button, ephemeral mode menu, typing throttle, text input, send button; input state is owned internally
- `components/contacts/ContactCard` — unified contact list row with 4 variants: `search-result`, `pending`, `sent`, `contact`; includes presence dot for accepted contacts
- `components/contacts/SearchBar` — search input + results list in a card; used in contacts page
- `components/profile/PhotoGallery` — photo grid with two modes: `editable` (upload slot, delete with confirm overlay) and view-only (tap to open lightbox); confirm state is internal
- `components/profile/ProfileHeader` — public profile identity card: avatar, name, bio, and contact action button (4 states: add / sent / incoming / message)

### Changed
- `app/chat/[roomId]/page.tsx` — reduced from ~800 lines to ~210 using `MessageBubble`, `DateSeparator`, `TypingIndicator`, `ChatInput`, `Lightbox`
- `app/contacts/page.tsx` — uses `ContactCard` and `SearchBar`
- `app/profile/page.tsx` — uses `PhotoGallery` (editable mode)
- `app/profile/[userId]/page.tsx` — uses `ProfileHeader`, `PhotoGallery` (view mode), and `Lightbox`

## [1.7.0] - 2026-03-28 — UI primitive component library

### Added
- `components/ui/Avatar` — image with initial-letter fallback; 5 sizes (xs → xl); gray/indigo color variants for user vs group room contexts
- `components/ui/Badge` — count badge with dot/count/pill variants; used for nav overlays, unread counts, and section headings
- `components/ui/Button` — unified button with 6 variants (primary, secondary, danger, success, ghost, warning), 3 sizes, pill shape, and `loading` prop with inline spinner
- `components/ui/Input` — labeled text input with `error`, `helper`, `dirty` (orange border for unsaved changes), and `labelHidden` props
- `components/ui/Skeleton` — `animate-pulse` primitive block; page-level loading composites now compose it
- `components/ui/Modal` — full-screen backdrop with Escape key and backdrop-click dismissal; replaces inline lightbox in `/profile/[userId]`
- `components/ui/Toast` — global toast notification system: `ToastProvider` (wired in `providers.tsx`) + `useToast()` hook
- `components/ui/PresenceDot` — sm/md sized online/offline indicator; replaces inline `<span>` in contacts list and chat header
- `loading.tsx` and `error.tsx` route boundaries for all authenticated segments: `/chat`, `/chat/[roomId]`, `/contacts`, `/profile`, `/profile/[userId]`

### Changed
- `NavBar` — uses `Avatar` and `Badge`
- `app/contacts/page.tsx` — uses `Avatar`, `Badge`, `Button`, `Input`, `PresenceDot`
- `app/chat/page.tsx` — uses `Avatar`, `Badge`, `Button`, `Skeleton`
- `app/chat/[roomId]/page.tsx` — uses `Avatar`, `PresenceDot`, `Skeleton`
- `app/profile/page.tsx` — uses `Button`, `Input`, `Skeleton`
- `app/profile/[userId]/page.tsx` — uses `Avatar`, `Button`, `Modal`, `Skeleton`

## [1.6.0] - 2026-03-26 — Public profiles and photo gallery

### Added
- `profile_photos` table (migration `000010`): up to 6 showcase photos per user, ordered by position, cascades on user deletion
- DB trigger `trg_create_profile_on_register` (migration `000011`): auto-creates an empty profile row when a user registers — no lazy creation needed
- `CategoryGallery` upload category: jpeg/png/webp, max 10 MB; DB constraint updated (migration `000012`)
- `GET /profiles/{userID}` — public profile endpoint (requires auth); returns profile + gallery photos
- `PUT /profiles/me/avatar` — dedicated endpoint to update avatar URL independently from display name/bio
- `POST /profiles/me/photos` — add a gallery photo (max 6 enforced at service layer)
- `DELETE /profiles/me/photos/{photoID}` — remove a gallery photo (ownership verified)
- `Store` interface extended: `GetPhotosByUserID`, `CountPhotos`, `AddPhoto`, `DeletePhoto`, `UpdateAvatar`
- Service methods: `GetPublicProfile`, `AddPhoto`, `DeletePhoto`, `UpdateAvatar`
- `/profile/[userId]` public profile page: avatar, display name, bio, gallery (lightbox), contact action button (4 states: Add / Request sent / Accept / Message)
- Profile navigation entry points: avatar+name in contacts list (all 3 sections) and DM chat header
- Client-side file validation in `useUpload`: size and type checked before the network request with friendly error messages (e.g. "File is too large. Maximum size is 5 MB.")

### Changed
- `/profile` private page redesigned: skeleton loader, bio character counter (max 280), save button disabled when no changes, orange highlight on unsaved fields, auto-save avatar on upload, success messages auto-dismiss after 10 seconds
- Gallery section: always shows all 6 slots (filled + empty), trash icon delete button always visible, inline delete confirmation overlay, error feedback scoped to gallery section
- `PUT /profiles/me` no longer updates avatar — use `PUT /profiles/me/avatar` instead
- Server now mounts all profile routes via `Mount("/profiles", ...)` instead of `Handle("/profiles/me", ...)`

## [1.5.3] - 2026-03-25 — Chat UI/UX improvements

### Added
- Message grouping: consecutive messages from the same sender within a 5-minute window share avatar and sender name; gap between grouped messages is tighter
- Date separators between messages sent on different calendar days (`Today`, `Yesterday`, weekday, or `Day Month`)
- Skeleton loaders in chat room (animated bubbles) and room list (3 shimmer rows) replacing plain "Loading…" text
- New-message entrance animation (`animate-message-in`, 0.18s ease-out) applied to messages received via WebSocket; respects `prefers-reduced-motion`
- `aria-label` on all icon-only buttons (attach, close lightbox, back, ephemeral mode); visually hidden `<label>` for message and search inputs (WCAG 2.1)
- `active:scale-95` press feedback on image, video, and view-once tap targets
- Empty state for room list: icon, description, and "Go to contacts" CTA; retry button on error state
- Relative timestamps on room list rows (`now`, `2m`, `3h`, `yesterday`, weekday, `24 Mar`)
- Attachment type preview in room list uses SVG icons instead of emoji; button padding on contacts page raised to `py-2` (44px touch target)
- `MessageSummary.Type` field so the room list renders the correct attachment preview icon

### Changed
- Font size for timestamps and metadata raised from 10px to 12px
- `@media (prefers-reduced-motion: reduce)` block in `globals.css` disables all animations

## [1.5.2] - 2026-03-24 — Image thumbnails in chat

### Added
- `thumbnail_url` field on `Message` and WebSocket `serverMessage` — populated when the image processing worker has finished
- `chat.NewStore` accepts a `publicURL func(string) string` injected from the storage provider to convert storage keys to URLs
- Image bubbles in the message list display the thumbnail when available, falling back to the full-resolution URL; clicking still opens the lightbox with the full-res URL
- `LEFT JOIN uploads` on `message_id` in `ListMessages` SQL to fetch `thumbnail_key`

## [1.5.0] - 2026-03-24 — Read receipts

### Added
- Read receipts: when a user marks a room as read, a `{"event":"read_receipt","room_id":"…","user_id":"…","read_at":"…"}` WS frame is broadcast to all room members
- `MarkRead` now returns the written `last_read_at` timestamp (via `RETURNING last_read_at`) so the handler can broadcast the exact DB value
- `NotifyRoomRead func(roomID, userID string, readAt time.Time)` callback added to `HandlerConfig` and wired in `main.go`
- `peer_last_read_at` field added to `RoomSummary` / `ListRooms` response — seeds the initial read-receipt state without an extra round-trip
- `readReceipts: Map<userId, timestampMs>` state in `useChat` — updated on every incoming `read_receipt` WS event and exported as `ReadReceipts` type
- Single ✓ (gray) on own messages — confirms delivery to server
- Double ✓✓ (indigo) on own messages when peer's effective read-at ≥ message created_at; effective read-at is `max(peer_last_read_at from API, WS receipt)`
- `PUT /chat/rooms/{id}/read` called automatically whenever a new WS message from another user arrives while the room is open, giving the sender instant ✓✓

## [1.4.0] - 2026-03-24 — Typing indicators

### Added
- Real-time typing indicators: when a user types in a chat room, a `{"event":"typing","user_id":"…","room_id":"…","display_name":"…"}` WS frame is broadcast to all room members
- 2-second server-side debounce per client prevents flooding the hub with typing events
- `GetDisplayName` store method (queries `profiles` table) used by the WS handler to populate `display_name` on the typing frame
- `"X is typing…"` indicator rendered above the input bar; filters out the current user's own events; supports "are typing" for multiple concurrent typers
- Client-side 2-second throttle on the text input `onChange` to limit typing frame frequency
- Auto-clear: typing entries older than 3 s are removed from the map via a 1-second interval in `useChat`
- `sendTyping()` exported from `useChat` hook

## [1.3.2] - 2026-03-23 — Image processing worker

### Added
- `internal/worker` package: `Client` (enqueuer) and `Server` (processor) wrapping asynq; non-blocking `Start`, graceful `Shutdown`
- `image:process` task: decodes JPEG / PNG / WebP / GIF, re-encodes JPEG/PNG (stripping all EXIF metadata including GPS coordinates and device model), generates a 480px JPEG thumbnail stored at `thumbnails/{storage_key}.jpg`
- `thumbnail_key *string` column on `uploads` (migration `000007_add_thumbnail_key`)
- `Store.SetThumbnailKey` method and `Service.SetEnqueuer` — image uploads trigger processing after confirm; enqueue errors are logged but do not fail the HTTP response
- `GetObject` / `PutObject` added to `Storage` interface, `LocalStorage`, and `S3Storage`

## [1.3.1] - 2026-03-23 — S3-compatible storage and Docker setup

### Added
- `S3Storage` implementing the `Storage` interface via `minio-go/v7`; pre-signed PUT URLs expire after 15 minutes — file bytes never pass through the backend
- `minioClient` interface injected for unit test coverage without a running server
- `STORAGE_PROVIDER=s3` wired in `main.go`; parses `S3_ENDPOINT`, auto-sets `UseSSL` from URL scheme; `local` remains the default
- `docker-compose.yml` — dev infra: Postgres 17, Redis 7, MinIO with health checks and named volumes
- `docker-compose.prod.yml` — full stack: infra + `minio-init` (bucket creation, public policy) + backend + frontend, startup order guaranteed by health checks
- `backend/Dockerfile` — multi-stage: compiles in `golang:1.25-alpine`, runs in `alpine:3.21`
- `frontend/Dockerfile` — multi-stage: builds with `node:22-alpine`, `output: standalone`, runs as non-root `nextjs` user (uid 1001)
- `env.production.example` — production variable template with inline documentation
- `NEXT_PUBLIC_IMAGE_HOSTNAME` build arg for `next/image` to allow MinIO/CDN URLs

## [1.3.0] - 2026-03-23 — Chat attachments and ephemeral messages

### Added
- Chat attachment support: send images, videos, and files directly in chat rooms via `sendAttachment` WebSocket frame
- Ephemeral messages: view-once (tap-to-view) and TTL-based (15m / 30m / 1h / 6h / 12h / 24h) modes for all message types (text and attachments)
- Lightbox overlay for viewing images and videos inline; media view-once messages stream binary from the server before deletion to avoid 404 race
- Tombstone messages: instead of hard-deleting view-once or TTL-expired messages, the DB record is converted to a tombstone (`tombstone=true`, content cleared) so chat history shows a persistent placeholder ("View-once message" / "Message expired")
- Migration `000009_add_tombstone_to_messages`: `tombstone boolean NOT NULL DEFAULT FALSE` column on `messages`
- `TombstoneMessage` store method used by both `ViewOnceMessage` (after all viewers have read) and the ephemeral cleaner (TTL expiry)
- TTL countdown badge on received ephemeral messages (color shifts: gray → amber < 1h → red < 10min); client-side 1-minute interval re-renders without a server round-trip
- Differentiated "tap to view" cards for received view-once messages: compact pill for text ("Tap to read"), tall card with type-specific icon for image / video / file
- Sent view-once bubble shows content type label ("Photo · View once", "Video · View once", etc.)
- Ephemeral mode picker label "Applies to messages & attachments" to clarify scope
- `message_deleted` WebSocket event keeps messages in history as tombstones (no longer removes from array)
- `MinIO` (`localhost:9000`) added to `next/image` `remotePatterns` for local development

### Changed
- TTL options updated from `1h / 24h / 7d` to `15m / 30m / 1h / 6h / 12h / 24h`; `TTL7Days` constant removed
- `ListMessages` SQL now includes tombstone records (`OR m.tombstone`) and excludes non-tombstone expired rows
- View-once text cleanup runs synchronously (no goroutine) since there is no streaming race for text content
- `EphemeralStore` interface uses `TombstoneMessage` instead of `DeleteMessage`

## [1.2.0] - 2026-03-15 — Profile avatars and avatar display across the app

### Added
- Profile avatar upload: click-to-upload circle on the profile page using the `useUpload` hook (jpeg/png/webp, ≤ 5 MB)
- `avatar_url` field on `Profile` struct, stored with `NULLIF`/`COALESCE` pattern in Postgres
- Avatar display in the navbar: user's own avatar next to the "Profile" link (fetched from `GET /profiles/me`)
- Avatar display in contacts: all three sections (accepted, pending, sent) and search results show contact avatars with initial-letter fallback
- Presence dot repositioned as a badge on the avatar circle (bottom-right corner) in accepted contacts
- Avatar display in chat list: peer avatar for DM rooms, initial circle for groups
- Avatar display in chat room header: peer avatar next to name and online status
- Avatar display in chat messages: sender avatar beside each received message bubble
- `avatar_url` added to `AcceptedContact`, `PendingRequest`, `SentRequest`, `UserSummary` (contacts package)
- `peer_avatar_url` added to `RoomSummary`, `sender_avatar_url` added to `Message` and WebSocket `serverMessage` (chat package)
- All SQL queries updated to select `COALESCE(p.avatar_url, '')` from the profiles join

### Changed
- `PUT /profiles/me` request/response now includes `avatar_url`
- `updateRequest` and `profileResponse` structs include `avatar_url`
- `Store.Upsert` and `Service.UpdateMyProfile` accept `avatarURL` parameter
- Contacts SQL queries (ListAccepted, ListPending, ListSent, SearchUsers) return avatar_url
- Chat SQL queries (ListRooms, SaveMessage, ListMessages) return avatar_url
- Frontend interfaces updated across all chat and contacts pages

---

## [1.1.0] - 2026-03-15 — Storage interface, LocalStorage, and uploads API

### Added
- `internal/storage` package: `Storage` interface (`GenerateUploadURL`, `PublicURL`, `Delete`); `LocalStorage` implementation (filesystem + in-memory upload tokens with TTL); `NewLocalHandler` for dev file upload/serve (`PUT /put/{token}`, `GET /*`)
- `storage/validation.go`: `ValidateUpload` (category × content-type × size), `SanitizeFilename`, `ParseCategory`; allowed types and size limits per category (`avatar`: jpeg/png/webp ≤ 5 MB; `chat-attachment`: jpeg/png/webp/gif/mp4/mov/pdf ≤ 50 MB)
- `internal/uploads` package: `Upload` domain type, `Store` interface (Postgres CRUD), `Service` (request → confirm lifecycle), HTTP handler
- `POST /uploads/request` — validates category/type/size, creates a `pending` row, returns an upload URL
- `POST /uploads/{id}/confirm` — marks upload as `committed`, returns public URL
- Migration `000006_create_uploads`: `uploads` table with `storage_key` unique index, `status` (pending/committed/failed), cleanup index on pending rows
- `useUpload` frontend hook: 3-step upload flow (request → PUT → confirm)
- `middleware.ContextWithUserID` helper for tests
- Full test coverage: validation (11 cases), sanitization, LocalStorage (generate/consume/delete), local handler (upload/serve/path-traversal), uploads handler (request/confirm/auth/forbidden/not-found)

### Changed
- `server.New()` accepts `uploadHandler` and optional `localStorageHandler`
- `main.go` wires `STORAGE_PROVIDER` env var (default `local`) to select storage backend
- `.gitignore` excludes `backend/data/` (local upload directory)

---

## [1.0.0] - 2026-03-15 — Presence: online/offline and last seen

### Added
- `internal/presence` package: `Store` (Redis + Postgres), `Heartbeat` (SET NX + Expire + update `last_seen_at`), `Offline` (DEL key), `GetPresence` (Redis pipeline EXISTS + Postgres `last_seen_at`), `ContactIDs` (for SSE fan-out)
- `POST /presence/heartbeat` — marks the authenticated user as online; emits `presence_online` SSE to all contacts on first heartbeat (NX)
- `DELETE /presence/heartbeat` — immediately marks the user as offline; emits `presence_offline` SSE to all contacts
- `GET /presence?ids=id1,id2,...` — returns `[{user_id, online, last_seen_at}]` for up to 100 users; online status from Redis, last seen from Postgres
- Migration `000005_add_presence`: `last_seen_at TIMESTAMPTZ` column on `users`
- `presence_online` and `presence_offline` SSE event types added to the `ContactEvent` union
- `useHeartbeat` hook: calls `POST /presence/heartbeat` every 20 seconds; fires immediately on mount and on `visibilitychange` to visible; stops when tab is hidden
- `usePresence` hook: polls `GET /presence?ids=...` every 30 seconds; reacts instantly to `presence_online` / `presence_offline` SSE events via optional `subscribe` param; `formatLastSeen` utility ("just now", "X minutes ago", etc.)
- `SignOutButton` client component: calls `DELETE /presence/heartbeat` before `signOut()` so the user goes offline immediately
- Green presence dot on each contact in the contacts list
- DM chat room header now shows peer display name (fallback to email), presence dot, and "Online" / "Last seen X ago" status

### Changed
- `server.New()` now accepts `presenceHandler http.Handler`
- `NotificationsContext` mounts `useHeartbeat` app-wide — single heartbeat for the entire session
- `contacts/store.go` and `chat/store.go`: `COALESCE(NULLIF(display_name, ''), email)` ensures users with an empty `display_name` always show their email instead of a blank label
- Sign-out button on the home page converted from a server-action form to `<SignOutButton />` (client component)

---

## [0.9.0] - 2026-03-15 — Real-time chat with WebSocket and Redis Pub/Sub

### Added
- `internal/chat` package: domain models (`Room`, `RoomSummary`, `Message`), `Store` interface, `Service` (delegation layer), `Hub` (channel-based WebSocket fan-out), and HTTP handlers
- `Hub`: goroutine-safe broadcast hub; channels (`register`, `unregister`, `broadcast`) are the only mutation path — no mutex on hot path; slow clients are evicted (full send buffer closes connection)
- Redis Pub/Sub integration: `Hub.Publish` writes to `chat:room:{roomID}`; `listenRedis` goroutine per room forwards messages to local clients — enables horizontal scaling across multiple server instances
- `POST /chat/rooms/dm` — get or create a DM room (idempotent via `dm_key` canonical index)
- `POST /chat/rooms` — create a named group room
- `GET /chat/rooms` — list rooms for the authenticated user with last message and unread count
- `GET /chat/rooms/{id}/messages?before=&limit=` — paginated message history (cursor-based, newest-first)
- `PUT /chat/rooms/{id}/read` — mark all messages as read
- `GET /chat/rooms/{id}/ws?token=` — WebSocket endpoint (JWT via query param, same pattern as SSE)
- Migration `000004_create_chat`: `rooms`, `room_members`, `messages`, `message_views` tables; `dm_key` partial unique index; `expires_at` and `view_once` columns on `messages` (groundwork for ephemeral messages)
- Frontend chat list page (`/chat`) with unread badge per room
- Frontend chat room page (`/chat/[roomId]`) with history, live WebSocket messages (dedup by ID), mark-read on enter, connected indicator, and send on Enter
- `useChat` hook: derives WebSocket URL from `API_URL`, auto-reconnect with exponential backoff
- "Message" button on the contacts page — calls `POST /chat/rooms/dm` and redirects to the room
- `REDIS_URL` env var wired in `main.go`; Redis client is pinged at startup

### Changed
- `server.New()` now accepts `chatHandler http.Handler` and `chatWSHandler http.Handler`; WS endpoint is registered outside `requireAuth` (handles its own JWT validation)
- NavBar: "Messages" link added before Contacts

---

## [0.8.0] - 2026-03-15 — Global nav bar with real-time notification badge

### Added
- `NotificationsContext` (`contexts/NotificationsContext.tsx`): manages the single SSE connection for the entire app and acts as an event bus via `subscribe(listener) → unsubscribe` — any component reacts to real-time events without opening its own connection
- `NavBar` component (`components/NavBar.tsx`): persistent navigation bar on all authenticated pages; app name links to `/`; Contacts link shows a live badge with the pending request count (updates in real time, capped at `9+`)
- `refreshPendingCount()` exposed from context so pages can sync the badge after local accept/decline actions

### Changed
- `providers.tsx` now wraps the app with `NotificationsProvider` and renders `<NavBar />`
- Contacts page subscribes to the context event bus instead of calling `useNotifications` directly — eliminates the duplicate SSE connection that existed when on `/contacts`
- Event handler in the contacts page uses `useRef` explicitly (pattern now documented in code) instead of relying on the hook's internal ref

---

## [0.7.0] - 2026-03-14 — Real-time contact removal

### Added
- `contact_removed` SSE event: `deleteHandler` now notifies the other party when a contact is removed or a pending request is declined/cancelled
- Two new handler tests: `TestDelete_NotifiesOtherParty` and `TestDelete_NotifiesRequester_WhenAddresseeDeletes`
- `contact_removed` event type added to `ContactEvent` union in `useNotifications.ts`
- Contacts page now reacts to `contact_removed` in real time (removes the entry from accepted contacts, pending, and sent lists)

### Changed
- `Store.Delete`, `Service.Delete`, and `Manager.Delete` now return `(*Contact, error)` instead of `error`, using a `DELETE … RETURNING` query so the handler knows both participants
- `deleteHandler` now accepts the `handlerConfig` parameter (same pattern as `sendRequestHandler` and `acceptHandler`)
- `useNotifications` hook uses a `useRef` to keep the `onEvent` callback always up-to-date without re-creating the SSE connection — fixes stale-closure bug where `contact_accepted` events were missed after state changed
- Store-layer unit tests for `Delete` now use `rowFn` (mock the `QueryRow` path) instead of `execFn`

---

## [0.6.0] - 2026-03-14 — Real-time notifications via SSE

### Added
- `internal/notifications` package: thread-safe `Hub` and SSE HTTP handler at `GET /notifications/stream`
- `Notifier` interface in the notifications package so other packages can push events without importing the Hub directly
- `contacts.WithNotifier` functional option: injects a `Notifier` into the contacts handler
- `sendRequestHandler` now notifies the addressee with a `contact_request` event
- `acceptHandler` now notifies the requester with a `contact_accepted` event
- `useNotifications` hook in the frontend (`hooks/useNotifications.ts`): opens an SSE connection with exponential backoff reconnect
- Contacts page now reacts to `contact_request` (re-fetches pending list) and `contact_accepted` (moves sent entry to accepted) in real time
- Pending Requests section now shows a count badge

### Changed
- `server.New` accepts a new `notificationsHandler http.Handler` parameter
- SSE endpoint is registered outside the `RequireAuth` middleware group; auth is done via `?token=` query param (browser `EventSource` does not support custom headers)

---

## [0.5.0] - 2026-03-14 — Modern Go 1.22–1.24 refactor

### Changed
- `config.EnvOrDefault` now uses `cmp.Or` (Go 1.22)
- `server.NormalizeCORSOrigins` now uses `strings.SplitSeq` (Go 1.24)
- All test functions use `t.Context()` instead of `context.Background()` (Go 1.24); `context.Background()` is kept only in `t.Cleanup` closures and `pgxpool.New` calls

### Fixed
- `profiles.UpdateMyProfile` now wraps `ErrInvalidInput` with `fmt.Errorf("%w", ...)` so `errors.Is` works correctly in the HTTP handler (previously returned 500 instead of 400 for empty display name)

---

## [0.4.1] - 2026-03-14 — Fix remove contact

### Fixed
- `ListAccepted` now returns `AcceptedContact` (with `contact_id`) instead of `UserSummary` so the frontend passes the correct row ID to `DELETE /contacts/{id}`
- Removed dangling `displayName` helper in the contacts page that caused a `ReferenceError` in the search results section

### Changed
- Added `AcceptedContact` type (`contact_id`, `user_id`, `email`, `display_name`) in the contacts package
- `scanAcceptedContacts` helper added in store layer (mirrors `scanPendingRequests` / `scanSentRequests`)

---

## [0.4.0] - 2026-03-14 — Contacts UX improvements and routing fix

### Fixed
- Auth handler routing: replaced `net/http.ServeMux` with chi router so path stripping works correctly when mounted at `/auth` (was returning 404 on login/register)
- Accept contact: `ListPending` now returns `PendingRequest` (with `contact_id`) so the frontend passes the correct row ID to `PUT /contacts/{id}/accept`

### Added
- `GET /contacts/sent` endpoint — outgoing pending requests with `SentRequest` type so users can see and cancel them
- `PendingRequest` and `SentRequest` types expose `contact_id` for accept/decline/cancel actions
- Search results exclude users who already have any contact relationship with the caller (`NOT EXISTS` subquery)
- Dark theme across all frontend pages (`bg-gray-950` base, `bg-gray-900` cards, white/gray text)
- Sent requests section in the contacts page with a Cancel button

---

## [0.3.0] - 2026-03-14 — Contacts system

### Added
- `contacts` table migration (status machine: `pending → accepted`, `blocked`; unique pair constraint; no self-contact)
- `GET /users/search?q=` — user search by email or display name (ILIKE, 20-row limit, excludes self)
- `POST /contacts` — send a contact request
- `GET /contacts` — list accepted contacts
- `GET /contacts/pending` — list incoming pending requests
- `PUT /contacts/{id}/accept` — accept a pending request
- `DELETE /contacts/{id}` — remove or decline a contact
- Contacts page in the frontend with search, pending requests, and contact list sections

### Changed
- `server.New()` accepts `contactsHandler` and `requireAuth` parameters
- Auth handler routes changed to relative paths (`POST /login`, `POST /register`) and mounted at `/auth`

---

## [0.2.0] - 2026-03-14 — Private profiles and JWT auth

### Added
- JWT HS256 tokens issued on login/register (`golang-jwt/jwt/v5`)
- `RequireAuth` middleware validates Bearer tokens and injects `userID` into context
- `profiles` table migration with `display_name` and `bio`
- `GET /profiles/me` — lazy profile creation on first fetch
- `PUT /profiles/me` — update display name and bio
- Profile page in the frontend
- `SessionProvider` wrapper so `useSession()` works in client components
- `accessToken` exposed through NextAuth session callbacks

---

## [0.1.0] - 2026-03-14 — Auth foundation

### Added
- Project structure: Next.js 15 frontend, Go 1.25+ backend, chi router
- PostgreSQL connection pool with `pgx/v5`, `golang-migrate` for schema migrations
- `users` table with bcrypt password hashing
- `POST /auth/register` — create account with email + password validation
- `POST /auth/login` — authenticate and return user
- Login and register pages (Next.js, NextAuth.js credentials provider)
- `GET /health` endpoint
- CORS configuration via environment variable
- GitHub Actions CI (lint + test on push/PR)
