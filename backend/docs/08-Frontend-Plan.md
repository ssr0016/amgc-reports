# 08 Frontend Plan (Phase 4)

**Status:** Reviewed by the owner. Frontend work is not started; the backend task B-F1 (section 2.1) is implemented.
**Depends on:** `05-Backend-Schema.md`, `07-Reports-Decisions.md`, `API.md`, and the Swagger spec (`docs.go`, `swagger.json`).
**Scope:** the Reports feature for signed-in users (login, list, create, edit, delete, approve). Out of scope for v1: admin screens (roles, permissions, users), Excel export (see D-10), email flows (E-1).

## 1. Decisions

### 1.1 Domain layout: custom domain, two subdomains (owner decision)

`up.railway.app` is on the Public Suffix List, so two `*.up.railway.app` hosts are different sites. The Strict CSRF cookie would not be sent, and every write would fail (PRD R-02). Decision: one registrable domain, two subdomains.

| Host | Serves | Hosting |
|---|---|---|
| `app.<domain>` | Next.js frontend | Vercel (D-F2) |
| `api.<domain>` | Go backend | Railway (custom domain) |

Both hosts are the same site, so the `Lax` session cookie and the `Strict` CSRF cookie are both sent on `fetch` with `credentials: "include"`. No backend code change is needed. Rejected: a same-origin proxy (every request would reach the backend from the proxy's IP, so the per-IP auth limiters would share one bucket; the IP trust would have to be re-measured) and `SameSite=None` (both cookies would have to change, and CSRF protection gets weaker).

Backend configuration only:
- `CORS_ALLOWED_ORIGINS=https://app.<domain>` (it defaults to `http://localhost:3000`).
- `APP_URL=https://api.<domain>`; `APP_ENV=production` (the session cookie is `Secure` only then).
- Railway custom domain `api.<domain>`; check `/health` and that `peer_ip` and `remote_ip` in the logs still look right (the real client IP must come through the Railway edge as before).

### 1.2 Data fetching: from the browser (client-side)

The session cookie has no `Domain`, so it belongs to `api.<domain>` only. The Next.js server on `app.<domain>` never receives it. Consequences:
- Authenticated data is fetched from the browser (`fetch` with `credentials: "include"`), not in Server Components.
- `proxy.ts` (formerly middleware) cannot see the session, so it cannot gate pages. Route protection is client-side: a layout checks the session and redirects to `/login`. Expect a brief loading state, not a flash of private content, because the pages hold no data until the API answers.
- Optional later change (backend): set `Domain=.<domain>` on the session cookie. Then Server Components and `proxy.ts` could use it. That is a security-relevant change (the cookie goes to every subdomain), so it is not in v1. Record it as D-F1.

### 1.3 Framework

Next.js 16.4 (existing scaffold), React 19, Tailwind 4, TypeScript. `cacheComponents` is already enabled in `next.config.ts`; pages that show private data are client components, so they are not prerendered with user data. **Next 16 differs from earlier versions** (`middleware` is now `proxy`; Cache Components changes how request-time data works). Before writing any code, read the relevant guides in `frontend/node_modules/next/dist/docs/01-app/02-guides/` (`authentication-with-cache-components.md`, `data-security.md`, `forms.md`, `client-side-data-fetching/`) and the `proxy.md` file convention.

Libraries (small, to be confirmed at setup): a data-fetching cache (TanStack Query or SWR), a schema validator for forms (Zod), and Vitest plus Testing Library and Playwright for tests. No UI kit is decided; Tailwind only for v1.

## 2. Backend contract the frontend relies on

| Topic | Behaviour |
|---|---|
| CSRF | `GET /auth/csrf` returns `{"csrf_token": "..."}` and sets the `HttpOnly` `csrf_token` cookie. Send the token as `X-CSRF-Token` on every POST, PUT, PATCH and DELETE. A missing or wrong token is 403 (`CSRF token invalid or missing`). Keep the token in memory; refetch once on that 403. |
| Session | Cookie set by login. Lifetime 24 h, idle timeout 30 min. Any 401 means "not signed in": clear client state and go to `/login`. |
| Numbers | JSON numbers, 0 to 9999999999.99, at most 2 decimals. The server returns the database scale (`0.5` comes back as `0.50`). Use `json.Number`-safe handling: parse with `Number()` for display, but send the user's text as a number without float rounding (for example parse the string, validate it, send as a number literal). A reported `0` is different from an absent week. |
| Metrics | 21 snake_case keys: `worship_service`, `sunday_school`, `prayer_meeting`, `bible_studies`, `mens_fellowship`, `womens_fellowship`, `youth_fellowship`, `children_fellowship`, `tithes_offering`, `home_visited`, `bible_study_group_led`, `sermon_preached`, `person_newly_contacted`, `person_followed_up`, `person_evangelized`, `outreach`, `training`, `leadership`, `baptism`, `other`, `family_day`. Weeks `week1` to `week5`; send only weeks with a value. |
| Update | `PUT` is a full replace. Every text field left out is cleared, and the whole metric set is replaced. The edit form must send every field. A missing `year` keeps the current one. |
| Toggle | `PATCH /reports/:id/complete` returns metadata only (`id`, `completed`, `completed_at`, `completed_by`). Merge it into the cached report; do not expect text fields. |
| Delete | 204, no body. The month and year can be filed again. |
| IDs | May have gaps. Never assume contiguous IDs. |
| List | `{data, meta: {page, limit, total, total_pages}}`; `limit` 1 to 100; filters `month`, `year`, `worker`, `area`, `church`. Without `reports:read_any` the server returns only the caller's reports. |
| Empty fields | Empty text is stored as NULL and omitted from responses. Treat a missing field as `""`. |

### 2.1 Role and permissions in `/auth/me` (B-F1, implemented)

`GET /auth/me` returns `{"user_id", "name", "email", "role", "permissions": [...]}`. `permissions` is always an array (never null), sorted by name, for example `["reports:delete", "reports:read", "reports:write", "users:read"]`. A user without a role gets `"role": ""` and `[]`. The values are read live from the database, so a role change shows on the next call; do not cache them past the session, and call `/auth/me` once at app start and after login, not on a timer (it costs two queries). A 401 means the session is gone or its user was deleted. The UI uses the list to show or hide actions (for example Approve needs `reports:approve`; seeing everyone's reports needs `reports:read_any`); the server still enforces everything.

## 3. Auth flow

1. App start: `GET /auth/csrf` (sets the cookie, returns the token), then `GET /auth/me`. 200 means signed in; 401 means show `/login`.
2. Register: `POST /auth/register` (`email`, `name`, `password`; password rules from the validator). 409 means the email is taken. Rate limited (5 per minute per IP, burst 3): show the `Retry-After` wait on 429. New accounts get the `user` role. Email verification is not enforced at login (H-3 is blocked on E-1).
3. Login: `POST /auth/login`. 401 invalid credentials, 403 account locked (show "try again later"). The server renews the session token; refetch `/auth/csrf` and `/auth/me` afterwards.
4. Logout: `POST /auth/logout`, clear all client caches.
5. Any 401 mid-session: clear caches, redirect to `/login`, keep the intended URL.

## 4. Features (v1)

| Screen | Behaviour |
|---|---|
| Login, Register | As above. Field errors from 422 `details` (a map of field name to message). |
| Reports list | Filters (month, year, worker, area, church), pagination, newest first, loading and empty states. Rows show month, year, worker, church, completed badge. |
| Report form (create and edit) | Fields per D-7 limits (`worker`, `area_assignment`, `church_name` ≤ 150; `names` ≤ 5,000; `narrative_report`, `challenges`, `prayer_request` ≤ 10,000; year 2000 to 2100). Metrics grid: 21 metrics by 5 weeks, decimals with at most 2 places, blank means not reported, `0` means reported zero. Edit loads the full report and sends every field. Warn before leaving with unsaved changes. |
| Report detail | Read-only view, with Edit, Delete and (if permitted) Approve or Unapprove. |
| Delete | Confirm dialog, then soft delete. |
| Approve | Visible to users with `reports:approve`. Optimistic update from the metadata response. |
| Excel export | Out of scope (D-10: the audit must be written server-side where the download happens, so the export waits for a server endpoint). |

## 5. Error handling

| Status | Meaning | UI |
|---|---|---|
| 401 | No or expired session | Clear state, go to login, return to the page after sign-in |
| 403 `CSRF token invalid or missing` | Stale CSRF token | Refetch the token and retry the request once |
| 403 `insufficient permissions` | Missing permission | Show a "not allowed" message; hide the control next time |
| 404 | Missing, deleted, or not yours (the same response, by design) | "Report not found" page |
| 409 | Duplicate month and year (create or edit) | Inline message on the month field with a link to the existing report if known |
| 422 | Validation failed | Map `details` (field to message) onto form fields |
| 429 | Rate limited | Read the `Retry-After` header (exposed by CORS), disable the action and show a countdown |
| 500 | Server error | Generic message; the response carries no detail |
| Network | Offline or timeout | Retry option; never discard the user's form input |

All error bodies look like `{"error": CODE, "message": text}`, plus `details` for 422.

Rate limits to know: reports writes are 60 per minute with a burst of 10, per signed-in user. Reports reads (list and detail) are 60 per minute with a burst of 20, per signed-in user. Every `/api/v1` route also has a per-IP limit of 120 per minute with a burst of 60. Auth routes are per IP: login and register 5 per minute with burst 3.

## 6. Testing strategy

- **Unit (Vitest):** API client (CSRF retry, 401 handling, 429 parsing, error mapping), number and metric handling, form schema.
- **Component (Testing Library):** list filters and pagination, form validation and full-replace behaviour, permission-gated buttons.
- **End to end (Playwright):** against a local backend and Postgres: register, login, create, duplicate (409), edit, approve as editor (403 as user), delete, logout. Runs in CI with a Postgres service.
- **Contract:** types generated from `swagger.json` (convert Swagger 2.0 to OpenAPI 3 first) and checked in CI so a backend change breaks the build, not the user.
- **Production smoke:** with a dedicated test account and fake data (audit rows are permanent).

## 7. Deployment

- Frontend on Vercel, Netlify or Cloudflare Pages at `app.<domain>`; one environment variable `NEXT_PUBLIC_API_URL=https://api.<domain>/api/v1`.
- Backend: Railway custom domain `api.<domain>`; set `CORS_ALLOWED_ORIGINS`, `APP_URL` (section 1.1).
- Local development: frontend on `http://localhost:3000`, backend on `http://localhost:8080`. The default `CORS_ALLOWED_ORIGINS` already allows this. Both are `localhost`, which browsers treat as the same site, so the cookies work without HTTPS; the session cookie is `Secure` only when `APP_ENV=production`.
- CI: the existing frontend job (install, lint, build) gains typecheck, unit tests and, later, the Playwright job.

## 8. Task breakdown and estimates

Hours are for one developer and are estimates, not commitments. They exclude Excel export and admin screens.

| ID | Task | Hours |
|---|---|---|
| F0 | Buy the domain, DNS, Railway custom domain, hosting project, env vars, CORS and `APP_URL`; verify cookies and `Secure` flags in production | 3 |
| F1 | Read the Next 16 docs listed in 1.3; project setup (env, folders, lint, typecheck) | 3 |
| F2 | API client: fetch wrapper with credentials, CSRF fetch and retry, error mapping, 429 and 401 handling | 5 |
| F3 | Generated types from the Swagger spec, checked in CI | 2 |
| B-F1 | Backend: role and permissions in `/auth/me` (section 2.1). **Delivered** (commits 874830c, 510e294); not counted in the total | 0 |
| F4 | Auth UI: register, login, logout, session bootstrap, client-side route guard | 8 |
| F5 | Permissions context from `/auth/me` and gated controls | 2 |
| F6 | Reports list: filters, pagination, states | 8 |
| F7 | Report form: create and edit, 21 by 5 metrics grid, validation, full-replace, unsaved-changes guard | 16 |
| F8 | Report detail and delete with confirmation | 4 |
| F9 | Approve and unapprove UI | 3 |
| F10 | Error UX across screens (409, 422 mapping, 429 countdown, network) | 5 |
| F11 | Tests: unit 6, component 6, Playwright 10 | 22 |
| F12 | CI: typecheck, tests, Playwright with Postgres | 4 |
| F13 | Accessibility and responsive polish | 6 |
| F14 | Production smoke test and fixes | 2 |
| | **Total** | **93** |

### F4 details (added 2026-10-14)

**401 hook (from F2).** `setOnUnauthorized(handler)` in `src/lib/api.ts` fires on every 401 except `/auth/login` (a wrong password). It also fires for `/auth/me`, which is how the session bootstrap learns the user is signed out. If the handler redirects to `/login`, the bootstrap on `/login` can loop. F4 must do one of: skip the redirect when already on `/login`; make `getCurrentUser` bypass the hook; or guard the hook against `/auth/me`.

**Resolved in F4.** The 401 loop is avoided with `silent401` on `getCurrentUser`, so the `/auth/me` bootstrap does not fire the hook. The route guard lives in `src/app/(app)/layout.tsx`.

**Later tasks (not in F4).** Forgot-password, reset-password and verify-email pages. The backend endpoints exist (`/auth/forgot-password`, `/auth/reset-password`, `/auth/verify-email`).

**For F10.** `SessionProvider` treats any `/auth/me` failure (network error, 5xx) as signed out, so a server blip sends a signed-in user to `/login`. F10 should add a retry state that tells "signed out" (401) from "could not check".

**Register then sign in.** `/register` creates the account, then signs in. If the sign-in fails (network error, rate limit), the account already exists, so retrying the form shows "already registered" and the user must use `/login`. Register and login share one rate-limit bucket (burst 3), so the pair spends two tokens.

**Dev note.** After moving or deleting a route file, `tsc` can fail on a stale `.next/dev/types/validator.ts` that points at the old path. It is a build artifact, not a bug: run `rm -rf .next`.

### F7 details (added 2026-10-14)

**Form state.** Plain controlled state, no Zod or react-hook-form (one server-side source of truth; revisit only if the form grows). Values are strings while typing so a blank week (not reported) stays different from `0` (reported zero); `toInput` in `src/lib/report-form.ts` drops blank weeks and sends every text field, because PUT is a full replace. The edit page loads the report with no cache (`gcTime: 0`) so a stale copy can never overwrite newer data.

**Backend limitation: server week errors have no metric.** On a 422, a bad metric key comes back as `metrics[key]`, but a bad week value comes back as a bare `week1`..`week5` with no metric name, so the server cannot say which row is wrong. The client validator (`validate`) uses the exact keys `metrics.<key>.<week>` and mirrors the server's number rule, so this should only show if the two rules drift apart. `fromServerDetails` maps `metrics[key]` onto that metric's row and puts the bare week errors in one general message. A backend change to name the metric would let the form highlight the cell.

**Draft saving left out (later option).** The form does not save a draft to `localStorage`. Narrative reports can hold sensitive pastoral content and church devices may be shared, so a draft would outlive the session. What is covered: a failed save never clears the form, the browser warns before the tab is closed with unsaved edits, and Cancel asks first. The warning does not cover in-app navigation (Back, a link): the App Router has no leave-blocker. If drafts are wanted later, keep them in `sessionStorage`, key them by user, and clear them on sign-out.

**Instant-navigation warning (dev only).** Next 16 validates that a page can render instantly on navigation. The `(app)` layout shows "Loading…" until `/auth/me` answers, so on the server it never renders the page, and Next reports the page as a "dropped segment". `instant = false` must be exported by the page segment itself, and a Client Component cannot export it, so `/reports`, `/reports/new` and `/reports/[id]/edit` are thin Server Component pages that export it and render the client view from a sibling file (`reports-list.tsx`, `new-report.tsx`, `edit-report.tsx`). A layout above the page does not help. Use the same split for any new signed-in page. `/` has no page of its own (a config redirect to `/reports`), so it never warned once the old home page was gone. Checked in the browser on 2026-10-14: the console is empty on `/reports`, `/`, `/reports/new`, `/reports/[id]/edit` and `/login`. The warning is dev-only and never blocked the build.

**Live checks (2026-10-14, local, admin).** Valid save redirects and lists the row; a duplicate month and year shows the 409 on the month field with the input kept; a blank week is absent and a typed `0` is stored as `0`; editing one field leaves every other field and metric unchanged. Not yet checked: the `user` role (own reports only), the 429 countdown, a real 422, and the phone layout.

**Known issue, not a bug: hydration warning from a browser extension.** With some extensions installed (seen with ColorZilla) the dev console shows "A tree hydrated but some attributes of the server rendered HTML didn't match the client properties", with `cz-shortcut-listen="true"` on `<body>`. The extension adds that attribute before React hydrates, so it is not our code. React prints this warning in development only, and the app behaves the same. Ignore it, or test in a private window or with the extension off. We do not add `suppressHydrationWarning` to `<body>`: it would hide real mismatches there too. (The root layout already sets it on `<html>`.)

### Account onboarding (Part B, decided 2026-10-14)

**Rule (owner).** Only an admin creates accounts: there are 24 churches, and the admin gives each pastor an account to report with. Self-registration is gone.

**Backend (done).**
- `POST /auth/register` is removed (route, handler and `AuthService.Register`). Requests to it never create an account.
- `POST /admin/users` creates an account: `email`, `name`, `password`, optional `role_id` (default `user`) and the profile text `area_assignment`, `church_name`. `PUT /admin/users/:id` replaces the name and profile text. Both are in the admin-role group and also require `users:write`, so only an admin can call them (an editor holds `users:write` but is not an admin). Each change and its audit row (`CREATE_USER`, `UPDATE_USER`) are written in one transaction.
- `users` has nullable `area_assignment` and `church_name` (150 characters). It has no `worker` column: a report's worker defaults to the user's `name`. The first version added one; migration `20261014110000_drop_user_worker` removed it as redundant (the report itself keeps its own `worker`, which stays editable). `/auth/me` returns them as strings (empty when unset). The API reference is `API.md`.
- Admin-created accounts get no email verification token (verification is not enforced at login, and email delivery is E-1).

**Frontend (still to do).**
1. Run `npm run gen:api` (the CI check fails until the generated types match the new `swagger.json`).
2. `/reports/new` fills worker (the user's `name`), area and church (from `/auth/me`) as **editable defaults** (decision D-8: `worker` is independent of `created_by`, so they are not locked).
3. Delete the `/register` page and the "No account? Register" link (the endpoint is gone).
4. Show Worker, Area and Church with labels on each report in the list and the detail view.
5. Admin create-user and edit-user screens (F9 or F10). Until they exist, the admin creates and edits users with `curl` or a script using an admin session.

**Known gaps.**
- Existing accounts and the seeded admin have empty profiles until an admin sets them with `PUT /admin/users/:id`.
- There is no admin way to reset a password or deactivate an account yet; the existing password-reset flow needs email (E-1).
- `NewAuthService` still takes a verification service it no longer uses; remove the parameter in a cleanup.
- `01-PRD.md`, `02-App-Flow.md`, `03-Tech-Stack.md` and `06-Implementation-Plan.md` still describe self-registration (FR-AUTH-01, the middleware table, the rate-limit notes); they are analysis documents and have not been rewritten.

**Also pending.** The previous-month rule is enforced in the UI only (`/reports/new` locks the period); the API still accepts any month. Doing it in the backend needs decisions on the server timezone, whether admins may file late reports, and the integration tests that create reports for arbitrary months. F8 is partly done: the `/` redirect and the app header are in; the alert dialog and the report detail with delete are parked.

### F14 details (added 2026-10-14)

**Deployment architecture.** `app.<domain>` is the Next.js frontend on Vercel. `api.<domain>` is the Go backend on Railway. Cloudflare holds the DNS records for both. Both hosts are on one registrable domain, so they are the same site (section 1.1). This is the setup that the production smoke test in F14 runs against (see F0 for the setup work).

**Cost estimate.** About ₱3,900 to ₱5,400 per year in total (D-F2, D-F6). Estimate only; check the domain and hosting prices when buying.

**Cookie domain fix requirement.** D-F1 sets `Domain=.<domain>` on the session cookie. F14 must verify in production that: the cookie carries the domain and the `Secure` flag; `app.<domain>` receives it; `Lax` and `Strict` behaviour is unchanged; and CSRF writes still work from the browser.

**Client IP behind Cloudflare (F0, F14; no code until the domain exists).** Once DNS is proxied through Cloudflare, the real client address arrives in `CF-Connecting-IP`, not `X-Real-IP`. The backend reads `X-Real-IP` only from Railway's trusted range (`TrustedProxyExtractor`), so behind Cloudflare it would probably see Cloudflare's address. Every user would then share a few rate-limit buckets (the login, global and read limits are per IP), which is R-F1. The fix is to read `CF-Connecting-IP`, and only when the TCP peer is a Cloudflare or Railway range. Verify on the real deployment: log `peer_ip` and `remote_ip` for requests from two different networks and confirm they differ. The limiters are in memory, so they hold for one instance only.

Suggested order: F0 and F1 first (they unblock everything and carry the infrastructure risk), then F2, F3, F4, F5, then F6 to F10 as vertical slices (list, then form, then detail, delete and approve), tests alongside each slice, and F12 to F14 last.

## 9. Open decisions and risks

| ID | Question | Notes |
|---|---|---|
| D-F1 | Set `Domain=.<domain>` on the session cookie so Server Components and `proxy.ts` can use the session? | Not in v1. Security trade-off: the cookie is sent to every subdomain. |
| D-F2 | Which domain, and which hosting? | **Decided (owner):** a cheap `.com` or `.app` domain (for example `amgreports.com`; not bought yet) and Vercel (native Next.js, free tier). |
| D-F3 | Approve B-F1 (permissions in `/auth/me`)? | **Decided and delivered.** |
| D-F4 | English labels for the 21 metrics in the UI | **Done in F7.** The labels come from the legacy Express field names and live in `METRICS` in `src/lib/report-form.ts`. |
| D-F5 | Excel export timing | Waits for a server-side export endpoint (D-10). |
| R-F1 | The first production run on a custom domain is untested: cookie `Secure` flags, CSRF cookie scheme detection behind the proxy, `peer_ip` and `remote_ip` | Verify in F0 before building features on top. |
| R-F2 | The scaffold is Next 16.4, whose conventions differ from older versions | Read the bundled docs first (section 1.3). |
| R-F3 | Phone and slow-network use is likely for church workers | Keep the form resilient (save-draft locally, never lose input on a failed save). |
| R-F4 | `npm audit` reports 5 high-severity findings (GHSA-vfj7-8cjw-p6xm, `braces` stack-exhaustion DoS) through `eslint-config-next` → `@next/eslint-plugin-next` → `fast-glob` → `micromatch` → `braces`. Dev-only lint tooling: `npm audit --omit=dev` finds 0. Present before F3 (checked 2026-10-14). | No production exposure. Not fixed: `npm audit fix --force` would downgrade `eslint-config-next` to 14.x, which breaks Next 16. Wait for a newer `braces` or `eslint-config-next` release, then re-run `npm audit`. |
| R-F5 | Open redirect through the login `?next=` value (found in review, fixed in `f1469f5`): a leading-slash check let `/\t/evil.com` and `/\n/evil.com` through, and browsers strip tabs and newlines, so they resolved to another origin. | `safeNext` in `src/lib/auth-form.ts` now rejects control characters and checks the resolved origin. Any new redirect that takes a URL from the query string must go through `safeNext`. |
