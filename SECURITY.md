# Security Policy

Circl is a privacy-first contact platform: private profiles, secure chat, and
strong data-minimization guarantees. Security reports are taken seriously and
handled with priority.

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Report vulnerabilities privately via
[GitHub private vulnerability reporting](../../security/advisories/new)
(the *Security* tab → *Report a vulnerability*).

Include what you can:

- A description of the issue and its impact.
- Steps to reproduce (a curl transcript or a minimal PoC is ideal).
- The affected component (frontend route, API endpoint, WebSocket flow, worker).

You can expect an acknowledgement within **72 hours** and a status update as
the report is triaged. Please allow a reasonable window for a fix to be
developed and deployed before any public disclosure.

## Scope

In scope:

- The Go backend API and WebSocket/SSE surfaces (`backend/`).
- The Next.js frontend, middleware, and session handling (`frontend/`).
- Authentication, authorization, and privacy guarantees — anything that lets a
  user read data (profiles, messages, presence, albums) they should not see.
- The image upload, processing, and moderation pipeline.
- Rate-limit or anti-abuse bypasses.

Out of scope:

- Denial of service by raw volume.
- Reports from automated scanners with no demonstrated impact.
- Issues requiring a compromised device or physical access.
- Social engineering of the operator or users.

## Handling of user data in reports

If reproducing an issue would expose real user data, stop at the proof of
concept and describe the rest. Never exfiltrate, retain, or share user content
encountered during research.
