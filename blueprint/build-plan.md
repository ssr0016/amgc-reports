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
- [x] 14. Production deploy on Render (API) and Neon (Postgres); H-4 + API migrate verified in prod
- [x] 27. L-3: reset-password strongpass enforcement

## P0 — Done

- [x] 15. Finding #1: plain `user` keeps `read_any`, but free text and `tithes_offering` are hidden from non-owners (D-18)

## P1 — Launch

- [ ] 17. F14: production smoke test (blocked on F0, which is deferred)
- [ ] 18. Rotate the Neon Postgres password
- [ ] 19. Seed idempotency

## P2 — Admin Panel

- [ ] 20. Phase A (~28h)
  - [x] 20a. Admin shell: `/admin`, tablist with arrow keys, guard, header link, titles (~3h)
  - [x] 20b. Stats endpoint and the four cards, with Swagger and `gen:api` in the same commit (~5h)
  - [x] 20c. Reports tab: reuse `GET /reports`, filters, approve/unapprove, empty state, "X total" (~5h)
  - [x] 20d. Users tab: reuse the existing users list, "You" badge, role filter (~3h)
  - [ ] 20e. Preview modal: metadata, the two metric tables, Avg per D-A5 (~4h)
  - [x] 20f. Sidebar navigation and profile menu: left sidebar on every signed-in page (Reports for everyone; Admin for admins only, the Users list is the Admin Users tab; Tracker and Logs hidden until Phase B); top bar with only the app name, the avatar with name and Sign out; "Change password" moves into the avatar menu; the sidebar collapses behind a "Menu" button below `md` (768 px)
- [ ] 21. Phase B (~21h)
- [ ] 22. Phase C (~49h), including exports and user status
  - [x] 22a. User status (D-A10): `status` column (active, inactive, disciplinary, deceased), login check after the password, session revoke in the same transaction, `PUT /admin/users/:id/status`, status badge, filter and Edit dropdown, `CHANGE_USER_STATUS` audit with its migration, self and last-admin protection (~9-10h)
  - [x] 22b. Soft delete and restore of users (D-A11): reason, confirmation dialog, Deleted filter, email stays reserved, audit with its migration (~6h)
  - [x] 22c. Reports of deleted and deceased users: owner status in the reports API and the "Deleted user" and "Archived" badges (~4h)
  - [x] 22d. Password confirm field on every password form: `confirm_password` (required, must equal the new password, 422 on `confirm_password`) in `CreateUserRequest`, `ChangePasswordRequest` and the public `ResetPasswordRequest`; the matching field on the create-user, change-password and reset-token forms (~4h)
  - [x] 22e. Admin reset password (D-A12): `POST /admin/users/:id/reset-password` (admin role and `users:write`) sets a new password for another user with `new_password` and `confirm_password` (22d rules); it ends all of that user's sessions and clears their login lockout; refused with a 403 for your own account ("You cannot reset your own password. Use the Change Password page instead."); `ADMIN_RESET_PASSWORD` audit in the same transaction with its migration (CHECK constraint, same deploy); "Reset password" button on the Edit User page opens a dialog with the two fields, show/hide, "Generate random password" and "Copy" (the password is never shown again or stored in the clear); email delivery stays with E-1 (~10h)
  - [x] 22g. Excel export in the legacy layout: admin-only `POST /admin/reports/:id/excel` (single) and `POST /admin/reports/excel` (bulk), `DOWNLOAD_SINGLE_EXCEL` / `DOWNLOAD_BULK_EXCEL` audit (shipped as feature 22 in `cf0fcb6`; F-05 sheet-name fix `d9e32a3`)
  - [ ] 22f. Excel export preview (D-A15): the row "Excel" and "Download Bulk" buttons open a modal that shows the workbook's sheets as read-only tables from a new `POST /admin/reports/excel/preview` (admin only, single and bulk, no audit row); a sheet selector for bulk; only the modal's "Download" calls the existing export (which builds the file again and writes the `DOWNLOAD_*_EXCEL` audit row); "Cancel" closes with no download and no audit (~6h)

> Phase contents are in `backend/docs/06-Implementation-Plan.md` (commit `a18b572`). `TODO (confirm)`: split them into tasks when you start Phase A.

## P3 — Email

- [ ] 23. E-1: email delivery
- [ ] 24. Password recovery (forgot-password flow)
- [ ] 25. Reset-password UI

## Deferred

- [ ] 16. F0: domain setup (`amgcreports.com` chosen; deferred to the end by the owner, 2026-10-16; not a blocker for the admin panel)
- [ ] 26. D-F7: Cloudflare proxy and IP-limit handling

## Not planned

Dashboards and notifications are not on the roadmap.
