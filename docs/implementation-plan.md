# Guía de implementación — Web de contactos con chat seguro (Go + Next.js)

## 1. Alcance funcional
- Perfiles privados: solo usuarios autenticados pueden ver/buscar otros perfiles.
- Datos: nombre, bio, fotos, preferencias de búsqueda (evitar exponer PII sensible).
- Búsqueda interna: filtros básicos, paginación; sin enumeración pública.
- Chat 1:1 y salas generales; historial persistente.
- Presencia online/offline con "última vez visto".
- Subida y entrega de fotos con CDN.

## 2. Stack recomendado
- Frontend: Next.js (App Router) + React + TypeScript + Tailwind. Playwright para e2e.
- Backend: Go (chi/echo) + middlewares; WebSockets (nhooyr/gorilla).
- Auth: OIDC/Auth.js o proveedor (Auth0/Clerk/Cognito). Cookies httpOnly + refresh rotado.
- DB: PostgreSQL + SQLC/Ent; migraciones con golang-migrate.
- Cache/tiempo real: Redis (presencia, Pub/Sub chat, rate limits, blacklist de tokens).
- Colas/worker: asynq para imágenes y tareas de mantenimiento.
- Storage: S3/R2 + URLs firmadas; procesamiento de imágenes (bimg/imagor) en worker.
- Infra: Frontend en Vercel; backend en Fly.io/Render/AWS; Postgres (Neon/RDS), Redis (Upstash/ElastiCache).
- Calidad: ESLint/Prettier, golangci-lint, Jest/Vitest (frontend), tests Go + Testcontainers, CI GitHub Actions.
- Observabilidad: Logs estructurados (zerolog/zap), métricas/tracing (OpenTelemetry/Prometheus), errores (Sentry).

## 3. Arquitectura lógica
- Front: rutas protegidas por middleware; CSR para chat; SSR solo con sesión válida.
- BFF/API:
  - Auth: login/logout, refresh rotado, revocación (Redis blacklist).
  - Profiles: CRUD privado, paginación, sin exponer contactos directos sin consentimiento.
  - Search: requiere sesión; índices en Postgres; respuestas genéricas para evitar enumeración.
  - Chat: WebSockets autenticados; mensajes 1:1 y salas; persistencia en Postgres; fanout vía Redis Pub/Sub.
  - Presencia: heartbeats a Redis con TTL; cálculo de online/offline/last seen.
  - Media: firma de uploads a S3; hooks de post-proceso; limpieza de huérfanos.
- Worker: procesamiento de imágenes, expiración de sesiones revocadas, limpieza de presencia.

## 4. Seguridad
- TLS extremo a extremo; HSTS; CSP estricta (sin unsafe-inline), deshabilitar eval.
- Cookies httpOnly, Secure, SameSite=Lax/Strict; rotación de refresh tokens.
- CSRF (si usas cookies); CORS restringido; rate limiting por IP y user en login/búsqueda/chat.
- Validación y saneamiento en servidor; límites de payload y tamaño de archivo.
- Protección contra enumeración: respuestas genéricas en login/reset; búsqueda solo autenticada.
- Logs sin PII; IP hasheada si se requiere; cifrado de backups; secretos en vault/KMS.
- Roles mínimos: usuario/admin; chequear ownership en recursos (perfiles, chats).

## 5. Modelo de datos (base)
- users: id, email (único), hash de pass si aplica, estado, created_at, last_login_at/ip_hash.
- profiles: user_id (FK), display_name, bio, campos buscables, foto_principal_url, settings de privacidad.
- messages: id, chat_id, sender_id, body, media_url?, created_at, delivered_at, read_at.
- chats: id, tipo (direct|room), participantes (tabla pivot para directos), metadata.
- rooms: id, nombre, descripción, visibilidad (pública interna), creador.
- Índices: búsqueda en profiles, messages(chat_id, created_at) para paginación; constraints FK.

## 6. Flujo de tiempo real
- Handshake WS con token de sesión (o cookie + sesión validada).
- Suscripción a: chats del usuario, rooms públicas, canal de presencia.
- Presencia: heartbeat cada N segundos → set en Redis con TTL; caída del TTL marca offline.
- Fanout: API publica mensaje → guarda en Postgres → emite a Redis Pub/Sub → reenvía por WS a suscriptores.

## 7. Manejo de media
- Front solicita URL firmada → sube a S3 directamente.
- Worker procesa (resize, strip metadata) → guarda variantes → actualiza URL en DB.
- Entrega vía CDN; políticas anti-hotlink.

## 8. Plan por fases

- [x] **Fundación**: repo, CI/CD (GitHub Actions), linters, entornos dev/prod, secretos.
- [x] **Auth base**: registro y login con bcrypt; JWT HS256; `RequireAuth` middleware; NextAuth.js con credentials provider; cookies httpOnly.
- [x] **Perfiles privados**: `GET/PUT /profiles/me`; lazy creation; página de perfil en frontend.
- [x] **Contactos**: búsqueda de usuarios; envío/aceptación/rechazo/cancelación de solicitudes; `DELETE /contacts/{id}`; estado machine `pending → accepted`.
- [x] **Notificaciones en tiempo real (SSE)**: `notifications.Hub`; `GET /notifications/stream?token=`; eventos `contact_request`, `contact_accepted`, `contact_removed`; badge en NavBar; `NotificationsContext` (bus de eventos, una sola conexión SSE por sesión).
- [x] **Chat y salas**: WebSocket (`GET /chat/rooms/{id}/ws?token=`); Redis Pub/Sub fan-out; DMs y grupos; persistencia en Postgres; historial paginado; conteo de no leídos; badge en NavBar vía evento SSE `new_message`; mensajes efímeros en schema (`expires_at`, `view_once`).
- [ ] **Presencia**: heartbeat a Redis con TTL; online/offline/last seen en UI.
- [ ] **Media**: URLs firmadas para upload directo a S3/R2; worker asynq (resize, strip metadata); entrega vía CDN; mensajes con foto/archivo en chat.
- [ ] **Mensajes efímeros**: TTL worker (`expires_at`); view-once (`view_once` + `message_views`); limpieza automática.
- [ ] **Indicadores de escritura y receipts**: eventos `typing` y `read` sobre WebSocket.
- [ ] **QA/hardening**: tests e2e (Playwright); SAST/Dependabot; revisión CSP/HSTS/CORS; rate limiting.
- [ ] **Observabilidad y despliegue**: logs estructurados, métricas, tracing (OpenTelemetry); despliegue en Vercel + Fly.io/Render; DB y Redis gestionados.

## 9. Testing
- Unit: handlers y servicios (auth, chat, profiles).
- Integración: DB y Redis (Testcontainers).
- E2E: Playwright (flujos de login, perfiles, búsqueda, chat y presencia).
- Seguridad: rate limit, CSRF (si cookies), headers (CSP/HSTS), tamaño de payload.

## 10. Operación
- SLIs: latencia HTTP/WS, tasa de entrega de mensajes, errores 5xx, expiración de heartbeats.
- Alertas: caída de WS, colas atrasadas, errores de worker, espacio en disco, conexiones DB.
- Backups cifrados y probados; rotación de claves periódica.
