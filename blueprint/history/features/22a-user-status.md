# Feature: User status

**From build-plan:** feature 22a
**Build attempt:** 1
**Branch:** feature/user-status
**Status:** verified

## Goal

Give every account a status (active, inactive, disciplinary, deceased) that an admin can change, and make the server enforce it: only an active account can sign in or keep a session. The Users tab shows the status and filters by it, and the Edit page changes it. Changes are audited, take effect at once by ending the user's sessions, and cannot lock the admin out (no self-change, never the last active admin). This feature also records D-A1 (amended), D-A10 and D-A11 in `docs/DECISIONS.md`.

## In scope

### Database and model (one migration, shipped in the same deploy as the code that writes the new audit action)

- `users.status TEXT NOT NULL DEFAULT 'active'` with `chk_users_status CHECK (status IN ('active','inactive','disciplinary','deceased'))`. Existing rows become `active` through the default. No index: the table holds about 24 rows (owner's earlier index idea skipped, an upgrade path if users ever grow).
- `chk_audit_logs_action` gains `CHANGE_USER_STATUS` (drop and re-add the constraint, as `20261015100000_allow_change_password_audit_action.sql` did, with a `NOT VALID` rollback list in `Down`).
- Go: `model.User.Status` and `UserResponse.Status` (`json:"status"`, always present), constants for the four values with a list, and `AuditChangeUserStatus` added to `AuditActions` (a test already compares that list with the CHECK constraint).

### Login and sessions (server)

- `AuthService.Login` order stays: find by email, lockout check, password check, **then** the status check. A wrong password on any account is still the generic 401, so the status messages never reveal that an email exists.
- After a correct password, a non-active account is a 403 with exactly these messages: inactive "Your account is inactive. Contact your admin."; disciplinary "Your account is under review. Contact your admin."; deceased "This account is closed.". A successful refusal does not reset the failed-attempt counter and does not create a session.
- The locked message becomes "Account locked. Try again later." (owner's wording; today "account is locked, try again later"), in `Login` and in the `Me` error mapping in `auth_handler.go`.
- An existing session stops working at once, two ways: (1) the status change revokes the user's sessions in the same transaction (`UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, the same tombstone `UpdatePassword` uses); (2) `GetWithRole` now returns the status, and `RequireRole`, `RequirePermission` and `AuthService.Me` answer 401 "authentication required" for a non-active user. (2) is the net for a session the revocation missed.

### Status change (server)

- `PUT /admin/users/:id/status`, body `{"status": "<one of the four>"}`, in the `admin` group (so `RequireRole("admin")`) with `perm("users:write")`, like `PUT /admin/users/:id`. Returns the updated `UserResponse`.
- `UserService.ChangeStatus(ctx, actorID, ip, id, status)` runs in one transaction with the audit row:
  1. Lock the active admins' rows (`SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id WHERE r.name = 'admin' AND u.status = 'active' ORDER BY u.id FOR UPDATE`), so two admins deactivating each other at the same time cannot both pass the check.
  2. Refuse a change to your own account: 403 "You cannot change your own status."
  3. Refuse moving the only active admin away from `active`: 409 "At least one active admin is required."
  4. Same status as now: 200, no write, no audit row, no session revoke.
  5. Update `status` and `updated_at`; when the new status is not `active`, revoke the user's sessions.
  6. Insert the `CHANGE_USER_STATUS` audit row: target the user's email (`userAudit`), details `{"from": "...", "to": "...", "sessions_revoked": N}`. The details never hold a password, hash or token.
- Errors: unknown id 404 "user not found"; non-numeric id 400; status not one of the four 422 with a `status` field error; not signed in 401; not admin 403. A failed audit insert rolls the whole change back (500).

### List and filter (server)

- `GET /users` (the call the Users tab makes) gains `status` (`all` or empty = no filter, otherwise one of the four). An unknown value is 422 with a `status` field error. `UserRepository.List` takes the status. `UserResponse` carries `status`.
- Swagger annotations on the new and changed handlers, then `make swagger` and `npm run gen:api` in the same commit (CI fails on drift in `frontend/src/lib/api-types.ts`). `User` in `lib/api-schema.ts` lists `status` among the required fields.

### Users tab and Edit page (frontend)

- **Users tab (`UsersPanel`):** a **Status** column after Role with a badge, and a **Status** filter select (default "All statuses", then Active, Inactive, Disciplinary, Deceased) beside the Role filter. The status filter is server-side (the query key includes it); the Role filter stays client-side (D-19). "No users match." covers an empty filtered result.
- **Badge:** a small `UserStatusBadge`. Every state shows its **text label** (colour is never the only cue): active green, inactive gray, disciplinary yellow, deceased black or near-black in light and dark themes. Colours must pass axe contrast in both themes.
- **Edit page:** under the profile form, a separate **Status** section: a labelled select with the four values (current one selected) and an **Update status** button, disabled while unchanged or saving. Moving a user away from Active asks first in an `AlertDialog` (the reports-delete pattern: Cancel, and a confirm button that is not `AlertDialogAction`, so it stays open while saving) that says the user is signed out everywhere and cannot sign in until set back to Active. Success: a toast with the server's answer, the `["users"]` and `["user", id]` queries refetch. An API error (403 self, 409 last admin, 404, anything else) shows its message in a `role="alert"` and leaves the select as the user chose. When the Edit page is the signed-in admin's own account the select is disabled with the note "You cannot change your own status." (the server still enforces it).
- **Login page:** drop the hard-coded 403 text so the server's message is shown (it is now one of four sentences). The 401 text stays.

### Docs (same commit)

- `docs/DECISIONS.md`: amend **D-A1** to "Deactivate (soft delete), not hard delete. The user row is kept and marked deleted. A hard delete is never done (`audit_logs.performed_by` is `ON DELETE SET NULL`)", and mark it built across 22a and 22b rather than Phase C only; add **D-A10 User status** (the four values, what each blocks, the login order and messages, session revoke, self and last-admin protection, no reason field on a status change) and **D-A11 Soft delete and restore** (decided here, built in 22b: reversible, optional reason, self and last-admin protection, the email stays reserved and a deleted user is restored, not recreated, a deleted user's reports stay visible with a "Deleted user" badge in 22c, hiding them later needs a JOIN and conflicts with D-18). Index rows for both.
- `docs/API.md`: the new endpoint and the `status` filter and field. `docs/ARCHITECTURE.md`: the new audit action, the migration in the history, and the status check beside the session notes.

## Out of scope

- Soft delete, restore, the Deleted filter and the deletion reason (22b); the "Deleted user" and "Archived" badges and owner status on reports (22c).
- Choosing a status when creating a user (a new account is `active`); a reason on a status change; email notice to the user; the Users card in the stats (it keeps counting every account); the Tracker (Phase B).
- Changing a role away from admin (`PUT /admin/users/:id/role`) can still remove the last admin; that gap belongs to a separate fix and is not widened here.
- Any change to password reset or email verification (an inactive user can still reset a password but still cannot sign in).
- The area and church dropdowns and the role descriptions (the later Create User work).

## Build loop

`workflow.stepReview` is `feature`: review once after all steps. `checkpointCommits` is disabled; `/complete` makes the final commit. Test-first inside each step. This feature carries a migration and writes an audit action: the CHECK migration must ship in the same deploy as the code (Railway runs `/app/api migrate` before the new code starts; the new column has a default, so the old code keeps working until then). The frontend deploys on Vercel on its own, so push only when the backend is ready. Stop any dev server before `npm run build` (or build with a separate `NEXT_DIST_DIR` and restore `tsconfig.json`). Backend commands run from `backend/` (`make test`, `make test-integration` needs Docker, `make lint`, `make swagger`), frontend commands from `frontend/`.

## Build steps

- [x] 1. **Migration, model and repository (tests first).** Add a goose migration `2026101612xxxx_add_user_status.sql` with the column, `chk_users_status`, the extended audit CHECK and a safe `Down`. Write the integration tests first (`database` and `repository` packages, Docker): a new user is `active`; an invalid status is rejected by the constraint; `CHANGE_USER_STATUS` is accepted by `chk_audit_logs_action`; `UpdateStatus` returns the user with the new status and `ErrNotFound` for a missing id; `RevokeSessions` ends only that user's live sessions; the active-admin lock query returns only active admins. Then add `Status` to `model.User` and `UserResponse`, the constants and `AuditChangeUserStatus` (in `AuditActions`), `status` to `userColumns`, `scanUser`, `GetWithRole` and the list scans, and the repository methods `UpdateStatus`, `RevokeSessions` and `LockActiveAdmins` on `UserRepository` and `MockUserRepo`.
  Done when: `make test` and `make test-integration` pass (new tests red first, then green), including the existing test that compares `AuditActions` with the CHECK constraint, and `make lint` and `make vet` pass.
- [x] 2. **Login and session enforcement (tests first).** In `auth_service_test.go` write the cases first: each non-active status after a correct password gives its own error; a wrong password on an inactive account is still `ErrInvalidCredentials` (no leak) and counts toward lockout; an active account still logs in; `Me` returns `ErrUnauthorized` for a non-active user. In the handler tests: the three 403 messages and the new locked wording, for `Login` and `Me`. In the middleware tests: `RequireRole` and `RequirePermission` give 401 for a non-active user. Then add the errors, the check after the password in `Login`, the `Me` check, the middleware check and the mapping in `auth_handler.go`.
  Done when: `make test` passes (red first, then green) and no existing login, lockout or change-password test changed except the locked wording.
- [x] 3. **Status service and endpoint (tests first).** Unit tests with mocks for `UserService.ChangeStatus`: self refused (403 error), last active admin refused (409 error), a non-last admin allowed, unchanged status writes and audits nothing, a move to non-active revokes sessions and a move to `active` does not, the audit row carries `from`, `to` and the revoked count, an unknown user is `ErrUserNotFound`, and an audit failure rolls the change back. Handler tests for `PUT /admin/users/:id/status` (200, 400, 403, 404, 409, 422) and a route test beside `adminUserRoutes` in `router/admin_users_routes_test.go` that a non-admin gets 403. An integration test (Docker) that two concurrent deactivations of the two remaining admins cannot both succeed, and that a user's old session cookie gets 401 on `/auth/me` right after the change. Then add `ChangeStatus`, the request type with `validate:"required,oneof=..."`, the handler with swag annotations, the `UserAdminService` method and the route.
  Done when: `make test`, `make test-integration`, `make lint` pass (red first, then green).
- [x] 4. **List filter and API contract (tests first).** Handler and repository tests: `GET /users?status=inactive` returns only inactive users, `status=all` or none returns everyone, an unknown value is 422, and each user carries `status`. Then change `List` and `ListUsers`, update the Swagger annotations, run `make swagger` and `npm run gen:api`, and add `status` to the `User` type in `lib/api-schema.ts`.
  Done when: `make test` passes (red first, then green), `make swagger` then `npm run gen:api` leave no further diff in `frontend/src/lib/api-types.ts`, and `npm run typecheck` passes.
- [x] 5. **Users tab, Edit page and login page (tests first).** Write the Vitest cases first: the badge shows the right label and class for each status; the Users tab renders the Status column, and choosing a status in the filter calls `listUsers` with it and keeps the Role filter client-side; the Edit page's Update status button is disabled while unchanged, asks for confirmation when leaving Active, calls `changeUserStatus` once, shows the server's error in a `role="alert"`, and disables the select on your own account; the login page shows the server's 403 message instead of the old fixed text. Then add `lib/user-status.ts` (values, labels), `components/user-status-badge.tsx`, `listUsers(email, status)` and `changeUserStatus(id, status)` in `lib/users.ts`, the column and filter in `UsersPanel`, the Status section on the Edit page, and the login change.
  Done when: `npm test` shows the new cases red first and then green, and `npm run lint`, `npm run typecheck` and `npm run build` pass.
- [x] 6. **Browser checks, docs and full suites.** `e2e/user-status.spec.ts` with its own account created by the admin API (a status change ends that account's sessions, so it must not reuse the shared saved sessions): an inactive account is refused at sign-in with the inactive message and, once set back to Active, can sign in; a wrong password on a non-active account still reads "Invalid email or password."; an open session of the user stops working right after the admin sets it inactive; the admin cannot change their own status (the select is disabled and the API answers 403); the Users tab shows the badge and the Status filter narrows the list. In `e2e/a11y.spec.ts` scan the Users tab with the badges visible (light and dark) and the Edit page with the Status section. Write D-A1 (amended), D-A10, D-A11 and the API and architecture notes as in Docs.
  Done when: `make test`, `make test-integration`, `make lint`, `npm test`, `npm run lint`, `npm run typecheck`, `npm run build` and `npm run e2e` pass, or any that cannot run is named in the summary with the reason.

## Files / areas

- New: `backend/internal/database/migrations/2026101612xxxx_add_user_status.sql`; `frontend/src/lib/user-status.ts`; `frontend/src/components/user-status-badge.tsx` (+ test); `frontend/e2e/user-status.spec.ts`.
- Edit (backend): `internal/model/user.go`, `internal/model/report_constants.go`, `internal/repository/interfaces.go`, `user_repo.go`, `mock_repo.go`, `internal/service/auth_service.go`, `user_service.go`, `internal/handler/auth_handler.go`, `admin_user_handler.go`, `user_handler.go`, `internal/middleware/rbac.go`, `internal/router/router.go` and `admin_users_routes_test.go`, the matching tests, `backend/docs/swagger.*`.
- Edit (frontend): `src/lib/users.ts`, `src/lib/api-schema.ts`, `src/lib/api-types.ts` (generated), `src/app/(app)/admin/users/users-list.tsx` (+ test), `src/app/(app)/admin/users/[id]/edit/edit-user.tsx`, `src/app/login/page.tsx` (+ test), `e2e/a11y.spec.ts`.
- Docs: `docs/DECISIONS.md`, `docs/API.md`, `docs/ARCHITECTURE.md`.
- Read only: `internal/session/store.go` (revoked sessions are never found), `UserRepo.UpdatePassword` (the revoke pattern), `reports-list.tsx` (the confirm dialog pattern).

## Data / contracts

- **Statuses:** `active` | `inactive` | `disciplinary` | `deceased`, lower case, as stored and as sent. "Deleted" is not a status (22b uses `deleted_at`).
- **Audit:** action `CHANGE_USER_STATUS`, target type `User`, target the user's email; `details` `{from, to, sessions_revoked}`. Written in the same transaction as the change; an actor name and role snapshot as for the other user actions.
- **API:** `PUT /admin/users/{id}/status` request `{status}`; 200 `UserResponse`; errors 400 id, 401, 403 (not admin, or own account), 404, 409 (last active admin), 422 (`status`). `GET /users?status=` as above. Error bodies keep the existing `apperror` shape.
- **Trusted actor:** the signed-in admin is read from the session (`sm.GetInt64(..., "user_id")`), never from the body; the target comes from the path.
- **Atomicity and concurrency:** status update, session revoke and audit insert are one transaction; the admin rows are locked `FOR UPDATE` in a fixed order (`ORDER BY u.id`) before the last-admin check.
- **Session effect:** revoked sessions are tombstoned with `revoked_at`, as `UpdatePassword` does; the session store never finds them.
- **Rendering:** names and emails render as text only; the status label comes from a fixed map, not from the server string.

## Testing

- Go: `make test`, `make test-integration` (Docker, testcontainers), `make lint`, `make vet`. Vitest: `npm test`. Playwright with axe: `npm run e2e` (the e2e API starts itself against a fresh database; the test Postgres must be up on :5440, `make db-start`). Never run e2e against production.
- Not claimed unless run: any visual match to a design, any production or deployed check, and any load or timing claim for the lock.

## Notes for the AI

- Keep it small: no new service, no status history table, no index, no feature flag. One migration, three repository methods, one service method, one endpoint, one badge component.
- The status check in Login comes **after** the password so the messages cannot be used to find out which emails exist. Do not move it earlier.
- Do not touch `ChangeRole` (known gap above) or the reports code. Do not edit `frontend/AGENTS.md`.
- A new audit action needs its CHECK migration in the same deploy as the code that writes it (`AGENTS.md` guardrail). Do not push; the owner pushes.
- Match the surrounding style: Squirrel builders in the repositories, `apperror` for errors, `userAudit` for the audit row, `NativeSelect` and `AlertDialog` on the frontend.

## Open questions

None block the build. Choices made where the request was silent, all reversible: an unchanged status is a quiet 200 (no audit row); any self status change is refused, not only the one away from Active; the index from the owner's design is skipped; the stats Users card still counts every account (revisit with the Tracker); the Edit page changes the status in its own section and request, not inside the profile form.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":18757,"specSha256":"b90b6a62abdf5efe4143a3830395940ecadb45fd5ed06503fa2fbd082e98d1a4","branch":"refs/heads/feature/user-status","head":"8386fd084c75332c670d45427c8710702b3e44d0","baseRef":"refs/heads/main","baseCommit":"3bbd7063f1d078b1cf05a67ec88ccf654f71ea93","sourceTree":"9e245b721d4fe10e0e372a8ba67606bcf1c4f29a","landing":"local-merge","absentOptional":[]} -->

## Verification

- `make test`, `make vet`, `make lint`: pass (lint: 0 issues).
- `make test-integration` (Docker): pass, all 13 packages.
- `npm test`: 27 files, 248 tests passed. `npm run lint` and `npm run typecheck`: exit 0. `npm run build`: exit 0 (dev server stopped first).
- `npm run e2e`: 72 passed (Playwright with axe), including `user-status.spec.ts`.
- `make swagger` then `npm run gen:api`: no further diff in `frontend/src/lib/api-types.ts`.
- Not run: `/check`, a builder `/audit`. No screenshot or visual comparison was taken by the agent, and nothing was checked against production. The two-admin lock is covered by an integration test (10 rounds), not by a load test.
- Carried forward, still open in `blueprint/context/findings.md` (P3, not archived because they are not resolved): F-01 (badge falls back to the raw server string for an unknown status) and F-02 (`/auth/change-password` and `/auth/logout` check the session only, so a login racing a status change can leave one live session).

## Independent review

**Status:** passed
**Target commit:** 8386fd084c75332c670d45427c8710702b3e44d0
**Base commit:** 3bbd7063f1d078b1cf05a67ec88ccf654f71ea93
**Base ref:** main
**Spec hash:** b90b6a62abdf5efe4143a3830395940ecadb45fd5ed06503fa2fbd082e98d1a4
**Prepared by:** claude
**Builder model:** claude-sonnet-5-5
**Requested reviewer:** claude
**Requested model:** runtime default (exact model not known until reviewer starts)
**Requested execution:** automatic
**Requested at:** 2026-10-17T02:03:00Z
**Workflow:** regular
**Check required:** no
**Reviewer adapter:** claude
**Reviewer model:** claude-sonnet-5-5
**Reviewer context:** fresh subagent
**Actual execution:** automatic
**Reviewed at:** 2026-10-17T02:20:00Z
**Scope:** current
**Lenses:** quality, security, performance, tests
**Verdict:** passed
**Check result:** not-required

### Commands

- `git rev-parse HEAD`, `git merge-base main HEAD`, `sha256sum blueprint/context/current-feature.md`, `git status --short`: pass (all match the request; only review.md differed)
- `git diff 3bbd706..8386fd0` (backend service, repository, middleware, handler, model, migration; frontend status files): pass
- `cd backend && go vet ./...`: pass
- `cd backend && go test ./internal/...`: pass
- `cd frontend && npx vitest run`: pass (27 files, 248 tests)
- Mutation check in a scratch copy (remove status checks, expect tests to fail): unavailable, the copy failed to build

### Evidence

- Login: status check at auth_service.go:123 runs after bcrypt and after RecordFailedLogin; a wrong password on a non-active account returns ErrInvalidCredentials; refusal skips ResetLoginAttempts. Tests cover each status, the generic 401, and no counter reset.
- Sessions: RequireRole, RequirePermission and AuthService.Me answer 401 "authentication required" for a non-active user, with unit tests per gate. ChangeStatus revokes sessions inside the transaction.
- ChangeStatus: LockActiveAdmins (ORDER BY u.id FOR UPDATE OF u) runs first in the transaction; self is refused before it, last-admin check uses the locked list; update, revoke and audit share one WithTx. Integration tests cover audit rollback and the two-admin race (10 rounds).
- Migration: Up adds the column, chk_users_status and the extended audit CHECK; Down restores the old list with NOT VALID and drops the column. The CHECK list matches model.AuditActions (existing comparison test updated).
- GET /users validates status (all or empty means no filter, unknown is 422) and the route is admin-only.
- Frontend: labels come from the fixed STATUS_LABEL map; the Edit page and Users tab follow the spec. See F-01 for the unknown-value fallback.

### Findings

- F-01 [P3] open
- F-02 [P3] open

### Remaining risk

- Docker integration tests (`make test-integration`), Playwright e2e, `make lint` and `npm run build` were not run, so the migration, the concurrency lock and the browser flows rest on code reading plus the unit suites.
- The mutation check on the unit tests could not run, so "tests fail without the code" is judged by reading the assertions, not by execution.
- ChangeStatus reads the target with GetByID outside the transaction; it is serialized by the admin lock in practice, but this was not exercised under load.

## Try it now

1. Start the stack (`make db-start` and `make dev` in `backend/`, `npm run dev` in `frontend/`) and sign in as an admin.
2. Open `/admin?tab=users`: each row has a Status badge with a text label, and a **Status** filter next to Role (default "All statuses").
3. Open a non-admin user's Edit page. In the **Status** section choose **Inactive** and press **Update status**: a dialog says the user is signed out everywhere. Confirm; a toast shows the result.
4. In a private window, sign in as that user with the right password: "Your account is inactive. Contact your admin." With a wrong password you still see the generic "Invalid email or password."
5. Set the user back to **Active** (no confirmation) and sign in again: it works.
6. Open your own Edit page: the status select is disabled. Calling `PUT /admin/users/{your id}/status` returns 403.

Unaffected regression path: Approve and Unapprove still work in the Reports tab of `/admin`.
