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
- **CI/CD**: GitHub Actions

## Documentation
- [Implementation plan](docs/implementation-plan.md)
- [Production readiness checklist](docs/production-readiness.md)

## Project status
- ✅ **Foundation**: repo structure, linters, CI/CD
- ✅ **Auth**: registration, login, JWT tokens, NextAuth.js session
- ✅ **Private profiles**: display name, bio — `GET /profiles/me`, `PUT /profiles/me`
- ✅ **Contacts**: search, send/accept/decline/remove requests — full contacts lifecycle
- ✅ **Real-time notifications**: SSE (`GET /notifications/stream`), global nav badge, contact request/accepted/removed events
- ✅ **Chat and rooms**: WebSocket DMs and group rooms, Redis Pub/Sub fan-out, message history, unread counts
- ✅ **Presence**: online/offline dot on contacts list, "Online" / "Last seen X ago" in DM chat header, instant updates via SSE
- ✅ **Storage infrastructure**: Storage interface abstraction, LocalStorage (dev), uploads API (request → confirm lifecycle)
- ✅ **Profile avatars**: upload from profile page, displayed in navbar, contacts list, chat list, chat room header, and message bubbles
- ✅ **Chat attachments**: images, videos, and files in chat; ephemeral (view-once + TTL) messages
- ✅ **S3-compatible storage**: MinIO backend with pre-signed PUT URLs; Docker Compose dev and prod setup
- ✅ **Image processing**: asynq background worker — EXIF strip and 480px thumbnail generation for JPEG/PNG uploads
- ✅ **Typing indicators**: real-time "X is typing…" via WebSocket with 2s server-side debounce
- ✅ **Read receipts**: ✓ / ✓✓ on sent messages; updates in real time via WebSocket
- ✅ **Image thumbnails in chat**: thumbnails served from storage instead of full-res URLs in message list
- ✅ **Chat UI**: message grouping, date separators, skeleton loaders, new-message animation, relative timestamps, attachment type previews
- ✅ **Public profiles + gallery**: public profile view, photo gallery (up to 6 photos), profile navigation from contacts and chat header
- ✅ **Usernames**: unique handles (`[a-z0-9_]`, 3–30 chars), immutable once set, used in all profile URLs (`/profile/[username]`)
- ✅ **Extended profiles**: date of birth (18+ enforced), gender, location (autocomplete via Photon/OSM), interests tags
- ✅ **Registration with profile seeding**: username + DOB collected at signup, profile seeded immediately after account creation
- ✅ **User blocking**: block/unblock users, bidirectional suppression in browse/search/contacts/chat, WebSocket message filtering, performance-optimized batch queries
- ✅ **User reporting**: report users with reason, rate limited (10/hour), auto-suspend after 3+ reports in 7 days
- ✅ **Admin moderation**: admin role, user suspension/activation, report management (resolve/dismiss with notes)
- ✅ **Account safety**: login lockout (5 failed attempts = 15 min lockout), password complexity (8+ chars, upper/lower/number/special), rate limiting (configurable per IP)

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
# → {"status":"ok","env":"development","db":"ok"}
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
npm run dev      # Development server
npm run build    # Production build
npm run lint     # ESLint
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

All protected routes require `Authorization: Bearer <token>`.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/health` | — | Server and DB status |
| `POST` | `/auth/register` | — | Create account |
| `POST` | `/auth/login` | — | Login, returns JWT |
| `GET` | `/profiles/me` | ✅ | Get own profile (auto-created) |
| `PUT` | `/profiles/me` | ✅ | Update display name and bio |
| `PUT` | `/profiles/me/avatar` | ✅ | Update avatar URL independently |
| `GET` | `/profiles/{ref}` | ✅ | Get any user's public profile + gallery (ref = UUID or username) |
| `GET` | `/profiles/available?username=` | ✅ | Check username availability |
| `POST` | `/profiles/me/photos` | ✅ | Add a gallery photo (max 6) |
| `DELETE` | `/profiles/me/photos/{id}` | ✅ | Delete a gallery photo |
| `GET` | `/users/search?q=` | ✅ | Search users by email, display name, or username |
| `GET` | `/profiles/interests?q=` | ✅ | Autocomplete interests from existing tags |
| `POST` | `/contacts` | ✅ | Send a contact request |
| `GET` | `/contacts` | ✅ | List accepted contacts |
| `GET` | `/contacts/pending` | ✅ | List incoming pending requests |
| `GET` | `/contacts/sent` | ✅ | List outgoing pending requests |
| `PUT` | `/contacts/{id}/accept` | ✅ | Accept a pending request |
| `DELETE` | `/contacts/{id}` | ✅ | Remove or decline a contact |
| `POST` | `/contacts/{id}/block` | ✅ | Block a user (removes contact if exists) |
| `DELETE` | `/contacts/{id}/block` | ✅ | Unblock a user |
| `GET` | `/contacts/blocked` | ✅ | List blocked users |
| `GET` | `/notifications/stream?token=` | — | SSE stream for real-time events |
| `POST` | `/chat/rooms/dm` | ✅ | Get or create a DM room |
| `POST` | `/chat/rooms` | ✅ | Create a named group room |
| `GET` | `/chat/rooms` | ✅ | List rooms with last message and unread count |
| `GET` | `/chat/rooms/{id}/messages` | ✅ | Paginated message history |
| `PUT` | `/chat/rooms/{id}/read` | ✅ | Mark room as read |
| `GET` | `/chat/rooms/{id}/ws?token=` | — | WebSocket connection for real-time chat |
| `POST` | `/presence/heartbeat` | ✅ | Mark self as online (send every ~20s) |
| `DELETE` | `/presence/heartbeat` | ✅ | Mark self as offline immediately (on logout) |
| `GET` | `/presence?ids=` | ✅ | Batch presence query (online + last seen) |
| `POST` | `/uploads/request` | ✅ | Request an upload URL (validates type/size) |
| `POST` | `/uploads/{id}/confirm` | ✅ | Confirm upload completed |
| `POST` | `/push/subscribe` | ✅ | Subscribe to web push notifications |
| `DELETE` | `/push/unsubscribe` | ✅ | Unsubscribe from push notifications |
| `GET` | `/push/vapid-public-key` | ✅ | Get VAPID public key for subscription |

## Repository structure

```
circl/
├── frontend/               # Next.js app
│   ├── app/               # App Router pages and layouts
│   │   ├── chat/          # Chat list and room pages
│   │   ├── contacts/      # Contacts page
│   │   ├── login/         # Login page
│   │   ├── profile/       # Private profile page (edit)
│   │   │   └── [username]/  # Public profile page (read-only)
│   │   └── register/      # Register page
│   ├── components/        # Shared UI components (NavBar, SignOutButton)
│   ├── contexts/          # React contexts (NotificationsContext / SSE event bus)
│   ├── hooks/             # Custom hooks (useNotifications, useChat, useHeartbeat, usePresence, useUpload)
│   ├── lib/               # Auth config (NextAuth.js)
│   ├── types/             # next-auth type augmentation
│   └── package.json
├── backend/               # Go API
│   ├── cmd/api/           # Server entry point (main.go)
│   ├── internal/          # Business logic (clean architecture)
│   │   ├── auth/          # Register/login handler, service, store
│   │   ├── contacts/      # Contacts handler, service, store
│   │   ├── config/        # Env helpers
│   │   ├── db/            # Connection pool, migrations runner
│   │   ├── middleware/    # JWT RequireAuth middleware
│   │   ├── chat/          # Chat rooms, Hub (WebSocket fan-out), store, handler
│   │   ├── notifications/ # SSE Hub, Notifier interface, stream handler
│   │   ├── presence/      # Redis heartbeat, offline, batch presence query
│   │   ├── storage/       # Storage interface, LocalStorage, file validation
│   │   ├── uploads/       # Upload lifecycle (request → confirm), Postgres tracking
│   │   ├── profiles/      # Profile handler, service, store
│   │   ├── server/        # Chi router, CORS, health handler
│   │   └── token/         # JWT generate/validate
│   ├── migrations/        # SQL migrations (up + down)
│   └── go.mod
├── docs/
│   └── implementation-plan.md
├── .github/workflows/     # CI/CD
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
