# AMGCReports - Project Overview

<!-- blueprint:source-hash a11a2d6c10a6f9267ebca2bb4904cba402a44ea8b8024ce1fd27172ecba74f0c -->

> Pastoral reports management for 24 churches: monthly submissions with 5 weeks × 21 metrics, admin review, approval and Excel export.

## Problem

Pastoral reporting across 24 churches is collected piecemeal. AMGCReports gives workers one place to submit reports and administrators one place to review and approve them. Weekly activity is recorded inside one monthly report.

## Users

| Role | Access established by the plans |
|---|---|
| admin | Full access, approvals, user and role management, Excel export; project plan also allows late-filing |
| editor | `write_any` and `delete_any`; cannot approve |
| user | Church worker; `read_any` exposes others' metadata and counts while hiding free text and tithes (D-18) |
| custom role | Administrator-defined permissions; admin tabs and routes open only when permissions allow (37a–37b) |

## Usage model

- Internal reporting for 24 churches; no monetization, subject to the project's confirmation note.
- Closed system with admin-managed passwords (D-A18); email delivery and public password-recovery UI are explicitly skipped in the build plan.
- Session authentication, CSRF, rate limiting, request body limits and trusted-proxy client IP handling.
- Audit changes with their mutations in the same transaction where the feature specifies it. A new audit action requires its CHECK-constraint migration in the same deploy.
- Accessibility includes axe checks, keyboard focus, skip links, announced updates and reduced motion.
- Cloudflare proxy/IP-limit handling is deferred (D-F7). Broad dashboards and notifications are outside the roadmap, with specific scoped dashboard/alert items noted below.

## Features

Order and identities follow `build-plan.md`; checked items are shown as shipped unless explicitly skipped. Do not infer completion of unchecked items from existing code or test counts.

| ID | State | Purpose |
|---|---|---|
| 1 | Shipped | Backend config, logging, Echo, health, metrics, error handling, validation and pagination |
| 2 | Shipped | Database migrations, seed and testcontainers helpers |
| 3 | Shipped | Sessions, CSRF, rate limits, body limits and trusted-proxy addresses |
| 4 | Shipped | Users, roles, permissions and RBAC middleware |
| 5 | Shipped | Login, `/auth/me` and logout |
| 6 | Shipped API | Email verification and password-reset endpoints |
| 7 | Shipped | Change password API/UI and `CHANGE_PASSWORD` audit |
| 8 | Shipped; headline | Monthly report create/read/update/delete, approve/unapprove and previous-month rule |
| 9 | Shipped | Admin user management; editor approval permission revoked |
| 10 | Shipped | Swagger-derived frontend API types; CI rejects drift |
| 11 | Shipped | App shell, login, report screens, admin users and permission gates |
| 12 | Shipped | Vitest and Playwright coverage, including accessibility, keyboard and reflow |
| 13 | Shipped | CI lint, unit/integration tests, builds, gosec, frontend and E2E |
| 14 | Shipped | Render API and Neon database production deployment |
| 27 | Shipped | Strong-password enforcement on reset-password API |
| 15 | Shipped | Hide report free text and tithes from non-owners (D-18) |
| 17 | Open; blocked | Production smoke test awaits deferred domain setup (16) |
| 18 | Complete | Neon password rotated; owner verified production connectivity and superadmin login on 2026-10-24 |
| 19 | Complete | Seed idempotency verified; archive records the existing integration test and its passing evidence |
| 20 | Complete | Phase A: all children 20a–20f checked |
| 20a | Shipped | Admin shell, accessible tabs, guard, header link and titles |
| 20b | Shipped | Stats endpoint and four cards, with generated API types |
| 20c | Shipped | Reports tab with filtering, approval and empty state |
| 20d | Shipped | Users tab, own-account badge and role filter |
| 20e | Shipped | Report preview metadata, two metric tables and averages (D-A5) |
| 20f | Shipped | Sidebar and profile menu (Profile, Change Password, Sign Out); Menu drawer below 1024 px |
| 21 | Complete | Phase B: audit logs and tracker |
| 21a | Shipped | Read-only Logs tab, expandable allow-listed details, Manila period/action filters, 50 per page, no IP; sign-in/out/failure auditing |
| 21b | Shipped | Submission Tracker tab (D-A3) |
| 22 | Complete | Phase C: account lifecycle and Excel exports |
| 22a | Shipped | User status changes, login refusal, session revocation, filters/badges and audit |
| 22b | Shipped | Soft delete/restore users with reason, confirmation and reserved email |
| 22c | Shipped | Deleted/deceased owner badges on retained reports |
| 22d | Shipped | Required matching `confirm_password`; API errors on that field |
| 22e | Shipped | Admin resets another user's password, ends sessions, clears lockout and audits; dialog offers generation/copy; self-reset refused |
| 22g | Shipped | Admin-only single/bulk Excel downloads in legacy layout, each audited |
| 22f | Shipped | Read-only workbook preview and sheet selector; only Download writes export audit |
| 22h | Shipped | Bulk Report Dashboard lists every church as Approved/Pending/Not Filed; metric sheets include approved reports only |
| 37 | Complete | Dynamic RBAC (D-A21) |
| 37a | Shipped | Custom role CRUD and permission checklist; built-ins read-only; occupied roles cannot be deleted; self/last-admin protections and audit |
| 37b | Shipped | Permission-gated admin routes/tabs for custom roles |
| 42 | Complete | Report workflow |
| 42a | Shipped | Pending, Approved and Needs Revision statuses (Draft removed, D-A25); unapprove moves to Needs Revision |
| 42b | Shipped | Admin notes on review actions; UI requires a note to unapprove |
| 42c | Shipped | Revision requests; a saved Needs Revision report returns to Pending with `last_revised_at` and user notes |
| 42d | Removed (D-A25) | Private server-backed drafts, left out of other readers' lists, stats and the Tracker |
| 42e | Removed (D-A25) | Submit a draft for review (`PATCH /reports/:id/submit`, `SUBMIT_REPORT`) |
| 42f | Partly removed (D-A25) | Server autosave removed; the browser backup stays, and deleting a report clears it |
| 42g | Shipped | One current report per filing period (`GET /reports/current`), no duplicate starts |
| 43 | Complete | Admin review tools |
| 43a | Shipped | Bulk approve/unapprove; selection survives filters |
| 43b | Shipped | Tracker submission and review statuses |
| 43c | Shipped | Community Reports feed, separate from personal reports and admin management |
| 43d | Shipped | Excel preview styled like the exported workbook |
| 28 | Backlog | Show and validate password-policy rules while typing |
| 29 | Backlog | Browser idle detection, warning and sign-out after 30 minutes; server idle TTL already exists |
| 30 | Backlog | Audit-log CSV export; decide whether export itself needs a new audit action/migration |
| 31 | Backlog | Consistent empty/loading states on non-admin pages |
| 32 | Backlog | Copy password on create-user; already present in reset-password dialog |
| 33 | Shipped | Profile-picture upload (resized in the browser, R2 storage) and display; initials fallback |
| 38 | Backlog | Server-side admin Users pagination; current list capped at 100 |
| 34 | Backlog | Confirm Neon retention/PITR window and test restore |
| 35 | Backlog | Sentry alerts to Slack or email |
| 36 | Backlog | Address the 12 open P3 findings; consult findings ledger before repair |
| 16 | Deferred | Domain setup; `amgcreports.com` chosen, deferred to the end by the owner |
| 26 | Deferred | Cloudflare proxy and IP-limit handling |
| 23–25 | Skipped, not built | Email delivery, forgot-password flow and reset-password UI (D-A18) |
| 40 | Partly shipped | Analytics tab: attendance charts, period filter and comparison, KPI and full metrics summaries, CSV and PDF export (40a–40c); church filter, submission compliance, organization-wide reporting start (D-A31, D-A32), Excel export and mobile table fix (40e); multi-year growth charts open (40d) |

The first unchecked leaf is 17 and is explicitly blocked on deferred domain setup (16). Feature 19 and Phase A are complete. Preserve the plan's IDs, including 22g before 22f.

## Data model

These are logical field contracts derived from the plans, not a replacement for the existing migrations. PostgreSQL identifiers, exact column types, keys and all unspecified fields must be checked against `backend/internal/database/migrations/` and the schema reference before implementation.

| Model | Fields/types established or implied by the features | Relationships and constraints |
|---|---|---|
| User | `email`, `name`, `area_assignment`, `church_name` (text); `role_id` (role identifier); `status` (enum: active/inactive/disciplinary/deceased); deletion reason (text) and deletion state; `avatar_url` (optional public URL) | One role per user; deleted emails remain reserved; non-active accounts cannot sign in; self/last-admin protection |
| Role | Identifier; role name (text); chosen permission set | Built-in admin/editor/user plus custom roles; cannot delete a role while users hold it |
| Permission | Identifier; permission name (text, e.g. `roles:write`) | Grants link roles and permissions through `role_permissions`; seeded on API start |
| Report | Owner identifier; worker, area and church (text); month and year (period); five weeks of 21 metric values; `status` (pending/approved/needs_revision); `admin_notes`, `user_notes` (text); `last_revised_at`; soft-deletion state | One report per month as stated in project plan; drafts are visible only to their owner; owner retained after user deletion/death; non-owner field privacy under D-18 |
| Audit entry | Action (allowed enum); action-specific details (structured payload) | Allow-list on insert; report/account/role/export/auth events; new actions require migration |
| Session | User relationship and sign-in lifecycle | Password resets/status changes revoke sessions; authenticated API use |
| Refresh token | Refresh/idle lifecycle; default `REFRESH_IDLE_TTL` 30 minutes | Session refresh; browser refresh currently prevents an open tab becoming idle |
| Verification token / password reset | Token lifecycle for existing verification/reset APIs | Public recovery UI and email delivery remain skipped |

> TODO: plans do not enumerate primary-key types, token fields, report metric names/types, audit columns or exact uniqueness keys. Derive no new schema from these omissions. Plans describe monthly reports despite introductory “weekly reports” wording.

Audit scope: 22 actions (listed in the project plan and `docs/ARCHITECTURE.md`): report, submit, user, export, sign-in and role events. `SUBMIT_REPORT` (42e) is the latest.

## Tech stack

- Backend: Go 1.26, Echo, pgx, goose, swaggo, golangci-lint v2, air.
- Frontend: Next.js 16.4, React 19.3, TypeScript 5, Tailwind 4, shadcn/Radix, TanStack Query and Sonner.
- API types: Swagger → OpenAPI → `openapi-typescript` (`npm run gen:api`); CI rejects drift.
- Tests: Go unit/testcontainers integration, Vitest, Playwright/axe. Plans' numeric test counts are historical, not current verification evidence.
- CI: `.github/workflows/ci.yml` covers lint, tests, integration, builds, gosec, frontend and E2E.

## Monetization

None; internal tool. Project plan retains `TODO (confirm)`.

## UI/UX

Use existing shadcn/Radix components and Tailwind styling. Follow `backend/docs/04-Content-Guidelines.md`. Preserve keyboard focus, skip links, announced updates and reduced motion.

- Reports list, new/edit forms and read-only report view; worker/area/church prefilled from profile.
- `/auth/me` supplies session identity; change-password UI is shipped.
- Report workflow: a report exists once submitted (D-A25), browser backup of unsent work, admin and revision notes, and one current report per filing period.
- Community Reports lists every report for workers (D-18 redaction), separate from their own reports.
- `/admin` has Reports (with bulk approve/unapprove), Tracker, Users, Logs, Roles and Analytics tabs, gated by permissions; user/role writes retain their specified protections.
- Admin Edit User includes status, deletion/restore and password reset workflows. Reset dialog requires matching passwords and supports generate/copy; the generated password is not retained or shown again after completion.
- Excel preview shows read-only sheets; Cancel downloads nothing and writes no audit; Download rebuilds the file and audits it.
- Sidebar is responsive; below 1024 px it moves into a Menu drawer (modal overlay). The account menu holds Profile, Change Password and Sign Out.

## Deployment

- Render hosts the API; Neon hosts PostgreSQL; Vercel hosts the frontend (`app.<domain>`). Docker Compose supports local/production setups.
- Cloudflare proxy handling remains deferred; `amgcreports.com` is chosen in the build plan but setup is deferred.
- Seed permissions, roles and the admin account on API start. This is distinct from production migration execution.
- Repository deployment guardrail: production does not auto-migrate; Render Free has no Pre-Deploy Command or Shell. Run migrations locally against Neon before pushing `main`, and verify Render's actual deploy trigger in its dashboard.

> TODO: the plans omit exact build/start commands, output directory, health path and most environment-variable names. Consult existing deployment documentation; do not invent provider configuration.

## Open questions and plan drift

- Email recovery: project plan says blocked on email delivery; build plan explicitly skips email and recovery UI under D-A18. Preserve shipped token APIs separately from skipped user flows.
- Domain setup (#16) remains deferred; `amgcreports.com` is chosen because `amgreports.com` is taken.
- The Phase A splitting note remains obsolete despite its checked completion state.
- The project plan summarizes shipped features; detailed contracts (custom roles, status, delete/restore, Tracker, preview, report workflow) live in the build plan and `docs/`. Backlog additions are absent from the project plan.
- Build plan says dashboards/notifications are not planned while explicitly shipping stats, Tracker and an export Report Dashboard and proposing Sentry alerts (#35). Treat those as named scopes; broader notification/dashboard work needs a decision.
- The project plan allows admin late-filing; preserve that contract when planning report changes. No runtime verification is part of this overview refresh.
- The project plan does not list seed idempotency as a shipped feature; the build plan now marks #19 complete, and its archive records the verification.
