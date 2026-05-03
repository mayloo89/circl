# Expert Review Plan — Security, Trust & Safety, UX

**Date:** 2026-05-02
**Scope:** Findings from a social-platform expert review (security + web/mobile UX + product gaps), turned into an actionable, sequenced plan of PRs.
**Goal:** Bring Circl from "technically mature" to "ready for real-user launch" without compromising the best-practices-showcase bar.

Each item has:
- **Severity / priority**
- **File:line** (current code)
- **Concrete fix**
- **Acceptance criteria**

---

## Executive summary

Strong foundations: JWT + refresh rotation, Redis blacklist, structured logging, observability, OpenAPI, i18n, navigation refactor, onboarding wizard.

Three areas block production launch:
1. **Privacy data exposure** — public profile / browse / presence leak more than they should.
2. **Trust & Safety surface** — no Terms / Privacy / Guidelines, no NSFW detection, no data export, no appeals process, no in-app block-list.
3. **Mobile polish** — iOS safe-area handling, touch targets, single-line chat composer.

---

## P0 — Critical (must ship before any real-user launch)

### S-1. Public profile leaks private fields to any authenticated user
- **File:line:** `backend/internal/profiles/handler.go:206`
- **Issue:** `GET /profiles/{ref}` returns DOB, email, exact lat/lng to any logged-in user.
- **Fix:** branch on `caller == owner || isAcceptedContact(caller, owner)`. Public response: display_name, username, bio, avatar, interests, age (computed), city/region (no coords). Private response adds DOB, email, exact location, last_seen.
- **Acceptance:** integration test — non-contact gets `email=""`, `dob=null`, no `lat/lng`.

### S-2. Soft-deleted users still appear in browse and contact search
- **File:line:** `backend/internal/profiles/handler.go:499`, `backend/internal/contacts/contacts.go:107`
- **Fix:** add `WHERE u.status NOT IN ('deleted','purged','suspended','banned')` to both queries; same for presence batch lookups.
- **Acceptance:** integration test — soft-delete a user → not in browse, not in contact search, presence returns `unknown`.

### S-3. WebSocket JWT in query string is logged by proxies/CDN
- **File:line:** `backend/internal/chat/handler.go:713-722`
- **Fix:** add `POST /auth/ws-ticket` returning a single-use 60-second ticket stored in Redis. WS upgrade reads `?ticket=`, `DEL`s on use.
- **Acceptance:** WS handshake works; ticket reuse returns 401; access logs no longer carry JWTs.

### S-4. Presence visible to non-contacts
- **File:line:** `backend/internal/presence/handler.go`
- **Fix:** intersect requested `user_ids` with `accepted_contacts(caller)` server-side; return `unknown` for everyone else. Same gate on the SSE presence stream.
- **Acceptance:** integration test — non-contact gets `unknown` even if user is online.

### S-5. Secrets committed to backend/.env (JWT, VAPID)
- **File:line:** `backend/.env:12,17-20`
- **Fix:** rotate JWT_SECRET and VAPID keys. Confirm `backend/.env` is gitignored and not tracked (`git ls-files backend/.env` returns nothing). Replace with `.env.example` placeholders only.
- **Acceptance:** gitleaks CI passes; `git log -p -- backend/.env` shows file removal.

### S-6. CSP allows `'unsafe-inline'` for scripts
- **File:line:** `frontend/next.config.ts:39`
- **Fix:** Next.js 16 nonce-based CSP via middleware — mint per-request nonce, propagate to `<script>` tags, drop `'unsafe-inline'` from `script-src`.
- **Acceptance:** `curl -I` shows `Content-Security-Policy: ... script-src 'self' 'nonce-XXX'`; app loads with no CSP violations in console.

### S-7. Missing rate limit on reports and contact requests
- **File:line:** `backend/internal/reports/handler.go`, `backend/internal/contacts/handler.go:85-130`
- **Fix:** apply per-user limiter — 10 reports/hour, 100 contact requests/day, via existing `ratelimit` package.
- **Acceptance:** 11th report in an hour returns 429; CI test added.

### TS-1. No Terms / Privacy / Community Guidelines pages
- **Where:** missing routes under `frontend/app/[locale]/`
- **Fix:** add `/terms`, `/privacy`, `/guidelines`, `/safety` MDX-driven pages. Link from registration footer, login, Settings.
- **Acceptance:** routes exist in all three locales; registration flow has "I agree to Terms + Privacy" checkbox.

### TS-2. No data export (GDPR Art. 20 / CCPA)
- **Fix:** `POST /account/export` enqueues an asynq job → generates JSON + media zip in S3 → emails one-time signed URL (14-day retention).
- **Acceptance:** user requests export → email received with valid link → zip contains profile, messages, contacts, photos.

### TS-3. No NSFW / CSAM detection on uploads
- **File:line:** `backend/internal/worker/image_task.go`
- **Fix:** add a moderation stage in the worker before commit. Minimum: open-source NSFW classifier server-side or AWS Rekognition / Cloudflare Images moderation. Evaluate PhotoDNA / Thorn Safer for CSAM.
- **Acceptance:** uploaded test NSFW sample is rejected with `code=upload_rejected_moderation`; admin notified.

### TS-4. Missing critical report categories
- **Fix:** add `underage`, `threat_violence`, `csam`, `impersonation`. CSAM auto-pages on-call and bypasses normal queue.
- **Acceptance:** admin queue sorts CSAM/threats first; oncall paged on CSAM via existing alerting.

### TS-5. Age verification is self-declared only
- **Fix:** at minimum, log "claims 18+" with timestamp + IP at registration. Document the policy in `/safety`. Plan for Stripe Identity / Veriff in a paid tier or for users flagged by reports.
- **Acceptance:** audit log table populated; safety page documents the policy.

---

## P1 — High (next 2–3 PRs after P0)

### Mobile UX

#### M-1. iOS safe-area not respected
- **Files:**
  - `frontend/components/nav/TopBar.tsx:51` (notch overlap)
  - `frontend/app/[locale]/providers.tsx:75` (`pb-16` insufficient)
  - `frontend/components/ui/Toast.tsx:40` (renders under BottomNav)
  - `frontend/app/[locale]/profile/[username]/page.tsx:514` (sticky bar under nav)
- **Fix:** add Tailwind utilities `pt-safe`, `pb-safe`, `pb-safe-nav` once and apply globally. AppShell content gets `pb-[calc(4rem+env(safe-area-inset-bottom))]`.
- **Acceptance:** iPhone X simulator — no nav/toast overlap, action bars visible.

#### M-2. Inputs trigger iOS auto-zoom
- **File:line:** `frontend/components/ui/Input.tsx:39`
- **Fix:** `text-base` (16px) on the base input class. Add `inputMode="email"` + `autoComplete="email"` on login/register; correct `autoComplete="new-password"` / `current-password"` everywhere.
- **Acceptance:** focus on email/password input on iOS Safari does not zoom.

#### M-3. Chat composer is a single-line input
- **File:line:** `frontend/components/chat/ChatInput.tsx:164-181`
- **Fix:** auto-growing `<textarea rows={1} />` capped at `max-h-40`. Enter sends; Shift+Enter newline (desktop). Mobile: dedicated send button, no Enter-sends ambiguity.
- **Acceptance:** typing 3 lines expands the composer; sending clears and resets height.

#### M-4. Photo upload missing camera capture
- **File:line:** `frontend/app/[locale]/onboarding/photo/page.tsx:31-39`
- **Fix:** `capture="user"` on avatar/onboarding selfie. Leave it off on the gallery so library access works.
- **Acceptance:** mobile Safari opens front camera directly on tap.

#### M-5. Push prompt fires too early
- **File:line:** `frontend/components/PushPrompt.tsx:14-38`
- **Fix:** delay until first contact accepted **or** first DM received. Track via `usePushPromptEligibility` hook reading from notifications context.
- **Acceptance:** fresh registration → no prompt; after first message received → prompt appears.

#### M-6. Touch targets below 44pt
- **Files:**
  - `frontend/components/ui/BottomSheet.tsx:48` (`p-1.5` close)
  - `frontend/components/home/ProfileCompletenessBanner.tsx:97`
  - `frontend/components/nav/TopBar.tsx:63-68` (bell)
- **Fix:** uniformly `p-2.5` minimum on icon buttons, or wrap in `min-h-11 min-w-11`.
- **Acceptance:** lighthouse a11y "tap targets" passes; manual test with smallest finger.

### Web / desktop / a11y

#### W-1. Browse filters in bottom sheet on desktop
- **File:line:** `frontend/app/[locale]/browse/page.tsx:32`
- **Fix:** render `<FilterPanel />` as sticky left rail at `lg:`; bottom sheet only below `lg:`.
- **Acceptance:** on desktop, filters visible without click; on mobile unchanged.

#### W-2. Chat is single-column on desktop
- **File:line:** `frontend/app/[locale]/chat/[roomId]/page.tsx`
- **Fix:** at `lg:`, two-pane layout — room list (320px) on left, active room on right.
- **Acceptance:** desktop chat shows both panes; switching room is instant.

#### W-3. Browse filter state not in URL
- **File:line:** `frontend/app/[locale]/browse/page.tsx`
- **Fix:** sync filters to `searchParams` via `router.replace`; restore on mount.
- **Acceptance:** refresh preserves filters; URL is shareable.

#### W-4. Form inputs missing aria-invalid / aria-describedby
- **File:line:** `frontend/components/ui/Input.tsx:37-44`
- **Fix:** generate id, set `aria-invalid={!!error}` and `aria-describedby={error ? id+'-error' : undefined}`. Move focus to first invalid field after submit error.
- **Acceptance:** axe-core CI passes for form pages; VoiceOver announces errors.

#### W-5. BottomSheet has no focus trap / focus restore
- **File:line:** `frontend/components/ui/BottomSheet.tsx:40-62`
- **Fix:** port focus-trap from `Modal.tsx`; restore focus to trigger on close.
- **Acceptance:** keyboard test — Tab does not escape sheet; Escape closes and returns focus.

#### W-6. Color contrast — gray-500/600 on dark surfaces
- **Files:** `frontend/components/ui/Input.tsx:43`, `frontend/app/[locale]/chat/page.tsx:263`
- **Fix:** raise to `text-gray-400` minimum on `bg-gray-900`.
- **Acceptance:** axe-core / Lighthouse contrast checks pass for body text.

#### W-7. robots / metadata.robots not configured
- **Fix:** add `app/robots.ts` with `Disallow: /` (or be intentional about indexable routes). `metadata: { robots: { index: false } }` on profile/chat/contacts/admin layouts.
- **Acceptance:** `/robots.txt` returns disallow; profile pages have `<meta name="robots" content="noindex">`.

### Trust & Safety / privacy controls

#### TS-6. No in-app block-list management
- **Fix:** `Settings → Blocked users` listing avatar + name + Unblock action.
- **Acceptance:** block from profile → appears in list → unblock from list works.

#### TS-7. No notification toggles per category
- **Fix:** four toggles in Settings → Notifications: chat messages, contact requests, channel mentions, system. Persist in `profile_preferences`.
- **Acceptance:** toggling off chat-message notifications stops only that category.

#### TS-8. No read-receipts / typing-indicator opt-out
- **Fix:** two settings; backend respects them when emitting `typing` and `read` WS events (suppress emit AND suppress receive — symmetric).
- **Acceptance:** A turns off read receipts → A doesn't see B's reads either; symmetric for typing.

#### TS-9. No appeals process for suspended/banned users
- **Fix:** on suspension, send email with reason + a `/appeal/{token}` link (unauthenticated). Admin queue gets an "Appeals" tab.
- **Acceptance:** suspended user receives email; can submit appeal; admin sees it in queue.

---

## P2 — Medium (next phase)

### Discovery & retention
- **D-1. No ranking in browse.** Score = shared interests + recency of activity + distance, with deterministic shuffle per session. (`backend/internal/profiles/store.go` Browse query.)
- **D-2. No "pause discovery" toggle.** One boolean on `profile_preferences`.
- **D-3. No primary-photo selector** in `frontend/components/profile/PhotoGallery.tsx`.
- **D-4. Profile completeness shown but not gated.** Blur browse cards <40% completeness; CTA on user's own card.

### Communication
- **C-1.** Message reactions, reply-to threading, in-room message search.
- **C-2.** Link previews in chat (server-fetched OG metadata, cached).
- **C-3.** Image lightbox: swipe-to-close, pinch-to-zoom (`frontend/components/chat/Lightbox.tsx`).

### Groups & channels
- **G-1.** Groups: avatar, description, per-room mute, invite links, admin transfer.
- **G-2.** Channels: slowmode, per-channel kick (separate from platform ban), pinned announcements.

### i18n quality
- **I-1.** Hardcoded `"en"` locale in `chatHelpers.ts` `toLocaleDateString`. Use `useLocale()`.
- **I-2.** Hardcoded English strings in browse distance ("km away", "< 1 km away"). Move to messages.
- **I-3.** BottomNav labels: verify ES/PT don't wrap at 360px width.

### Performance / layout
- **P-1.** Page-level `"use client"` everywhere — split into server shell + client island.
- **P-2.** Lazy-load `PhotoGallery`, `Lightbox`, `CreateGroupModal` via `dynamic()`.
- **P-3.** Hardcoded font sizes (`text-[10px]`, `text-[11px]`, `text-[12px]` in chat) break Dynamic Type. Replace with Tailwind tokens.

---

## P3 — Polish

- 404 / `not-found.tsx` per locale, branded.
- `app/manifest.ts` for PWA add-to-home.
- Pull-to-refresh on chat list and browse.
- Offline banner on `navigator.onLine` change.
- In-app notification inbox (persistent log of past SSE events).
- Email digest job for dormant users (>14 days).
- Status page or in-app degraded-service banner driven by `/health`.
- Feature-flag system (env-driven minimum, e.g., `unleash` long-term).
- Maintenance-mode flag in config.
- Marketing landing page for unauthenticated visitors.

---

## Recommended PR sequence

| PR  | Title                                  | Scope                                                                                  |
|-----|----------------------------------------|----------------------------------------------------------------------------------------|
| #76 | Privacy hardening                      | S-1 contact-only fields · S-2 soft-delete filter · S-4 presence gating · S-7 rate limits |
| #77 | Secrets + CSP + WS ticket              | S-5 rotate + remove · S-6 nonce-based CSP · S-3 WS ticket exchange                       |
| #78 | Legal + data rights                    | TS-1 Terms/Privacy/Guidelines · TS-2 data export · TS-9 appeals page                     |
| #79 | Image moderation + report categories   | TS-3 NSFW classifier · TS-4 expanded categories · message-level reports                  |
| #80 | Mobile safe-area + composer            | M-1 safe-area · M-2 input zoom fix · M-3 textarea composer · M-4 capture · M-6 targets   |
| #81 | Desktop split-view + filters rail      | W-1 desktop filters · W-2 chat split-view · W-3 URL state                                |
| #82 | Accessibility pass                     | W-4 form ARIA · W-5 BottomSheet focus trap · W-6 contrast · W-7 robots                   |
| #83 | Privacy controls UI                    | TS-6 block list · TS-7 per-category notif · TS-8 read-receipts toggle · D-2 pause       |
| #84 | Discovery v2                           | D-1 ranking · D-3 primary photo · D-4 completeness gating                                |
| #85+| Communication v2                       | Reactions → reply-to → search → link previews                                            |

PRs #76–#79 unblock real-user launch. The rest is differentiation.

---

## Acceptance gates per PR

Every PR in this plan must:
- Update `docs/implementation-plan.md`, `README.md`, `CHANGELOG.md`.
- Pass full CI locally (`./run-ci-local.sh`) before push.
- Hold 98%+ coverage on handler + service layers (store methods integration-only).
- Target `develop`, not `main`.
- Use the established commit/PR style (single-line commits, no Co-Authored-By, PR template per #2).

---

## Out of scope (explicitly deferred)

- Identity verification beyond DOB self-declaration (TS-5 stays at "audit log + policy doc" for now).
- Native iOS/Android apps — current scope is web + PWA.
- Monetization (premium tier, ads, boosts).
- A/B testing infrastructure beyond simple env-driven flags.
- Voice / video calling.
