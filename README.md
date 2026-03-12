# Circl — Private contact platform with secure chat

Private profiles and real-time chat. Only authenticated users can view, search, and message other users.

## Stack
- **Frontend**: Next.js 15+ (App Router) + React 19 + TypeScript + Tailwind CSS 4
- **Backend**: Go 1.25+ (chi router) + WebSockets
- **Auth**: NextAuth.js (Auth.js) v5 — JWT + httpOnly cookies
- **DB**: PostgreSQL 17+, migrations via golang-migrate
- **Cache / real-time**: Redis 7+ (presence, Pub/Sub, rate limits)
- **Queues**: asynq (image processing, maintenance tasks)
- **Storage**: S3/R2 + CDN
- **CI/CD**: GitHub Actions

## Documentation
- [Implementation plan](docs/implementation-plan.md)

## Project status
- ✅ **Foundation**: repo structure, linters, CI/CD
- ✅ **Auth base**: NextAuth.js (frontend) + chi router (backend)
- ✅ **Database foundation**: PostgreSQL + golang-migrate, real bcrypt auth
- ⏳ **Private profiles**: in progress
- ⏳ **Search**: pending
- ⏳ **Chat and rooms**: pending
- ⏳ **Presence**: pending
- ⏳ **Media and workers**: pending

## Local setup

### Requirements
- Node.js 20+
- Go 1.25+
- PostgreSQL 17+ (e.g. [Postgres.app](https://postgresapp.com) on macOS)

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

## Repository structure

```
circl/
├── frontend/               # Next.js app
│   ├── app/               # App Router pages
│   ├── lib/               # Auth config, utilities
│   └── package.json
├── backend/               # Go API
│   ├── cmd/api/           # Server entry point (main.go)
│   ├── internal/          # Business logic
│   │   ├── auth/          # Login handler, service, store
│   │   ├── config/        # Env helpers
│   │   ├── db/            # Connection pool, migrations
│   │   └── server/        # Router, health handler
│   ├── migrations/        # SQL migrations (up + down)
│   └── go.mod
├── docs/
│   └── implementation-plan.md
├── .github/workflows/     # CI/CD
└── README.md
```

## Contributing
1. Branch off `develop`: `git checkout -b feature/your-feature`
2. Make changes and write tests (target: 98%+ coverage)
3. Commit with descriptive messages in English
4. Push and open a PR targeting `develop`

## License
MIT
