# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

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
