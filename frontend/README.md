# Frontend — Circl

Next.js 15+ con App Router, React 19, TypeScript y Tailwind CSS 4.

## Requisitos
- Node.js 20+
- npm o yarn

## Setup rápido

```bash
npm install
cp .env.example .env.local
npm run dev
```

Abre [http://localhost:3000](http://localhost:3000).

**Credenciales de prueba**: `test@example.com` / `password`

## Scripts

- `npm run dev`: Servidor de desarrollo
- `npm run build`: Build para producción
- `npm run start`: Inicia servidor prod
- `npm run lint`: ESLint
- `npm run format`: Prettier (próximamente)
- `npm test`: Jest/Vitest (próximamente)

## Estructura

```
app/
  api/auth/[...nextauth]/   # NextAuth.js handlers
  login/                    # Login page
  page.tsx                  # Home (protegida)

components/               # React components reutilizables

lib/
  auth.ts                  # Auth.js config y helpers

public/                   # Static assets

styles/                   # CSS global (Tailwind)
```

## Env Variables

Copia `.env.example` a `.env.local` y edita según necesites:

- `NEXT_PUBLIC_API_URL`: URL del backend (default: http://localhost:8080)
- `NEXT_PUBLIC_WS_URL`: URL WebSocket (default: ws://localhost:8080)
- `NEXTAUTH_URL`: URL de la app (default: http://localhost:3000)
- `NEXTAUTH_SECRET`: Secret para sesiones (generar con: `openssl rand -base64 32`)

## Auth

Usando NextAuth.js v5 (Auth.js) con:
- Estrategia JWT para tokens
- Cookies httpOnly para almacenar sesiones
- Middleware de protección de rutas
- Mock de credenciales (será integrado con backend Go)

## Next Steps

- [ ] Integrar login con backend Go (reemplazar mock)
- [ ] Refresh token rotation
- [ ] Página de registro
- [ ] MFA (opcional)
- [ ] Perfiles y búsqueda
- [ ] Chat en tiempo real (WebSockets)
- [ ] Presencia online/offline

## Troubleshooting

| Problema | Solución |
|----------|----------|
| Puerto 3000 en uso | Next.js preguntará automáticamente si usar otro puerto |
| NEXTAUTH_SECRET inválido | Generar: `openssl rand -base64 32` |
| Módulos no encontrados | Ejecutar: `npm install` |
| Cambios no reflejados | Reiniciar: `npm run dev` |
