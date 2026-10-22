# AMGCReports - Project Overview

<!-- blueprint:source-hash 6b6934f80cba2b5b33588158bed6e3d5941c541a24fd22c019db95c575f808df -->

> Pastoral reports management for 24 churches: workers file weekly reports, admins review and approve.

## Problem

Pastoral reporting across 24 churches was collected piecemeal. AMGCReports gives church workers one place to submit reports and admins one place to review and approve them.

## Users

| Role | Access |
|------|--------|
| admin | Full access, including approve and late-filing a previous month |
| editor | `write_any` + `delete_any`; cannot approve (`20261014120000_revoke_editor_approve`) |
| user | `read_any`; sees every report's metadata and counts, not free text or `tithes_offering` (D-18) |

## Usage model

- Internal tool, no monetization.
- Rate limiting, body limit and trusted-proxy client IP are enforced. The Cloudflare proxy breaks IP-based limits (D-F7, deferred).
- Every mutation is audited. Accessibility is a stated bar (axe, keyboard, reflow, reduced motion).

## Features

Build order from `build-plan.md` (27 items). Shipped (1-15 and 27) are done.

1. **Backend foundation** - config, logger, Echo, health, metrics, apperror, validator, pagination.
2. **Database** - goose migrations, seed, testcontainers helpers.
3. **Sessions/CSRF/limits** - sessions, CSRF, rate limit, body limit, trusted-proxy IP.
4. **Users/RBAC** - users, roles, permissions, RBAC middleware.
5. **Auth** - login, `/auth/me`, logout.
6. **Verification/reset API** - email verification tokens, password reset.
7. **Change-password (F-1)** - endpoint, UI, `CHANGE_PASSWORD` audit.
8. **Reports API (headline)** - create/read/update/soft delete, approve/unapprove, previous-month rule (D-F8), audit.
9. **Admin users API** - admin CRUD; editor loses approve.
10. **OpenAPI** - generated frontend types.
11. **Frontend F1-F13** - shell, login, session, reports list/new/edit, admin users list, permission-gated UI.
12. **Frontend tests** - Vitest (144), Playwright (47) with axe.
13. **CI** - Go lint/test/integration/build/gosec, frontend checks, e2e.
14. **Prod deploy** - Render (API) and Neon (Postgres); H-4 + API migrate verified in prod.
15. **Finding #1 (D-18)** - hide free text and tithes from non-owners.
16. **F0 domain setup** - `amgcreports.com` chosen; deferred to the end (listed under Deferred in the plan).
17. **F14 prod smoke test** - blocked on 16 (deferred).
18. **Rotate Neon Postgres password.**
19. **Seed idempotency.**
20. **Admin Panel Phase A** (~28h, no schema change; tests/axe/docs spread over the sub-items), in order: 20a shell and tabs, 20b stats endpoint and cards, 20c Reports tab, 20d Users tab, 20e preview modal, 20f sidebar navigation and profile menu.
21. **Admin Panel Phase B** (~21h).
22. **Admin Panel Phase C** (~49h) - includes exports and user status, in order: 22a user status (D-A10: `status` active, inactive, disciplinary or deceased; login check after the password; session revoke in the same transaction; `PUT /admin/users/:id/status`; badge, filter and Edit dropdown; `CHANGE_USER_STATUS` audit and migration; self and last-admin protection), 22b soft delete and restore of users (D-A11: reason, confirmation dialog, Deleted filter, email stays reserved, audit and migration), 22c reports of deleted and deceased users (owner status in the reports API, "Deleted user" and "Archived" badges), 22d password confirm field (`confirm_password` required and equal to the new password, 422 on `confirm_password`, in the create-user, change-password and public reset-token requests and forms), 22e admin reset password (D-A12: `POST /admin/users/:id/reset-password`, admin only; `new_password` and `confirm_password`; ends the user's sessions and clears the lockout; refused for your own account; `ADMIN_RESET_PASSWORD` audit and migration; Reset password button and dialog with generate and copy on the Edit User page), 22g Excel export (shipped as 22 in `cf0fcb6`: admin-only `POST /admin/reports/:id/excel` and `POST /admin/reports/excel` in the legacy layout, `DOWNLOAD_*_EXCEL` audit), 22f Excel export preview (D-A15: row "Excel" and "Download Bulk" open a modal with the workbook's sheets as read-only tables from new `POST /admin/reports/excel/preview`, admin only, single and bulk, no audit row; sheet selector for bulk; only the modal's "Download" calls the existing export, which rebuilds the file and writes `DOWNLOAD_*_EXCEL`; "Cancel" means no download and no audit), 22h bulk export dashboard and approved-only sheets (D-A16: the bulk export and its preview start with a "Report Dashboard" sheet, one row per non-admin, non-deleted account with a church name, columns Church, Worker, Area, Status = Approved, Pending or Not Filed for the selected month; metric sheets only for approved reports in the D-A6 layout; a month with none still downloads with the dashboard alone; the 100 cap and the audit `count` cover approved sheets; single export unchanged).
23. **E-1 email delivery.**
24. **Forgot-password flow** - needs 23.
25. **Reset-password UI** - needs 23.
26. **D-F7 Cloudflare proxy/IP-limit handling** - deferred.
27. **L-3 reset-password strongpass** - enforced on `/auth/reset-password` (shipped).

Not planned: dashboards, notifications.

## Data model

PostgreSQL via goose. Latest migration `20261021120000_add_user_soft_delete`.

### reports

- One per month; 5 weeks x 21 metrics per week.
- Worker, area, church prefilled from the user profile.
- `status` Pending -> Approved; soft delete.
- Free text and `tithes_offering` visible to owner only (D-18).
- Month is evaluated in PHT; only admin can late-file (D-F8).

### users / roles / permissions / role_permissions

- User has a role; roles map to permissions. Seeded on API start (permissions, roles, admin).

### audit log

- Written in the same transaction as the change. 14 actions: `CREATE_REPORT`, `UPDATE_REPORT`, `DELETE_REPORT`, `APPROVE_REPORT`, `UNAPPROVE_REPORT`, `CREATE_USER`, `UPDATE_USER`, `CHANGE_PASSWORD`, `DOWNLOAD_SINGLE_EXCEL`, `DOWNLOAD_BULK_EXCEL`, `CHANGE_USER_STATUS`, `DELETE_USER`, `RESTORE_USER`, `ADMIN_RESET_PASSWORD`.
- A new action needs a CHECK-constraint migration in the same deploy.

### verification tokens, password resets, sessions

- Support email verification, reset and login sessions.

> TODO: exact columns live in `backend/docs/05` and the migrations; the plans list tables only.

## Tech stack

- **Go 1.26, Echo, pgx, goose, swaggo, golangci-lint v2.13.2, air** - backend.
- **Next.js 16.4, React 19.3, TypeScript 5, Tailwind 4, shadcn/Radix, TanStack Query** - frontend.
- **openapi-typescript** (`npm run gen:api`) - API types; CI fails on drift.
- **Go unit + testcontainers, Vitest, Playwright + axe** - tests.
- **GitHub Actions** (`.github/workflows/ci.yml`) - CI.

## Monetization

None. Internal tool.

## UI/UX

shadcn over Radix, Tailwind 4, Sonner toasts. Skip link, focus management, announced list updates. Copy rules: `backend/docs/04-Content-Guidelines.md`.

> TODO: route list is not in the plans (see `backend/docs/07`).

## Deployment

- Render: backend; Neon: Postgres. Docker compose for local/prod. Frontend: Vercel (`app.<domain>`). DNS on Cloudflare (per `AGENTS.md`).
- Production does not auto-migrate, and Render's Free tier runs neither the Pre-Deploy Command nor Shell: run migrations locally against Neon (`make migrate-up DATABASE_URL='<Neon owner URL>'`) before pushing `main` (H-1).
- Domain: `amgcreports.com` chosen, not set up; feature 16 deferred.

> TODO: env var names, health path, and build commands are not in the plans.

## Open questions

- Phase B is unsplit (`backend/docs/08-Frontend-Plan.md` section 10). Phase A is split into 20a-20f and Phase C into 22a-22h (22g is the shipped export, listed before 22f).
- D-A10 and D-A11 are named by the build plan but not yet recorded in `docs/DECISIONS.md`; 22a records them with the D-A1 amendment.
- Monetization "none" is `TODO (confirm)`: no decision yet.
