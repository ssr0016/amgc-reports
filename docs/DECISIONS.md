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
| F-1 | Change password | Done | here |
| L-3 | Reset-password strength | Done | here |
| R-1 | Railway pre-deploy source of truth | Verified | here |
| D-18 | Report field visibility | Decided | here |
| D-19 | Role filter is client-side | Decided 2026-10-16 | here |

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
- **Decided:** deactivate, not delete; Phase C, with F-3 and H-2.
- **Question:** the design has an Active or Inactive status and a Delete action. Users have neither a flag nor a delete endpoint.
- **Recommendation:** deactivate (a flag, reversible, keeps audit history) instead of hard delete, in Phase C with F-3 and H-2. `audit_logs.performed_by` is `ON DELETE SET NULL`, so a hard delete would orphan the Logs tab. The alternative is to leave the Status column and Delete button out for now.

#### D-A2 Username
- **Decided:** show the email; no username.
- **Question:** the design shows `@username`. Users have email and name only.
- **Recommendation:** show the email, no username. A username needs a column, uniqueness, login changes and seeding.

#### D-A3 Tracker denominator
- **Decided:** the `user` role; "For Review" is submitted and not approved.
- **Question:** who counts as expected to submit (role `user` only, or anyone with `reports:write`), and is "For Review" simply submitted and not approved?
- **Recommendation:** the `user` role; "For Review" as stated, with no new status.

#### D-A4 Login and logout audit, and the IP address
- **Decided:** successful logins and logouts only; the IP is not shown.
- **Question:** log every login and logout, or logins only? Log failed logins? Show the IP address in the Logs tab?
- **Recommendation:** successful logins and logouts, no failed logins (the rate limiter and lockout already cover them), IP address not shown in the table. The actions need a CHECK constraint migration.

#### D-A5 The Avg column
- **Decided:** average of the weeks with a value; a dash when there are none. This settles D-5 for the preview.
- **Question:** is the average over the weeks reported, or over five weeks with a blank as 0? This is D-5, which was deferred.
- **Recommendation:** average of the weeks that have a value; show a dash when none. D-5 says a blank means not reported and 0 means reported zero, so counting blanks as 0 would invent data.

#### D-A6 Excel export
- **Decided:** `excelize`, approved as a new dependency. The owner still has to provide the legacy template; until then the layout is not defined and Phase C item 10 is blocked.
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

- **Excel audit actions:** `DOWNLOAD_SINGLE_EXCEL` and `DOWNLOAD_BULK_EXCEL` are in the audit enum but no download endpoint is confirmed. Recorded as Reserved, Phase C (see [ARCHITECTURE.md](ARCHITECTURE.md)).
- **Monetization:** none planned. TODO (confirm): no decision yet.

### D-A8 Approval lives in the admin panel
**Date:** 2026-10-16 · **Status:** Decided by the owner (admin panel feature 20c)

- **Decision:** Approve and Unapprove are in the Reports tab of `/admin` only. `/reports` no longer has them: it shows each report's status read-only and keeps Edit, View and Delete.
- **Admin Reports tab:** one month at a time. Month and Year dropdowns only (no "Any", no text filters), starting on the previous month in Asia/Manila (D-A7) and able to go back to any year from 2026. Columns: Worker, Month, Year, Area of Assignment, Church, Status, Action, Download. The header reads "N total reports".
- **Excel (Phase C, D-A6, D-10):** a "Download Bulk" button and a per-row "Excel" control are disabled placeholders; there is no endpoint and no request. The stats cards stay on the previous month and do not follow the dropdowns.
- **Why:** approval is an admin task and the admin panel is where an admin reviews who has reported; the list is for the people who write and edit reports.
- **Consequence:** the approve endpoint (`PATCH /reports/:id/complete`) is unchanged; the API and its permission (`reports:approve`, admin only) are as before. An approved report is undone from the same tab.

### D-19 Role filter is client-side
**Date:** 2026-10-16 · **Status:** Decided by the owner (admin panel feature 20d)

- **Decision:** the Role filter on the Users tab (and on `/admin/users`) narrows the rows already loaded. `GET /admin/users` has no role parameter and the API is unchanged.
- **Known limitation:** the list returns at most `LIMIT` (100) users, so the filter only sees those. Upgrade to a server-side filter (a `role_id` query parameter, then `make swagger` and `npm run gen:api`) when the user count exceeds `LIMIT`.
- **Why:** there are 24 churches, so the dataset is small; no backend change and about 3h instead of 5h.
- **Where:** `UsersPanel` in `frontend/src/app/(app)/admin/users/users-list.tsx`, shared by the Users tab and the standalone page.
