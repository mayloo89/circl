# Backend — Circl

Go 1.25+ con chi router, WebSockets y PostgreSQL.

## Requisitos
- Go 1.25+
- PostgreSQL 15+
- Redis 7+ (para presencia y Pub/Sub)

## Setup rápido

```bash
go mod download
go mod tidy
cp .env.example .env
go run ./cmd/api
```

API estará disponible en `http://localhost:8080`.

Prueba el health check:
```bash
curl http://localhost:8080/health
```

## Scripts

- `go run ./cmd/api`: Inicia servidor de desarrollo
- `go build ./cmd/api -o bin/api`: Build para producción
- `go test ./...`: Ejecuta todos los tests
- `go vet ./...`: Análisis estático
- `golangci-lint run`: Linting completo (requiere instalación)

## Estructura

```
cmd/
  api/
    main.go              # Servidor HTTP, router, middlewares

internal/
  auth/                  # Autenticación y JWT
  profiles/              # Perfiles de usuarios
  chat/                  # Mensajes y salas
  db/                    # Repositorios y queries

pkg/                     # Utilidades compartidas

migrations/              # SQL migrations

.env                     # Variables de entorno (no versionado)
.env.example             # Plantilla de variables
go.mod / go.sum          # Dependencias
```

## Env Variables

Copia `.env.example` a `.env` y edita según necesites. Las más importantes:

- `PORT`: Puerto del servidor (default: 8080)
- `ENV`: Entorno (development/staging/production)
- `DB_*`: Credenciales PostgreSQL
- `REDIS_URL`: URL de Redis
- `JWT_SECRET`: Secreto para firmar JWTs
- `CORS_ALLOWED_ORIGINS`: Orígenes permitidos (comma-separated)

## Dependencias principales

- **chi/v5**: HTTP router ligero y eficiente
- **cors**: Middleware CORS
- **godotenv**: Carga variables de entorno desde `.env`
- **uuid**: Generación de UUIDs

## Next Steps

- [ ] Typing indicators — broadcast typing events over WebSocket
- [ ] Read receipts — per-user per-message delivery and read acknowledgment
- [ ] Frontend thumbnails — display `thumbnail_url` in chat message list
- [ ] OpenAPI / Swagger documentation
- [ ] Structured logging (slog)

## Troubleshooting

| Problema | Solución |
|----------|----------|
| Puerto 8080 en uso | Cambiar en `.env`: `PORT=9000 go run ./cmd/api` |
| Go version < 1.25 | Verificar: `go version` |
| Dependencias desactualizadas | Actualizar: `go get -u ./...` y `go mod tidy` |
