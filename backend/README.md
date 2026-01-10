# Backend — Circl

Go 1.22+ con chi/echo, WebSockets y PostgreSQL.

## Setup

```bash
go mod download
go run ./cmd/api
```

API en [http://localhost:8080](http://localhost:8080).

## Scripts
- `go run ./cmd/api`: inicia servidor
- `go test ./...`: ejecuta tests
- `go vet ./...`: análisis estático
- `golangci-lint run`: linting completo

## Estructura
```
/cmd/api          # main y setup HTTP
/internal
  /auth           # middlewares y servicios
  /profiles       # perfiles
  /chat           # chat y presencia
  /db             # repositorios
/pkg              # utilidades compartidas
/migrations       # SQL
```

## Variables de entorno
Ver `.env.example`.
