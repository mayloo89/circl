# UX Audit — Circl

**Fecha:** 2026-04-25  
**Objetivo:** Plataforma de contactos sociales — conocer gente, conversaciones, encuentros  
**Stack frontend:** Next.js 15, TypeScript, Tailwind CSS, next-auth, next-intl  
**Design system recomendado:** Vibrant/block-based, paleta indigo+CTA naranja, tipografía Nunito+DM Sans, mobile-first

---

## 1. Bugs reales (errores de código)

### 1.1 Browse: subtitle siempre muestra "noResults"
**Archivo:** `frontend/app/[locale]/browse/page.tsx`

```tsx
// Estado CON resultados — muestra el mensaje de vacío como subtítulo:
<p className="mt-1 text-sm text-gray-400">{t("noResults")}</p>
// Debería ser un subtítulo descriptivo, ej: "Discover people near you"
```

### 1.2 Unblock dialog: mensaje incorrecto
**Archivo:** `frontend/app/[locale]/contacts/page.tsx`

```tsx
<ConfirmDialog
  title={t("unblock")}
  message={tc("unknownError")}  // BUG: muestra "Unknown error" — debería ser "Unblock [nombre]?"
  confirmLabel={t("unblock")}
  onConfirm={() => unblockConfirm && unblock(unblockConfirm.user_id)}
/>
```

### 1.3 Color inconsistencia: Login usa `blue-600`, el resto usa `indigo-600`
**Archivo:** `frontend/app/[locale]/login/page.tsx`

- Login: `bg-blue-600`, `focus:ring-blue-500`, `text-blue-400`
- Todo lo demás: `bg-indigo-600`, `ring-indigo-500`, `text-indigo-400`
- Son colores distintos en Tailwind (#2563EB vs #4F46E5). Rompen identidad de marca.

### 1.4 Register: icono de correo es emoji
**Archivo:** `frontend/app/[locale]/register/page.tsx`

```tsx
// En la pantalla de "check your email":
<div className="text-5xl text-indigo-400">✉</div>
// Emoji como ícono estructural — debe ser SVG
```

### 1.5 Back buttons rompen historial del browser
**Archivos:** `frontend/app/[locale]/chat/page.tsx`, `frontend/app/[locale]/contacts/page.tsx`

```tsx
// Ambas páginas usan push en lugar de back:
<Button onClick={() => router.push("/")}>←</Button>
// Debería ser router.back() — push("/") destruye el historial de navegación
```

---

## 2. Crítico — Mobile UX

### 2.1 Sin bottom navigation
La NavBar es un header horizontal de links de texto. En mobile los links están colapsados en un dropdown del avatar. Un usuario tiene que:
1. Scrollear al top de la página
2. Abrir el menú del avatar
3. Buscar el link deseado

Para una app social (80%+ uso mobile), el patrón correcto es **bottom tab bar** con íconos + labels:

```
[ Browse ] [ Chat 3 ] [ + ] [ Contacts 2 ] [ Profile ]
```

**Impacto:** Sin esto, la app es prácticamente inusable en mobile para navegación entre secciones.

### 2.2 Sin estado activo en la navegación
Ningún link del NavBar tiene indicación visual de qué página está activa. El usuario no sabe dónde está dentro de la app.

### 2.3 Filtros en Browse usan `<details>` en mobile

```tsx
// frontend/app/[locale]/browse/page.tsx
<details className="mb-4 lg:hidden">
  <summary className="cursor-pointer text-sm text-indigo-400">Filters</summary>
```

`<details>` no tiene animación, no tiene overlay, no tiene affordance de mobile. Debería ser un **bottom sheet** con slide-up animation.

### 2.4 Touch targets insuficientes (mínimo 44×44px)

| Elemento | Problema |
|----------|----------|
| Botón "←" en chat header | Texto plano unicode, ~16px |
| "Block" en chat header | `text-xs text-gray-600`, ~12px |
| "×" para remover intereses | Un carácter sin padding |
| Links de texto en NavBar | ~20px de alto |

---

## 3. Alto impacto — Flujo de usuario

### 3.1 Landing page post-login es una página muerta
**Archivo:** `frontend/app/[locale]/page.tsx`

La "home" autenticada contiene solo 3 botones:
- My Profile
- Contacts  
- Sign Out

Ningún valor, ninguna razón para volver. Una plataforma de contactos debería mostrar en el home:
- Perfiles nuevos cerca del usuario
- Solicitudes de contacto pendientes
- Conversaciones con mensajes recientes
- Actividad de la red

**Solución mínima:** Redirigir `/` → `/browse` hasta tener un dashboard real.

### 3.2 Sin onboarding post-registro

Después de verificar email, el usuario llega a la home con 3 botones. No existe:
- Guía para completar el perfil (foto, bio, intereses, ubicación)
- Indicador de completitud del perfil
- Sugerencias de qué hacer primero
- Cualquier tipo de walkthrough

Para una app de contactos, **un perfil vacío = nadie te va a contactar, y el usuario no sabe por qué**. El onboarding es crítico para retención D1/D7.

### 3.3 Sin indicador de completitud de perfil

Si un usuario registrado no tiene foto, bio, ni intereses, aparece en browse con un avatar vacío y sin datos. Otros usuarios no van a enviarle solicitud, y él no entiende por qué no tiene actividad.

Solución: barra de progreso en el perfil propio con pasos concretos ("Add a photo +30%", "Write your bio +20%", etc.).

### 3.4 Browse: UX de filtros genera mucha fricción

Los inputs de edad son `<input type="number" className="w-20">` — en mobile requieren teclado numérico y no hay feedback visual del rango seleccionado.

Flujo actual problemático:
1. Tap en campo "Min age" → teclado numérico
2. Escribir número
3. Tap en campo "Max age"
4. Escribir número
5. Scrollear hasta "Max distance"
6. Escribir número
7. Scrollear hasta botón "Apply"
8. Tap Apply → nueva carga

Solución: sliders de rango para edad y distancia, con feedback visual inmediato.

### 3.5 Sin botón "scroll to bottom" en chat

Cuando el usuario scrollea hacia atrás en el historial, no hay forma de volver al mensaje más reciente sin scrollear manualmente. WhatsApp, Telegram, Slack — todos tienen un FAB (↓) con el count de mensajes no vistos.

### 3.6 Chat list no se actualiza en tiempo real

La lista de conversaciones (`/chat`) se carga una sola vez al montar y no refleja mensajes nuevos hasta que el usuario navega fuera y vuelve. El badge en el nav se actualiza via SSE pero la lista en sí no. Inconsistencia de estado.

### 3.7 No hay búsqueda en la lista de conversaciones

Con muchas conversaciones, sin búsqueda el usuario tiene que scrollear manualmente. Todos los chats de referencia (WhatsApp, Telegram, iMessage) tienen búsqueda en el top de la lista.

---

## 4. Alto impacto — Páginas individuales

### 4.1 Perfil público: `<h1>` genérico
**Archivo:** `frontend/app/[locale]/profile/[username]/page.tsx`

```tsx
<h1 className="text-3xl font-bold text-white">{t("title")}</h1>
// Muestra "Profile" — debería ser el nombre de la persona
// Importante para accesibilidad (screen readers) y potencialmente SEO
```

### 4.2 Perfil público: sin hero visual
La primera impresión es un avatar de 96px centrado en una tarjeta gris. En apps de contactos la foto debería ocupar al menos 50% del viewport above the fold.

Layout actual:
```
┌─────────────────────────┐
│  [Avatar 96px centrado] │
│  Nombre                 │
│  Bio                    │
│  [Botones de acción]    │
└─────────────────────────┘
```

Layout sugerido:
```
┌─────────────────────────┐
│                         │
│   [Hero photo — 55vh]   │  ← Primera impresión visual fuerte
│                         │
│  ← Back          ⋯ More │
├─────────────────────────┤
│  Nombre, 28             │
│  📍 Buenos Aires · 3km  │
│                         │
│  [Message] [Add contact]│
│                         │
│  About · Interests      │
│  Photos                 │
└─────────────────────────┘
```

### 4.3 Perfil propio: sin modo "vista previa"

El usuario edita su perfil pero no puede ver cómo lo ven los demás. Un botón "Preview profile" que lleve a `/profile/{username}` sería de alta utilidad.

### 4.4 Browse: acción doble en el card es confusa

```tsx
// El card entero es un Link que navega al perfil
<Link href={`/profile/${profile.username}`}>
  ...
  // Pero el botón de contacto bloquea el link con stopPropagation
  <div onClick={(e) => e.preventDefault()}>
    <SendRequestButton ... />
  </div>
</Link>
```

En mobile, un tap en el botón vs. el resto de la tarjeta da resultados completamente distintos. El patrón más claro: el card navega al perfil, y en el perfil se puede agregar contacto. O el card tiene dos acciones explícitas separadas visualmente.

### 4.5 Contacts: search results mezclados con la lista

La barra de búsqueda está arriba y los resultados aparecen intercalados con las secciones de pending/sent/accepted. Difícil distinguir qué son search results vs. contactos establecidos. Debería haber separación visual clara o una vista de resultados separada.

### 4.6 Contacts: back button genérico con texto incorrecto

```tsx
<Button variant="ghost" aria-label="Go to home" onClick={() => router.push("/")}>←</Button>
// aria-label dice "Go to home" pero el texto visual solo es "←"
// El arrow unicode solo no es suficiente como label visual
```

---

## 5. Medio — Componentes y detalles de UI

### 5.1 Login/Register: sin toggle show/hide en contraseña

Ningún campo de password tiene el botón de mostrar/ocultar. Usuarios no pueden verificar lo que escriben.

```tsx
// login/page.tsx y register/page.tsx
<input type="password" />  // sin botón para mostrar/ocultar
```

### 5.2 Register: date input nativo es malo en mobile

`<input type="date">` en iOS Safari muestra un wheel picker. Para una fecha de nacimiento (que puede ser 30-40 años atrás), el usuario tiene que scrollear una rueda muy larga. Alternativa: tres dropdowns (Día / Mes / Año) o input de texto con máscara (DD/MM/YYYY).

### 5.3 Settings: idioma activo no tiene indicación visual

```tsx
// settings/page.tsx — los 3 botones son idénticos visualmente
{routing.locales.map((locale) => (
  <button className="rounded-md px-3 py-1.5 text-sm text-gray-300">
    {t(LOCALE_LABEL_KEYS[locale])}
  </button>
))}
// No hay ring, background, ni font-weight diferente para el locale activo
```

### 5.4 Browse: empty state sin CTA accionable

```tsx
<p className="text-lg font-semibold text-gray-300">{t("noResults")}</p>
// No hay botón "Clear filters" ni ninguna sugerencia de acción
```

### 5.5 NavBar: fetch del perfil en cada render

```tsx
// NavBar.tsx — se ejecuta en CADA página porque NavBar se remonta
useEffect(() => {
  fetch(`${API_URL}/profiles/me`, ...)
}, [status, session])
```

Esta llamada sucede en cada navegación. Debería estar en un contexto global compartido o usar SWR/React Query con cache.

### 5.6 Chat room: estado de loading es texto plano

```tsx
// chat/[roomId]/page.tsx — loading state
if (status === "loading") {
  return (
    <div className="flex h-full items-center justify-center">
      <p className="text-gray-400">{t("loading")}</p>  // texto plano
    </div>
  )
}
// Debería usar el componente Skeleton (que ya existe) como hace el rest de la app
```

### 5.7 Contacts: presence en la lista

Los contactos muestran el dot de presencia (`online={presence[c.user_id]?.online}`), pero la lista no está ordenada por presencia. Los contactos online deberían aparecer primero (como hace el sidebar de miembros en el chat).

### 5.8 Bloquear usuario: botón muy poco visible con acción destructiva

```tsx
// ProfileHeader.tsx
<button className="text-xs text-gray-600 hover:text-red-400">
  Block user
</button>
```

`text-gray-600` sobre `bg-gray-900` tiene contraste ~2.5:1 (mínimo WCAG: 4.5:1). Aunque la acción es destructiva y se quiere discreta, no puede ser invisible para screen readers o usuarios con visión reducida.

---

## 6. Arquitectura UX — Recomendaciones estructurales

### 6.1 Rediseño de navegación

**Mobile (<1024px) — Bottom Tab Bar:**

```
┌─────────────────────────────────────────┐
│  [Browse] [Chat 3] [+] [Contacts 2] [Me]│
└─────────────────────────────────────────┘
```

- Máximo 5 items
- Íconos SVG + labels de texto
- Badge de notificaciones en Chat y Contacts
- "+" como acción central (iniciar conversación, crear grupo)
- Activo: color primario + indicator dot/underline

**Desktop (≥1024px) — Sidebar izquierdo:**

```
┌──────┬────────────────────────────────┐
│ Logo │                                │
│ ─── │                                │
│ 🔍  │                                │
│ 💬  │     Main content               │
│ 👥  │                                │
│ ─── │                                │
│ 👤  │                                │
└──────┴────────────────────────────────┘
```

### 6.2 Home post-login

Opción A (mínimo): redirigir `/` → `/browse`  
Opción B (dashboard): feed compuesto por:

```
┌─────────────────────────────────────┐
│ Pending requests (if any)           │
│ ┌────┐ ┌────┐                       │
│ │    │ │    │  [Accept] [Decline]   │
│ └────┘ └────┘                       │
├─────────────────────────────────────┤
│ New people near you                 │
│ [Profile card] [Profile card] →     │
├─────────────────────────────────────┤
│ Recent conversations                │
│ [Avatar] Name — last message  12m  │
│ [Avatar] Name — last message   3h  │
└─────────────────────────────────────┘
```

### 6.3 Onboarding flow post-registro

Flujo sugerido (máximo 4 pasos, skippable):

```
Step 1: Add your photo      (skip)
Step 2: Write your bio      (skip)
Step 3: Add your interests  (skip)
Step 4: Set your location   (skip)
──────────────────────────────────
         [Continue →]
         [Do this later]
```

Con barra de progreso en el perfil propio mostrando % de completitud y pasos pendientes.

### 6.4 Perfil público como hero

Ver sección 4.2 para el layout propuesto. La foto debería ser el elemento dominante above the fold.

### 6.5 Browse con filtros como bottom sheet en mobile

```tsx
// Header del browse en mobile:
<div className="flex items-center justify-between">
  <h1>Discover</h1>
  <button onClick={() => setFilterSheetOpen(true)}>
    <FilterIcon />
    {activeFilterCount > 0 && <Badge count={activeFilterCount} />}
  </button>
</div>

// Bottom sheet con overlay + slide-up:
<BottomSheet open={filterSheetOpen} onClose={() => setFilterSheetOpen(false)}>
  <FilterPanel ... />
</BottomSheet>
```

---

## 7. Resumen por prioridad

### 🔴 Crítico (bugs + bloqueos de usabilidad)

| # | Problema | Archivo |
|---|----------|---------|
| 1 | Sin bottom navigation en mobile | `NavBar.tsx` |
| 2 | Landing post-login sin valor | `app/[locale]/page.tsx` |
| 3 | Sin onboarding post-registro | — (nueva feature) |
| 4 | Browse subtitle muestra "noResults" | `browse/page.tsx` |
| 5 | Unblock dialog mensaje incorrecto | `contacts/page.tsx` |
| 6 | Color inconsistencia blue vs indigo | `login/page.tsx` |

### 🟠 Alto (impacto directo en retención)

| # | Problema | Archivo |
|---|----------|---------|
| 7 | Sin estado activo en nav | `NavBar.tsx` |
| 8 | Touch targets insuficientes (←, Block, ×) | Múltiples |
| 9 | Back buttons rompen historial | `chat/page.tsx`, `contacts/page.tsx` |
| 10 | Sin indicador de completitud de perfil | `profile/page.tsx` |
| 11 | h1 del perfil público es genérico | `profile/[username]/page.tsx` |
| 12 | Sin hero photo en perfil público | `profile/[username]/page.tsx` |
| 13 | Filtros browse: UX de mobile con `<details>` | `browse/page.tsx` |
| 14 | Sin scroll-to-bottom button en chat | `chat/[roomId]/page.tsx` |
| 15 | Chat list no actualiza en tiempo real | `chat/page.tsx` |

### 🟡 Medio (polish y completitud)

| # | Problema | Archivo |
|---|----------|---------|
| 16 | Sin password show/hide en login/register | `login/page.tsx`, `register/page.tsx` |
| 17 | Date input nativo problemático en mobile | `register/page.tsx` |
| 18 | Settings: idioma activo sin indicación | `settings/page.tsx` |
| 19 | Browse empty state sin CTA de clear filters | `browse/page.tsx` |
| 20 | Contacts: search results mezclados con lista | `contacts/page.tsx` |
| 21 | Contacts: lista sin orden por presencia | `contacts/page.tsx` |
| 22 | NavBar fetch de perfil no cacheado | `NavBar.tsx` |
| 23 | Chat loading state es texto, no skeleton | `chat/[roomId]/page.tsx` |
| 24 | "Block user" botón con contraste insuficiente | `ProfileHeader.tsx` |
| 25 | Register: emoji como ícono en success screen | `register/page.tsx` |
| 26 | Browse: acción doble en card (link + button) | `browse/page.tsx` |
| 27 | Sin preview "Ver como otros me ven" | `profile/page.tsx` |
| 28 | Sin búsqueda en lista de conversaciones | `chat/page.tsx` |

---

## 8. Quick wins (< 30 min cada uno)

1. Cambiar `blue-600` → `indigo-600` en login/page.tsx
2. Fix unblock dialog message en contacts/page.tsx
3. Fix browse subtitle (texto incorrecto) en browse/page.tsx
4. Cambiar `router.push("/")` → `router.back()` en chat y contacts
5. Reemplazar emoji `✉` con SVG en register/page.tsx
6. Agregar estado activo al NavBar usando `usePathname()`
7. Agregar `p-2` a botones "←" para aumentar touch target
8. Mostrar idioma activo con ring/background diferente en settings
9. Ordenar contactos online-primero en contacts/page.tsx
10. Fix loading state en chat room (usar `<MessageSkeletons />` ya existente)
