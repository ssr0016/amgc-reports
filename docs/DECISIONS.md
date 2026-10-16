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
| F-1 | Change password | Done | here |
| L-3 | Reset-password strength | Done | here |
| R-1 | Railway pre-deploy source of truth | Verified | here |

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
