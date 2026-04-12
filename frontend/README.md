# Frontend — Circl

Next.js 15 (App Router), React 19, TypeScript y Tailwind CSS 4.

## Requisitos

- Node.js 20+
- npm

## Setup rápido

```bash
npm install
cp .env.example .env.local
npm run dev
```

App disponible en `http://localhost:3000`.

## Scripts

| Comando | Descripción |
|---------|-------------|
| `npm run dev` | Servidor de desarrollo |
| `npm run build` | Build de producción |
| `npm run start` | Servidor de producción |
| `npm run lint` | ESLint |
| `npm test` | Vitest (unit tests) |
| `npx playwright test` | Tests E2E |

## Estructura

```
app/
  (auth)/                 # Login, registro, forgot/reset password, verify email
  admin/                  # Panel de administración (layout guard por rol)
    page.tsx              # Dashboard con métricas
    users/                # Gestión de usuarios (suspend, ban, hard delete, roles)
    reports/              # Revisión de reportes
    channels/             # Gestión de canales públicos
  browse/                 # Explorar perfiles cercanos
  chat/
    page.tsx              # Lista de salas
    [roomId]/             # Sala de chat
    channels/             # Canales públicos
  contacts/               # Lista y solicitudes de contactos
  profile/
    edit/                 # Edición de perfil propio
    [username]/           # Perfil público
  settings/               # Push notifications, cambio de contraseña, eliminar cuenta

components/
  ui/                     # Avatar, Badge, Button, Input, Skeleton, Modal, Toast, PresenceDot
  chat/                   # MessageBubble, ChatInput, Lightbox, TypingIndicator, GroupMembersPanel
  contacts/               # ContactCard, SearchBar
  profile/                # ProfileHeader, PhotoGallery
  NavBar.tsx

hooks/                    # useUpload, useHeartbeat, usePresence, usePush
lib/
  auth.ts                 # NextAuth config con role string en JWT/session
  chatHelpers.ts
  validation.ts           # Zod schemas
types/
  next-auth.d.ts          # Session: { accessToken, role, reactivated }
  chat.ts
```

## Variables de entorno

| Variable | Descripción |
|----------|-------------|
| `NEXT_PUBLIC_API_URL` | URL del backend (default: `http://localhost:8080`) |
| `NEXT_PUBLIC_WS_URL` | URL WebSocket (default: `ws://localhost:8080`) |
| `NEXTAUTH_URL` | URL de la app (default: `http://localhost:3000`) |
| `NEXTAUTH_SECRET` | Secret para sesiones (`openssl rand -base64 32`) |
| `BACKEND_URL` | URL del backend desde el servidor Next.js (default: `http://localhost:8080`) |

## Auth y roles

NextAuth.js v5 con proveedor de credenciales. El JWT del backend incluye un claim `role` (`user` / `admin` / `super_admin`) que se propaga a `session.role`.

- **Rutas admin** (`/admin/*`): guard en `app/admin/layout.tsx` — redirige a `/` si `role` no es `admin` ni `super_admin`
- **Super admin**: botones de hard delete y cambio de rol sólo visibles cuando `role === "super_admin"`

## Testing

- **Unit**: Vitest + @testing-library/react + msw; 133 tests en `components/ui`, hooks y lib
- **E2E**: Playwright; fixtures con creación de usuario vía API; flujos de auth, perfil, contactos y chat
