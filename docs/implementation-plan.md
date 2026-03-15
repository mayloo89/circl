# Implementation Plan — Private contact platform with secure chat (Go + Next.js)

## 1. Functional scope
- Private profiles: only authenticated users can view and search other profiles.
- Data: display name, bio, photos, search preferences (avoid exposing sensitive PII).
- Internal search: basic filters, pagination; no public enumeration.
- 1:1 chat and group rooms; persistent message history.
- Online/offline presence with "last seen".
- Photo and file uploads with CDN delivery.

## 2. Stack
- Frontend: Next.js (App Router) + React + TypeScript + Tailwind. Playwright for e2e.
- Backend: Go (chi) + middlewares; WebSockets (gorilla/websocket).
- Auth: NextAuth.js (Auth.js) v5 with credentials provider; httpOnly cookies; JWT HS256.
- DB: PostgreSQL + golang-migrate for schema migrations; pgx/v5 connection pool.
- Cache / real-time: Redis (presence, Pub/Sub for chat fan-out, rate limits, token blacklist).
- Queues / worker: asynq for image processing and maintenance tasks.
- Storage: S3/R2 + pre-signed URLs; image processing (bimg/imagor) in worker.
- Infra: Frontend on Vercel; backend on Fly.io/Render/AWS; Postgres (Neon/RDS), Redis (Upstash/ElastiCache).
- Quality: ESLint/Prettier, golangci-lint, Go tests, CI via GitHub Actions.
- Observability: structured logs (zerolog/zap), metrics/tracing (OpenTelemetry/Prometheus), error tracking (Sentry).

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

- [x] **Foundation**: repo, CI/CD (GitHub Actions), linters, dev/prod environments, secrets.
- [x] **Auth**: registration and login with bcrypt; JWT HS256; `RequireAuth` middleware; NextAuth.js credentials provider; httpOnly cookies.
- [x] **Private profiles**: `GET/PUT /profiles/me`; lazy profile creation; profile page in frontend.
- [x] **Contacts**: user search; send/accept/decline/cancel requests; `DELETE /contacts/{id}`; `pending → accepted` state machine.
- [x] **Real-time notifications (SSE)**: `notifications.Hub`; `GET /notifications/stream?token=`; `contact_request`, `contact_accepted`, `contact_removed` events; NavBar badge; `NotificationsContext` (single SSE connection per session, event bus for all subscribers).
- [x] **Chat and rooms**: WebSocket (`GET /chat/rooms/{id}/ws?token=`); Redis Pub/Sub fan-out; DMs and group rooms; Postgres persistence; paginated history; unread counts; NavBar badge via `new_message` SSE event; ephemeral message schema (`expires_at`, `view_once`).
- [x] **Presence**: Redis heartbeat with TTL; `POST /presence/heartbeat` (20s interval, tab-visibility-aware); `DELETE /presence/heartbeat` (immediate offline on logout); `GET /presence?ids=` batch query; `presence_online` / `presence_offline` SSE fan-out to contacts; green dot on contacts list; peer name + online status in DM chat header; `last_seen_at` persisted in Postgres; `formatLastSeen` utility.
- [x] **Media — storage infra**: `Storage` interface (LocalStorage for dev, S3Storage for prod); `uploads` table; `POST /uploads/request` + `POST /uploads/{id}/confirm` lifecycle; file validation (type, size); `useUpload` frontend hook.
- [ ] **Media — avatar upload**: profile avatar UI; `PUT /profiles/me` with `avatar_url`; avatar display in contacts and chat.
- [ ] **Media — chat attachments**: attachment button in chat; image/file/video messages; render in message list.
- [ ] **Media — S3 provider**: `S3Storage` implementation (aws-sdk-go-v2); pre-signed URLs; CDN delivery.
- [ ] **Media — image processing**: asynq worker; resize/thumbnails; strip EXIF metadata.
- [ ] **Ephemeral messages**: TTL cleanup worker (`expires_at`); view-once logic (`view_once` + `message_views`); automatic deletion.
- [ ] **Typing indicators and read receipts**: `typing` and `read` events over WebSocket.
- [ ] **QA / hardening**: e2e tests (Playwright); SAST/Dependabot; CSP/HSTS/CORS review; rate limiting.
- [ ] **Observability and deployment**: structured logs, metrics, tracing (OpenTelemetry); deploy to Vercel + Fly.io/Render; managed DB and Redis.

## 9. Testing strategy
- Unit: handlers and services (auth, chat, profiles, contacts).
- Integration: real Postgres and Redis (skipped when env vars not set).
- E2E: Playwright (login, profiles, search, chat, presence flows).
- Security: rate limiting, headers (CSP/HSTS), payload size limits.

## 10. Operations
- SLIs: HTTP/WS latency, message delivery rate, 5xx error rate, heartbeat expiry.
- Alerts: WS drops, queue backlogs, worker errors, disk space, DB connections.
- Encrypted and tested backups; periodic key rotation.
