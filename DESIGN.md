---
name: Circl
description: A private contact platform where intimacy is architecture
colors:
  primary: "#3888CC"
  primary-light: "#2470B0"
  primary-hover: "#3080BC"
  accent: "#E87090"
  accent-light: "#D4607A"
  background: "#0A1020"
  background-light: "#D8E8F5"
  foreground: "#F0F6FF"
  foreground-light: "#0C1A24"
  surface: "#0F1828"
  surface-light: "#EBF3FC"
  surface-elevated: "#162030"
  surface-elevated-light: "#DDEAF7"
  muted: "#3A5878"
  strong: "#5AA0DC"
  success: "#22c55e"
  danger: "#f87171"
typography:
  display:
    fontFamily: "Nunito, ui-sans-serif, system-ui, sans-serif"
    fontWeight: 700
    lineHeight: 1.15
    letterSpacing: "-0.01em"
  headline:
    fontFamily: "Nunito, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.25rem"
    fontWeight: 600
    lineHeight: 1.3
  title:
    fontFamily: "Nunito, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 600
    lineHeight: 1.4
  body:
    fontFamily: "DM Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.6
  label:
    fontFamily: "DM Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 500
    lineHeight: 1.4
    letterSpacing: "0.01em"
rounded:
  sm: "4px"
  md: "6px"
  card: "16px"
  pill: "9999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "#ffffff"
    rounded: "{rounded.sm}"
    padding: "8px 16px"
    typography: "{typography.body}"
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
  button-accent:
    backgroundColor: "{colors.accent}"
    textColor: "#ffffff"
    rounded: "{rounded.sm}"
    padding: "8px 16px"
  button-accent-hover:
    backgroundColor: "#E87090"
  button-secondary:
    backgroundColor: "#374151"
    textColor: "#e5e7eb"
    rounded: "{rounded.sm}"
    padding: "8px 16px"
  button-ghost:
    textColor: "#9ca3af"
    rounded: "{rounded.sm}"
  input-default:
    backgroundColor: "#1f2937"
    textColor: "{colors.foreground}"
    rounded: "{rounded.md}"
    padding: "8px 12px"
---

# Design System: Circl

## 1. Overview

**Creative North Star: "The Encrypted Garden"**

Circl is a private room, not a stage. The design system embodies that premise in every surface: users came here because they do not want to be seen by the wrong people. The visual language communicates security through texture and care, not through padlocks and warning banners. Every interaction should feel like closing a door behind you — the world outside goes quiet, and what remains is attentive and unhurried.

The system is dark by default. The physical scene: someone on their phone in a quiet moment, in low ambient light, doing something personal. Dark mode is not a preference toggle; it is the atmosphere. Light mode exists as a full peer — bright-surface blue-tints for users in well-lit environments — and it carries equal design care, but the dark theme is where the product lives.

The palette is a direct political act. Both primary blue (#3888CC / #2470B0) and accent rose (#E87090 / #D4607A) are drawn from the trans flag. Their co-presence on the same screen is never accidental, never decorative. The system reserves this palette for its literal purpose: primary for trust and navigation, rose for human connection.

**Key Characteristics:**
- Dark-first; warm, low-light, unhurried atmosphere
- Trans-flag palette used with intention — blue for structure, rose for relationship
- Components are tactile and discreet: present without announcing themselves
- Elevation through tonal surface stacking, not dramatic shadows
- Rounded-full for relational elements (avatars, badges, presence); subtly-rounded for actions (buttons, inputs)
- Motion is only functional: message-in (0.18 s ease-out), slide-up (0.25 s ease-out), reduced-motion respected unconditionally

## 2. Colors: The Trans Flag Palette

The palette has exactly two named accent families. Using them both on the same screen is the design's most deliberate statement.

### Primary
- **Deep Trans Blue** (`#3888CC` dark / `#2470B0` light): All interactive structure — nav active states, focus rings, links, primary buttons, presence dots when online. The color of the encrypted channel: stable, trusted, reliable.
- **Primary hover** (`#3080BC` dark / `#3580BC` light): Hover state for all primary-blue interactive elements. Subtly darker; no hue shift.
- **Brand Strong** (`#5AA0DC` dark / `#1A5A8A` light): Emphasis text on primary surfaces; active badge fills in dark mode.
- **Brand Muted** (`#3A5878` dark / `#5E9CCE` light): Supporting text, captions, secondary icon tints.

### Secondary (Accent)
- **Trans Flag Rose** (`#E87090` dark / `#D4607A` light): The conversion accent. Used exclusively on the highest-stakes relational actions: send contact request, accept request, message first. Never used decoratively. Its rarity is its meaning.

### Neutral
- **Sealed Night** (`#0A1020`): Page background in dark mode. Near-black with a perceptible blue cast — not `#000000`, never pure black.
- **Private Chamber** (`#0F1828`): First-level surface (cards, sidebars, panels) in dark mode.
- **Lifted Surface** (`#162030`): Elevated surfaces (dropdowns, bottom sheets, input backgrounds on dark panels).
- **Moonlit White** (`#F0F6FF`): Primary text on dark surfaces.
- **Blue Daylight** (`#D8E8F5`): Page background in light mode. Blue-tinted, never neutral gray.
- **Cloud Surface** (`#EBF3FC`): Cards and panels in light mode.

### Named Rules
**The Two-Flag Rule.** The primary blue and the accent rose are drawn from the trans flag and used only in their intended roles. Blue is structure; rose is relationship. Do not use rose for warnings, errors, or decoration. Do not use blue for emotional emphasis. Their co-presence is the design's identity — do not dilute it.

**The Dark Canvas Rule.** Dark mode background is `#0A1020`, not `#000000` or `#111111`. The blue cast is structural: it differentiates the surface tiers (background → surface → elevated) without needing different hues. Never introduce a pure black or a warm-gray background.

## 3. Typography

**Display Font:** Nunito (Google Fonts, `font-display: swap`), variable `--font-nunito`
**Body Font:** DM Sans (Google Fonts, `font-display: swap`), variable `--font-dm-sans`

**Character:** Nunito's rounded terminals bring warmth to a product that could otherwise feel clinical; it carries the heading hierarchy without hardness. DM Sans provides clear, unhurried reading at small sizes — designed for screens, with optical sizes that stay readable at 12–14 px. Together they are warm-functional: expressive in titles, invisible in body text.

### Hierarchy

- **Display** (Nunito, 700, clamp-scaled 24–48 px, line-height 1.15, tracking −0.01 em): Page-level hero titles only. The home hero greeting, the onboarding splash. Never used for section headings inside content pages.
- **Headline** (Nunito, 600, 20 px, line-height 1.3): Primary page headings, modal titles, section titles at the top of full-screen views.
- **Title** (Nunito, 600, 16 px, line-height 1.4): Card headings, widget titles, sidebar group labels.
- **Body** (DM Sans, 400, 14 px, line-height 1.6): All prose, chat messages, descriptions, helper text. Max line length 65 ch on desktop.
- **Label** (DM Sans, 500, 12 px, line-height 1.4, tracking +0.01 em): Badges, timestamps, section dividers (uppercase when used as category labels), input labels, navigation item text.

### Named Rules
**The Heading Font Rule.** Nunito is the heading font only. Never apply `font-display` (the Nunito variable) to body paragraphs, input values, or labels. If a component needs presence without size, use `font-medium` or `font-semibold` in DM Sans rather than reaching for a display font.

## 4. Elevation

Circl is a tonal-layering system. Depth is conveyed through stacked dark surfaces, not through dramatic shadows. In dark mode, three surface tiers separate foreground from background without any shadow at all: `#0A1020` (canvas) → `#0F1828` (surface) → `#162030` (elevated).

Shadows exist as the exception: used to signal that something floats independently of the page (a card in a list, a bottom sheet, a dropdown). In light mode, card shadows include a subtle blue-tinted ring, reinforcing the palette without adding visual noise.

### Shadow Vocabulary
- **Card at rest** (dark): `0 4px 16px -4px rgba(0,0,0,0.4)` — used on profile cards, album tiles, contact cards. Soft and ambient.
- **Card hover** (dark): `0 8px 24px -4px rgba(0,0,0,0.5)` — lifts the card 4 px of perceived height on hover. The transition is `0.2 s ease-out`.
- **Card at rest** (light): `0 1px 8px -2px rgb(36 112 176/0.12), 0 0 0 1px rgb(36 112 176/0.08)` — the dual shadow gives a faint blue-tinted ring plus soft drop.
- **Card hover** (light): `0 4px 16px -4px rgb(36 112 176/0.22), 0 0 0 1px rgb(36 112 176/0.12)` — the ring intensifies to mark interaction readiness.

### Named Rules
**The Flat-By-Default Rule.** Sidebars, nav bars, chat panes, and headers are flat — no shadow separates them from the canvas. Only free-floating cards and sheet overlays use shadow. If you find yourself adding a shadow to a fixed layout element, the layout hierarchy is probably wrong.

## 5. Components

Components are tactile and discreet. Hover states are subtle; focus rings are present but not dramatic. The system is competent, not performative.

### Buttons

- **Shape:** Subtly rounded (4 px radius). Pill variant (`border-radius: 9999px`) reserved for tag filters and presence-aware actions only.
- **Primary** (Deep Trans Blue): `background #3888CC`, `color #fff`, `padding 8px 16px`, `border-radius 4px`. Hover: `background #3080BC`, `transform none`. Focus: `ring-2 ring-brand-hover ring-offset-2` using background as offset color.
- **Accent** (Trans Flag Rose): `background #E87090`, `color #fff`, `box-shadow 0 1px 3px rgba(0,0,0,0.15)`. Used for the one emotional CTA per screen. Hover: `opacity 0.85`.
- **Secondary**: `background #374151`, `color #e5e7eb`, `ring-1 ring-gray-500/60`. Low-contrast ghost sibling for cancel/back actions.
- **Ghost**: Text only, `color #9ca3af`, hover `color #e5e7eb`. Never a background. Used for icon-adjacent controls (close, back in context menus).
- **Danger**: `background rgba(239,68,68,0.1)`, `color #f87171`, `ring-1 ring-red-500/25`. Softened red — destructive without being alarming.
- **Disabled state**: `opacity: 0.5`, `cursor: not-allowed`, all variants.

### Chips / Interest Tags
- **Style**: `bg-brand-primary/15 text-brand-strong rounded-full px-3 py-0.5 text-xs font-medium` — brand-tinted background with slightly stronger text for legibility.
- **State**: No selected/unselected variant; chips are read-only display of profile attributes. Color conveys category membership, not selection state.

### Cards
- **Corner Style**: Gently rounded (16 px radius, `--radius-card`).
- **Background**: `#0F1828` (dark mode) / `#EBF3FC` (light mode).
- **Border**: `ring-1 ring-brand-primary/20` — a very faint blue ring that reinforces the palette without drawing attention.
- **Shadow**: See Elevation section. At rest, ambient only; on hover, lifted.
- **Internal Padding**: `p-4` (16 px) standard; `p-3` (12 px) for compact list-item cards.

### Inputs / Fields
- **Style**: `background #1f2937 (dark) / #fff (light)`, `border 1px solid #374151`, `border-radius 6px (rounded-md)`, `padding 8px 12px`, `font-size 14px`.
- **Focus**: `border-color #3080BC`, `ring: 0 0 0 1px #3080BC` (single pixel ring, no offset). The border shift is the signal; no glow effect.
- **Error**: `border-color #f87171`, error text `text-xs text-red-400` beneath the field. `role="alert"` on the error paragraph.
- **Dirty/unsaved**: `border-color orange-500` — a system-level affordance for unsaved changes.
- **Disabled**: `opacity-50 cursor-not-allowed`.
- **Label**: `text-sm font-medium text-gray-300 block` above the field. Labels are never placeholder-only.

### Navigation
- **Sidebar (≥ lg)**: Fixed left, `bg-gray-900 border-r border-gray-800`. Nav items: `text-sm text-gray-400`, active: `text-foreground bg-gray-800/60 font-medium`. Collapsed state: icon-only (24 × 24 px SVG) aligned center.
- **Bottom nav (< lg)**: Fixed bottom, `bg-gray-900 border-t border-gray-800 pb-safe`. Max 5 items; each `44 × 44 px` touch target. Active item: `text-brand-primary` icon + label.
- **Top bar (< lg)**: `bg-gray-900 border-b border-gray-800`, `h-14`. Back arrow + page title + optional action. All interactive elements `44 × 44 px`.
- **Admin horizontal tab nav (≥ md)**: Items use `-mb-px border-b-2` flush against the header's bottom border. Active: `border-brand-primary text-foreground`. Inactive: `border-transparent text-gray-400`. Uses `items-stretch` on the flex parent so the underline bleeds flush.

### Signature Components

**Avatar**: Circular, 5 sizes (xs=24, sm=28, md=32, lg=40, xl=96 px). With photo: `ring-1 ring-gray-700` (dark). Fallback: initial letter, `bg-brand-primary/40 text-brand-primary dark:text-gray-300` for users, `bg-brand-strong text-white` for group rooms. Never use `bg-gray-700` for fallbacks — it collapses to near-white in light mode.

**PresenceDot**: Two sizes (sm=6 px inline status, md=10 px avatar badge). `bg-green-400` online, `bg-gray-600` offline. `role="status"` with localized `aria-label`. Never conveys state by color alone — the label is always present.

**MessageBubble**: Own messages right-aligned, `bg-brand-primary/90 text-white rounded-tl-2xl rounded-tr-sm rounded-bl-2xl rounded-br-2xl`. Others left-aligned, `bg-gray-800 text-foreground rounded-tl-sm rounded-tr-2xl rounded-bl-2xl rounded-br-2xl`. Entering animation: `message-in 0.18s ease-out` (opacity 0→1, translateY 8px→0). Reduced-motion: no animation.

## 6. Do's and Don'ts

### Do:
- **Do** use Deep Trans Blue (`#3888CC`) for all navigation, interactive structure, and primary actions. It is the architecture color.
- **Do** use Trans Flag Rose (`#E87090`) for at most one CTA per screen — the action that represents human connection (send request, accept, message first).
- **Do** stack dark surfaces (`#0A1020` → `#0F1828` → `#162030`) for depth before reaching for shadows.
- **Do** use `rounded-full` for all circular/relational elements (avatars, presence dots, badges, interest tags) and `rounded` (4 px) or `rounded-md` (6 px) for action elements (buttons, inputs, chips).
- **Do** include `aria-label` on every icon-only button. Include `role="status"` on every presence or live-update indicator. Include `role="alert"` on every inline error.
- **Do** wire every modal, bottom sheet, and dropdown through `useFocusTrap` or `useMenuKeyboard`. Keyboard access is not optional.
- **Do** respect `prefers-reduced-motion` by disabling all animations in the media query block already in `globals.css`.
- **Do** use `min-h-[44px] min-w-[44px]` with `flex items-center justify-center` on any icon-only button to enforce minimum touch targets.
- **Do** write all user-facing strings, including `aria-label` values, through `useTranslations`. No hardcoded English anywhere in component bodies.
- **Do** use `font-display: swap` for all web fonts. Both Nunito and DM Sans are already configured this way.

### Don't:
- **Don't** use Trans Flag Rose for warnings, errors, or destructive actions. Those use danger red. Rose is for relationship; conflating it with danger corrupts the system's meaning.
- **Don't** use gradient text (`background-clip: text` + gradient). Use a single solid foreground color. Emphasis is through weight or size, not color gradients.
- **Don't** use `border-left` greater than 1 px as a colored stripe on cards, alerts, or list items. This is the SaaS-sidebar cliché from the anti-reference list. Use full borders, background tints, or nothing.
- **Don't** apply glassmorphism decoratively. Blur is used only to signal that a background element is dismissed (modals, sheets). Never as surface decoration.
- **Don't** make the product feel like **OnlyFans** (no creator monetization aesthetic, no subscription tier iconography, no public-profile "follow" patterns).
- **Don't** make the product feel like **generic SaaS** (no dashboard-blue Inter typography, no card-grid-with-icon-heading-text patterns, no gradient hero metrics).
- **Don't** make the product feel like a **hookup-app neon** experience (no hot-pink, no aggressive gradients, no swipe-culture motion patterns like spring/elastic transitions).
- **Don't** use `bg-gray-700` for avatar fallbacks. In light mode (blue-tinted gray scale), `gray-700` renders near-white. Use `bg-brand-primary/40` instead.
- **Don't** hardcode theme values as `#000` or `#fff`. Every neutral is blue-tinted. The darkest surface is `#0A1020`; the lightest background is `#D8E8F5`.
- **Don't** put more than one primary CTA (blue) and one accent CTA (rose) on the same screen. Rarity is the point.
