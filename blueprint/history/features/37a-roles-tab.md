# Feature: Roles tab

**From build-plan:** feature 37a
**Build attempt:** 1
**Branch:** feature/roles-tab
**Status:** verified

## Goal

Admins manage roles in a new **Roles** tab: create custom roles (e.g. "Reviewer", "CGP Coordinator"), pick their permissions from a grouped checklist, rename and delete them, and give any role to a user when creating or editing the account. Every change is audited. This is the first half of Dynamic RBAC (D-A21); 37b lets custom roles use the admin panel.

## In scope

**Decisions (owner, 2026-10-22, D-A21):**
1. Creating, editing and deleting roles needs the admin role **and** `roles:write`.
2. A role that still has users cannot be deleted; the admin reassigns them first.
3. The permission checklist shows every permission in the database, grouped by category.
4. The Create and Edit User forms have a role dropdown listing every role.

**Rules this spec adds (they follow from the code; change them at review if you disagree):**
- **Built-in roles are read-only.** `admin`, `editor` and `user` cannot be renamed, deleted or have their permissions changed (409). The code checks the `admin` and `user` names (`RequireRole`, the Tracker, the Excel dashboard, the last-admin guard), and `seed.go` re-grants these roles' permissions on every startup, so an edit would break access or silently revert. Today only `user` is protected.
- **The permission list is fixed.** The UI never creates or deletes permissions; a permission no code checks does nothing.
- **A user's role change is guarded like a status change (22a):** the admin role and `users:write`; you cannot change your own role (403); the last active admin cannot be moved off `admin` (409).

**Backend**
- Role writes go through a new `accesscontrol` service, each in one transaction with its audit row (AGENTS.md): `CREATE_ROLE`, `UPDATE_ROLE`, `DELETE_ROLE`, `CHANGE_USER_ROLE`, plus their migration widening `chk_audit_logs_action`.
- `POST /admin/roles` takes the name, description and `permission_ids` together; `PUT /admin/roles/:id` replaces all three; `DELETE /admin/roles/:id`. All three need `roles:write`.
- `GET /admin/roles` adds `user_count` to each role.
- The unaudited `POST` and `DELETE /admin/roles/:id/permissions[/:pid]` routes are removed. No frontend code calls them, and `PUT` replaces them.
- `PUT /admin/users/:id/role` moves into the user service with the guards above and its audit row.

**Frontend**
- **Roles tab:** tab order is Reports, Tracker, Users, Roles, Logs; `?tab=roles`.
  - The table shows Name (with a "Built-in" badge), Description, Users and Permissions (count).
  - "New role", "Edit" and "Delete" appear only for `can("roles:write")`.
  - Delete is disabled with "Reassign its N users first" when `user_count > 0`. Otherwise it opens a confirmation dialog.
- **Role form:** `/admin/roles/new` and `/admin/roles/:id/edit`.
  - Fields: name (2-50 characters), description (up to 255), and the permission checklist.
  - The checklist has one `<fieldset>`/`<legend>` per category:
    - **Reports** for `reports:*`
    - **Users** for `users:*`
    - **Admin** for `roles:*` and `permissions:*`
    - any other resource under its own name
  - Each checkbox is labelled with the action in words, with the code (e.g. `reports:approve`) shown beside it.
  - A built-in role opens read-only: the checklist is disabled, with a note saying why.
- **Users:** the Create User form already lists every role from `GET /admin/roles` (confirm custom roles appear). The Edit User page gets a **Role** section like the Status section: a dropdown and Save.
  - It is hidden on your own account, with a note.
  - A 409 shows the server's message.
- **Logs tab:** the four new actions with labels and detail rows.
- **States:** loading, error with Retry, 403 ("You are not allowed to manage roles."), validation errors on the fields (422), 409 messages (duplicate name, built-in role, role in use, last admin).

**Docs:** D-A21 in `docs/DECISIONS.md`, `docs/API.md`, and the audit action list in `docs/ARCHITECTURE.md`. Swagger, `make swagger`, `npm run gen:api`.

## Out of scope

- **37b:** permission-gated admin routes and tabs. The whole `/admin` area still requires the `admin` role, so a custom role's permissions apply only outside it (filing, reading and deleting reports, and `PATCH /reports/:id/complete` through the API).
- Creating or deleting permissions in the UI. The `/admin/permissions` write routes are left as they are.
- Changing who the Tracker and Excel dashboard count (D-A3 and D-A16 stay as decided). A custom-role worker who files reports is not in the Tracker; see Open questions.
- Ending sessions on a role change. `RequireRole` and `RequirePermission` already read the role from the database on every request (H-2), so a change applies on the next request.
- Bulk reassigning users when a role is deleted.

## Build loop

`workflow.stepReview` is `feature`: build every step, then one review at the end. `workflow.checkpointCommits` is `disabled`. Tests come first in every step. Run `make lint` to 0 issues before presenting. This touches authorization and audit, so the independent review applies (`when-sensitive`).

**Deploy:** the migration only widens a CHECK, so it is backward compatible. Run it locally against Neon **before** pushing `main` (H-1, `backend/SETUP.md` §3).

## Build steps

- [x] **1. Audit actions and migration (tests first)**
  - Add `ActionCreateRole`, `ActionUpdateRole`, `ActionDeleteRole` and `ActionChangeUserRole` in `backend/internal/audit/model.go`.
  - Add their `DetailKeys`:
    - `CREATE_ROLE`: `name`, `permissions`
    - `UPDATE_ROLE`: `from`, `to`, `added`, `removed`
    - `DELETE_ROLE`: `name`
    - `CHANGE_USER_ROLE`: `from`, `to`
  - Add migration `backend/internal/database/migrations/<timestamp>_allow_role_audit_actions.sql`, modelled on `20261022120000_allow_login_audit_actions.sql`. Up adds the four actions; Down is `NOT VALID`.
  - **Done when:**
    - a `CheckDetails` test accepts those keys and refuses another key for each action
    - `make test` passes
    - `make test-integration` applies the migration up and down

- [x] **2. Role store and service (tests first)**
  - **Store** (`acimpl/role_store.go`):
    - `ListWithUserCounts`: one query, counting every account that holds the role, soft-deleted ones included, because they hold the foreign key.
    - `ReplacePermissions(ctx, roleID, ids)`: delete then insert, inside the caller's transaction.
    - The built-in guard covers `admin`, `editor` and `user` (`accesscontrol.BuiltInRoles`), not only `DefaultRoleName`.
  - **Service** (`acimpl/service.go`, new `accesscontrol.RoleService`): `Create`, `Update`, `Delete`, each in `db.InTransaction` with its audit row, actor and IP.
    - Permission ids are checked against the database: an unknown id is `ErrUnknownPermission` (422).
    - `Update` with no change writes no row.
    - `Update` audits `from` and `to` only when the name changed, and `added` and `removed` as permission names.
    - A built-in role is `ErrProtectedRole`.
    - A role with users is `ErrRoleInUse` (with the count).
  - **Done when:** `make test` (service with mocks) and `make test-integration` (store) cover:
    - create with permissions
    - rename, adding and removing permissions, with one audit row each
    - a no-op update writes no row
    - built-in rename, delete and permission edit are refused
    - deleting a role in use is refused, including when its only user is soft-deleted
    - an unknown permission id is refused
    - an audit failure rolls back the role change

- [x] **3. Handlers, routes, Swagger (tests first)**
  - `role_handler.go` calls the service:
    - `POST` accepts `permission_ids`
    - `PUT` is a full replace (name required, `permission_ids` required, may be empty)
    - `List` returns `user_count`
  - Error mapping:
    - 409 "role name already exists"
    - 409 "built-in roles cannot be changed"
    - 409 "role is assigned to N users; reassign them first"
    - 422 on `permission_ids`
    - 404 for a missing role
  - In `router/router.go`, `POST`, `PUT` and `DELETE /admin/roles` add `perm("roles:write"), limits.Write`. The single `POST` and `DELETE …/permissions` routes and their handlers are deleted.
  - Run `make swagger`, then `npm run gen:api`.
  - **Done when:**
    - a router test shows each write route gives 401 without a session, 403 for a non-admin, 403 for an admin without `roles:write`, and passes for an admin with it
    - handler tests cover each error above
    - `make test` and `make lint` (0 issues) pass
    - `api-types.ts` changes only for these paths and types

- [x] **4. Change a user's role (tests first)**
  - Move `AdminUserHandler.ChangeRole`'s logic into the user service as `ChangeRole(ctx, actor, ip, id, roleID)`, in one transaction:
    - `LockActiveAdmins`, as `setStatus` does
    - your own account: 403 "You cannot change your own role."
    - moving the last active admin off `admin`: 409
    - an unchanged role: 200 with no audit row
    - otherwise a `CHANGE_USER_ROLE` row with `from` and `to` as role names
  - The route adds `perm("users:write"), limits.Write`.
  - **Done when:**
    - service tests cover self, last admin, unchanged, success, and an unknown role (404)
    - the existing `fk_race_test.go` cases still pass
    - `make test` passes

- [x] **5. Frontend data and the Roles tab (tests first)**
  - `frontend/src/lib/roles.ts`: `listPermissions`, `createRole`, `updateRole`, `deleteRole`, `changeUserRole`, `BUILT_IN_ROLES`, and a pure `groupPermissions(perms)` returning `{label, permissions}[]` in the order Reports, Users, Admin, then the rest. Keep `listRoles` in `lib/users.ts` and re-use it.
  - `frontend/src/app/(app)/admin/roles.tsx`: the tab described in In scope, with the delete `AlertDialog` (focus returns to the trigger).
  - `admin-panel.tsx`: accept `roles` and add the trigger and content between Users and Logs.
  - **Done when** `npm test`, `npm run lint` and `npm run typecheck` pass, and Vitest covers:
    - `groupPermissions`, including an unknown resource
    - the tab's loading, error with Retry, and 403 states
    - the built-in badge
    - controls hidden without `roles:write`
    - delete disabled with the reassign text, and delete confirmed
    - `?tab=roles` and the tab order

- [x] **6. Role form pages (tests first)**
  - `frontend/src/app/(app)/admin/roles/new/` and `roles/[id]/edit/`: a thin `page.tsx` (`instant = false`, as the user pages do) and a client form, with the grouped checklist.
    - Field errors come from 422 `details` with `aria-describedby` and focus on the first error (`focusFirstError`).
    - A 409 shows `role="alert"`.
    - A built-in role renders read-only.
    - "Cancel" goes back to `/admin?tab=roles`, and success goes there with a toast.
  - **Done when:** Vitest covers create with permissions, rename with permission changes (the request body is the full set), the 409 and 422 errors, and a read-only built-in role. Lint and typecheck pass.

- [x] **7. Role on the user forms (tests first)**
  - `users/[id]/edit/role-section.tsx`, modelled on `status-section.tsx`: a dropdown of every role, and Save calling `changeUserRole`.
    - It is hidden for your own account, with "You cannot change your own role."
    - It shows the server's 409 message.
    - On success it invalidates the user and users-list queries.
  - Confirm the Create User dropdown lists custom roles, and add a test.
  - **Done when:** Vitest covers the section's success, 409, own account, and a custom role in both forms. `npm test` passes.

- [x] **8. Logs labels, end-to-end, docs**
  - `frontend/src/lib/audit-logs.ts`: the four actions with labels ("Created role", "Updated role", "Deleted role", "Changed user role") and detail rows.
  - `frontend/e2e/roles.spec.ts`: an admin creates "E2E Reviewer" with `reports:read_any`, gives it to the e2e user from Edit User, sees the delete refused with the reassign text, gives the user back the `user` role, deletes the role, and sees the four rows in the Logs tab. Clean up in `finally`.
  - Add the Roles tab and the role form to `a11y.spec.ts`, both themes plus the 320 px reflow check.
  - Docs: D-A21, `docs/API.md` and `docs/ARCHITECTURE.md`.
  - **Done when:**
    - `npm run e2e` passes for `roles.spec.ts` and `a11y.spec.ts`
    - the earlier frontend and backend checks still pass
    - the docs name the endpoints, guards and audit actions

- [x] **9. Repair F-13: delete-role dialog focus (tests first)**
  - `frontend/src/app/(app)/admin/roles.tsx`: remember the Delete button that opened the dialog and return focus to it on close, falling back to `#main-content` after a delete (the `reports-list.tsx` pattern).
  - **Done when:** Vitest covers Escape returning focus to the Delete button and a delete moving focus to the main content; both fail without the fix. `npm test` passes.

## Files / areas

- **Backend:**
  - `internal/audit/model.go`
  - `internal/database/migrations/<ts>_allow_role_audit_actions.sql`
  - `internal/accesscontrol/{accesscontrol.go or model.go, acimpl/role_store.go, acimpl/service.go, rest/role_handler.go}` and their tests
  - `internal/user/{user.go, userimpl/user.go, rest/admin_user_handler.go}` and their tests
  - `internal/router/router.go` and its route tests
  - the API wiring in `cmd/api/app/server.go`
  - generated `docs/swagger*`
- **Frontend:**
  - `src/lib/{roles.ts, users.ts, audit-logs.ts, api-schema.ts, api-types.ts}`
  - `src/app/(app)/admin/{roles.tsx, admin-panel.tsx, roles/new/*, roles/[id]/edit/*, users/[id]/edit/role-section.tsx, users/[id]/edit/edit-user.tsx}` and their tests
  - `e2e/{roles.spec.ts, a11y.spec.ts}`
- **Docs:** `docs/DECISIONS.md`, `docs/API.md`, `docs/ARCHITECTURE.md`

## Data / contracts

**`GET /admin/roles`** (admin role). The page shape is unchanged, and each role adds `user_count`:

```json
{ "id": 4, "name": "Reviewer", "description": "Approves reports", "user_count": 2,
  "permissions": [{ "id": 11, "name": "reports:read_any", "resource": "reports", "action": "read_any" }] }
```

**`POST /admin/roles`** (admin role and `roles:write`) → 201 with that body:

```json
{ "name": "Reviewer", "description": "Approves reports", "permission_ids": [11, 16] }
```

**`PUT /admin/roles/:id`**: the same body, as a full replace → 200. **`DELETE /admin/roles/:id`** → 204.

Validation and errors:
- `name` is 2-50 characters and unique, case-sensitive as today. `description` is up to 255. `permission_ids` holds unique, existing ids, and may be empty.
- 409 for a duplicate name, a built-in role, or a role in use (the message carries the count).
- 422 with `details` per field.

**`PUT /admin/users/:id/role`** (admin role and `users:write`), body `{ "role_id": 4 }` → 200 with the user.
- 403 for your own account.
- 409 for the last active admin.
- 404 for an unknown user or role.

**Removed:** `POST /admin/roles/:id/permissions` and `DELETE /admin/roles/:id/permissions/:pid`.

**Audit rows** (written in the same transaction as the change; target type `Role` or `User`; names, never ids alone):

| Action | Details |
|---|---|
| `CREATE_ROLE` | `name`, `permissions` |
| `UPDATE_ROLE` | `from`/`to` when renamed, `added`/`removed` |
| `DELETE_ROLE` | `name` |
| `CHANGE_USER_ROLE` | `from`, `to` |

## Testing

- **Go:**
  - `make test`: service, handlers and routes
  - `make test-integration`: store, migration up and down, rollback on an audit failure
  - `make lint`
- **Frontend:** `npm test`, `npm run lint`, `npm run typecheck`, and `npm run e2e` (roles and a11y).
- CI checks `api-types.ts` for drift.

## Notes for the AI

- **Follow the user service's `setStatus` for the transaction and lock pattern**, and `status-section.tsx` for the Edit section.
- **Role names and descriptions are user-controlled:** render them as React text only, and audit names, not free text from the request.
- **The built-in list is one exported constant** (`accesscontrol.BuiltInRoles`), used by the store guard and the service, and mirrored in `lib/roles.ts` for the badge.
- **The checklist's group labels and action words are frontend wording only.** The server never sees them.
- **Don't change `seed.go`.** Built-in roles stay seed data (AGENTS.md); custom roles live only in the database.
- **The `RequireRole(userRepo, "admin")` group stays.** Replacing it is 37b.

## Open questions

1. **Custom-role workers in the Tracker and Excel dashboard.** A "CGP Coordinator" who files reports is not counted, because D-A3 counts only the `user` role. The fix would be "anyone with `reports:write` who isn't an admin". It doesn't block the build; it's a separate decision.
2. **Soft-deleted users holding a role.** They block deletion because they hold the foreign key, and `user_count` includes them. Step 7 needs to check that a deleted user's role can be changed from Edit User (or after a restore). If it can't, the 409 message should say "including deleted accounts".


<!-- blueprint:completion {"schemaVersion":2,"specBytes":16992,"specSha256":"e5145c783942488ff9a1ac17d76d83b30d06aab3253789f43b532e4b24854e58","branch":"refs/heads/feature/roles-tab","head":"5b29353ec23ffdb46fb9012d2e4a3ea7b8c5e95e","baseRef":"refs/heads/main","baseCommit":"78870e1a4f5d04ffa77f402fcc465ce28b942eff","sourceTree":"68f439fa9b601761982a298bafbb046d31b8e9a6","landing":"local-merge","absentOptional":[]} -->

## Findings

### 37a/F-13 [P2] closed - Closing the delete-role dialog drops keyboard focus instead of returning it to the Delete button

**File:** frontend/src/app/(app)/admin/roles.tsx:162
**Found:** 2026-10-22 by /audit independent (scope: current; lens: quality, tests)
**Why it matters:** The spec (step 5) asks for the delete `AlertDialog` with "focus returns to the trigger". The dialog is opened from state (`open={role !== null}`) with no `AlertDialogTrigger`, and `AlertDialogContent` has no `onCloseAutoFocus`. Radix's modal content calls `preventDefault()` and then `context.triggerRef.current?.focus()` on close; with no trigger that ref is null, so focus falls to `<body>` after Cancel, Escape or a successful delete. A keyboard or screen-reader user loses their place in the table. No Vitest case checks focus after closing. `reports-list.tsx:320-324` already has the fix pattern.
**Suggested fix:** Keep a ref to the Delete button that opened the dialog and add `onCloseAutoFocus={(e) => { e.preventDefault(); (opener.current?.isConnected ? opener.current : document.getElementById("main-content"))?.focus(); }}`, as reports-list does (after a delete the row is gone). Add a Vitest case that Cancel puts focus back on the Delete button.
**Resolution:** 2026-10-22 /implement: `roles.tsx` keeps a ref to the Delete button that opened the dialog, and `onCloseAutoFocus` returns focus to it, or to `#main-content` once the row is gone. Two new Vitest cases in `roles.test.tsx` fail without the fix and pass with it. Awaiting re-review.
Closed 2026-10-22 by /audit independent (fresh subagent, 78870e1..5b29353): `roles.tsx:96-98` records the opener on click and `roles.tsx:174-179` returns focus to it, or to `#main-content` once the row is gone, the same as `reports-list.tsx`. `roles.test.tsx:133-155` checks Escape and a delete; `npm test` passes. A refused delete keeps the dialog open with the opener still connected. No new defect found.

## Independent review

**Status:** passed
**Target commit:** 5b29353ec23ffdb46fb9012d2e4a3ea7b8c5e95e
**Base commit:** 78870e1a4f5d04ffa77f402fcc465ce28b942eff
**Base ref:** main
**Spec hash:** e5145c783942488ff9a1ac17d76d83b30d06aab3253789f43b532e4b24854e58
**Prepared by:** claude
**Builder model:** claude-opus-5-5
**Requested reviewer:** claude
**Requested model:** claude-opus-5-5
**Requested execution:** automatic
**Requested at:** 2026-10-22T14:42:39+00:00
**Workflow:** regular
**Check required:** no
**Reviewer adapter:** claude
**Reviewer model:** claude-opus-5-5
**Reviewer context:** fresh subagent
**Actual execution:** automatic
**Reviewed at:** 2026-10-22T14:48:52+00:00
**Scope:** current
**Lenses:** quality, security, performance, tests
**Verdict:** passed
**Check result:** not-required

### Commands

- `cd backend && make lint`: pass (0 issues)
- `cd backend && make test`: pass
- `cd backend && make test-integration`: pass (Docker available; all packages ok)
- `cd frontend && npm run lint`: pass
- `cd frontend && npm run typecheck`: pass
- `cd frontend && npm test`: pass (41 files, 426 tests)
- `cd frontend && npm run e2e`: not run (optional; needs the full stack)

### Evidence

- Preconditions: HEAD 5b29353, `git merge-base main HEAD` = 78870e1, spec SHA-256 matches, only review.md and findings.md dirty.
- Whole 78870e1..5b29353 delta reviewed (64 files, both commits) against spec 37a, all four lenses.
- Role writes: `acimpl/service.go` runs each change with its audit row in one `InTransaction`; the role row is locked `FOR UPDATE` before Update and Delete; the store guards built-ins in the statement (`role_store.go` Update/Delete).
- User role change: `userimpl/user.go` ChangeRole/setRole locks active admins, refuses self (403) and moving the last active admin off admin (409), and audits role names.
- Routes: `router.go` adds `perm("roles:write")`/`perm("users:write")` and `limits.Write`; the per-permission routes are removed (`roles_routes_test.go`).
- Migration `20261022200000_allow_role_audit_actions.sql` widens the CHECK; Down is NOT VALID; up/down covered by the integration run.
- Frontend renders role names as React text only; F-13 repair re-examined in `roles.tsx:96-98,174-179`.

### Findings

- F-13 [P2] closed (repair re-examined)
- F-15 [P3] open: permission list read uses the pool, not the transaction
- F-16 [P3] open: permission checklist error not linked with aria-describedby
- F-14 [P3] remains open (unchanged)

### Remaining risk

- `npm run e2e` (roles.spec.ts, a11y.spec.ts) not run in this review.
- `make swagger` / `npm run gen:api` drift not re-run (would write files); CI checks `api-types.ts`.
- Dashboard activity (`blueprint/.state/run.json`) not published: this reviewer was limited to writing review.md and findings.md.

## Try it now

Run or do:

In `backend/`: `make db-start`, then `make migrate-up` and `make dev`. In `frontend/`: `npm run dev`. Sign in at http://localhost:3000 as the admin (your local `ADMIN_EMAIL` / `ADMIN_PASSWORD`) and open **http://localhost:3000/admin?tab=roles**. Click **New role**, name it "Reviewer", tick `reports:read_any`, and save. Open a worker's **Edit** page, set their role to Reviewer, then back on the Roles tab press **Delete Reviewer**.

Expected:

The Roles tab lists `admin`, `editor` and `user` with a **Built-in** badge and only **View**, plus Reviewer with **Edit** and **Delete**. Once a worker holds Reviewer, its Delete button is disabled with "Reassign its 1 user first". Give the worker back the `user` role, delete Reviewer, and the confirm dialog closes with focus back on the page. The **Logs** tab shows Created role, Changed user role (twice) and Deleted role rows.

What this proves:

Custom roles can be created, assigned and deleted from the UI under the D-A21 guards, and each write leaves its audit row.
