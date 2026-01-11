# Circl — Web de contactos con chat seguro

Plataforma privada de perfiles y chat en tiempo real. Solo usuarios autenticados pueden ver/buscar otros perfiles y comunicarse.

## Stack
- **Frontend**: Next.js 15+ (App Router) + React 19 + TypeScript + Tailwind CSS 4
- **Backend**: Go 1.25+ (chi router) + WebSockets
- **Auth**: NextAuth.js (Auth.js) v5 con JWT + httpOnly cookies
- **DB**: PostgreSQL 15+ + SQLC/Ent (por implementar)
- **Cache/tiempo real**: Redis 7+ (presencia, Pub/Sub, rate limits)
- **Colas**: asynq (procesamiento de imágenes, tareas)
- **Storage**: S3/R2 + CDN
- **CI/CD**: GitHub Actions

## Documentación
- [Plan de implementación](docs/implementation-plan.md)

## Estado del proyecto
- ✅ **Fundación**: Estructura, linters, CI/CD
- ✅ **Auth base**: NextAuth.js (frontend) + chi router (backend)
- ⏳ **Perfiles privados**: En progreso
- ⏳ **Búsqueda**: Pendiente
- ⏳ **Chat y salas**: Pendiente
- ⏳ **Presencia**: Pendiente
- ⏳ **Media y workers**: Pendiente

## Setup rápido

### Requisitos
- Node.js 20+
- Go 1.25+
- Docker (opcional, para Postgres/Redis locales)
- PostgreSQL 15+
- Redis 7+

### Frontend

```bash
cd frontend
npm install
cp .env.example .env.local
# Edita .env.local si es necesario
npm run dev
```

Abre [http://localhost:3000](http://localhost:3000).

**Credenciales de prueba**: `test@example.com` / `password`

### Backend

```bash
cd backend
cp .env.example .env
# Edita .env si es necesario (DB, Redis, etc.)
go run ./cmd/api
```

API disponible en [http://localhost:8080](http://localhost:8080).

Prueba:
```bash
curl http://localhost:8080/health
```

## Env Variables

Cada carpeta tiene `.env.example`:
- **Frontend**: Copia a `.env.local`
- **Backend**: Copia a `.env`

## Scripts

### Frontend
```bash
cd frontend
npm run dev      # Desarrollo
npm run build    # Build prod
npm run lint     # ESLint
npm run format   # Prettier (próximamente)
npm test         # Tests (próximamente)
```

### Backend
```bash
cd backend
go run ./cmd/api           # Desarrollo
go build ./cmd/api         # Build
go test ./...              # Tests
go vet ./...               # Análisis estático
golangci-lint run          # Linting completo
```

## CI/CD
GitHub Actions ejecuta automáticamente:
- Linting (ESLint para frontend, go vet/golangci-lint para backend)
- Tests (cuando estén listos)
- Chequeos de seguridad (Dependabot)

## Estructura del repositorio

```
circl/
├── frontend/               # Next.js app
│   ├── app/               # App Router pages
│   ├── components/        # React components
│   ├── lib/               # Auth, API client, utilities
│   ├── public/            # Static assets
│   └── package.json
├── backend/               # Go API
│   ├── cmd/api/           # Server entry point
│   ├── internal/          # Business logic
│   │   ├── auth/
│   │   ├── profiles/
│   │   ├── chat/
│   │   └── db/
│   ├── pkg/               # Shared utilities
│   ├── migrations/        # SQL migrations
│   ├── go.mod
│   └── .env
├── docs/                  # Documentation
│   └── implementation-plan.md
├── .github/workflows/     # CI/CD
├── .gitignore
├── .editorconfig
└── README.md
```

## Contribuir
1. Crea una rama: `git checkout -b feature/nombre-feature`
2. Realiza cambios y tests
3. Commit con mensajes descriptivos
4. Push y abre un PR

## Licencia
MIT
