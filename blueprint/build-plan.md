# Build Plan — AMGCReports

> Shipped items are checked from the real code and git history. Roadmap items come from the owner. `TODO (confirm)` marks an inferred split. IDs are append-only; new items may not be sequential.

## Shipped

- [x] 1. Backend foundation: config, logger, Echo server, health, metrics, apperror, validator, pagination
- [x] 2. Database: goose migrations, seed, testcontainers helpers
- [x] 3. Sessions, CSRF, rate limiting, body limit, trusted-proxy client IP
- [x] 4. Users, roles, permissions, RBAC middleware
- [x] 5. Auth: login, `/auth/me`, logout
- [x] 6. Email verification and password-reset API
- [x] 7. F-1: change-password endpoint, UI and audit action (`CHANGE_PASSWORD`)
- [x] 8. Reports API: create, read, update, soft delete, approve/unapprove, previous-month rule (D-F8), audit log
- [x] 9. Admin users API, editor loses approve (`20261014120000`)
- [x] 10. OpenAPI generation with generated frontend types
- [x] 11. Frontend F1–F13: app shell, login, session, reports list/new/edit, admin users list, permission-gated UI
- [x] 12. Frontend tests: Vitest (144) and Playwright (47), including axe a11y, keyboard and reflow checks
- [x] 13. CI: Go lint/test/integration/build/gosec, frontend lint/typecheck/build, e2e
- [x] 14. Production deploy on Railway; H-4 + API migrate verified in prod
- [x] 27. L-3: reset-password strongpass enforcement

## P0 — Done

- [x] 15. Finding #1: plain `user` keeps `read_any`, but free text and `tithes_offering` are hidden from non-owners (D-18)

## P1 — Launch

- [ ] 16. F0: domain setup (`amgreports.com` is taken; pick another)
- [ ] 17. F14: production smoke test (blocked on F0)
- [ ] 18. Rotate the Railway Postgres password
- [ ] 19. Seed idempotency

## P2 — Admin Panel

- [ ] 20. Phase A (~28h)
- [ ] 21. Phase B (~21h)
- [ ] 22. Phase C (~32h), including exports

> Phase contents are in `backend/docs/06-Implementation-Plan.md` (commit `a18b572`). `TODO (confirm)`: split them into tasks when you start Phase A.

## P3 — Email

- [ ] 23. E-1: email delivery
- [ ] 24. Password recovery (forgot-password flow)
- [ ] 25. Reset-password UI

## Deferred

- [ ] 26. D-F7: Cloudflare proxy and IP-limit handling

## Not planned

Dashboards and notifications are not on the roadmap.
