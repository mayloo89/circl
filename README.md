# Circl — Web de contactos con chat seguro

Plataforma privada de perfiles y chat en tiempo real. Solo usuarios autenticados pueden ver/buscar otros perfiles y comunicarse.

## Stack
- **Frontend**: Next.js (App Router) + React + TypeScript + Tailwind
- **Backend**: Go (chi/echo) + WebSockets
- **Auth**: OIDC/Auth.js con cookies httpOnly + refresh rotado
- **DB**: PostgreSQL + SQLC/Ent
- **Cache/tiempo real**: Redis (presencia, Pub/Sub, rate limits)
- **Colas**: asynq (procesamiento de imágenes, tareas)
- **Storage**: S3/R2 + CDN

## Documentación
- [Plan de implementación](docs/implementation-plan.md)

## Setup rápido

### Requisitos
- Node.js 20+
- Go 1.22+
- Docker (opcional, para Postgres/Redis locales)
- PostgreSQL 15+ y Redis 7+

### Frontend
```bash
cd frontend
npm install
npm run dev
```

### Backend
```bash
cd backend
go mod download
go run ./cmd/api
```

## Variables de entorno
Ver `.env.example` en cada carpeta.

## CI/CD
GitHub Actions: linting, testing y chequeos de seguridad en cada push/PR.

## Licencia
MIT
