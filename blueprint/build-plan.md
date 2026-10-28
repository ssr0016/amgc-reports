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
- [x] 12. Frontend tests: Vitest (575) and Playwright (110), including axe a11y, keyboard and reflow checks
- [x] 13. CI: Go lint/test/integration/build/gosec, frontend lint/typecheck/build, e2e
- [x] 14. Production deploy on Render (API) and Neon (Postgres); H-4 + API migrate verified in prod
- [x] 27. L-3: reset-password strongpass enforcement
- [x] 42. Report workflow (2026-10-26 to 2026-10-27; not specced in this plan, recorded from git)
  - [x] 42a. Report statuses Draft, Pending, Approved and Needs Revision (`reports.status`, `20261026120000_add_report_status`; `draft` added by `20261027120000_add_report_drafts`, removed by D-A25); unapprove moves a report to Needs Revision (`8250a33`)
  - [x] 42b. Admin notes on approve, unapprove and revision requests (`admin_notes`, `20261026130000_add_report_admin_notes`); the UI requires a note to unapprove, one report or in bulk (`436fe35`, `24d4ac8`, `1d99781`)
  - [x] 42c. Revision tracking: `PATCH /reports/:id/revision`; saving a Needs Revision report returns it to Pending, stamps `last_revised_at` and keeps the worker's `user_notes` (`20261026140000_add_report_revision_tracking`, `df7bf8c`)
  - [x] 42d. Server-backed drafts: `draft: true` on create saves a private draft that other readers' lists, stats and the Tracker leave out (`30a2029`). **Removed by D-A25** (`20261027140000_remove_report_drafts`): drafts became Pending.
  - [x] 42e. Submit flow: `PATCH /reports/:id/submit` moves the owner's draft to Pending and audits `SUBMIT_REPORT` (CHECK constraint in `20261027120000_add_report_drafts`). **Removed by D-A25**: nothing writes `SUBMIT_REPORT` now; old rows still read.
  - [x] 42f. Autosave on the report form; deleting a report clears its saved copy (`bb8a482`, `7e6541e`). **D-A25** removed the server autosave; the browser backup (`localStorage`) stays.
  - [x] 42g. Current-report flow: `GET /reports/current` returns the caller's report for the filing period, so starting a new report opens the existing one instead of a duplicate (`8d5ad86`, `065660a`, `c9ed71e`, `9cc2a76`)
- [x] 43. Admin review tools (2026-10-26; not specced in this plan, recorded from git)
  - [x] 43a. Bulk approve and unapprove on the admin Reports tab; the selection survives filter changes (`e5712c6`, `fa37765`)
  - [x] 43b. Tracker split into a submission status (submitted, no report) and a review status (approved, pending, needs revision) (`d4df498`)
  - [x] 43c. Community Reports feed for every worker, separate from personal reports and from admin management (`38800d1`, `3108105`, `cd2600b`)
  - [x] 43d. Excel preview styled like the exported workbook, single and bulk (`e0e2c66`, `ad954c8`, `0e03422`, `9f103b1`, `b342f9d`)

## P0 — Done

- [x] 15. Finding #1: plain `user` keeps `read_any`, but free text and `tithes_offering` are hidden from non-owners (D-18)

## P1 — Launch

- [ ] 17. F14: production smoke test (blocked on F0, which is deferred)
- [x] 18. Rotate the Neon Postgres password (owner verified production database connectivity and superadmin login, 2026-10-24)
- [x] 19. Seed idempotency

## P2 — Admin Panel

- [x] 20. Phase A (~28h)
  - [x] 20a. Admin shell: `/admin`, tablist with arrow keys, guard, header link, titles (~3h)
  - [x] 20b. Stats endpoint and the four cards, with Swagger and `gen:api` in the same commit (~5h)
  - [x] 20c. Reports tab: reuse `GET /reports`, filters, approve/unapprove, empty state, "X total" (~5h)
  - [x] 20d. Users tab: reuse the existing users list, "You" badge, role filter (~3h)
  - [x] 20e. Preview modal: metadata, the two metric tables, Avg per D-A5 (~4h)
  - [x] 20f. Sidebar navigation and profile menu: left sidebar on every signed-in page (Reports for everyone; Admin for admins only, the Users list is the Admin Users tab; Tracker and Logs hidden until Phase B); top bar with only the app name and the profile menu (avatar), which holds Profile, Change Password and Sign Out (`be683f0`); below `lg` (1024 px) the sidebar moves into a "Menu" drawer that opens as a modal overlay (`(app)/layout.tsx`, `profile-menu.tsx`)
- [x] 21. Phase B (~21h)
  - [x] 21a. Audit Logs UI, live 2026-10-22 (`487f7f5`, security fix `4f6bac4`; production smoke test passed) (D-A20; spec, plan and tickets T1-T7 in `.specify/specs/001-audit-logs-ui/`): `LOGIN`, `LOGOUT` and `LOGIN_FAILED` audit with their migration (same deploy), details allow-list enforced at `Insert`, admin-only `GET /admin/audit-logs` (Manila month and year, action filter, 50 per page, no IP), read-only Logs tab with expandable details; amends D-A4 (failed logins now logged) and D-10 (read API)
  - [x] 21b. Tracker tab (D-A3)
- [x] 22. Phase C (~49h), including exports and user status
  - [x] 22a. User status (D-A10): `status` column (active, inactive, disciplinary, deceased), login check after the password, session revoke in the same transaction, `PUT /admin/users/:id/status`, status badge, filter and Edit dropdown, `CHANGE_USER_STATUS` audit with its migration, self and last-admin protection (~9-10h)
  - [x] 22b. Soft delete and restore of users (D-A11): reason, confirmation dialog, Deleted filter, email stays reserved, audit with its migration (~6h)
  - [x] 22c. Reports of deleted and deceased users: owner status in the reports API and the "Deleted user" and "Archived" badges (~4h)
  - [x] 22d. Password confirm field on every password form: `confirm_password` (required, must equal the new password, 422 on `confirm_password`) in `CreateUserRequest`, `ChangePasswordRequest` and the public `ResetPasswordRequest`; the matching field on the create-user, change-password and reset-token forms (~4h)
  - [x] 22e. Admin reset password (D-A12): `POST /admin/users/:id/reset-password` (admin role and `users:write`) sets a new password for another user with `new_password` and `confirm_password` (22d rules); it ends all of that user's sessions and clears their login lockout; refused with a 403 for your own account ("You cannot reset your own password. Use the Change Password page instead."); `ADMIN_RESET_PASSWORD` audit in the same transaction with its migration (CHECK constraint, same deploy); "Reset password" button on the Edit User page opens a dialog with the two fields, show/hide, "Generate random password" and "Copy" (the password is never shown again or stored in the clear); email delivery stays with E-1 (~10h)
  - [x] 22g. Excel export in the legacy layout: admin-only `POST /admin/reports/:id/excel` (single) and `POST /admin/reports/excel` (bulk), `DOWNLOAD_SINGLE_EXCEL` / `DOWNLOAD_BULK_EXCEL` audit (shipped as feature 22 in `cf0fcb6`; F-05 sheet-name fix `d9e32a3`)
  - [x] 22f. Excel export preview (D-A15): the row "Excel" and "Download Bulk" buttons open a modal that shows the workbook's sheets as read-only tables from a new `POST /admin/reports/excel/preview` (admin only, single and bulk, no audit row); a sheet selector for bulk; only the modal's "Download" calls the existing export (which builds the file again and writes the `DOWNLOAD_*_EXCEL` audit row); "Cancel" closes with no download and no audit (~6h)
  - [x] 22h. Bulk export dashboard and approved-only sheets (D-A16): the bulk export and its preview start with a "Report Dashboard" sheet listing every church with Approved, Pending or Not Filed for the month; metric sheets only for approved reports, in the existing layout; single export unchanged (~6h)
- [x] 37. Dynamic RBAC (owner, 2026-10-22; D-A21)
  - [x] 37a. Roles tab: list roles with their permissions and user counts; create, rename and delete custom roles (admin role and `roles:write`; a role with users cannot be deleted); a permission checklist grouped by category saved as one set; `admin`, `editor` and `user` read-only; role dropdown on Create and Edit User with last-admin and self protection; audit of role and role-permission changes with their migration
  - [x] 37b. Permission-gated admin panel: admin routes and tabs check permissions instead of the `admin` role, so a custom role (e.g. "Reviewer") can use the parts its permissions allow

> Phase contents are in `backend/docs/06-Implementation-Plan.md` (commit `a18b572`). `TODO (confirm)`: split them into tasks when you start Phase A.

## Backlog (owner, 2026-10-22)

Future features, not yet specced. Notes say what already exists, so a spec starts from the gap.

### Security and account management

- [ ] 28. Password policy enforced in the UI: show the `strongpass` rules on every password form and check them as the user types (the API already enforces them, L-3 and 22d)
- [ ] 29. Session timeout after 30 minutes idle: the server already ends a sign-in after 30 minutes without a refresh (`REFRESH_IDLE_TTL`, default 30m), but an open tab keeps refreshing; the gap is the browser's idle detection, a warning, and sign-out on idle
- [ ] 30. CSV export of the audit logs (a non-goal of 21a; needs its own audit action and migration if the export itself is audited)

### UX and error handling

- [ ] 31. Empty states and loading spinners: a consistent pass across pages (the admin panel's tabs and Excel preview got a shared `Loading` spinner and `ErrorRetry` in the 2026-10-22 fix; the gap is the non-admin pages)
- [ ] 32. "Copy to clipboard" for admin-managed passwords: already on the admin reset-password dialog (22e); the gap is the create-user form
- [x] 33. User profile pictures (avatars): `POST /users/me/avatar` (JPEG, PNG or WebP, resized in the browser, stored in R2), shown in the profile menu and the users list, initials when there is none (`20261024120000_add_user_avatar_url`, `b38a5d3`)
- [ ] 38. Server-side pagination for the admin Users list: `GET /users` has no pages and the tab shows at most 100 accounts (`LIMIT` in `users-list.tsx`); deferred from the 2026-10-22 admin panel fix (owner)

### DevOps and infrastructure

- [ ] 34. Neon database backups: confirm the plan's retention and point-in-time restore window, and test a restore
- [ ] 35. Sentry error alerts to Slack or email

### Codebase health

- [ ] 36. Fix the open P3 findings in `blueprint/context/findings.md`: F-01, F-02, F-03, F-04, F-06, F-07, F-08, F-10, F-11, F-14, F-15 and F-16, 12 in all (F-05 and F-09 are no longer open: F-05 was fixed in `d9e32a3`; F-09 was a P2, closed in 22h)

## Deferred

- [ ] 16. F0: domain setup (`amgcreports.com` chosen; deferred to the end by the owner, 2026-10-16; not a blocker for the admin panel)
- [ ] 26. D-F7: Cloudflare proxy and IP-limit handling
- [x] 24. Password recovery (forgot-password flow): skipped, not built (D-A18)
- [x] 25. Reset-password UI: skipped, not built (D-A18)

## Future / Backlog

Future features aligned with `system-gaps.md`; not yet specced.

- [ ] 23. E-1: email delivery: deferred to Future/Backlog, not built (previously skipped under D-A18); prerequisite for email notifications
- [ ] 39. Email Notifications (Report Approved/Unapproved): batch bulk-action emails, include admin remarks and reversal reasons, handle bounces, and show in-app updates; keep sensitive metrics out of emails
- [ ] 40. Advanced Analytics Dashboard (Charts): partly shipped as the admin Analytics tab (`c183d2a` to `a3704b2`), `GET /admin/analytics/attendance` (`tracker:read`)
  - [x] 40a. Attendance dashboard: weekly trend line chart and church breakdown bar chart (Recharts) with data tables, a metric picker, and approved-only or all statuses
  - [x] 40b. Period filter (full year, quarter or month), comparison with the same period last year, KPI cards and a full metrics summary
  - [x] 40c. Period-aware CSV export and a printable PDF
  - [ ] 40d. Growth charts across several years `TODO (confirm)`: the scope beyond the one-year comparison is not specced
  - [x] 40e. Overview-first page with every church and a Metric Detail sheet (`46e2415`); church filter, submission compliance matrix, one organization-wide reporting start with per-church overrides (D-A31, D-A32), CSV exports (`a402cf5`); analytics Excel export (`a5ff5a7`, `d48e281`, `6877749`, `0d1e7d2`); narrower wrapping sticky first column on phones (`a3704b2`)
- [ ] 41. Mobile PWA (Progressive Web App): installable mobile experience with offline support via service workers
