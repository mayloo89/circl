# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api ./cmd/api

# Runtime stage — minimal image with just the binary and migrations
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

# Non-root user for reduced attack surface
RUN addgroup -S circl && adduser -S -G circl circl

WORKDIR /app

COPY --from=builder --chown=circl:circl /bin/api ./api
COPY --from=builder --chown=circl:circl /app/migrations ./migrations

USER circl

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./api"]
