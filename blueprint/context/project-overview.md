# AMGCReports - Project Overview

<!-- blueprint:source-hash 905af16253e0bc8a1711a0b1c68c60b8dce9ad70976e6772203de29995caefe0 -->

> Pastoral reports management for 24 churches: church workers file monthly reports (5 weeks x 21 metrics), admins review and approve.

## Problem

Pastoral reporting across 24 churches was collected piecemeal. AMGCReports gives church workers one place to submit reports and admins one place to review, approve and export them.

## Users

| Role | Access |
|------|--------|
| admin | Full access: approve, late-file a previous month, admin panel, exports, user and role management |
| editor | `write_any` + `delete_any`; cannot approve (`20261014120000_revoke_editor_approve`) |
| user | Church worker; `read_any` sees every report's metadata and counts, not free text or `tithes_offering` (D-18) |
| custom roles | Planned in 37a: admin-defined roles (e.g. "Reviewer") with a chosen set of permissions |

Closed system (D-A18): no self-registration; admins create accounts and set passwords.

## Usage model

- Internal tool, no monetization (`TODO (confirm)` in the plan).
- Rate limiting, body limit and trusted-proxy client IP are enforced. The Cloudflare proxy would break IP-based limits (D-F7, deferred).
- Every mutation is audited in the same transaction. Sign-ins, sign-outs and refused sign-ins are audited too (21a).
- Accessibility is a stated bar: axe, keyboard, reflow at 320 px, reduced motion.

## Features

In build-plan order. Done: 1-15, 21, 22, 27, and 20 except 20e.

1-14, 27. **Platform (shipped)** - backend foundation, database and seed, sessions/CSRF/limits, users and RBAC middleware, auth, verification and reset API, change password (F-1), OpenAPI types, frontend F1-F13, tests, CI, and the Render + Neon deploy; L-3 reset-password strongpass.
8. **Reports API (headline)** - create, read, update, soft delete, approve/unapprove, previous-month rule (D-F8), audit.
15. **Finding #1 (D-18)** - free text and tithes hidden from non-owners.
17. **F14 production smoke test** - blocked on F0 (16, deferred).
18. **Rotate the Neon Postgres password.**
19. **Seed idempotency.**
20. **Admin panel Phase A** - 20a shell and tabs, 20b stats cards, 20c Reports tab, 20d Users tab, 20f sidebar and profile menu (done); **20e preview modal** (open: metadata, the two metric tables, Avg per D-A5).
21. **Admin panel Phase B (done)** - 21a Audit Logs UI (`LOGIN`, `LOGOUT`, `LOGIN_FAILED`, admin-only `GET /admin/audit-logs`, read-only Logs tab, D-A20); 21b Tracker tab (D-A3).
22. **Admin panel Phase C (done)** - 22a user status (D-A10), 22b soft delete and restore (D-A11), 22c owner badges on reports, 22d password confirm field, 22e admin reset password (D-A12), 22g Excel export in the legacy layout, 22f export preview (D-A15), 22h bulk Report Dashboard and approved-only sheets (D-A16).
37. **Dynamic RBAC (next, D-A21)**
    - 37a Roles tab: custom roles with a grouped permission checklist, `roles:write`, no delete while users hold the role, built-in roles read-only, role dropdown on Create and Edit User with last-admin and self protection, audited, with a migration.
    - 37b permission-gated admin panel: `reports:export`, `tracker:read`, `audit:read` and `users:list` (seeded, admin only by default) open the review tabs to custom roles; account and role management stay admin-only.
28-36. **Backlog (not specced)** - 28 password rules in the UI, 29 idle sign-out in the browser, 30 audit-log CSV export, 31 empty and loading states pass, 32 copy password on create-user, 33 avatars, 34 Neon backup and restore test, 35 Sentry alerts, 36 open P3 findings.
16, 26. **Deferred** - 16 F0 domain setup (`amgcreports.com` chosen); 26 D-F7 Cloudflare proxy and IP limits.
23-25. **Skipped (D-A18)** - E-1 email delivery, forgot-password flow, reset-password UI.

Not planned: dashboards, notifications.

## Data model

PostgreSQL via goose (`backend/internal/database/migrations/`). Exact columns live in the migrations and `backend/docs/05`.

### users

- Profile: `email` (unique, reserved even after delete), `name`, `area_assignment`, `church_name`.
- `role_id` -> roles. One role per user.
- `status`: one of `active`, `inactive`, `disciplinary` or `deceased`. Login is refused unless the status is `active` (22a).
- Soft delete with a reason, and restore (22b).

### roles / permissions / role_permissions

- Roles map to permissions through `role_permissions`.
- Built-in roles: `admin`, `editor` and `user` (`user` is the default). Their grants are seed data in `seed.go`, applied on every API start.
- Permission names are `resource:action` over users, roles, permissions and reports (e.g. `reports:approve`, `roles:write`).
- 37a adds custom roles. The built-in roles stay read-only.

### reports

- One per user, month and year; 5 weeks x 21 metrics.
- A blank week means "not reported", which is different from 0 (D-5).
- Worker, area and church are prefilled from the profile.
- `completed`: Pending, then Approved.
- Soft delete.
- Free text and `tithes_offering` are owner-only (D-18).

### audit log

- Written in the same transaction as the change, with a details allow-list per action.
- A new action needs a CHECK-constraint migration in the same deploy.
- Report actions: `CREATE_REPORT`, `UPDATE_REPORT`, `DELETE_REPORT`, `APPROVE_REPORT`, `UNAPPROVE_REPORT`.
- User actions: `CREATE_USER`, `UPDATE_USER`, `CHANGE_PASSWORD`, `CHANGE_USER_STATUS`, `ADMIN_RESET_PASSWORD`, `DELETE_USER`, `RESTORE_USER`.
- Export actions: `DOWNLOAD_SINGLE_EXCEL`, `DOWNLOAD_BULK_EXCEL`.
- Sign-in actions: `LOGIN`, `LOGOUT`, `LOGIN_FAILED`.
- 37a adds `CREATE_ROLE`, `UPDATE_ROLE`, `DELETE_ROLE` and `CHANGE_USER_ROLE`.

### tokens and sessions

- Refresh tokens (`20261021100000_create_refresh_tokens`), plus verification tokens and password resets.

## Tech stack

- **Go 1.26, Echo, pgx, goose, swaggo, golangci-lint v2, air** - backend.
- **Next.js 16.4, React 19, TypeScript 5, Tailwind 4, shadcn/Radix, TanStack Query** - frontend.
- **openapi-typescript** (`npm run gen:api`) - API types from Swagger; CI fails on drift.
- **Go unit + testcontainers, Vitest, Playwright + axe** - tests.
- **GitHub Actions** - lint, test, integration, build, gosec, frontend, e2e.

## Monetization

None. Internal tool.

## UI/UX

shadcn over Radix, Tailwind 4, Sonner toasts. Skip link, focus management, announced list updates. Copy rules: `backend/docs/04-Content-Guidelines.md`.

- `/login`: sign in.
- `/reports`, `/reports/new`, `/reports/:id/edit`: the reports list and the report form.
- `/account/password`: change password.
- `/admin?tab=` with these tabs:
  - `reports`, `tracker`, `users`, `logs`
  - `roles` (planned in 37a)
- `/admin/users/new`, `/admin/users/:id/edit`.

## Deployment

- **API:** Render (Docker, root `backend`). **Database:** Neon (Postgres). **Frontend:** Vercel. **DNS:** Cloudflare.
- **Migrations:** production does not auto-migrate, and Render's Free tier runs neither the Pre-Deploy Command nor Shell. Run migrations locally against Neon before pushing `main`: `make migrate-up DATABASE_URL='<Neon owner URL>'` (H-1).
- **Domain:** `amgcreports.com` is chosen but not set up (16, deferred).

> TODO: env var names, health path and build commands are not in the plans (see `backend/SETUP.md`).

## Open questions

- See the contradictions reported by `/overview`; fix them in `project-plan.md`, then re-run.
