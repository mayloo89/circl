# Contributing to Circl

Thanks for your interest in contributing. This document covers the workflow
and the conventions the project holds itself to.

## Getting started

Follow the [local setup guide in the README](README.md#local-setup) to get the
backend, frontend, Postgres, and Redis running. The full stack (including
MinIO, Mailpit, and the observability suite) is available via
`docker compose up`.

## Workflow

- All pull requests target **`develop`**, never `main`.
- Branch from `develop`; keep branches focused on one change.
- Every PR updates the three living documents where relevant:
  - `docs/implementation-plan.md` — the single source of truth for scope and
    roadmap.
  - `CHANGELOG.md` — Keep-a-Changelog format, entry under `[Unreleased]`.
  - `README.md` — if the change is user- or operator-visible.
- PR descriptions follow the house structure: **Overview, Changes, Security,
  Testing, Next Steps, Related**.
- All artefacts — branches, commits, PRs, code, comments — are written in
  English.

## Commit style

- A single short sentence in English describing the change.
- No conventional-commit prefixes (`feat:`, `fix:`, `chore:`).
- No commit body, no trailers.

```
Fail closed in auth middleware when NextAuth session resolution errors
```

## Quality gates

Run the full local CI before pushing (spins up Postgres + Redis in Docker):

```bash
./run-ci-local.sh
```

Or the individual gates:

```bash
# frontend
cd frontend
npx tsc --noEmit && npm run lint && npm run test && npm run build

# backend
cd backend
go test ./...
golangci-lint run
```

- Handler and service layers aim for **98%+ unit coverage**; store methods are
  integration-test-only.
- OpenAPI changes must pass `npx @redocly/cli lint docs/openapi.yaml`.

## Code conventions

- **Build for scale** — clean, scalable solutions over pragmatic shortcuts.
- **Comments** — default to none; add one only when the *why* is non-obvious
  (hidden constraint, surprising invariant, workaround for a specific bug).
- **Validation** — validate at system boundaries; trust internal code.
- **Modern Go** — `errors.Is`, `cmp.Or`, `omitzero` JSON tags, `for range n`,
  `strings.Cut`, `t.Context()`.
- **i18n** — every user-facing string goes through `useTranslations(...)`;
  all three locales (`en`/`es`/`pt`) are updated together, including
  `aria-label` values.
- **Accessibility** — dialogs use `useFocusTrap`, custom menus use
  `useMenuKeyboard`, decorative SVGs carry `aria-hidden="true"`, icon-only
  buttons carry `aria-label`.

## Architectural invariants

Some structural decisions are deliberate — please don't "fix" them:

- Messages (`/chat`) and channels (`/chat/channels`) are **separate surfaces**;
  channels are single-pane on every breakpoint because leaving a channel is
  irreversible.
- WebSocket auth uses single-use Redis tickets — never a JWT in the URL.
- The CSP uses a per-request nonce; `'unsafe-inline'` must not return to
  `script-src`.
- Public profile responses are redacted for non-contacts; presence is gated to
  accepted contacts.

## Security issues

Never open a public issue for a vulnerability — see [SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions are licensed under the
[GNU AGPL-3.0](LICENSE), the project's license.
