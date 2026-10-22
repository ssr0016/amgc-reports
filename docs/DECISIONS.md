# Decision log

Append-only. Add new entries at the bottom; never rewrite an old one — add a dated note or a superseding entry instead.
Format per entry: ID, title, date, status, then context/decision/rationale as the entry needs.

Scope: shared decisions (deployment, frontend, admin panel, auth). Decisions that only shape the Reports feature
(D-1..D-11, D-14, D-F8) live in [backend/docs/07-Reports-Decisions.md](../backend/docs/07-Reports-Decisions.md).

## Index

| ID | Title | Status | Where |
|---|---|---|---|
| D-1..D-11, D-14 | Reports feature decisions | Decided | 07 |
| D-F1 | Cookie domain | Decided | here |
| D-F2 | Domain choice | Decided; domain taken, replacement open (F0) | here |
| D-F6 | Hosting combo | Decided | here |
| D-F7 | Cloudflare proxy and IP limits | Deferred | here |
| D-F8 | Previous-month rule | Decided | 07 |
| D-12 | Login with unverified email | Pending | here |
| D-A1..D-A7 | Admin panel | Decided 2026-10-15 | here |
| D-A8 | Approval lives in the admin panel | Decided 2026-10-16 | here |
| D-A9 | Navigation is a left sidebar; the profile menu holds Change password | Decided 2026-10-16 | here |
| D-A10 | User status (active, inactive, disciplinary, deceased) | Decided 2026-10-16; built in 22a | here |
| D-A11 | Soft delete and restore of users | Decided 2026-10-16; built in 22b | here |
| D-A12 | Admin reset password | Decided 2026-10-17; built in 22e | here |
| D-A15 | Excel export preview before download | Decided 2026-10-22; built in 22f | here |
| D-A16 | Bulk export: Report Dashboard sheet and approved-only metric sheets | Decided 2026-10-22; built in 22h | here |
| D-A17 | Excel sheets follow the legacy workbook's styling | Decided 2026-10-22 | here |
| D-A18 | Closed system: no self-registration, no email recovery for now | Decided 2026-10-22 | here |
| D-A19 | Report Dashboard: Month and Year columns, owner's new reference styling | Decided 2026-10-22 | here |
| D-A20 | Audit Logs UI and sign-in audit | Decided 2026-10-22; built in 21a | here |
| D-A21 | Dynamic RBAC: Roles tab, custom roles, audited role changes | Decided 2026-10-22; built in 37a | here |
| F-1 | Change password | Done | here |
| L-3 | Reset-password strength | Done | here |
| R-1 | Railway pre-deploy source of truth | Verified | here |
| D-18 | Report field visibility | Decided | here |
| D-19 | Role filter is client-side | Decided 2026-10-16 | here |
| D-A14 | Every password form asks twice; the server enforces it | Decided 2026-10-17; built in 22d | here |
| D-AR1 | Architecture alignment with the company codebase, and JWT auth | Decided 2026-10-21; phases 0 to 3 built | here |
| H-1 | Hosting moves from Railway to Render and Neon | Decided 2026-10-21 | here |

## Log

### D-F1 Cookie domain fix
**Date:** 2026-10-14 · **Status:** Decided

- **Decision:** Set `Domain=.amgreports.com` on the session cookie so Server Components and `proxy.ts` on `app.amgreports.com` can read the session.
- **Trade-off:** the cookie is sent to every subdomain of the registrable domain. Keep other subdomains under the owner's control.
- **Implications:** a backend change (session cookie config), with a test that the cookie carries the domain. Until it ships, 08 section 1.2 (client-side fetching, client-side route guard) still applies. 08 section 9 lists this as "not in v1"; this entry records the owner's later decision.


### D-F2 Domain choice
**Date:** 2026-10-14 · **Status:** Decided; amgreports.com is taken, replacement not chosen (F0 open)

- **Decision:** `amgreports.com` if it is available. Otherwise another cheap `.com` or `.app` domain (08 section 9). Not bought yet (task F0).
- **Update (2026-10-16):** `amgreports.com` is taken. Pick another cheap `.com` or `.app` domain in F0. D-F1 and the examples in 08 still use `amgreports.com` as a placeholder until then.


### D-F6 Hosting combo
**Date:** 2026-10-14 · **Status:** Decided

- **Decision:** Vercel (frontend, `app.<domain>`), Railway (Go backend, `api.<domain>`) and Cloudflare (DNS for the domain).


### D-F7 Cloudflare proxy and IP limits
**Date:** 2026-10-14 · **Status:** Deferred (not set up)

- **Context:** Cloudflare holds the DNS for the domain (D-F6). Putting its proxy in front of the API would make every request arrive from Cloudflare addresses, which breaks the per-IP rate limits.
- **Decision:** deferred; the proxy is not set up. Related: D-16 (trusted proxy) in 06 §13.2 and `middleware.TrustedProxyExtractor`.
- **TODO (confirm):** this entry is summarised from the owner's launch notes. The original D-F7 wording was not in the docs; edit it to match.

### D-12 Login with `email_verified=false`
**Date:** 2026-10-14 · **Status:** Pending

- **Finding (2026-10-14 integration test):** an account with `email_verified=false` can sign in and use Reports. This matches 08 section 3 (verification is not enforced at login; H-3 is blocked on E-1).
- **Status:** pending. No change until the owner decides whether login must require a verified email.


### D-A1..D-A7 Admin panel
**Date:** 2026-10-15 · **Status:** Decided by the owner (all seven recommendations accepted). Supports 08 §10.

#### D-A1 User status and delete
- **Decided (amended 2026-10-16):** deactivate (soft delete), not hard delete. The user row is kept and marked deleted. A hard delete is never done (`audit_logs.performed_by` is `ON DELETE SET NULL`). Built in 22a (the status, D-A10) and 22b (soft delete and restore, D-A11), both in Phase C. Session revocation uses the existing `sessions.revoked_at`; the last-admin protection (F-3) lands in 22a.
- **Question:** the design has an Active or Inactive status and a Delete action. Users have neither a flag nor a delete endpoint.
- **Recommendation:** deactivate (a flag, reversible, keeps audit history) instead of hard delete, in Phase C with F-3 and H-2. `audit_logs.performed_by` is `ON DELETE SET NULL`, so a hard delete would orphan the Logs tab. The alternative is to leave the Status column and Delete button out for now.

#### D-A2 Username
- **Decided:** show the email; no username.
- **Question:** the design shows `@username`. Users have email and name only.
- **Recommendation:** show the email, no username. A username needs a column, uniqueness, login changes and seeding.

#### D-A3 Tracker denominator
- **Decided:** the `user` role; "For Review" is submitted and not approved.
- **Confirmed by the owner (2026-10-22), built in 21b:** inactive, disciplinary and deceased `user` accounts are tracked (with their status badge; no report counts as Not Submitted); deleted accounts are not. Editors are excluded, so the Tracker's set differs from the Excel Report Dashboard's (D-A16, every non-admin account with a church). `GET /admin/tracker` (admin role and `reports:read_any`), Tracker tab between Reports and Users.
- **Wording (owner, 2026-10-22):** the Tracker displays the filed-not-approved state as "Pending", the word the Reports tab, the stats cards and the Excel dashboard use. The API code stays `for_review`.
- **Question:** who counts as expected to submit (role `user` only, or anyone with `reports:write`), and is "For Review" simply submitted and not approved?
- **Recommendation:** the `user` role; "For Review" as stated, with no new status.

#### D-A4 Login and logout audit, and the IP address
- **Decided:** successful logins and logouts only; the IP is not shown.
- **Amended (owner, 2026-10-22):** failed logins are logged too (`LOGIN_FAILED`, D-A20). The IP is still stored and still never shown, now also left out of the API response.
- **Question:** log every login and logout, or logins only? Log failed logins? Show the IP address in the Logs tab?
- **Recommendation:** successful logins and logouts, no failed logins (the rate limiter and lockout already cover them), IP address not shown in the table. The actions need a CHECK constraint migration.

#### D-A5 The Avg column
- **Decided:** average of the weeks with a value; a dash when there are none. This settles D-5 for the preview.
- **Question:** is the average over the weeks reported, or over five weeks with a blank as 0? This is D-5, which was deferred.
- **Recommendation:** average of the weeks that have a value; show a dash when none. D-5 says a blank means not reported and 0 means reported zero, so counting blanks as 0 would invent data.

#### D-A6 Excel export
- **Decided:** `excelize`, approved as a new dependency. The owner provided the legacy template ("Church Growth Program Report September 2026.xlsx", kept out of Git because it holds real reports) on 2026-10-21; feature 22 builds the export from it.
- **Layout (decided by the owner, 2026-10-21):** one sheet per report in the template's church-sheet layout (title rows, Month of/Worker/Area/Church, Week 1-5 and an `AVERAGE` formula column). `leadership` goes on "Leadership Conference" (row 21); "Leadership Training" (row 22) stays blank because there is no separate value. `baptism` goes on the template's blank row 19, labelled "Baptism". "Person Led To Christ" holds `person_evangelized`, with `names` below it. The template's "Report Dashboard" and "SUMMARY" sheets are left out.
- **Access and limits (decided by the owner, 2026-10-21):** admins only, single (`POST /admin/reports/:id/excel`) and bulk (`POST /admin/reports/excel`, one month). Bulk is capped at 100 reports per file (422 above). Each download writes one `DOWNLOAD_SINGLE_EXCEL` or `DOWNLOAD_BULK_EXCEL` audit row server-side (D-10).
- **Question:** which library, and what layout? The repo has no template and the legacy workbook was not provided.
- **Recommendation:** `excelize` (needs owner approval as a new dependency), with the legacy template as the layout. Without the template the layout would be invented.

#### D-A7 Default period
- **Decided:** the previous month in Asia/Manila (D-F8).
- **Question:** the cards say "September 2026". Is that the previous month or the current one?
- **Recommendation:** the previous month in Asia/Manila (D-F8), the same as the reports list.

### F-1 Change password
**Date:** 2026-10-16 · **Status:** Done

- **Endpoint:** `POST /auth/change-password` with `current_password` and `new_password`; answers 204. It sits behind `RequireAuth` and the per-user write limit.
- **Errors:** a wrong current password is a 422 on `current_password` (not 401, so the client does not read it as a lost session). 422 on `new_password` for a weak password, the same as the current one, or over 72 bytes (checked in the service: bcrypt counts bytes, the validator counts characters). 403 when the account is locked. 429 when rate limited.
- **Lockout:** the current password uses the same failure counter as login (5 wrong attempts lock the account for 15 minutes).
- **Sessions:** the other sessions of the user end; the one in use stays and its token is renewed. The password update and the `CHANGE_PASSWORD` audit row share one transaction.
- **UI:** `/account/password` (Account link in the header). Per-field show/hide toggles, no confirm field, plain `useState` form like the others.
- **P0 incident:** the service (commit `065d32f`) was deployed before the migration that lets `audit_logs` hold `CHANGE_PASSWORD`. The audit insert failed the CHECK, so every call answered 500. Fixed by migration `20261015100000`. Based on transaction semantics in `ChangePassword`: the audit insert shares the transaction with the password update, so a CHECK violation rolls back the whole change. Not verified against production logs. Lesson: a new audit action ships in the same deploy as its migration; the integration test `TestModelConstants_MatchCheckConstraints` now covers the action.
- **Deferred:** a confirm-new-password field, a forgot-password flow (blocked on E-1), an admin password reset, and E2E cases for the locked (403) and rate-limited (429) states.

### L-3 Reset-password strength
**Date:** 2026-10-16 · **Status:** Done (commit `34dd9e8`)

- **Decision:** `POST /auth/reset-password` applies `strongpass` to `new_password` and the 72-byte bcrypt check in `PasswordResetService`; a weak password is a 422 on `new_password`.
- **Why:** reset was weaker than account creation and change-password.

### R-1 Railway pre-deploy source of truth
**Date:** 2026-10-16 · **Status:** Verified in the Railway dashboard

- The Pre-deploy Command (`/app/api migrate`, timeout 300 s) is set in the Railway UI (pastoral-reports → Settings → Deploy). `backend/railway.json` only mirrors it; Railway does not read it.
- Wait for CI is **disabled**: a push to `main` deploys immediately, without waiting for the CI workflow.
- See [docs/ARCHITECTURE.md](ARCHITECTURE.md).

### D-18 Report field visibility
**Date:** 2026-10-16 · **Status:** Decided by the owner (resolves Finding #1)

- **Context:** the default `user` role holds `reports:read_any` (D-2, 2026-10-15) so everyone can see who has filed. That also exposed the narrative, the names of new believers, prayer requests and the tithes to every signed-in user (D-1 privacy note).
- **Decision:** keep `read_any`, but withhold sensitive fields from a caller who is not the report's owner (`created_by` = caller) and holds neither `reports:write_any` nor `reports:delete_any`.
  - **Hidden:** `names`, `narrative_report`, `challenges`, `prayer_request`, and the `tithes_offering` metric (financial).
  - **Visible:** `id`, `month`, `year`, `worker`, `area_assignment`, `church_name`, `completed`, `completed_at`, `completed_by`, `created_by`, `created_at`, `updated_at`, and the other 20 metrics.
  - Editors (`write_any`), admins and owners see everything. `delete_any` also counts, as the owner chose.
- **Where:** `ReportService.redact`, applied by `Get` and `List` (`Create`, `Update` and `ToggleComplete` are only reachable by owners or `write_any`, or return metadata only). The detail page shows "Restricted" in place of the fields; the frontend derives this with `isRestricted` (`lib/report-form.ts`), so the API and Swagger are unchanged.
- **Rationale:** the Tracker and "who has reported" need metadata, not pastoral content.
- **Consequence:** the rule lives in two places (backend and frontend); change both together. See [ARCHITECTURE.md](ARCHITECTURE.md).

### Unconfirmed items
**Date:** 2026-10-16 · **Status:** Open

- **Monetization:** none planned. TODO (confirm): no decision yet.

### D-A8 Approval lives in the admin panel
**Date:** 2026-10-16 · **Status:** Decided by the owner (admin panel feature 20c)

- **Decision:** Approve and Unapprove are in the Reports tab of `/admin` only. `/reports` no longer has them: it shows each report's status read-only and keeps Edit, View and Delete.
- **Admin Reports tab:** one month at a time. Month and Year dropdowns only (no "Any", no text filters), starting on the previous month in Asia/Manila (D-A7) and able to go back to any year from 2026. Columns: Worker, Month, Year, Area of Assignment, Church, Status, Action, Download. The header reads "N total reports".
- **Excel (Phase C, D-A6, D-10):** "Download Bulk" exports the selected month and the per-row "Excel" control exports one report (built in feature 22). The stats cards stay on the previous month and do not follow the dropdowns.
- **Why:** approval is an admin task and the admin panel is where an admin reviews who has reported; the list is for the people who write and edit reports.
- **Consequence:** the approve endpoint (`PATCH /reports/:id/complete`) is unchanged; the API and its permission (`reports:approve`, admin only) are as before. An approved report is undone from the same tab.

### D-19 Role filter is client-side
**Date:** 2026-10-16 · **Status:** Decided by the owner (admin panel feature 20d)

- **Decision:** the Role filter on the Users tab (and on `/admin/users`) narrows the rows already loaded. `GET /admin/users` has no role parameter and the API is unchanged.
- **Known limitation:** the list returns at most `LIMIT` (100) users, so the filter only sees those. Upgrade to a server-side filter (a `role_id` query parameter, then `make swagger` and `npm run gen:api`) when the user count exceeds `LIMIT`.
- **Why:** there are 24 churches, so the dataset is small; no backend change and about 3h instead of 5h.
- **Where:** `UsersPanel` in `frontend/src/app/(app)/admin/users/users-list.tsx`, shared by the Users tab and the standalone page.

### D-A9 Navigation is a left sidebar; the profile menu holds Change password
**Date:** 2026-10-16 · **Status:** Decided by the owner (admin panel feature 20f)

- **Decision:** every signed-in page has a left sidebar (`<nav aria-label="Main">`): Reports for everyone, Admin for admins only. Tracker and Logs are not rendered until Phase B. The top bar keeps the app name, the profile menu (avatar, name and role) and Sign out. "Change password" is the only item of the profile menu; the "Account", "Admin" and "Users" header links are gone. There is no Users link: the list is the Users tab of `/admin`.
- **Small screens:** below `md` (768 px, so also at 320 px) the sidebar is hidden behind a "Menu" button (`aria-expanded`, `aria-controls`). Opening it pushes the page content down rather than overlaying it. The build-plan said "at 320 px"; the owner chose `md`.
- **One way to the users list:** the Users tab of `/admin` (D-19). `/admin/users` now redirects to `/admin?tab=users`, so old links and bookmarks work; `?tab=users` only picks the tab on arrival (tab state is otherwise in memory, 20a). `/admin/users/new` and `/admin/users/[id]/edit` stay, and the sidebar marks Admin on them. This reverses the earlier choice to keep both routes.
- **Why:** one place for navigation that grows with Phase B, and a header that no longer wraps at 320 px for an admin.
- **Consequence:** hiding Admin is a convenience; the pages and the API still enforce the role. Routes, titles and the API are unchanged. Frontend only: `sidebar.tsx`, `profile-menu.tsx`, `components/ui/dropdown-menu.tsx` and `layout.tsx` in `frontend/src/app/(app)/`.

### D-A10 User status
**Date:** 2026-10-16 · **Status:** Decided by the owner (admin panel feature 22a, Phase C)

- **Decision:** every account has a status: `active`, `inactive`, `disciplinary` or `deceased` (`users.status`, `chk_users_status`, default `active`). Only an active account can sign in or keep a session. "Deleted" is not a status (D-A11).
- **Login order:** email, lockout, password, **then** status. A wrong password on any account is the generic 401, so the status messages never reveal which emails exist. After a correct password: inactive "Your account is inactive. Contact your admin."; disciplinary "Your account is under review. Contact your admin."; deceased "This account is closed." (all 403). The locked message is now "Account locked. Try again later.". A refused login does not reset the failed-attempt counter and starts no session.
- **Sessions:** moving a user away from active revokes their sessions in the same transaction as the change (`sessions.revoked_at`). `GetWithRole` returns the status, and `RequireRole`, `RequirePermission` and `/auth/me` answer 401 for a non-active user, as a net for a session the revoke missed. A value the code does not know is refused, never let through; an empty status (only a hand-built test value) counts as active.
- **Protections:** an admin cannot change their own status (403), and the only active admin cannot be moved away from active (409). The active admins' rows are locked in id order before the check, so two admins cannot lock each other out at the same moment.
- **API and audit:** `PUT /admin/users/:id/status` and `GET /users?status=`; `CHANGE_USER_STATUS` with `{from, to, sessions_revoked}`, written with the change. The same status as now is a quiet 200 that writes and audits nothing. No reason is asked for a status change (a reason belongs to deletion, D-A11).
- **Known gaps:** `PUT /admin/users/:id/role` can still change the last admin to another role; fix separately (`/fix`). The Users card in the admin stats still counts every account. Password reset and email verification are unchanged: an inactive user can reset a password but still cannot sign in.
- **Choices made without the owner:** no index on `status` (about 24 users); the status is changed in its own section of the Edit page with its own request, not inside the profile form.

### D-A11 Soft delete and restore of users
**Date:** 2026-10-16 · **Status:** Decided by the owner; built in 22b (the report badges in 22c)

- **Decision:** deleting a user is a soft delete: `deleted_at`, `deleted_by` and an optional `deleted_reason` (an empty textarea by default). Nothing is hard-deleted. A confirmation dialog says: "This will soft-delete {name} ({email}). The user cannot sign in. Their reports stay, but are hidden from the active list. You can restore them later." Deleting and restoring are audited (`restored_by` is in the audit row).
- **Restore:** an admin action, reversible. The user is listed again.
- **Protections:** same as D-A10: not yourself, and never the last active admin.
- **Login:** a soft-deleted user is the generic 401 "Invalid email or password." (no enumeration).
- **Email:** soft-deleted users keep their email reserved. Recreate is via restore, not a new account.
- **Users tab:** "All" (default) leaves out the soft-deleted; a "Deleted" filter shows them; the Delete action is for active rows and Restore for deleted ones.
- **Reports:** a soft-deleted user's reports stay visible and are not hidden; the owner is marked with a "Deleted user" badge (22c). A future enhancement may hide them (requires a JOIN, conflicts with D-18). A deceased user's reports stay in the history, marked "Archived".
- **Choices made without the owner:** "Deleted" is a list filter (`GET /users?status=deleted`), not a status; deleting a deleted user or restoring one that is not deleted is a quiet 200 that writes and audits nothing; the reason is at most 500 characters; restore keeps the user's status and asks for no confirmation; creating a user with a deleted user's email is a 409 "This email belongs to a deleted user. Restore them instead."; forgot-password ignores a deleted user like an unknown email; edit, status, role and reset-password still work on a deleted user (harmless, since they cannot sign in until restored).


### D-A14 Password confirmation
**Date:** 2026-10-17 · **Status:** Decided; built in 22d

- **Decision:** every form that sets a new password asks for it twice: create user (`/admin/users/new`), change password (`/account/password`), and the public reset-token request. The API requires `confirm_password` on `POST /admin/users`, `POST /auth/change-password` and `POST /auth/reset-password`, and it must equal `password` or `new_password` exactly (no trimming, no case folding). Missing is a 422 "is required" and different is a 422 "does not match", both on `confirm_password`.
- **Why:** an admin gives credentials to church workers by hand (D-A2: no self-registration), and a typo in a password nobody has typed twice locks the worker out. The server enforces it too so no other caller can skip it.
- **Order of checks:** validation runs before any service call, so a mismatch never costs a bcrypt hash, never counts as a wrong current password toward the lockout, and never looks up a reset token.
- **Not stored:** `confirm_password` is read and dropped; it is not logged, audited or echoed in an error.
- **Reset-token form:** the page does not exist yet (build-plan 25), so 22d changed only that request and its docs. Item 25 must add the field when it builds the page.
- **Deploy order:** the API now rejects those bodies without the field and the old frontend does not send it, so the create-user and change-password forms fail with a 422 until the new frontend is live. Deploy both together.
- **Edit user** has no password field and is unchanged. The login form has one password and is unchanged.

### D-A12 Admin reset password
**Date:** 2026-10-17 · **Status:** Decided; built in 22e

- **Decision:** an admin can set a new password for another user: `POST /admin/users/:id/reset-password` (admin role and `users:write`), body `new_password` and `confirm_password` under the D-A14 rules, answers 204. The Edit User page has a "Reset password" button that opens a dialog with the two fields, show/hide, "Generate random password" and "Copy password".
- **Why:** there is no self-registration (D-A2), admins hand the credentials to church workers, and there is no e-mail recovery until E-1. A worker who forgot the password needs an admin.
- **No "view password":** a password is stored as a bcrypt hash and cannot be read back. The admin sees the new password once, in their own browser, and gives it to the worker. E-mailing it or a reset link is E-1 (build-plan 23 to 25).
- **What a reset does, in one transaction:** sets the hash, ends **all** of the user's sessions, clears their failed-login counter and lockout (so a locked-out worker can sign in with the new password), and writes the audit row.
- **Audit:** `ADMIN_RESET_PASSWORD`, target the user's email, details `{"sessions_ended": N}` only; never the password, its hash or the confirmation. Added to `chk_audit_logs_action` by `20261017120000_allow_admin_reset_password_audit_action`, shipped in the same deploy as the code that writes it.
- **Not your own account:** an admin resetting themselves is a 403, "You cannot reset your own password. Use the Change Password page instead.", because that page asks for the current password and this route does not. The check runs after body validation, so a self-reset with an invalid body is a 422 first.
- **Who can be reset:** any status and any role, including another admin and an inactive, disciplinary or deceased account (D-A10: those can already reset by token but cannot sign in until Active). The reset does not change the status. It is audited.
- **The password in the browser:** it lives only in the dialog's state. After a reset the dialog stays open and shows it once, with Copy; closing it, Escape or Cancel clears it. It is not cached, logged, put in a URL, or kept in a toast. The generator is client-side (Web Crypto, 20 characters, no look-alikes `0 O 1 l I`, at least one lower case, upper case and digit).
- **Not built:** a "must change at next sign-in" flag (it needs a column), a bulk reset, a reset from the Users tab row, and a password history.
- **Deploy order:** the migration only widens a CHECK and the old code never writes the action, so the order is safe. The new frontend calls a route the old API lacks, so the Reset password button gets a 404 until Railway finishes.

### D-A15 Excel export preview
**Date:** 2026-10-22 · **Status:** Decided; built in 22f

- **Decision:** the row "Excel" and "Download Bulk" buttons open a preview modal first. Only the modal's "Download" button downloads the file; "Cancel" closes it with no download.
- **Preview data (owner, 2026-10-22):** a new admin-only `POST /admin/reports/excel/preview` builds the same workbook as the export (D-A6) and returns each sheet's name and cell text as JSON. It writes no audit row. The server builds both, so sheet names, layout and averages match the file.
- **Download builds again (owner, 2026-10-22):** "Download" calls the existing export endpoint, which reads the current data, builds the file and writes the `DOWNLOAD_SINGLE_EXCEL` or `DOWNLOAD_BULK_EXCEL` audit row (D-10). A report changed after the preview opened gives a file that differs from the preview. No preview token is stored.
- **Bulk (owner, 2026-10-22):** a sheet selector shows one sheet at a time, with the report count. The 100-report cap (D-A6) applies to the preview too.
- **Not the 20e modal:** 20e (report preview with Avg, D-A5) stays as planned and is separate.
- **Built (22f):** the body is exactly one of `{report_id}` or `{month, year}` (422 otherwise). The response is each sheet's A1:G42 as displayed text, with the Average column calculated and a merged range's text in its top-left cell only. The preview uses the per-user read limiter, because a preview comes before every download and the write limiter would halve the export budget. It answers with `Cache-Control: no-store`. The modal is the house `AlertDialog`; it closes after a successful download and stays open after a failed one.

### D-A16 Bulk export dashboard and approved-only sheets
**Date:** 2026-10-22 · **Status:** Decided; built in 22h

- **Decision (owner, 2026-10-22):** the bulk export and its preview start with a sheet named "Report Dashboard", followed by one metric sheet per **approved** report of the month, in the existing D-A6 layout. Pending reports get no metric sheet. The single-report export is unchanged. This amends D-A6, which left the template's "Report Dashboard" sheet out.
- **Dashboard rows (owner):** one row per account that has a `church_name`, is not soft-deleted, and does not hold the admin role. Inactive, disciplinary and deceased accounts are included. There is no churches table: the account is the church. Columns: Church, Worker (the account's name), Area, Status.
- **Status (owner):** for the selected month and year, "Approved" when the account's report is approved, "Pending" when it exists but is not approved, and "Not Filed" when there is none. The database allows one active report per account per month, so each row has at most one report.
- **Month (owner):** the month and year selected in the admin Reports tab, the same as the bulk export today.
- **No approved reports (owner):** the file still downloads with the dashboard alone. No 404.
- **Cap and audit (owner):** the 100-report cap counts approved reports only. The `DOWNLOAD_BULK_EXCEL` audit row's `count` is the number of approved sheets.
- **Name clash (owner):** a church sheet named "Report Dashboard" gets the existing duplicate suffix, "Report Dashboard (2)".
- **Consequence:** an approved report whose owner is an admin or a deleted account still gets a metric sheet, but its owner has no dashboard row.
- **Built (22h):** one query joins accounts to their report of the month, so the dashboard has no page cap. Pending reports are filtered in the database, so the cap counts approved reports only. The bulk 404 "No reports for that month." is gone, and the 422 message reads "A bulk export holds at most 100 approved reports." The preview returns the dashboard at its real size (4 columns, header plus one row per account); report sheets stay 42 × 7. The modal labels the count "N approved". The service lists dashboard accounts only for a caller with `reports:read_any`, the same scope as the reports; the admin route always has it.

### D-A17 Excel sheets follow the legacy workbook's styling
**Date:** 2026-10-22 · **Status:** Decided

- **Decision (owner, 2026-10-22):** the Report Dashboard and the metric sheets look like the legacy workbook (`blueprint/reference/`, kept out of Git). Fonts, fills, borders and column widths are read from its `styles.xml` and sheet XML. This amends D-A16's dashboard columns. Cell positions, labels, merges and formulas stay as D-A6 set them.
- **Dashboard:** the title "Report Dashboard" is merged over A1:C1 (bold, underlined, 18 pt, blue on pink). The header row reads Churches, Status, Coordinator's Remark, Support Status (yellow), and church cells are peach. There are no borders, as in the template.
- **Worker and Area dropped (owner):** the template has no such columns. `DashboardRow` and its query no longer read them.
- **Colors (owner):** Approved is the template's blue (`#0000FF`). Pending and Not Filed are its red (`#FF0000`). Both use bold white text.
- **Remark columns:** Coordinator's Remark and Support Status are written blank and unstyled until a later feature fills them, so anything an admin types stays readable.
- **Metric sheets:**
  - The title is Times New Roman 18 in blue.
  - Rows 8 and 29 are black bands with white labels; "Other Statistics" stays visible.
  - Labels are Helvetica Neue 12, values and sub-items Arial 10.
  - Each cell has a right and bottom border, plus a left border on column A.
  - The second week header row is not bold.
- **Row heights:** the dashboard title row is 23.25 pt, and rows 40, 41 and 42 are 56.25, 29.25 and 45 pt. These are measured from the screenshot, because Google Sheets grows rows that the file stores at 15.75 pt and Excel does not.
- **Kept:** "Baptism" on row 19 uses the main label style (D-A6). The averages keep their number format; the template's `0` would round them (D-A5).

### D-A18 Closed system with admin-managed passwords
**Date:** 2026-10-22 · **Status:** Decided

- **Decision (owner, 2026-10-22):** AMGCReports is a closed system. There is no public sign-up. Only admins create accounts and set the initial password, which they give to the pastor out-of-band. A forgotten password is reset by an admin (D-A12, built in 22e).
- **Skipped for now:** build-plan 23 (E-1, email delivery), 24 (forgot-password flow) and 25 (reset-password UI) are checked off as skipped, not built. D-13 (which email provider) stays open and is not needed until they come back.
- **Already true:** self-registration was removed in `b7185ed` ("admin creates accounts; remove self-registration"). Neither the API nor the frontend has a register or sign-up route.

### D-A19 Report Dashboard with Month and Year columns
**Date:** 2026-10-22 · **Status:** Decided

- **Decision (owner, 2026-10-22):** the bulk export's Report Dashboard follows the owner's new reference screenshot (`blueprint/reference/image.png`, replaced 2026-10-22, kept out of Git). This amends D-A16's columns and D-A17's dashboard styling. The metric sheets keep D-A17.
- **Columns:** Churches, Month, Year, Status, Coordinator's Remark, Support Status, under a title merged over A1:F1.
- **Month and Year:** the account's report's month and year, so Approved and Pending rows show them and Not Filed rows leave both blank. The year is written as a number.
- **Styling:** bold blue (`#0000FF`) 18 pt title, not underlined, on lavender (`#DDDDFF`); bold gray (`#E0E0E0`) header; white cells with thin borders on every side; Month centered, Year bold and centered. Approved is green (`#00AA00`, as in the reference; owner, 2026-10-22); Pending and Not Filed stay red (`#FF0000`, D-A17). Widths are converted from the screenshot. The sheet prints landscape so the six columns fit on one page (owner).
- **Remark columns (owner, 2026-10-22):** an Approved row reads "Checked" in Coordinator's Remark and "Ready for Pick Up" in Support Status, both bold white on blue `#0000CC` and centered. Pending and Not Filed rows leave both blank, bordered and unfilled. The values come from the status alone; nothing is stored.
- **Unchanged:** only approved reports get a metric sheet (D-A16); every church account stays on the dashboard.

### D-A20 Audit Logs UI and sign-in audit
**Date:** 2026-10-22 · **Status:** Decided by the owner; built in 21a · **Spec:** `.specify/specs/001-audit-logs-ui/`

- **What is logged:** state-changing actions only. Reads, previews, the Logs tab itself and token refreshes write nothing. Three new actions: `LOGIN`, `LOGOUT`, `LOGIN_FAILED` (migration `20261022120000`, shipped in the same deploy as the code).
- **Login:** the `LOGIN` row shares the transaction with the refresh token; if it cannot be written, the login answers 500 and no session is left.
- **Logout:** `LOGOUT` only when the logout ends a live session (a token not yet revoked or expired). Best effort, written after the revoke commits: an audit failure must never leave a signed-out refresh token valid (security review, 2026-10-22).
- **Failed login:** one `LOGIN_FAILED` row per refusal the rate limiter lets through, with `reason` (`unknown_account`, `wrong_password`, `locked`, or the status). No actor. An unknown or deleted email gets no target and the typed email is not stored: people type passwords into that box, and D-A11 needs a deleted account to look unknown. Best effort: a write failure is logged and the client gets the usual refusal, unchanged.
- **Details allow-list:** `audit.DetailKeys` per action, checked in `Insert` (and the test mock), fail closed. Built from what the code wrote on 2026-10-22, so no existing write changed.
- **Read API:** `GET /admin/audit-logs`, admin role, one Asia/Manila month (`month`, `year`), optional `action`, 50 per page by default, newest first with `id` breaking ties. No `ip_address` in the SELECT or the response; no actor ID. Amends D-10 ("No read API").
- **Logs tab:** third tab of `/admin`; Month/Year default to the **current** Manila month (a log view must show today, unlike the Reports tab's previous month); Action filter; expandable rows with labelled details, never raw JSON. Read-only.
- **Not done (backlog):** CSV export (item 30), retention (D-10), filtering by actor or target.

### D-A21 Dynamic RBAC: the Roles tab
**Date:** 2026-10-22 · **Status:** Decided by the owner; built in 37a (Roles tab) and 37b (permission-gated admin panel) · **Spec:** `blueprint/context/current-feature.md` (archived under `blueprint/history/features/` by `/complete`)

- **Owner decisions:** (1) creating, editing and deleting roles needs the admin role **and** `roles:write`; (2) a role any account still holds cannot be deleted, so the admin reassigns them first; (3) the permission checklist shows every permission in the database, grouped by category; (4) Create and Edit User offer every role.
- **Built-in roles are read-only:** `admin`, `editor` and `user` (`accesscontrol.BuiltInRoles`) cannot be renamed, deleted or have their description or permissions changed (409 "built-in roles cannot be changed"). The code names them (`RequireRole`, the Tracker, the Excel dashboard, the last-admin guard) and `seed.go` re-grants their permissions on startup, so an edit would break access or silently revert. The store guards it in the statement; the service refuses first.
- **The permission list is fixed:** the UI never creates or deletes permissions; one that no code checks does nothing. Code adds permissions through `seed.go` (37b added four).
- **API:** `POST /admin/roles` and `PUT /admin/roles/:id` take `name`, `description` and `permission_ids` together (PUT is a full replace and needs `permission_ids`, which may be empty); `DELETE /admin/roles/:id`. All three need `roles:write` and the write limiter. `GET /admin/roles` adds `user_count`, which counts deleted accounts too because they keep the foreign key. The unaudited `POST` and `DELETE /admin/roles/:id/permissions[/:pid]` are removed.
- **A user's role change** (`PUT /admin/users/:id/role`) moved into the user service and is guarded like a status change (D-A10): the admin role and `users:write`; never your own account (403); the last active admin cannot be moved off `admin` (409, active admins locked first). The same role is a quiet 200 with no audit row. A deleted account may be changed, so a role it holds can be freed.
- **Audit (migration `20261022200000`):** `CREATE_ROLE` `{name, permissions}`, `UPDATE_ROLE` `{from, to}` when renamed plus `{added, removed}`, `DELETE_ROLE` `{name}`, `CHANGE_USER_ROLE` `{from, to}`, all as names, in the same transaction as the change. Target type `Role` for the first three. The description (free text) is never audited. An update that changes nothing writes no row.
- **37b, permission-gated admin panel (owner, 2026-10-22):** (1) four seeded permissions, held only by `admin` by default: `reports:export` (Excel and its preview), `tracker:read` (Tracker and stats cards), `audit:read` (Logs), `users:list` (`GET /users`, `/users/:id`); approval keeps `reports:approve`. (2) Account management (`/admin/users*`, including status, role, password reset, delete and restore), the Roles tab and `/admin/permissions*` stay admin-only: `users:write` includes resetting an admin's password, so opening it to a custom role would allow account takeover. (3) `editor` gets no new permissions. (4) `/admin` opens for the admin role or any admin-area permission, and each tab shows only with its permission; a `?tab=` the user may not open lands on the first allowed tab. The `/admin` group no longer requires the role as a whole: each route names its own gate. The plain `user` role's `users:read` and `reports:read_any` gate none of these routes. A non-admin `users:list` holder sees role ids, not names, because `GET /admin/roles` stays admin-only.
- **Not changed:** The Tracker and Excel dashboard still count only the `user` role (D-A3, D-A16); a custom-role worker who files reports is not counted, which is an open question for the owner.

### D-AR1 Architecture alignment with the company codebase, and JWT auth
**Date:** 2026-10-21 · **Status:** Decided; phases 0 to 3 built (JWT cutover and frontend refresh), not deployed
**Spec:** `blueprint/context/current-feature.md`

- **Decision:** adopt the parts of the company identity-service pattern that fit this stack (Echo, pgx, squirrel, goose), and replace scs sessions with JWT access tokens plus revocable refresh tokens. Not adopted: sqlx, `?` rebinding, a read/write pool split, the wrapped count query, a custom migrator, `/api/bo|aff|mem` paths.
- **Why:** match the company's layout and auth model without a rewrite of what already works. Sessions gave instant revocation (D-A10, F-1, D-A12); the JWT design keeps that through refresh-token revocation and `users.tokens_valid_after`.

| ID | Decision |
|---|---|
| D1 | Tokens travel in **HttpOnly cookies** (access `access_token`, refresh `refresh_token` on path `/api/v1/auth`). The CSRF double-submit stays, and D-F1 (cookie readable by Server Components and `proxy.ts`) stays valid. |
| D2 | Real JWT, not just the middleware shape. |
| D3 | Add `users.tokens_valid_after`: a token whose `iat` is older is refused, so a password change or admin reset kills access tokens at once. |
| D4 | New dependency `github.com/golang-jwt/jwt/v5`. |
| D5 | Access token 10 min, refresh token 24 h absolute, 30 min idle (the same as today's sessions). |
| D6 | Keep wire names (swagger schema names, JSON shapes). Rename Go types internally only. |
| D7 | Keep one central route table in `router.Setup`; no per-handler `Register`. |
| D8 | The folder restructure to `<domain>/{model,<x>impl,rest}` is **deferred**. Revisit after phase 3. |
| D9 | When the restructure happens, handlers live in a package named `rest` (not `http`). |
| D10 | Refresh-token reuse is logged with `slog` only. No audit action, so no CHECK-constraint migration. |
| D11 | The owner announces the cutover window. Every signed-in user is signed out at the JWT deploy. |

- **Phase 0 (built):** `internal/request` carries the caller (`WithUser`, `UserFrom`). `middleware.Authenticate(sm)` is the only code that reads the session; it runs in the groups that need a user, so public routes never touch it. `RequireAuth`, `RequireRole`, `RequirePermission`, `RateLimitByUser` and the report and admin-user handlers no longer take the session manager. Login, logout and change-password still do, until phase 2. No status code or response changed.
- **Phase 1 (built):** the transaction is carried in the context. `database.DB.InTransaction(ctx, fn)` opens it and passes `fn` a ctx that carries it; `database.DB.Querier(ctx)` returns that transaction or the pool. Repositories lost their `Querier` parameter, so a store call joins the transaction by using the ctx it was given. A nested `InTransaction` joins the outer one and commits nothing; commit and rollback happen once, at the outermost call. `repository.TxRunner` is now `InTransaction`, `repository.WithTx`, `Beginner` and `TxManager` are gone, and `ReportService` no longer takes a pool.
- **Behaviour note:** repository reads that used to run on the pool inside a service transaction (for example `GetByID` in `setStatus`) now run inside it. That is stricter and safe, and it removes a read that could see state the transaction was about to change. Code in a transaction must use the ctx its closure receives; the outer ctx runs outside it.
- **Not built yet:** phase 2 (JWT, refresh tokens, `/auth/refresh`, frontend refresh on 401, dropping the `sessions` table in a later migration), the `escapeLike` fix in `UserRepo.ListWithPagination`, and pool settings from env.
- **Deploy order for phase 2:** set `JWT_SECRET` and the other new variables in Railway **before** pushing; a push to `main` deploys at once.
- **Phase 2 and 3 (built):** `internal/auth/token` (HS256 pinned, iss/aud/exp checked, plus an integer `iatm` claim so `tokens_valid_after` compares exactly), `refresh_tokens` and `users.tokens_valid_after` (migration `20261021100000`), `POST /auth/refresh`, public `POST /auth/logout`, cookies in `handler/cookies.go`. Revocation checks live in `RequireRole`, `RequirePermission`, `Me` and `ChangePassword`. The frontend `api()` refreshes once on a 401 (single flight) and replays the request.
- **Deviations from the spec:** (1) `JWT_SECRET` is checked by `Config.ValidateAuth()` at server start, not in `Load()`, so `api migrate` (the Railway pre-deploy) works before the variable exists. (2) Logout is public and reads the refresh cookie, so it works after the access token expires. (3) The `session` package and scs are removed now; the `sessions` table stays until a later migration. (4) The handlers are still in `internal/handler`; the `rest` rename (D9) waits for the deferred restructure (D8).
- **Two tabs:** both may send the same refresh token within moments. The second is refused with a 401 but the cookies are left alone (`service.ErrRefreshReplayed`; the jar is shared and already holds the first tab's new tokens), and the sign-in survives. A rotated token seen again after 10 s is theft: the family is revoked and the cookies are cleared. Known limit: that second tab's frontend still signs out locally until it reloads, because its refresh call failed.
- **Deploy:** set `JWT_SECRET` (>= 32 bytes) in Railway before pushing; everyone is signed out at cutover (D11).
- **Refresh vs revoke race (fixed after review):** `rotate` used to lock the token row and insert the new token while a password change or status change revoked "all live tokens" in a separate statement; the revocation waited on the old row, then missed the new one, so a stolen refresh token used at that instant survived the reset. Lock order is now users, then refresh_tokens, everywhere: `rotate` takes `FOR SHARE` on the owner's users row (`UserRepo.LockUser`) before locking the token, and `RevokeSessions` updates users first (in its own transaction when the caller has none), as `UpdatePassword` and `UpdatePasswordKeepSession` already did. Two refreshes still share the lock and meet on the token row. Proved by `TestRefresh_RevocationDuringRotationEndsTheNewToken`, which fails without the lock.
- **Accepted:** logout and theft revocation do not reach an access token already issued (up to 10 min); password change, admin reset and status change do, through `tokens_valid_after`.


### H-1 Hosting moves from Railway to Render and Neon
**Date:** 2026-10-21 · **Status:** Decided

- **Decision:** the Go API runs on Render (Docker web service, root directory `backend`) and the database on Neon (PostgreSQL). The frontend stays on Vercel and DNS on Cloudflare. Supersedes the backend half of D-F6 and R-1; the Railway integration and `backend/railway.json` are removed.
- **Trusted proxies:** `TRUSTED_PROXY_CIDRS` defaults to `0.0.0.0/0` for now, because Render's proxy range is not verified. Trade-off: a client that can set `X-Real-IP` chooses its own address, so per-IP rate limits and audit IPs are weaker until the range is narrowed.
- **Not yet recorded:** how production migrations run (Pre-Deploy Command or by hand), auto-deploy and Wait-for-CI, and the health check path. Verify them in the Render dashboard and add a dated note here.
- **Update (2026-10-22, owner checked the Render dashboard):** the Pre-Deploy Command is `/app/api migrate`. It runs on the **new** image before traffic switches, and a failed migration stops the deploy. Migrations are embedded in the binary, so Render Shell on the running instance cannot apply a migration that is not deployed yet. Auto-deploy, Wait-for-CI and the health check path are still unrecorded.
- Older entries and history that mention Railway are left as written.
- **Update (2026-10-21):** `TRUSTED_PROXY_CIDRS=100.64.0.0/10` is set in Render. In production the server now refuses to start when `TRUSTED_PROXY_CIDRS` is unset or contains a `/0` range (`Config.ValidateProxy`, server only, so `api migrate` is unaffected). Outside production the `0.0.0.0/0` default remains. This replaces the open-default trade-off above.
- **Update (2026-10-22, owner, supersedes the Pre-Deploy note above):** the API is on Render's **Free** tier, where the Pre-Deploy Command and Render Shell do not run. Production migrations are run **by hand from a local machine against Neon** (`make migrate-up` with `DATABASE_URL` set to the Neon URL) **before** pushing to `main`. A migration must therefore be backward compatible with the code still running until the push deploys. Confirmed on the 21a deploy: the local migration, then the push, gave a working sign-in and Logs tab.
