# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

---

## [0.4.1] - 2026-03-14 — Fix remove contact

### Fixed
- `ListAccepted` now returns `AcceptedContact` (with `contact_id`) instead of `UserSummary` so the frontend passes the correct row ID to `DELETE /contacts/{id}`
- Removed dangling `displayName` helper in the contacts page that caused a `ReferenceError` in the search results section

### Changed
- Added `AcceptedContact` type (`contact_id`, `user_id`, `email`, `display_name`) in the contacts package
- `scanAcceptedContacts` helper added in store layer (mirrors `scanPendingRequests` / `scanSentRequests`)

---

## [0.4.0] - 2026-03-14 — Contacts UX improvements and routing fix

### Fixed
- Auth handler routing: replaced `net/http.ServeMux` with chi router so path stripping works correctly when mounted at `/auth` (was returning 404 on login/register)
- Accept contact: `ListPending` now returns `PendingRequest` (with `contact_id`) so the frontend passes the correct row ID to `PUT /contacts/{id}/accept`

### Added
- `GET /contacts/sent` endpoint — outgoing pending requests with `SentRequest` type so users can see and cancel them
- `PendingRequest` and `SentRequest` types expose `contact_id` for accept/decline/cancel actions
- Search results exclude users who already have any contact relationship with the caller (`NOT EXISTS` subquery)
- Dark theme across all frontend pages (`bg-gray-950` base, `bg-gray-900` cards, white/gray text)
- Sent requests section in the contacts page with a Cancel button

---

## [0.3.0] - 2026-03-14 — Contacts system

### Added
- `contacts` table migration (status machine: `pending → accepted`, `blocked`; unique pair constraint; no self-contact)
- `GET /users/search?q=` — user search by email or display name (ILIKE, 20-row limit, excludes self)
- `POST /contacts` — send a contact request
- `GET /contacts` — list accepted contacts
- `GET /contacts/pending` — list incoming pending requests
- `PUT /contacts/{id}/accept` — accept a pending request
- `DELETE /contacts/{id}` — remove or decline a contact
- Contacts page in the frontend with search, pending requests, and contact list sections

### Changed
- `server.New()` accepts `contactsHandler` and `requireAuth` parameters
- Auth handler routes changed to relative paths (`POST /login`, `POST /register`) and mounted at `/auth`

---

## [0.2.0] - 2026-03-14 — Private profiles and JWT auth

### Added
- JWT HS256 tokens issued on login/register (`golang-jwt/jwt/v5`)
- `RequireAuth` middleware validates Bearer tokens and injects `userID` into context
- `profiles` table migration with `display_name` and `bio`
- `GET /profiles/me` — lazy profile creation on first fetch
- `PUT /profiles/me` — update display name and bio
- Profile page in the frontend
- `SessionProvider` wrapper so `useSession()` works in client components
- `accessToken` exposed through NextAuth session callbacks

---

## [0.1.0] - 2026-03-14 — Auth foundation

### Added
- Project structure: Next.js 15 frontend, Go 1.25+ backend, chi router
- PostgreSQL connection pool with `pgx/v5`, `golang-migrate` for schema migrations
- `users` table with bcrypt password hashing
- `POST /auth/register` — create account with email + password validation
- `POST /auth/login` — authenticate and return user
- Login and register pages (Next.js, NextAuth.js credentials provider)
- `GET /health` endpoint
- CORS configuration via environment variable
- GitHub Actions CI (lint + test on push/PR)
