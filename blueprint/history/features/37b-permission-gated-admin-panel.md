# Feature: Permission-gated admin panel

**From build-plan:** feature 37b
**Build attempt:** 1
**Branch:** `feature/permission-gated-admin-panel`
**Status:** verified (2026-10-22)

## Goal

A custom role (e.g. "Reviewer") can open `/admin` and use the review areas its permissions allow: Reports (approve, Excel export), Tracker and stats cards, the users list (read-only) and Logs. Account management and the Roles tab stay admin-only. Second half of Dynamic RBAC (D-A21); owner decisions of 2026-10-22, recorded in this session.

## In scope

- **Four new seeded permissions** in `backend/internal/database/seed.go`: `reports:export` (resource `reports`, action `export`), `tracker:read` (`tracker`, `read`), `audit:read` (`audit`, `read`), `users:list` (`users`, `list`). The `admin` role gets them through the existing "every permission" grant. `editor` and `user` get none.
- **Route gates** in `backend/internal/router/router.go`. The `/admin` group drops its group-wide `RequireRole(userRepo, "admin")` and keeps `authn` and `RequireAuth()`. Each route gets its own gate:

  | Route | Gate after 37b | Before |
  |---|---|---|
  | `GET /users`, `GET /users/:id` | `perm("users:list")` | admin role |
  | `POST /admin/reports/excel`, `POST /admin/reports/:id/excel`, `POST /admin/reports/excel/preview` | `perm("reports:export")` | admin role + `reports:read_any` |
  | `GET /admin/stats` | `perm("tracker:read")` | admin role |
  | `GET /admin/tracker` | `perm("tracker:read")` | admin role + `reports:read_any` |
  | `GET /admin/audit-logs` | `perm("audit:read")` | admin role |
  | every `/admin/roles*`, `/admin/permissions*` route | `adminOnly` (+ existing `roles:write` on writes) | unchanged in effect |
  | every `/admin/users*` route (get, create, update, status, role, delete, restore, reset-password) | `adminOnly` + existing `users:write` on writes | unchanged in effect |

  `adminOnly := ourmiddleware.RequireRole(userRepo, "admin")`, declared next to `perm` and passed per route the same way. Limiters stay as they are.
- **Frontend gating** with the existing `can()` / `<Can>` and `isAdmin` from `useSession()`:
  - `frontend/src/lib/permissions.ts`: add the four names to `Permission`, and export `ADMIN_AREA: Permission[] = ["reports:approve", "reports:export", "tracker:read", "users:list", "audit:read"]`.
  - `frontend/src/lib/session.tsx`: add `canAdmin = isAdmin || ADMIN_AREA.some(can)` to the session value.
  - `frontend/src/app/(app)/sidebar.tsx`: show Admin when `canAdmin` (was `isAdmin`).
  - `frontend/src/app/(app)/admin/admin-panel.tsx`: the panel renders when `canAdmin`, otherwise the existing "You are not allowed to view the admin panel." Each tab trigger and content renders only when allowed: Reports (`reports:approve` or `reports:export`), Tracker (`tracker:read`), Users (`users:list`), Roles (`isAdmin`), Logs (`audit:read`). A `?tab=` that is missing, unknown or not allowed falls back to the first visible tab, in the existing order. `StatsCards` renders only with `tracker:read`.
  - `stats-cards.tsx`: `enabled`/render on `can("tracker:read")` instead of `isAdmin`.
  - `admin-reports.tsx`: wrap the row "Excel" button and "Download Bulk" in `<Can permission="reports:export">`, with the same `"—"` fallback the Approve cell uses for the row button. Approve is already `<Can permission="reports:approve">`.
  - `users/users-list.tsx`: the list query and the "not allowed" check use `can("users:list")`. The row Edit link, the "New user" button and the roles query (`GET /admin/roles` stays admin-only) stay `isAdmin`. Without the roles query, a non-admin sees the role filter hidden and the Role column falls back to the existing `#id` rendering.
- **Swagger**: update the `@Description` auth sentences on the stats, tracker, audit-logs, Excel (three) and `/users` handlers, then `make swagger` and `npm run gen:api`.
- **Docs**: amend D-A21 in `docs/DECISIONS.md` (37b decisions below), the permissions and `/admin` passage in `docs/ARCHITECTURE.md`, the auth lines in `docs/API.md`, and `blueprint/context/project-overview.md` line 44.

## Out of scope

- Opening any account-management write, the Edit/New User pages, `GET /admin/users/:id`, or the Roles tab to non-admins (owner decision 2).
- Granting new permissions to `editor` (decision 3).
- Showing role names to non-admin `users:list` holders (would need a `role_name` in the `/users` response or opening `GET /admin/roles`).
- Changing who the Tracker and Excel dashboard count (D-A3, D-A16 stay; the D-A21 open question stays open).
- Removing `reports:read_any` from the plain `user` role, or the unused `*:delete`/`permissions:*` permissions.
- A migration: permissions and grants are seed data (AGENTS.md), and there are no new audit actions.

## Build loop

`workflow.stepReview` is `feature`: build every step, one review at the end. `workflow.checkpointCommits` is `disabled`. Tests first in each step. Run `make lint` to 0 issues and `npm run lint && npm run typecheck` before presenting. This changes authorization, so independent review applies (`when-sensitive`).

**Deploy:** `SeedData` runs on API startup (`backend/cmd/api/app/server.go:98`), so production gets the four permissions, and `admin` gets them, on the first boot of the new code. No Neon migration is needed. The admin keeps every area because the grant and the new gates ship in the same binary.

## Build steps

- [x] **1. Seed the four permissions (tests first)**
  Add them to the `permissions` slice in `seed.go`. Extend `backend/internal/database/seed_integration_test.go`: after `SeedData`, the four exist, `admin` holds them, `editor` and `user` do not, and a second run is still a no-op.
  *Done when:* `make test-integration` passes the seed test.

- [x] **2. Route gates (tests first)**
  In the router tests, add a custom `reviewer` role holding `reports:read`, `reports:read_any`, `reports:approve`, `reports:export`, `tracker:read`, `users:list` and `audit:read`, plus `users:write` and `roles:write` to prove those don't open admin-only routes. Cover in the matching files (`stats_routes_test.go`, `audit_routes_test.go`, `admin_reports_routes_test.go`, `admin_users_routes_test.go`, `roles_routes_test.go`):
  - reviewer gets 200 on stats, tracker, audit-logs, Excel preview and both exports, and `GET /users`;
  - reviewer gets 403 on every `/admin/users*` and `/admin/roles*` route, including `PUT /admin/users/:id/role` and reset-password;
  - plain `user` gets 403 on stats, tracker, audit-logs, Excel and `GET /users` (it holds `reports:read_any` and `users:read`, which no longer matter here);
  - `admin` still gets 200 everywhere it did; a role missing one permission gets 403 on that route only.
  Then change `router.go` per the table, update the stale "group already requires admin role" comments, and remove `_ = roleRepo` if it's no longer needed.
  *Done when:* `make test` and `make lint` pass.

- [x] **3. Swagger and generated types**
  Update the `@Description` auth sentences, run `make swagger`, then `npm run gen:api`.
  *Done when:* `git diff frontend/src/lib/api-types.ts` holds only description changes and `npm run typecheck` passes.

- [x] **4. Session, sidebar and admin panel (tests first)**
  `permissions.ts`, `session.tsx` (`canAdmin`), `sidebar.tsx`, `admin-panel.tsx`, `stats-cards.tsx`. Extend `session.test.tsx`, `sidebar.test.tsx`, `admin-panel.test.tsx`, `stats-cards.test.tsx`:
  - a reviewer-style session sees Admin in the sidebar and only its allowed tabs; a session with only `tracker:read` sees Tracker and the cards, and no Reports tab;
  - `?tab=roles` for a non-admin lands on the first allowed tab; `?tab=logs` without `audit:read` likewise;
  - a plain user (`reports:read`, `reports:read_any`, `users:read`) sees no Admin link and gets the "not allowed" message on `/admin`;
  - admin sees all five tabs and the cards, as before.
  *Done when:* `npm test` passes these files.

- [x] **5. Tab contents (tests first)**
  `admin-reports.tsx` (Excel and Download Bulk behind `reports:export`) and `users-list.tsx` (list on `users:list`; Edit, New user and roles query stay `isAdmin`). Extend `admin-reports.test.tsx` and `users-list.test.tsx`: without `reports:export` there's no Excel or Download Bulk button and no export call; a `users:list` non-admin sees the list with no Edit link, no New user button, no role filter, and no `GET /admin/roles` request.
  *Done when:* `npm test`, `npm run lint` and `npm run typecheck` pass.

- [x] **6. Browser check and docs**
  Add a Playwright case in the existing admin e2e area for a reviewer-role user: sign in, open Admin, see only the allowed tabs, approve a report, and confirm `/admin?tab=roles` shows the first allowed tab instead; run axe on the restricted panel. Check how the existing e2e seeds users and roles before choosing the setup. Update `docs/DECISIONS.md` (D-A21 37b amendment), `docs/ARCHITECTURE.md`, `docs/API.md` and `project-overview.md`.
  *Done when:* `npm run e2e` passes the new case and the docs name the four permissions and the per-route gates.

## Files / areas

- Backend: `backend/internal/database/seed.go`, `seed_integration_test.go`; `backend/internal/router/router.go` and the route test files named in step 2; handler `@Description`s in `backend/internal/stats/rest/stats_handler.go`, `backend/internal/audit/rest/audit_handler.go`, the report Excel handlers and the `/users` list handler; `backend/docs/` swagger output.
- Frontend: `src/lib/permissions.ts`, `src/lib/session.tsx`, `src/lib/api-types.ts` (generated), `src/app/(app)/sidebar.tsx`, `src/app/(app)/admin/{admin-panel,stats-cards,admin-reports}.tsx`, `src/app/(app)/admin/users/users-list.tsx`, their tests, and the e2e spec.
- Docs: `docs/DECISIONS.md`, `docs/ARCHITECTURE.md`, `docs/API.md`, `blueprint/context/project-overview.md`.

## Data / contracts

- **Permissions** (`permissions` table, seed only, `ON CONFLICT (name) DO NOTHING`): `reports:export`, `tracker:read`, `audit:read`, `users:list`. Names are stable from this deploy on: the seed re-grants by name on every start, so a rename leaves the old row behind.
- **Denied responses** are the existing ones: 403 `insufficient permissions: <name>` from `RequirePermission`, 403 from `RequireRole`; 401 when the account can't sign in or the token was revoked. No new error shapes.
- **Response bodies** are unchanged. `GET /auth/me` already returns `permissions`, which the frontend reads.
- **Trusted actor**: the gates read the caller's role and permissions from the database on every request (H-2), so a role change applies on the next request.
- The Reports tab still lists through `GET /reports` (`reports:read`; `reports:read_any` to see every worker's reports). A custom role needs those to see the list; without them the tab shows its existing "not allowed" state.

## Testing

- Go unit route tests (step 2) and the seed integration test (step 1, Docker).
- Vitest for session, sidebar, admin panel, stats cards, admin reports, users list.
- One Playwright case with axe (step 6).
- No Verify command is declared in `project-overview.md`; the gate is `make test`, `make test-integration`, `make lint`, `npm test`, `npm run lint`, `npm run typecheck`, `npm run e2e`.

## Notes for the AI

- **Security boundary**: every account-management route stays behind `adminOnly`. A custom role holding `users:write` or `roles:write` must still get 403 on all of them, because `users:write` includes password reset, which could take over an admin's account. Step 2's tests prove this; don't drop them.
- The plain `user` role holds `users:read` and `reports:read_any` (`seed.go`). Never use either as an admin-area gate.
- Hiding a tab is convenience. The server enforces every gate; each tab keeps its 403 handling.
- Keep the tab order and the `router.replace` behaviour in `admin-panel.tsx`.
- `ADMIN_AREA` must match the permissions that open a tab. Update both together.
- D-A21's "the permission list is fixed" now means the UI never creates permissions. Code adds them through the seed. Say so in the amendment.
- The Roles checklist groups by resource (`groupPermissions`): `tracker` and `audit` get their own groups. Check that `actionWords` in `src/lib/roles.ts` reads well for the new actions `export` and `list`; add words if it falls back to the raw code.

## Verification (2026-10-22)

- `make test` (25 packages ok), `make test-integration` (26 ok, incl. `TestSeed_AdminAreaPermissions`), `make lint` 0 issues.
- `npm test` 437/437, `npm run lint` clean, `npm run typecheck` clean.
- `npm run e2e`: the new `custom-role-admin.spec.ts` passes; full runs had 1 unrelated failure each time (`roles.spec` once, `reports.spec` once), both passing alone. The baseline `main` (219cf47, without 37b) also failed 2 different specs (`reports.spec` "lists every user's reports", `tracker.spec`): the shared-database e2e suite is order-flaky, independent of 37b.



<!-- blueprint:completion {"schemaVersion":2,"specBytes":12926,"specSha256":"7921eb8245bb6ed216df34481be5a2565a753961db2fd7c479e4f58ca45ca8ec","branch":"refs/heads/feature/permission-gated-admin-panel","head":"7a64a35c37a4fd9a2ad1b21a998aef16b49145dd","baseRef":"refs/heads/main","baseCommit":"219cf47c619246c927e42fcca5947613d5c5a9c2","sourceTree":"5efb7934c33b7daa8846c43eec9fd3b4ca89f905","landing":"local-merge","absentOptional":[]} -->

## Independent review

**Status:** passed
**Target commit:** 7a64a35c37a4fd9a2ad1b21a998aef16b49145dd
**Base commit:** 219cf47c619246c927e42fcca5947613d5c5a9c2
**Base ref:** main
**Spec hash:** 7921eb8245bb6ed216df34481be5a2565a753961db2fd7c479e4f58ca45ca8ec
**Prepared by:** claude
**Builder model:** claude-opus-5-5
**Requested reviewer:** claude
**Requested model:** claude-opus-5-5
**Requested execution:** automatic
**Requested at:** 2026-10-22T23:44:35+08:00
**Workflow:** regular
**Check required:** no
**Reviewer adapter:** claude
**Reviewer model:** claude-opus-5-5
**Reviewer context:** fresh subagent
**Actual execution:** automatic
**Reviewed at:** 2026-10-22T23:49:33+08:00
**Scope:** current
**Lenses:** quality, security, performance, tests
**Verdict:** passed
**Check result:** not-required

### Commands

- `git rev-parse HEAD`, `git merge-base main HEAD`, `sha256sum blueprint/context/current-feature.md`, `git status --short`: pass (all match the request; only review.md dirty)
- `make test` (backend): pass (0 failures)
- `make lint` (backend): pass (0 issues)
- `make test-integration` (backend, Docker): pass (26 packages ok, incl. database seed tests)
- `npm test` (frontend): pass (41 files, 437 tests)
- `npm run lint` (frontend): pass
- `npm run typecheck` (frontend): pass

### Evidence

- `backend/internal/router/router.go:80-151`: every `/admin` route carries its own gate; all `/admin/roles*`, `/admin/permissions*` and `/admin/users*` routes carry `adminOnly` (writes also `users:write`/`roles:write`); no route is left ungated. Limiters still run after the gates.
- Plain `user` role (`seed.go:117-127`: `users:read`, `reports:read`, `reports:read_any`, `reports:write`, `reports:delete`) holds none of `users:list`, `tracker:read`, `audit:read`, `reports:export`, and no admin role, so it reaches no admin-area route or `GET /users*`. `editor` likewise holds none.
- `admin` gets the four new permissions via the all-permissions grant; `SeedData` runs on every startup including production (`cmd/api/app/server.go:98`), and built-in roles cannot be edited, so the admin cannot lose them.
- `reports:export` without `reports:read_any` does not widen read scope: `oneForExport`/`monthForExport` (`reportimpl/report.go:394-433`) still apply the `_any` ownership scope.
- Router tests cover a custom role holding `users:write` and `roles:write` getting 403 on every `/admin/users*`, `/admin/roles*`, `/admin/permissions*` route and on `GET /admin/users/:id`; plain user 403 on stats, tracker, audit, Excel and `/users`; admin without the specific permission 403.
- Frontend: tabs filtered by permission, `?tab=` fallback to first allowed tab, Roles/Edit/New user/roles query remain `isAdmin`; `ADMIN_AREA` matches the tab gates.

### Findings

- None

### Remaining risk

- `npm run e2e` not run (instructions forbid it); the new `frontend/e2e/custom-role-admin.spec.ts` and the axe scan are unverified by this reviewer. Builder reports the shared-database e2e suite is order-flaky on main too; not verified here.
- Unmatched `/api/v1/admin/*` paths now return 404 to any signed-in user instead of 403 to non-admins (route existence is visible); low impact, not recorded as a finding.
- `ADMIN_AREA` in `frontend/src/lib/permissions.ts` must be kept in step with the tab gates in `admin-panel.tsx` by hand (commented in code).

## Try it now

Run or do:

In `backend/`: `make db-start`, `make migrate-up`, then `make dev` (startup seeds the four new permissions). In `frontend/`: `npm run dev`. Sign in at http://localhost:3000 as the admin, open **http://localhost:3000/admin?tab=roles**, create a role "Reviewer" with only `tracker:read` ticked, and give it to a worker on their **Edit** page. Sign in as that worker in a private window and open **http://localhost:3000/admin?tab=roles**.

Expected:

The sidebar shows **Admin**. The panel shows the stats cards and a single **Tracker** tab, already selected: no Reports, Users, Roles or Logs tab, even though the URL asked for Roles. A plain-user worker sees no Admin link and "You are not allowed to view the admin panel." on /admin.

This proves:

A custom role opens only the admin areas its permissions allow, and a tab it may not open falls back to its first allowed tab.
