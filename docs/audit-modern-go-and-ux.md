# Audit: Modern Go + UX/UI

**Fecha:** 2026-05-01  
**Go version:** 1.25.5  
**Frontend:** Next.js + Tailwind CSS v4 (dark-only)  
**Skills:** use-modern-go (JetBrains), ui-ux-pro-max

---

## Backend — Modern Go (19 findings)

### 1. `errors.Is` en vez de `err == target` (6 findings)

La comparacion directa de errores no funciona con errores wrappend. Usar `errors.Is()`.

| # | Archivo | Linea | Outdated | Replacement |
|---|---------|-------|----------|-------------|
| 1 | `internal/presence/presence.go` | 45 | `err != nil && err != redis.Nil` | `err != nil && !errors.Is(err, redis.Nil)` |
| 2 | `internal/presence/presence.go` | 90 | `err != nil && err != redis.Nil` | `err != nil && !errors.Is(err, redis.Nil)` |
| 3 | `internal/db/db.go` | 61 | `err != nil && err != migrate.ErrNoChange` | `err != nil && !errors.Is(err, migrate.ErrNoChange)` |
| 4 | `internal/auth/refresh_store_test.go` | 67 | `err != ErrInvalidToken` | `!errors.Is(err, ErrInvalidToken)` |
| 5 | `internal/auth/refresh_store_test.go` | 77 | `err != ErrInvalidToken` | `!errors.Is(err, ErrInvalidToken)` |
| 6 | `internal/auth/refresh_store_test.go` | 92 | `err != ErrInvalidToken` | `!errors.Is(err, ErrInvalidToken)` |

> Nota: `presence.go` y `refresh_store_test.go` necesitan agregar `"errors"` al import.

### 2. `omitzero` en vez de `omitempty` en `*time.Time` y mapas (8 findings)

`omitempty` en `time.Time`/`*time.Time` casi nunca tiene la semantica correcta. `omitzero` (Go 1.24+) omite cuando el valor es zero, que es lo deseado.

| # | Archivo | Linea | Campo | Outdated | Replacement |
|---|---------|-------|-------|----------|-------------|
| 1 | `internal/chat/handler.go` | 72 | `ExpiresAt *time.Time` | `json:"expires_at,omitempty"` | `json:"expires_at,omitzero"` |
| 2 | `internal/chat/chat.go` | 110 | `PeerLastReadAt *time.Time` | `json:"peer_last_read_at,omitempty"` | `json:"peer_last_read_at,omitzero"` |
| 3 | `internal/chat/chat.go` | 128 | `ExpiresAt *time.Time` | `json:"expires_at,omitempty"` | `json:"expires_at,omitzero"` |
| 4 | `internal/uploads/uploads.go` | 34 | `CommittedAt *time.Time` | `json:"committed_at,omitempty"` | `json:"committed_at,omitzero"` |
| 5 | `internal/uploads/uploads.go` | 84 | `ExpiresAt *time.Time` | `json:"expires_at,omitempty"` | `json:"expires_at,omitzero"` |
| 6 | `internal/reports/reports.go` | 54 | `ReviewedAt *time.Time` | `json:"reviewed_at,omitempty"` | `json:"reviewed_at,omitzero"` |
| 7 | `internal/reports/reports.go` | 70 | `ReviewedAt *time.Time` | `json:"reviewed_at,omitempty"` | `json:"reviewed_at,omitzero"` |
| 8 | `internal/worker/otel.go` | 26 | `Trace traceCarrier` (map) | `json:"_trace,omitempty"` | `json:"_trace,omitzero"` |

### 3. `max()` builtin en vez de if-clamp (2 findings)

| # | Archivo | Linea | Outdated | Replacement |
|---|---------|-------|----------|-------------|
| 1 | `internal/worker/image_task.go` | 218-219 | `if dw < 1 { dw = 1 }` | `dw = max(dw, 1)` |
| 2 | `internal/worker/image_task.go` | 221-222 | `if dh < 1 { dh = 1 }` | `dh = max(dh, 1)` |

### 4. `for range n` en vez de `for i := 0; i < n; i++` (1 finding)

| # | Archivo | Linea | Outdated | Replacement |
|---|---------|-------|----------|-------------|
| 1 | `internal/ratelimit/redis_test.go` | 60 | `for i := 0; i < limit; i++` | `for range limit` |

> La variable `i` no se usa en el cuerpo del loop, asi que `range limit` es correcto.

### 5. `strings.Cut` en vez de `Index`+slice (1 finding)

| # | Archivo | Linea | Outdated | Replacement |
|---|---------|-------|----------|-------------|
| 1 | `cmd/api/main.go` | 467 | `req.Email[:strings.Index(req.Email, "@")]` | `local, _, _ := strings.Cut(req.Email, "@")` y usar `local` |

Antes:
```go
req.Username = "u_" + strings.ReplaceAll(req.Email[:strings.Index(req.Email, "@")], ".", "_")
```

Despues:
```go
local, _, _ := strings.Cut(req.Email, "@")
req.Username = "u_" + strings.ReplaceAll(local, ".", "_")
```

### Patrones ya modernizados (sin findings)

| Patron | Estado |
|--------|--------|
| `interface{}` -> `any` | Ya modernizado |
| `slices.Contains`/`slices.Index` | Ya modernizado (hub.go usa `slices.Index`) |
| `wg.Go()` en vez de `wg.Add(1)` + goroutine | Ya modernizado (loki.go) |
| `strings.SplitSeq` | Ya modernizado (server.go) |
| `mux.HandleFunc("GET /path", handler)` method routing | Ya modernizado |
| `sync.OnceFunc`/`sync.OnceValue` | No aplica |
| `b.Loop()` en benchmarks | No hay benchmarks |

---

## Frontend — UX/UI (~80+ findings)

**Stack:** Tailwind CSS v4, Next.js, custom components (sin shadcn/MUI)  
**Dark mode:** Hardcoded dark-only, sin light mode  
**Iconos:** Inline SVGs custom (sin libreria)

### Critico — Accesibilidad

#### 1. Emojis como iconos (5 findings)

Los screen readers no pueden interpretar emojis como iconos. Reemplazar con SVG.

| # | Archivo | Linea | Emoji | Replacement |
|---|---------|-------|-------|-------------|
| 1 | `app/[locale]/register/page.tsx` | 244 | checkmark | SVG checkmark + `aria-label="Available"` |
| 2 | `app/[locale]/register/page.tsx` | 247 | X mark | SVG X icon + `aria-label="Unavailable"` |
| 3 | `app/[locale]/chat/channels/page.tsx` | 87 | X close | SVG close icon |
| 4 | `app/[locale]/chat/[roomId]/page.tsx` | 510 | left arrow | SVG chevron-left icon |
| 5 | `components/chat/MessageBubble.tsx` | 182 | paperclip | SVG paperclip icon |

#### 2. Botones sin focus ring (20+ findings)

Todos los botones interactivos carecen de `focus:outline-none focus:ring-2 focus:ring-*`.

| Archivo | Lineas | Elementos |
|---------|--------|-----------|
| `components/contacts/ContactCard.tsx` | 42 | button onNavigate |
| `components/PushPrompt.tsx` | 20, 26 | Enable + Dismiss buttons |
| `components/chat/ChatInput.tsx` | 97, 120, 149 | Attach, ephemeral, menu buttons |
| `components/profile/PhotoGallery.tsx` | 46, 102, 120 | Photo click, delete, add buttons |
| `components/chat/MessageBubble.tsx` | 97, 126, 153, 166 | View-once, image, video buttons |
| `app/[locale]/chat/[roomId]/page.tsx` | 509, 515, 533, 544, 562 | Back, peer, block, group info, members |
| `app/[locale]/admin/reports/page.tsx` | 120, 140 | Toggle buttons |
| `app/[locale]/chat/channels/page.tsx` | 87 | Modal close button |

**Fix sugerido:** Agregar `focus:outline-none focus:ring-2 focus:ring-brand-hover` a cada button.

#### 3. Inputs sin label asociado (15+ findings)

| Archivo | Lineas | Inputs afectados |
|---------|--------|-----------------|
| `app/[locale]/admin/users/page.tsx` | 85, 94, 315, 318 | Duration, Reason, User select, Search |
| `app/[locale]/admin/channels/page.tsx` | 62, 70, 131, 138 | Name, Description (create + edit) |
| `app/[locale]/admin/reports/page.tsx` | 117, 137, 155, 163, 172 | Resolution, Action, Duration, Reason x2 |
| `app/[locale]/chat/page.tsx` | 194 | Search input sin `aria-label` |
| `app/[locale]/browse/page.tsx` | 301 | Search input sin `aria-label` |

**Fix:** Agregar `htmlFor`/`id` pairing o `aria-label` en inputs de busqueda.

#### 4. Imagenes sin alt text significativo (7+ findings)

| Archivo | Linea | Current | Fix |
|---------|-------|---------|-----|
| `components/ui/Avatar.tsx` | 30 | `alt=""` | `alt={name ?? "User avatar"}` |
| `components/profile/PhotoGallery.tsx` | 53, 82 | `alt=""` | `alt="Profile photo"` |
| `components/chat/Lightbox.tsx` | 33 | `alt=""` | `alt="Full size image"` |
| `components/home/NearbyProfilesWidget.tsx` | 41 | `alt=""` | `alt={name ?? "Nearby user"}` |
| `app/[locale]/profile/page.tsx` | 512 | `alt=""` | `alt="Your avatar"` |
| `app/[locale]/onboarding/photo/page.tsx` | 96 | `alt=""` | `alt="Profile photo preview"` |
| `app/[locale]/browse/page.tsx` | multiples | `alt=""` | `alt={user.display_name ?? "User"}` |

#### 5. Color como unico indicador (2 findings)

| Archivo | Linea | Issue | Fix |
|---------|-------|-------|-----|
| `components/ui/PresenceDot.tsx` | 13-18 | Online/offline solo por color | Agregar diferencia de forma: filled circle vs outline-only |
| `app/[locale]/chat/[roomId]/page.tsx` | 576 | Connection status dot solo por color | Agregar shape difference o text label visible |

### Alto — Usabilidad

#### 6. Botones sin `cursor-pointer` (23+ findings)

Tailwind v4 **no** agrega `cursor: pointer` a `<button>` por defecto (cambio vs v3).

**Fix global recomendado:** Agregar `cursor-pointer` al `base` class en `components/ui/Button.tsx:54`.

Botones adicionales fuera de `<Button>`:

| Archivo | Elementos |
|---------|-----------|
| `components/contacts/ContactCard.tsx` | onNavigate button |
| `components/PushPrompt.tsx` | Enable + Dismiss |
| `components/chat/ChatInput.tsx` | Attach + Ephemeral + Menu items |
| `components/profile/PhotoGallery.tsx` | Photo click + Delete + Add |
| `components/chat/MessageBubble.tsx` | View-once + Image + Video |
| `app/[locale]/chat/[roomId]/page.tsx` | Back + Peer + Block + Group info + Members |
| `app/[locale]/admin/reports/page.tsx` | Toggle pills |
| `app/[locale]/chat/channels/page.tsx` | Modal close |
| `app/[locale]/contacts/error.tsx` | Try again |

#### 7. Hover feedback incompleto (9 findings)

| Archivo | Linea | Issue | Fix |
|---------|-------|-------|-----|
| `components/PushPrompt.tsx` | 20 | hover sin `transition-colors` | Agregar `transition-colors` |
| `components/contacts/ContactCard.tsx` | 42 | `hover:opacity-80` sin transition | Agregar `transition-opacity` |
| `components/chat/MessageBubble.tsx` | 97, 126 | View-once buttons sin hover | Agregar `hover:opacity-80 transition-opacity` |
| `app/[locale]/chat/[roomId]/page.tsx` | 509 | Back button sin transition | Agregar `transition-colors` |
| `app/[locale]/chat/[roomId]/page.tsx` | 533 | Block button sin transition | Agregar `transition-colors` |
| `app/[locale]/chat/[roomId]/page.tsx` | 515, 544 | `hover:opacity-80` sin transition | Agregar `transition-opacity` |
| `components/profile/PhotoGallery.tsx` | 46 | `active:scale-95` sin hover | Agregar `hover:opacity-90` |

#### 8. `prefers-reduced-motion` incompleto

`globals.css` solo cubre `slide-up` y `pulse` keyframes. Las **59 transiciones** a nivel componente no se reducen.

Fix global:
```css
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
  }
}
```

#### 9. Contenido detras de navbar fijo

No hay `scroll-padding-top` para navegacion por anchor.

Fix global:
```css
html {
  scroll-padding-top: 3.5rem;
}
```

### Medio — Consistencia de diseno

#### 10. Max-widths inconsistentes

7 valores distintos sin sistema de design tokens:

| Max-Width | Uso | Proposito |
|-----------|-----|-----------|
| `max-w-sm` (384px) | Login, Reset-password, dialogs | Formularios chicos |
| `max-w-md` (448px) | Register, Forgot-password, Verify-email, Onboarding | Formularios medianos |
| `max-w-lg` (512px) | Chat list, Contacts, Profile | Contenido |
| `max-w-2xl` (672px) | Home, Settings, Channels browse | Contenido ancho |
| `max-w-6xl` (1152px) | Browse page | Grid full-width |
| Sin max-w | Admin tables | Tablas full-width |

Sistema sugerido: Dialogos `max-w-sm`, Formularios `max-w-md`, Contenido `max-w-2xl`, Grid `max-w-6xl`.

#### 11. Iconos con tamano inconsistente

| Tamano | Uso | Tipo |
|--------|-----|------|
| `h-3 w-3` (12px) | MessageBubble micro indicators | Micro |
| `h-3.5 w-3.5` (14px) | MessageBubble timestamps | Inline |
| `h-4 w-4` (16px) | Sidebar chevron, PushPrompt dismiss | Nav |
| `h-5 w-5` (20px) | ChatInput, TopBar, Sidebar nav icons | Acciones |
| `h-6 w-6` (24px) | Lightbox close, PhotoGallery | Standalone |
| `h-7 w-7` (28px) | Empty state decorations | Decorativo |
| **22x22 hardcoded** | **BottomNav** | **Outlier** |

Fix: Cambiar BottomNav de `width="22" height="22"` a `className="h-5 w-5"`.

#### 12. Layout shift en hover/press (7 findings)

| Archivo | Linea | Issue | Fix |
|---------|-------|-------|-----|
| `components/profile/PhotoGallery.tsx` | 49 | `active:scale-95` causa shift en grid | Usar `hover:opacity-90` o `will-change-transform` |
| `components/home/NearbyProfilesWidget.tsx` | 43 | `group-hover:scale-105` overflow en card | Usar `hover:opacity-90` o `overflow-hidden` |
| `app/[locale]/browse/page.tsx` | 119 | `group-hover:scale-105` en card image | Usar `hover:opacity-90` |
| `components/chat/MessageBubble.tsx` | 99, 128, 155, 168 | `active:scale-95` en flex containers | Usar `active:opacity-70` |

### Bajo — Future-proofing

#### 13. `scrollbar-hide` no definido

`NearbyProfilesWidget.tsx:106` usa `scrollbar-hide` que no es utilidad estandar de Tailwind v4.

Fix: Agregar a `globals.css`:
```css
.scrollbar-hide {
  scrollbar-width: none;
  &::-webkit-scrollbar {
    display: none;
  }
}
```

#### 14. Solo dark mode

La app usa colores dark hardcoded (`:root { --background: #030712; --foreground: #f9fafb; }`). No hay soporte para `prefers-color-scheme` ni toggle `dark:`.

Si se agrega light mode, todos los `text-gray-400`, `border-gray-700`, `bg-gray-800` serian invisibles en fondo blanco.

#### 15. Transiciones lentas/inconsistentes

| Archivo | Linea | Issue | Fix |
|---------|-------|-------|-----|
| `app/[locale]/providers.tsx` | 75 | Sidebar collapse `duration-200` abrupto | Considerar `duration-300` |
| `components/nav/Sidebar.tsx` | 170 | Mismo issue | Considerar `duration-300` |
| `app/[locale]/browse/page.tsx` | 111 | `transition-shadow` sin `duration-` | Agregar `duration-200` |

---

## Resumen

| Severidad | Categoria | Cantidad | Esfuerzo |
|-----------|-----------|----------|----------|
| Critico | Accesibilidad (focus, labels, alt, emojis, color-only) | ~49 | Medio |
| Alto | Usabilidad (cursor, hover, reduced-motion, scroll-padding) | ~33 | Bajo |
| Medio | Consistencia (max-widths, icon sizes, layout shift) | ~20 | Bajo |
| Bajo | Future-proofing (scrollbar-hide, dark-only, transitions) | ~5 | Bajo |
| **Total frontend** | | **~107** | |
| Modern Go | `errors.Is`, `omitzero`, `max()`, `range`, `strings.Cut` | **19** | Bajo |

### Orden sugerido de fix

1. Go backend — 19 cambios mecanicos, sin riesgo
2. `cursor-pointer` global en Button.tsx — 1 linea, fixea 23+ botones
3. `prefers-reduced-motion` + `scroll-padding-top` en globals.css — 2 bloques CSS
4. Focus rings — agregar a todos los `<button>` raw
5. Labels + alt text — accesibilidad WCAG
6. Emojis -> SVG — 5 reemplazos
7. Layout shift — reemplazar `scale-*` por `opacity-*`
8. Consistencia — max-widths, icon sizes, transitions
