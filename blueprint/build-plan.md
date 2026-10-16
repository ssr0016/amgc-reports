# Build Plan — AMGCReports

> Shipped items are checked from the real code and git history. Roadmap items come from the owner. `TODO (confirm)` marks an inferred split.

## Shipped

- [x] Backend foundation: config, logger, Echo server, health, metrics, apperror, validator, pagination
- [x] Database: goose migrations, seed, testcontainers helpers
- [x] Sessions, CSRF, rate limiting, body limit, trusted-proxy client IP
- [x] Users, roles, permissions, RBAC middleware
- [x] Auth: login, `/auth/me`, logout
- [x] Email verification and password-reset API
- [x] Change-password endpoint, UI and audit action (`CHANGE_PASSWORD`)
- [x] Reports API: create, read, update, soft delete, approve/unapprove, previous-month rule (D-F8), audit log
- [x] Admin users API, editor loses approve (`20261014120000`)
- [x] OpenAPI generation with generated frontend types
- [x] Frontend F1–F13: app shell, login, session, reports list/new/edit, admin users list, permission-gated UI
- [x] Frontend tests: Vitest (144) and Playwright (47), including axe a11y, keyboard and reflow checks
- [x] CI: Go lint/test/integration/build/gosec, frontend lint/typecheck/build, e2e
- [x] Production deploy on Railway; H-4 + API migrate verified in prod

> TODO (confirm): F-1 (Commits 1–4) and L-3 are recent commits the owner named. I did not find their scope in the survey, so they are covered under the items above and not listed on their own.

## P0 — Decide first

- [ ] Finding #1: decide whether a plain `user` may see all reports (`read_any`)

## P1 — Launch

- [ ] F0: domain setup (`amgreports.com` is taken; pick another)
- [ ] F14: production smoke test (blocked on F0)
- [ ] Rotate the Railway Postgres password
- [ ] Seed idempotency

## P2 — Admin Panel

- [ ] Phase A (~28h)
- [ ] Phase B (~21h)
- [ ] Phase C (~32h), including exports

> Phase contents are in `backend/docs/06-Implementation-Plan.md` (commit `a18b572`). `TODO (confirm)`: split them into tasks when you start Phase A.

## P3 — Email

- [ ] E-1: email delivery
- [ ] Password recovery (forgot-password flow)
- [ ] Reset-password UI

## Deferred

- [ ] D-F7: Cloudflare proxy and IP-limit handling

## Not planned

Dashboards and notifications are not on the roadmap.
