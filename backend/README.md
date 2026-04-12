# Backend — Circl

Go 1.25 API con chi router, WebSockets, Redis y PostgreSQL.

## Requisitos

- Go 1.25+
- PostgreSQL 15+
- Redis 7+

## Setup rápido

```bash
go mod download
cp .env.example .env
go run ./cmd/api
```

API disponible en `http://localhost:8080`. Health check:

```bash
curl http://localhost:8080/health
```

## Scripts

| Comando | Descripción |
|---------|-------------|
| `go run ./cmd/api` | Servidor de desarrollo |
| `go build -o bin/api ./cmd/api` | Build de producción |
| `go test ./...` | Todos los tests unitarios |
| `go test ./... -tags integration` | Tests de integración (requiere DB + Redis) |
| `go vet ./...` | Análisis estático |
| `golangci-lint run` | Linting completo |

## Estructura

```
cmd/api/
  main.go                # Entry point: router, middlewares, server

internal/
  admin/                 # Admin panel: usuarios, reportes, canales
  auth/                  # Registro, login, verificación de email, reset de contraseña
  chat/                  # WebSocket, mensajes, salas, canales públicos
  contacts/              # Solicitudes de contacto, búsqueda
  email/                 # Sender interface, ConsoleSender, SMTPSender
  middleware/            # RequireAuth, RequireAdmin, RequireSuperAdmin, CORS, rate limit
  notifications/         # SSE + push notifications (VAPID)
  presence/              # Estado online/offline vía Redis
  profiles/              # Perfiles, preferencias, browse, fotos, intereses
  reports/               # Reportes de usuarios
  server/                # Configuración del router HTTP (chi)
  storage/               # Interfaz Storage → LocalStorage / S3Storage
  testutil/              # OpenDB, NewRedis, CreateUser para tests de integración
  token/                 # JWT generate/verify; constantes RoleUser/RoleAdmin/RoleSuperAdmin
  uploads/               # Registros de upload, commit, thumbnails
  worker/                # Asynq workers: procesamiento de imágenes, purge de cuentas eliminadas

migrations/              # SQL migrations (000001–000024)
```

## Variables de entorno

| Variable | Descripción | Default |
|----------|-------------|---------|
| `PORT` | Puerto del servidor | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | — |
| `REDIS_URL` | Redis URL | `redis://localhost:6379` |
| `JWT_SECRET` | Secreto para firmar JWTs | — |
| `CORS_ALLOWED_ORIGINS` | Orígenes permitidos (comma-separated) | — |
| `EMAIL_SENDER` | `console` o `smtp` | `console` |
| `SMTP_HOST/PORT/USER/PASS` | Credenciales SMTP | — |
| `S3_ENDPOINT/BUCKET/KEY/SECRET` | Storage S3-compatible | — |
| `IMAGE_MAX_PX` | Tamaño máximo de imagen original (px) | `1024` |
| `TEST_ENDPOINTS_ENABLED` | Habilita endpoints de test E2E | `false` |

## Roles de usuario

El sistema usa un campo `role` string en lugar de booleanos:

| Rol | Acceso |
|-----|--------|
| `user` | Acceso estándar |
| `admin` | Panel admin: gestión de usuarios, reportes, canales |
| `super_admin` | Todo lo anterior + hard delete de cuentas + cambio de roles |

## Endpoints principales

### Auth
- `POST /auth/register` — registro
- `POST /auth/login` — login (devuelve JWT con `role` en el claim)
- `POST /auth/forgot-password`, `POST /auth/reset-password`
- `POST /auth/verify-email`, `GET /auth/verify-email?token=`

### Perfiles
- `GET/PUT /profiles/me`, `PUT /profiles/me/avatar`, `GET/PUT /profiles/me/preferences`
- `GET /profiles/{ref}` — acepta UUID o `@username`
- `GET /profiles/available?username=`
- `GET /profiles/browse` — filtros por distancia, edad, género, intereses

### Chat
- `GET /chat/rooms`, `POST /chat/rooms`, `GET /chat/rooms/{id}/messages`
- `WS /ws?token=` — WebSocket autenticado
- `GET /sse` — Server-Sent Events para notificaciones y presencia

### Admin (RequireAdmin)
- `GET /admin/users`, `PUT /admin/users/{id}/status`
- `GET /admin/reports`, `PUT /admin/reports/{id}`
- `GET/POST /admin/channels`, `PUT/DELETE /admin/channels/{id}`

### Admin (RequireSuperAdmin)
- `DELETE /admin/users/{id}` — hard delete inmediato
- `PUT /admin/users/{id}/role` — cambio de rol

## Testing

- **Unitarios**: mocks manuales; target 98%+ en handlers y servicios
- **Integración**: testutil (OpenDB + NewRedis); require `TEST_DATABASE_URL` y `TEST_REDIS_URL`
- **Store**: cubiertos sólo por tests de integración (0% en runs unitarios es aceptable)
