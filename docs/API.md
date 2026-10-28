# API Documentation

Base URL: http://localhost:8080/api/v1

## Authentication Flow

### 1. Accounts

There is no self-registration. An admin creates every account with `POST /admin/users`
(see "Create User" under Admin Endpoints) and gives the credentials to the user.

### 2. Login

POST /auth/login

    {
      "email": "user@example.com",
      "password": "StrongPass123!"
    }

Response (200): the user. Sets the `access_token` and `refresh_token` cookies (HttpOnly); neither is in the body.

### POST /auth/refresh

No body; reads the `refresh_token` cookie and needs the CSRF header. Response (204): sets a new access token and a new refresh token. The old refresh token stops working. 401 (and both cookies cleared) when the token is unknown, expired, idle longer than 30 min, already used, revoked, or its account is not active. The one exception: a token rotated within the last 10 s (a second tab) also gets a 401 but the cookies are left as they are. Clients call it once on a 401, then replay the request.

### POST /auth/logout

Needs no valid access token. Ends the sign-in named by the `refresh_token` cookie, clears both cookies, answers 200.

### 3. Authenticated Requests

Include session cookie with each request:

    curl -b cookies.txt http://localhost:8080/api/v1/auth/me

`/auth/me` returns `user_id`, `name`, `email`, `role`, `permissions`, and the profile text the
admin set for the account: `area_assignment` and `church_name` (empty strings when unset).
The report form starts from these, and from `name` for the report's worker.

### 4. Logout

POST /auth/logout

Response (200):

    {"message": "logged out"}

### 5. Change Own Password
```
POST /auth/change-password
{"current_password": "...", "new_password": "...", "confirm_password": "..."}
```
Requires a session and a CSRF token. Answers 204. The user's other sessions end; the current one stays
and its token is renewed. A wrong current password is a 422 on `current_password` (and counts toward the
login lockout: 5 wrong attempts lock the account for 15 minutes, then 403). A weak, unchanged or
over-72-byte new password is a 422 on `new_password`. A missing `confirm_password` is a 422 "is required" and one that differs from `new_password` (exact match, no trimming) is a 422 "does not match", both on `confirm_password`; this is checked first, so it never counts toward the lockout. The audit row is `CHANGE_PASSWORD`.

## Users

### List Users (paginated, `users:list`)

GET /users?page=1&limit=10&email=user&status=inactive

Optional `role_id` keeps only the accounts holding that role (422 on `role_id` when it is not a positive integer). Optional `status`: `all` or empty for every account, otherwise `active`, `inactive`, `disciplinary` or `deceased` (anything else is 422 on `status`). Every user carries its `status`.

Response (200):

    {
      "data": [...],
      "meta": {
        "page": 1,
        "limit": 10,
        "total": 42,
        "total_pages": 5
      }
    }

### Get User (`users:list`)

GET /users/:id

### Upload My Profile Picture

POST /users/me/avatar (any signed-in user; the write rate limiter)

`multipart/form-data` with one `avatar` file: JPEG, PNG or WebP, at most 512 pixels per side. The whole request is capped at 1 MiB (413 above it), so the client downscales first (`resizeAvatar`). Replaces the caller's picture and answers 200 with the user, whose `avatar_url` is the new public URL; the old file is removed from storage. 422 for another type or a larger image, 503 when picture storage (R2) is not configured.

## Admin Endpoints

Each route names its gate (37b). Account management (`/admin/users*`), roles and permissions require the `admin` role; the review areas (stats, Tracker, audit logs, Excel) need a permission instead, so a custom role can use them. The plain `user` role's `users:read` and `reports:read_any` open none of them.

### List Roles (paginated)

GET /admin/roles?page=1&limit=10

Each role carries its `permissions` (left out when it has none) and `user_count`, which counts deleted accounts too.

### Create, Replace and Delete Role (D-A21)

Admin role and `roles:write`; the write rate limiter. Each writes one audit row (`CREATE_ROLE`, `UPDATE_ROLE`, `DELETE_ROLE`).

POST /admin/roles → 201

    {
      "name": "Reviewer",
      "description": "Approves reports",
      "permission_ids": [11, 16]
    }

PUT /admin/roles/:id → 200, the same body as a full replace (`permission_ids` required, may be empty). A request that changes nothing audits nothing.

DELETE /admin/roles/:id → 204

DELETE /admin/roles/:id?reassign_to=4 → 204: every account holding the role (deleted ones included) moves to role 4 first, in the same transaction, each audited as `CHANGE_USER_ROLE` (D-A22).

- `name` 2-50 characters and unique; `description` up to 255; `permission_ids` unique and existing (422 on `permission_ids` otherwise).
- 409: "role name already exists", "built-in roles cannot be changed" (`admin`, `editor`, `user`), "role is assigned to N users; reassign them first".
- 404 for a missing role.
- 422 on `reassign_to` when it is the role being deleted, `admin`, or does not exist; 400 when it is not a positive integer.

The old `POST /admin/roles/:id/permissions` and `DELETE /admin/roles/:id/permissions/:pid` are removed.

### List Permissions (paginated)

GET /admin/permissions?page=1&limit=10&resource=users

### Get and Delete Permission

GET /admin/permissions/:id (admin role) → 200 with the permission. DELETE /admin/permissions/:id (admin role) → 204. Both answer 400 for a non-numeric id and 404 for an unknown permission. Permissions are seed data (`seed.go`); deleting one is not audited.

### Change User Role

PUT /admin/users/:id/role (admin role and `users:write`; the write rate limiter)

    {
      "role_id": 2
    }

Answers 200 with the user and audits `CHANGE_USER_ROLE` `{from, to}`. 403 for your own account, 409 when it would move the last active admin off `admin`, 404 for an unknown user or role. The same role answers 200 and audits nothing. A deleted account may be changed.

### Create User

POST /admin/users (admin role and `users:write`; this is the only way to create an account)

    {
      "email": "pastor@example.com",
      "name": "Pia",
      "password": "StrongPass123!",
      "confirm_password": "StrongPass123!",
      "role_id": 2,
      "area_assignment": "Metro Manila",
      "church_name": "AMGC Church"
    }

`email`, `name`, `password` and `confirm_password` are required; `confirm_password` must equal `password` exactly (422 on `confirm_password`). `role_id` is optional and defaults to the `user`
role. The two profile fields, `area_assignment` and `church_name`, are optional (150 characters each) and prefill the report form; a report's worker defaults to the user's `name`.
Response (201): the user, without the password. 409 if the email is taken, 422 for invalid
fields or an unknown `role_id`. The account and its audit row (`CREATE_USER`) are written in
one transaction. No verification token is created: email verification is not enforced at login.

### Update User

PUT /admin/users/:id (admin role and `users:write`)

    {
      "name": "Pia",
      "area_assignment": "Metro Manila",
      "church_name": "AMGC Church"
    }

Replaces the name and the two profile fields; text left out is cleared. The password and the
role are not changed here (use Change User Role). Response (200): the user. 404 for an unknown
user. Audited as `UPDATE_USER`.

### Change User Status

PUT /admin/users/:id/status (admin role and `users:write`)

    {
      "status": "inactive"
    }

`status` is `active`, `inactive`, `disciplinary` or `deceased`. Only an active account can sign in; moving a user away from active ends all of their sessions. Response (200): the user. 400 for a bad id, 403 when the target is your own account, 404 for an unknown user, 409 when it would leave no active admin, 422 for another `status` value. The same status as now is 200 and writes nothing. Audited as `CHANGE_USER_STATUS` (`from`, `to`, `sessions_revoked`).

### Delete and Restore User (D-A11)

DELETE /admin/users/:id (admin role and `users:write`), optional body `{"reason": "..."}` (at most 500 characters): a soft delete. The account cannot sign in, its sessions end, and its email stays reserved. Audited as `DELETE_USER`.

POST /admin/users/:id/restore (admin role and `users:write`) → 200 with the user. Clears the deleted mark, so the user can sign in again if their status is active; the status does not change. A user who is not deleted answers 200 and changes and audits nothing; otherwise audited as `RESTORE_USER`. 400 for a bad id, 404 for an unknown user.

### Reset User Password

POST /admin/users/:id/reset-password (admin role and `users:write`)

    {
      "new_password": "StrongPass123!",
      "confirm_password": "StrongPass123!"
    }

Sets a new password for another user (D-A12). There is no current password: an admin does not know
it, and nobody can read a password back. Response: **204**, no body. The rules are those of change-password:
a weak or over-72-byte `new_password` is a 422 on `new_password`, and a missing or different
`confirm_password` is a 422 on `confirm_password` ("is required" or "does not match", exact match).
The reset ends **all** of the user's sessions and clears their login lockout, so a locked-out worker can
sign in with the new password. It is audited as `ADMIN_RESET_PASSWORD` in the same transaction, with
details `{"sessions_ended": N}` only (never the password). Any status and any role may be reset, and the
status does not change.

Errors: 400 for a non-numeric id or a broken body, 401 not signed in, 403 not an admin, no `users:write`,
or **your own account** ("You cannot reset your own password. Use the Change Password page instead."),
404 unknown user, 422 as above, 429 over the write limit. A body that fails validation answers 422 before
the own-account 403. Requires a session and a CSRF token. The new password is shown to the admin only in
the browser, once; giving it to the worker is up to the admin (e-mail delivery is a later feature).

### Admin Stats (`tracker:read`)

GET /admin/stats?month=September&year=2026

Counts for the admin panel cards. `month` (English name, case-insensitive) and `year` go together; send both or neither. Without them the period is the previous month in Asia/Manila. Rate limited like report reads. Counts only: no report text, names or emails.

 {
 "month": "September",
 "year": 2026,
 "reports_total": 3,
 "reports_approved": 1,
 "reports_pending": 1,
 "reports_needs_revision": 1,
 "users_total": 7,
 "admins_total": 2,
 "expected_reports": 5,
 "no_report": 2,
 "churches_total": 5
 }

Report counts leave drafts out; `reports_pending` is filed and waiting for review. `expected_reports` counts active church accounts (the `user` role, not deleted, with a church name), `no_report` those with no filed report for the month, and `churches_total` their distinct church names. `users_total` and `admins_total` count every account that is not deleted, whatever the month (D-A24). Errors: 400 for a year that is not a positive number, 401 without a session, 403 without `tracker:read`, 422 when only one of month and year is sent or the month is not a month name.

### Tracker (`tracker:read`)

GET /admin/tracker?month=September&year=2026

Who has reported for one month (D-A3), for the admin panel's Tracker tab. One row per church account (D-A24): the `user` role, not deleted, with a church name, whatever its status (inactive, disciplinary and deceased accounts are included). Admins, editors, custom roles and accounts with no church name are not tracked. `submission_status` is `submitted` when the account has a filed (non-draft) report for the month, otherwise `no_report`; `review_status` is that report's `approved`, `pending` or `needs_revision`, and is left out when there is no report. Rows are ordered by church name, then name. The period rules and errors are those of `/admin/stats`; 403 without `tracker:read`. Rate limited like report reads. No report text.

 {
 "month": "September",
 "year": 2026,
 "rows": [
 { "user_id": 7, "name": "Juan Cruz", "email": "juan@example.org", "church_name": "Cavite", "user_status": "active", "submission_status": "submitted", "review_status": "pending" }
 ]
 }

`church_name` is `""` when the account has none; `rows` is `[]` when nobody is tracked.

### Analytics (`tracker:read`, D-A31)

GET /admin/analytics/attendance?year=2026&metric=worship_service&status=approved&period=q1&compare=true&churches=3,7

The Analytics tab's figures, computed once from the weekly report values (the exports use the same computation). `year` (2000-2100) and `metric` (one of the 22 metric keys; it selects the trend, week slots and church comparison) are required. `status` is `approved` (default) or `all` (pending, needs revision and approved). `period` is `full_year` (default), `q1` to `q4`, or a lowercase month name. `churches` is a comma-separated list of church ids (default: every church). `compare=true` adds `comparison`, the same counted months of the previous year. Counts only, no report text.

Only months that have ended (Asia/Manila) are counted (`cutoff_month`); a blank week is never 0 and never in a denominator; one report counts per church and month (the approved one, else the latest updated). Response (200): `{year, metric, status, period, generated_at, default_start_month, months, cutoff_month, church_options, churches, coverage, metrics, trend, slots, church_rows, compliance, compliance_totals, comparison?}`. `churches` are the churches with an active church account plus inactive ones with reports or an end month (`active`, effective `start_month`, `start_is_default`). `coverage.expected_reported` of `coverage.expected` is the coverage of owed church-months; `outside_reports` are reports for months not required, still in every total:
- `metrics`: every metric in group order with `total` (null when nothing was recorded), `observations` (recorded church-week values), `zero_observations`, `avg_per_church_service` and `network_weekly` (attendance only) and a per-church `{church_id, value, observations, state}` where `state` is `value`, `blank`, `not_approved` or `no_report`.
- `trend`: the selected metric per month, `value` null for no data, with `churches_reported`, `churches_expected` and `partial`. `slots`: per month and reporting-week slot (1-5, not calendar weeks).
- `church_rows`: one per church: total, recorded entries, average, previous-year figures and `change` (null when either side has no data or the previous is 0), Tithes & Offering, submitted, approved, expected, `completeness` (submission completeness: submitted / expected, every review status; separate from approval coverage). Under the Approved filter a church whose only reports are in review shows its review status (Pending approval or Needs revision), not No data.
- `compliance`: per church and month `{expected, state, report_id?, review_status?, timeliness?, submitted_on?}`, whatever the status filter. States: `not_required`, `not_reviewed`, `inactive`, `not_yet_open`, `awaiting_submission`, `overdue_grace`, `not_filed`, `submitted` (with `expected: false` for a month not required).

Errors: 400 for a bad year, metric, status, period, `compare` or `churches`, 401 without a session, 403 without `tracker:read`.

GET /admin/analytics/attendance/excel?<same query>

The same figures as a formatted `.xlsx` named `amgc-analytics_<year>_<period>_<status>_<scope>.xlsx`: Full Metrics Summary, Summary, Monthly Totals, Weekly Trend, Church Breakdown, Metric Detail, Weekly Detail, Submission Compliance and Notes. Each sheet's title block names the scope, status filter, generated time (Asia/Manila) and cutoff. Writes no audit row. Rate limited with the other Excel downloads (5 per minute per user). Same errors as above, plus 429.

GET /admin/analytics/csv?kind=weekly_detail&<same query>

One flat CSV table (UTF-8 with BOM, CRLF, RFC 4180 quoting; text that starts like a formula gets a leading `'`). `kind=weekly_detail` is one row per report, metric and week slot for every report of the period and churches (any status), with `Value State` (`reported` or `blank`) and `Included in Selected Analytics`; `kind=compliance` is one row per church and month. Missing numbers are empty, zeros stay `0`. Named `amgc-<kind>_<year>_<period>_<status>_<scope>.csv`. Same limits and errors as the Excel download.

### Church Reporting Months (admin role and `users:write`, D-A32)

PUT /admin/churches/:id/reporting

Body: `{"start_month": "", "end_month": "2026-12"}`. Optional per-church exceptions to the organization-wide start month (`default_start_month` in the analytics answer, September 2026), as activity months on the church, not a worker account. An empty `start_month` uses the organization default; an empty `end_month` is open-ended. Audits `UPDATE_CHURCH_REPORTING` (`{from, to}`). Response (200): `{id, name, start_month (effective), start_is_default, end_month, needs_review}`. Errors: 404 unknown church, 422 an end before the church's start.

### Audit Logs (`audit:read`)

GET /admin/audit-logs?month=October&year=2026&action=LOGIN_FAILED&page=1&limit=50

One page of the audit trail for one month in Asia/Manila, newest first. `month` (English name, case-insensitive) and `year` (2000 to 2100) are required; `action` is optional and must be one of the 22 audit actions. `limit` defaults to 50 here (at most 100). Rate limited like report reads. Read-only: no other method is routed, and viewing writes no audit row. The IP address is never returned (D-A4), and `details` holds only the action's allow-listed keys (D-A20).

 {
 "data": [
 {
 "id": 42,
 "performed_by_name": null,
 "performed_by_role": null,
 "action": "LOGIN_FAILED",
 "target_type": "User",
 "target_id": "7",
 "target_name": "worker@example.com",
 "details": {"reason": "wrong_password"},
 "created_at": "2026-10-22T08:05:09Z"
 }
 ],
 "meta": {"page": 1, "limit": 50, "total": 1, "total_pages": 1}
 }

Actor name and role are what was stored when the action happened. A failed login has no actor; one for an unknown or deleted email has no target either. Errors: 400 for a year that is not a number, 401 without a session, 403 without `audit:read`, 422 for a bad month or year (`details.month`) or an unknown action (`details.action`).

### Export Reports to Excel (`reports:export`)

POST /admin/reports/:id/excel
POST /admin/reports/excel

Both need `reports:export` and the CSRF header (POST, because each download writes an audit row). The workbook uses the legacy "Church Growth Program Report" sheet layout and styling (D-A6, D-A17): one sheet per report with Month of:, Worker:, Area of Assignment:, Name of Church/Outreach:, the Week 1-5 metric grid with an `AVERAGE` column, Names, Narrative Report, Challenges and Prayer Request. Blank weeks stay blank; 0 is written as 0.

The first returns one report, approved or not: its sheet, then a "Summary" sheet (D-A28) with a header (Metric, Week 1-5, Total, Average) and one row per metric; a church sheet named "Summary" gets " (2)". The second takes a body and returns the month's bulk workbook (D-A16):

- Sheet 1, "Report Dashboard" (D-A19): the title "Report Dashboard" merged over A1:F1, a header row (Churches, Month, Year, Status, Coordinator's Remark, Support Status), then one row per church account (D-A24: the `user` role, not deleted, with a church name; inactive, disciplinary and deceased accounts are included), ordered by church name. Month and Year are the account's report's (blank when it has not filed); the year is a number. Status is "Approved" (green), "Pending" or "No Report Submitted" (red) for that month. An Approved row reads "Checked" and "Ready for Pick Up" in the two remark columns (white on blue); other rows leave them blank.
- Then one sheet per **approved** report of the month (not soft-deleted, including those whose owner is deleted or deceased), ordered by church name. Pending reports get no sheet. A church sheet named "Report Dashboard" gets " (2)".

A month with no approved reports still returns the file, with the dashboard alone:


{"month":"September","year":2026}


Response (200): the `.xlsx` bytes with `Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, `Content-Disposition: attachment` (an ASCII `filename` and a UTF-8 `filename*`) and `Cache-Control: no-store`. Names: `<Church or "Report id"> <Month> <Year>.xlsx` and `Church Growth Program Report <Month> <Year>.xlsx`.

Each successful download writes one `DOWNLOAD_SINGLE_EXCEL` or `DOWNLOAD_BULK_EXCEL` audit row with `details` `{month, year, count}`, where the bulk `count` is the number of report sheets (0 or more); a failed one writes none. Errors: 400 for a non-numeric id, 401 without a session, 403 without `reports:export`, 404 for a missing or deleted report (single), 422 for a missing or invalid month or year (2000-2100) or more than 100 approved reports in the month. Both share the per-user write limit.

### Preview an Excel Export (`reports:export`)

POST /admin/reports/excel/preview

The admin Reports tab shows this before every download (D-A15). It builds the same workbook as the export and returns it as text. It writes no audit row. Same gate as the export (`reports:export`, CSRF header); it uses the per-user read limit. The body is exactly one of:


{"report_id":42}
{"month":"September","year":2026}


Response (200), `Cache-Control: no-store`: `{"count":1,"sheets":[{"name":"Report Dashboard","rows":[["Report Dashboard","","","","",""],["Churches","Month","Year","Status","Coordinator's Remark","Support Status"],["Bethel","September","2026","Approved","",""],["Abba","","","No Report Submitted","",""], ...]},{"name":"Bethel","rows":[["ANG MANANAMPALATAYANG GUMAWA","","","","","",""], ...]}]}`. Sheets come in the export's order with the export's sheet names; a month starts with the Report Dashboard (header plus one row per account), then, when the month has an approved report, Approved Reports (26 columns: title, header, one row per approved report and a Total row; D-A27), and `count` is the number of report sheets. A single report ends with its Summary sheet (8 columns: title, header, 21 metric rows). A report sheet's `rows` is always A1:G42 (42 rows of 7 strings) as Excel shows them: the month as "September 2026", tithes as "1,810.50" and the Average column calculated. A merged range's text is in its top-left cell only.

Errors: 400 for a malformed body, 401, 403 as above, 404 for a missing or deleted report, 422 when neither or both shapes are sent, for an invalid id, month or year (2000-2100), or more than 100 approved reports in the month.

## Reports

All Reports endpoints need a session cookie, and POST/PUT/PATCH/DELETE also need the CSRF token (see CSRF Protection). Each route needs one permission. A role may also hold an `_any` permission, which widens the scope from the caller's own reports to all reports.

Every full report response (list, get, create and update) includes `owner_status`, the owner's current account status, and `owner_deleted_at` (RFC 3339, omitted when the owner is not deleted). These are read-only metadata from the account, not snapshots from report creation. Readers redacted under D-18 (not the owner, no `reports:write_any` or `reports:delete_any`) get `owner_status` only when it is `deceased`; otherwise it is an empty string, so `inactive` and `disciplinary` stay private. Deleted and deceased owners' reports remain visible within the existing permission scope. The frontend labels them "Deleted user" and "Archived" respectively; both labels can appear together. Report approval is independent, and the approve/unapprove response still contains only report approval metadata. The report's own soft deletion still hides that report.


| Method | Path | Permission | `_any` widening |
|---|---|---|---|
| GET | /reports | `reports:read` | `reports:read_any` lists everyone's reports |
| POST | /reports | `reports:write` | |
| GET | /reports/:id | `reports:read` | `reports:read_any` |
| GET | /reports/current | `reports:read` | none: always the caller's own |
| PUT | /reports/:id | `reports:write` | `reports:write_any` |
| PATCH | /reports/:id/submit | `reports:write` | `reports:write_any` |
| DELETE | /reports/:id | `reports:delete` | `reports:delete_any` |
| PATCH | /reports/:id/complete | `reports:approve` | none: works on any report by ID |
| PATCH | /reports/:id/revision | `reports:approve` | none: works on any report by ID |

Every report has a `status`: `pending` (filed, waiting for review), `approved` or `needs_revision`. `completed` is `true` exactly when the status is `approved`. Reviews add `admin_notes`; a worker's reply to a revision request is `user_notes`, and `last_revised_at` records when they saved it. Readers with `reports:read_any` never see other people's drafts, and drafts are not counted in stats, the Tracker or analytics.

Note: IDs may have gaps. A failed create consumes a sequence value, so do not assume contiguous IDs.

Default roles: `user` has read, `read_any`, write and delete, so it reads every report and changes only its own. `editor` has everything except `approve`. `admin` has all seven and is the only role that approves. A write to another person's report without the `_any` permission returns 404, not 403.

### Create Report

POST /reports

    {
      "month": "January",
      "year": 2026,
      "worker": "Ana",
      "area_assignment": "North",
      "church_name": "Grace",
      "names": "",
      "narrative_report": "...",
      "challenges": "",
      "prayer_request": "",
      "metrics": {
        "tithes_offering": {"week1": 120.00, "week3": 0.5},
        "outreach": {"week2": 0}
      }
    }

Only `month` is required. `"draft": true` saves a private draft (status `draft`; send it for review with PATCH /reports/:id/submit); otherwise the report starts as `pending`. `user_notes` (at most 5000 characters) is optional. `year` defaults to the current year (2000 to 2100). Empty strings are stored as NULL (D-8) and left out of responses. Metric keys are the 21 snake_case keys (for example `worship_service`, `tithes_offering`, `family_day`; see 05-Backend-Schema.md section 2.4). Weeks are `week1` to `week5`; send only the weeks you have. A week that is absent means "not reported", which is different from `0`. Values are JSON numbers from 0 to 9999999999.99 with at most 2 decimals.

Response (201): a report.

    {
      "id": 1,
      "created_by": 2,
      "month": "January",
      "year": 2026,
      "worker": "Ana",
      "metrics": {
        "tithes_offering": {"week1": 120.00, "week3": 0.50},
        "outreach": {"week2": 0}
      },
      "completed": false,
      "status": "pending",
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:00:00Z"
    }

Numbers keep the database scale, so `0.5` comes back as `0.50`. `completed_at` and `completed_by` appear once approved. Errors: 409 if you already have an active report for that month and year; 422 for an unknown month or metric key, a value with more than 2 decimals, or a negative value.

### List Reports

GET /reports?page=1&limit=20&month=January&year=2026&worker=ana&area=north&church=grace&status=pending&mine=true

Newest first. `status` is `pending`, `approved` or `needs_revision`; `mine=true` keeps only your own reports. `month` matches the month name ignoring case; `worker`, `area` and `church` are partial, case-insensitive matches (`%` and `_` are treated literally). `limit` is 1 to 100. Without `reports:read_any` the list contains only your own reports.

Response (200):

    {
      "data": [ { "id": 1, "...": "..." } ],
      "meta": {"page": 1, "limit": 20, "total": 1, "total_pages": 1}
    }

### Get Report

GET /reports/:id

Response (200): a report. 404 if it does not exist, was deleted, or belongs to someone else (without `reports:read_any`).

### Get Current Report

GET /reports/current

Your own report for the filing period (the previous month in Asia/Manila), in any status, drafts included. Response (200): a report. 404 when you have none, so the client can offer to start one instead of creating a duplicate.

### Submit Report

PATCH /reports/:id/submit

No body. Moves your draft to `pending` and audits `SUBMIT_REPORT` in the same transaction. Response (200): the report. 404 when the report is missing, deleted, not yours (without `reports:write_any`) or not a draft.

### Update Report

PUT /reports/:id

Same body as create. This replaces the report: every text field you leave out is cleared, and the whole `metrics` set is replaced. Send every field. A missing `year` keeps the report's current year. Saving a `needs_revision` report moves it back to `pending`, stamps `last_revised_at` and stores `user_notes`. Response (200): the updated report. Errors: 404 (as for get), 409 if the new month and year collide with another of your reports, 422.

### Delete Report

DELETE /reports/:id

Soft delete. Response: 204 with no body. 404 if missing, already deleted, or someone else's (without `reports:delete_any`). The month and year can be filed again afterwards.

### Approve or Unapprove Report

PATCH /reports/:id/complete

Optional body `{"admin_notes": "..."}` (at most 5000 characters). Approves an open report, or unapproves an approved one into `needs_revision` (audited as `APPROVE_REPORT` or `UNAPPROVE_REPORT`). The note is kept on unapprove and cleared on approve; the API accepts an empty note, but the UI requires one to unapprove, one report or in bulk. Response (200) has metadata only, never report text:

    {"id": 1, "completed": true, "status": "approved", "completed_at": "2026-01-02T00:00:00Z", "completed_by": 3}

When unapproved, `completed` is `false`, `status` is `needs_revision` and the other two fields are left out. Use GET /reports/:id to read the report. 404 if it is missing or deleted.

### Request Revision

PATCH /reports/:id/revision

Optional body `{"admin_notes": "..."}`. Moves any report to `needs_revision`, clears its approval and stores the note; audited as `UNAPPROVE_REPORT`. Response (200): the same metadata as approve. 404 if it is missing or deleted.

### Reports Rate Limit

POST, PUT, DELETE and PATCH on /reports share one bucket per signed-in user: 60 per minute with a burst of 10 by default (`REPORTS_WRITE_RATE_LIMIT`, `REPORTS_WRITE_BURST`). Over the limit returns 429 with a `Retry-After` header. GET requests have their own limits (below).

The Excel downloads (POST /admin/reports/excel and POST /admin/reports/:id/excel) have their own bucket per user: 5 per minute with a burst of 5 (M5). The preview stays on the read limiter.

## Error Responses

### 400 Bad Request

    {"error": "BAD_REQUEST", "message": "invalid request body"}

### 401 Unauthorized

    {"error": "UNAUTHORIZED", "message": "authentication required"}

### 403 Forbidden

    {"error": "FORBIDDEN", "message": "insufficient permissions"}

### 404 Not Found

    {"error": "NOT_FOUND", "message": "resource not found"}

### 409 Conflict

    {"error": "CONFLICT", "message": "email already taken"}

### 422 Validation Error

    {
      "error": "VALIDATION_ERROR",
      "message": "Key: 'CreateUserRequest.Email' Error:Field validation..."
    }

### Global and read limits

Every `/api/v1` route has a per-IP limit: 120 per minute with a burst of 60, a constant in `cmd/api/app/server.go`. GET /reports, GET /reports/:id, the admin stats, tracker and audit logs, and the Excel preview also share one bucket per signed-in user: 60 per minute with a burst of 20 by default (`REPORTS_READ_RATE_LIMIT`, `REPORTS_READ_BURST`). The read limit runs after the permission check. `/health`, `/ready`, `/live` and `/metrics` are outside `/api/v1` and are never limited. Over the limit returns 429 with a `Retry-After` header.

### 429 Rate Limit

    {"error": "RATE_LIMIT_EXCEEDED", "message": "rate limit exceeded"}

## CSRF Protection

For POST/PUT/PATCH/DELETE requests, include CSRF token:

1. GET /api/v1/auth/csrf - returns `{"csrf_token": "<token>"}` and sets the HttpOnly `csrf_token` cookie
   (any GET also sets the cookie, but only this endpoint exposes the value to JavaScript)
2. Include header: X-CSRF-Token: <token>

Browser clients must call the endpoint with `credentials: "include"` and send the returned
token in the `X-CSRF-Token` header on every POST/PUT/DELETE.

Example:

    curl -c cookies.txt http://localhost:8080/health > /dev/null
    CSRF=$(grep csrf_token cookies.txt | awk '{print $NF}')

    curl -X POST http://localhost:8080/api/v1/auth/login \
      -H "Content-Type: application/json" \
      -H "X-CSRF-Token: $CSRF" \
      -b cookies.txt -c cookies.txt \
      -d '{"email": "<ADMIN_EMAIL>", "password": "<ADMIN_PASSWORD>"}'

## Complete Example

Full auth flow:

    # 1. Get CSRF token
    curl -c cookies.txt http://localhost:8080/health > /dev/null
    CSRF=$(grep csrf_token cookies.txt | awk '{print $NF}')

    # 2. An admin creates the account first (POST /admin/users, see "Create User");
    #    there is no self-registration. Then log in as that user:

    # 3. Login
    curl -X POST http://localhost:8080/api/v1/auth/login \
      -H "Content-Type: application/json" \
      -H "X-CSRF-Token: $CSRF" \
      -b cookies.txt -c cookies.txt \
      -d '{"email": "user@example.com", "password": "StrongPass123!"}'

    # 4. Get current user
    curl -b cookies.txt http://localhost:8080/api/v1/auth/me

    # 5. List users (paginated; admin session only, any other role gets 403)
    curl -b cookies.txt "http://localhost:8080/api/v1/users?page=1&limit=10"

---

## Email Verification

Registration automatically generates a verification token.

In dev: token is logged via slog:
  msg="verification token generated" user_id=X token=XYZ

### Verify Email

GET /api/v1/auth/verify-email?token=XXX

Response (200):
  {"message": "email verified successfully"}

Errors:
- 400 - invalid or expired token
- 400 - token already used

## Password Reset

### Request Reset

POST /api/v1/auth/forgot-password

Request:
  {"email": "user@example.com"}

Response (200):
  {"message": "if the email exists, a reset link has been sent"}

Note: Always returns 200 (security - don't reveal if email exists).

### Reset Password

POST /api/v1/auth/reset-password

Request:
  {
    "token": "reset-token-from-email",
    "new_password": "NewStrongPass123x",
    "confirm_password": "NewStrongPass123x"
  }

Response (200):
  {"message": "password reset successful"}

Errors:
- 400 - invalid or expired token
- 400 - token already used
- 422 - weak or over-72-byte `new_password`; `confirm_password` missing ("is required") or different from `new_password` ("does not match"), checked before the token

## Account Lockout

After 5 failed attempts from one IP, that IP is locked out of the account for 15 minutes. The count is per account and IP, so someone who knows an email cannot lock its owner out from another IP. A wrong current password on change-password counts the same way.

### Login When Locked

POST /api/v1/auth/login

Response (403):
  {"error": "FORBIDDEN", "message": "account is locked, try again later"}

### Auto-Unlock

After 15 minutes, the lock ends. A successful login resets the counter for that IP; an admin password reset clears it for every IP.

### Check Lock Status

SQL:
  SELECT u.email, f.ip, f.attempts, f.locked_until
  FROM login_failures f JOIN users u ON u.id = f.user_id
  WHERE u.email = 'user@example.com';

## Complete Auth Flow

Full example with email verify + password reset:

1. An admin creates the account
   POST /api/v1/admin/users
   Body: {"email":"user@example.com","name":"User","password":"StrongPass123","confirm_password":"StrongPass123"}

2. Email verification is not enforced at login, and an admin-created account gets no token
   (email delivery arrives with E-1). The verify endpoint still accepts existing tokens.

3. Verify email
   GET /api/v1/auth/verify-email?token=XXX

4. Login
   POST /api/v1/auth/login
   Body: {"email":"user@example.com","password":"StrongPass123"}

5. Forgot password (if needed)
   POST /api/v1/auth/forgot-password
   Body: {"email":"user@example.com"}

6. Reset password (get token from logs/email)
   POST /api/v1/auth/reset-password
   Body: {"token":"XXX","new_password":"NewStrongPass123x","confirm_password":"NewStrongPass123x"}

## Pagination

All list endpoints support pagination:

| Param | Default | Max |
|---|---|---|
| page | 1 | - |
| limit | 20 | 100 |

Examples:
  GET /api/v1/users?page=2&limit=10
  GET /api/v1/admin/roles?page=1&limit=5
  GET /api/v1/admin/permissions?resource=users&page=1&limit=10

Response:
  {
    "data": [...],
    "meta": {
      "page": 2,
      "limit": 10,
      "total": 42,
      "total_pages": 5
    }
  }

## RBAC (Admin Endpoints)

Requires role: admin.

### List Roles
  GET /api/v1/admin/roles?page=1&limit=10

### Create Role (admin role and roles:write)
  POST /api/v1/admin/roles
  Body: {"name":"Reviewer","description":"Approves reports","permission_ids":[11,16]}

### Replace Role (admin role and roles:write)
  PUT /api/v1/admin/roles/:id
  Body: {"name":"Reviewer","description":"","permission_ids":[11]}

### Delete Role (admin role and roles:write)
  DELETE /api/v1/admin/roles/:id

### List Permissions
  GET /api/v1/admin/permissions?page=1&limit=10&resource=users

### Create Permission
  POST /api/v1/admin/permissions
  Body: {"name":"posts:read","resource":"posts","action":"read"}

### Change User Role (admin role and users:write)
  PUT /api/v1/admin/users/:id/role
  Body: {"role_id": 2}
