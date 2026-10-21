# Feature: Soft delete and restore of users

**From build-plan:** feature 22b
**Build attempt:** 1
**Branch:** feature/soft-delete-and-restore-of-users

## Goal

An admin can soft-delete an account, with an optional reason and after a confirmation, and later restore it (D-A11). A deleted user cannot sign in or keep a session, and gets the same answer as an unknown email. Their email stays reserved. The Users tab hides deleted accounts by default and shows them under a "Deleted" filter. Delete is on the rows that are not deleted, and Restore is on the deleted ones. Both actions are audited in the same transaction as the change.

## In scope

- Migration: `users.deleted_at`, `users.deleted_by`, `users.deleted_reason`, plus the `DELETE_USER` and `RESTORE_USER` audit actions in `chk_audit_logs_action` (same deploy as the code that writes them).
- `DELETE /admin/users/:id` (optional reason) and `POST /admin/users/:id/restore`. Both are admin-only and need `users:write`.
- Protections as in D-A10: you cannot delete yourself (403), and the last active admin cannot be deleted (409).
- Deleting revokes the user's sessions in the same transaction.
- Sign-in and session checks: login, forgot-password, `RequireRole`, `RequirePermission`, `/auth/me` and refresh all refuse a deleted user.
- `GET /users`: the default ("all" or empty) leaves out deleted users. `status=deleted` returns only deleted users. Each of the four statuses returns only non-deleted users with that status.
- Creating a user with a deleted user's email gets a 409 that points to Restore.
- Users tab: a "Deleted" option in the Status filter, a "Deleted" badge, a Delete action with a confirmation dialog and reason textarea, and a Restore action.

## Out of scope

- Report owner badges ("Deleted user", "Archived") and the reports API owner status. Those are 22c.
- Hiding a deleted user's reports (D-A11 future enhancement).
- Hard delete, or any bulk delete or restore.
- The admin stats Users card count. It still counts every account (known gap from D-A10).
- Blocking edit, status, role or reset-password on a deleted user. These still work and are harmless, because the user cannot sign in until restored.
- The `PUT /admin/users/:id/role` last-admin gap (D-A10 known gap, `/fix`).
- Unused `Store.ListWithPagination` filtering (it has no callers). It only gets the new scan column.

## Build loop

`workflow.stepReview` is `feature`: build all steps, then stop for one review at the end. `workflow.checkpointCommits` is `disabled`: don't commit between steps. `/complete` makes the single feature commit on `feature/soft-delete-and-restore-of-users`.

For each step: write the test first where the step adds logic, make it pass, then run the step's check.

## Build steps

- [x] **1. Schema, model and the sign-in net.**
  - Add the goose migration `backend/internal/database/migrations/<timestamp>_add_user_soft_delete.sql`, with a timestamp later than `20261021100000`. Up does the following:
    - `deleted_at TIMESTAMPTZ NULL`
    - `deleted_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL`
    - `deleted_reason TEXT NULL` with `CHECK (char_length(deleted_reason) <= 500)`
    - Rebuild `chk_audit_logs_action` with the full current list plus `DELETE_USER` and `RESTORE_USER`.
  - Down follows the pattern in `20261017120000_allow_admin_reset_password_audit_action.sql`. It restores the previous list `NOT VALID`, so audit history is never deleted, and drops the three columns.
  - Add `audit.ActionDeleteUser` and `audit.ActionRestoreUser` to `backend/internal/audit/model.go` and to `Actions`. Raise the count in `model_test.go` to 14.
  - Add `DeletedAt *time.Time` (`db:"deleted_at"`) to `user.User`, and `DeletedAt *time.Time` (`json:"deleted_at,omitempty"`) to `UserResponse` and `ToResponse`.
  - `CanSignIn` returns false when `DeletedAt != nil`.
  - Add `deleted_at` to `userColumns`, `scanUser`, the scan in `List` and in `ListWithPagination`, and the select and scan in `GetWithRole`.
  - Because `RequireRole`, `RequirePermission`, `/auth/me` and refresh (`tokens.go:129`) already call `CanSignIn`, they now refuse a deleted user with 401.
  - **Done when:** `make test` passes, including a `status_test.go` case where a deleted active user cannot sign in. `make test-integration` passes, including the existing test that compares `audit.Actions` with the CHECK constraint, now with 14 actions. `make migrate-up` then `make migrate-down` runs cleanly on the dev database.

- [x] **2. Login, forgot-password, the admin lock and the list filter.**
  - `authimpl.Login`: a deleted user returns `auth.ErrInvalidCredentials` right after the lookup, before the lockout and password checks. It records no failed attempt, exactly like an unknown email. This gives the D-A11 generic 401 "Invalid email or password." with no enumeration through the lockout message.
  - `passwordResetService.RequestReset`: a deleted user is handled like an unknown account (log, return nil, no token).
  - `LockActiveAdmins`: add `AND u.deleted_at IS NULL`, so a deleted admin never counts as the active admin.
  - `store.List`: `status == "deleted"` adds `deleted_at IS NOT NULL`. Any other value adds `deleted_at IS NULL`, and a non-empty status also adds `status = ?`.
  - `UserHandler.ListUsers`: also accept `deleted`. The swagger `Enums` becomes `all, active, inactive, disciplinary, deceased, deleted`.
  - **Done when:** these unit tests pass with `make test`:
    - Login for a deleted user with the right password returns `ErrInvalidCredentials` and does not call `RecordFailedLogin`.
    - `RequestReset` for a deleted user creates no token.
    - `ListUsers` accepts `status=deleted`, and still returns 422 for `status=bogus`.
  - These integration tests pass with `make test-integration`:
    - `List` with "" excludes deleted users, with `deleted` returns only deleted users, and with `active` excludes a deleted active user.
    - `LockActiveAdmins` skips a deleted admin.

- [x] **3. Delete and restore: service, store, handlers, routes.**
  - New `user.Service` methods:
    - `Delete(ctx, actorID, ip, id, reason) (*User, error)`
    - `Restore(ctx, actorID, ip, id) (*User, error)`
  - New `user.Store` methods. Both use `RETURNING userColumns`, and a missing row is `database.ErrNotFound`. Update `MockStore` for both.
    - `SoftDelete(ctx, id, deletedBy, reason) (*User, error)` sets `deleted_at = NOW()`, `deleted_by`, `deleted_reason` (`nullable`) and `updated_at`.
    - `Restore(ctx, id) (*User, error)` clears all three columns and sets `updated_at`.
  - `Delete` works like `ChangeStatus`, but deletes:
    - Self returns `ErrCannotDeleteSelf` (403 "You cannot delete your own account."). This is checked before the transaction.
    - The reason is trimmed with `strings.TrimSpace`.
    - In one transaction: `LockActiveAdmins`, then `GetByID` (nil gives `ErrNotFound`).
    - An already-deleted user is a quiet success that writes and audits nothing.
    - If the target is the only active admin, return `ErrLastActiveAdmin` (409, same message as D-A10).
    - Then `SoftDelete`, then `RevokeSessions`, then the audit row `DELETE_USER` with `{reason, sessions_revoked}`.
  - `Restore`:
    - In one transaction: `GetByID` (nil gives `ErrNotFound`).
    - A user who is not deleted is a quiet success that writes and audits nothing.
    - Otherwise `Restore`, then the audit row `RESTORE_USER` with `{deleted_at, deleted_by, deleted_reason}` from the row before the restore. The restorer is the audit row's `performed_by`; that is the D-A11 "`restored_by` is in the audit row".
    - The status is not changed.
  - `Create`: when `GetByEmail` finds a deleted user, return a new `ErrEmailBelongsToDeletedUser`, which maps to 409 "This email belongs to a deleted user. Restore them instead." The concurrent unique-violation path stays `ErrEmailTaken`.
  - Handlers in `admin_user_handler.go`, both returning 200 with `UserResponse`, with godoc in the style of `ChangeStatus`:
    - `DeleteUser`: the body is `DeleteUserRequest{Reason string "json:reason" validate:"max=500"}`. An empty or missing body is allowed.
    - `RestoreUser`: no body.
  - Map the new errors in `userError`.
  - Routes in `router.go`, next to the status route, both with `perm("users:write")`:
    - `admin.DELETE("/users/:id", …)`
    - `admin.POST("/users/:id/restore", …)`
  - Add both to `admin_users_routes_test.go`.
  - **Done when:** these unit tests pass with `make test`:
    - Service tests with `MockStore`: self gives 403, last active admin gives 409, a missing user gives 404, an already-deleted user and a not-deleted restore are no-ops without audit, the audit details are as listed, and sessions are revoked on delete.
    - Handler tests: an over-long reason gives 422, a bad id gives 400.
    - The route table includes both new routes.
  - These integration tests pass with `make test-integration`:
    - Delete and restore are atomic with their audit rows.
    - The deleted user's refresh tokens are revoked.
    - Creating with a deleted user's email gives `ErrEmailBelongsToDeletedUser`.
    - A restored active user can sign in again.

- [x] **4. API types and the Users tab.**
  - Run `make swagger` (backend) and `npm run gen:api` (frontend).
  - Add `deleted_at?: string` to `User` in `frontend/src/lib/api-schema.ts` if the generated type does not carry it.
  - `frontend/src/lib/users.ts` gets two new functions:
    - `deleteUser(id, reason)`: `DELETE /admin/users/:id` with body `{reason}`. Check that `api()` sends a body on DELETE, and adjust it if needed.
    - `restoreUser(id)`: `POST /admin/users/:id/restore`.
  - `users-list.tsx`:
    - The Status filter gets `<option value="deleted">Deleted</option>` after the four statuses. "All statuses" still sends nothing.
    - A row with `deleted_at` shows a "Deleted" `Badge` in the Status cell, next to `UserStatusBadge`.
    - The actions cell has Edit plus:
      - **Delete** on rows that are not deleted. It is disabled with `aria-describedby` "You cannot delete your own account." on your own row.
      - **Restore** on deleted rows. It needs no confirmation (it is reversible), shows `toast.success("{name} restored")`, and invalidates `["users"]`.
    - Delete opens an `AlertDialog` with:
      - Title "Delete {name}?"
      - The exact D-A11 text: "This will soft-delete {name} ({email}). The user cannot sign in. Their reports stay, but are hidden from the active list. You can restore them later."
      - A `Textarea` labelled "Reason (optional)", empty by default, `maxLength` 500.
      - Cancel and a plain `Button` "Delete user" ("Deleting…" while pending), as in `status-section.tsx`.
    - When the dialog closes, the reason clears.
    - On success: close the dialog, `toast.success("{name} deleted")`, and invalidate `["users"]` and `["user", id]`.
    - On error: stay open and show `ApiError.message` in `role="alert"` inside the dialog, for example the 409 last-admin message. Other errors show "Could not delete the user. Try again."
    - Name, email and reason render only as React text, never as HTML.
    - Fix the comment in `user-status.ts`, "'Deleted' is not a status (22b)", so it says the Deleted filter is a separate server query.
  - **Done when:** `npm run lint`, `npm run typecheck` and `npm test` pass, with new `users-list.test.tsx` cases:
    - The Deleted option sends `status=deleted`.
    - A deleted row shows the badge and Restore, not Delete.
    - Your own row's Delete is disabled.
    - The dialog shows the D-A11 text with name and email, sends the reason, and closes and toasts on success.
    - A 409 stays in the dialog as an alert.
    - Restore calls the API and toasts.

- [x] **5. Record the decision details.**
  - In `docs/DECISIONS.md` D-A11, add a "Choices made without the owner" line:
    - `status=deleted` is a list filter, not a status.
    - Repeat delete and restore are quiet no-ops.
    - The reason is at most 500 characters.
    - Restore keeps the status and needs no confirmation.
    - Create with a deleted user's email gets a 409 pointing to Restore.
    - Forgot-password ignores deleted users.
    - Edit, status and reset-password still work on a deleted user.
  - Change its status line to "built in 22b".
  - **Done when:** the D-A11 entry reads that way, and `make lint` passes.

## Files / areas

- `backend/internal/database/migrations/` (new migration)
- `backend/internal/audit/model.go`, `model_test.go`
- `backend/internal/user/{user.go,model.go,status.go,status_test.go}`
- `backend/internal/user/userimpl/{store.go,user.go,mock.go}` and their tests (new `user_delete_test.go`, `user_delete_integration_test.go`)
- `backend/internal/user/rest/{admin_user_handler.go,user_handler.go}` and their tests (new `admin_user_delete_test.go`)
- `backend/internal/router/{router.go,admin_users_routes_test.go}`
- `backend/internal/auth/authimpl/{auth.go,password_reset.go}` and their tests
- Swagger output from `make swagger`
- `frontend/src/lib/{api-types.ts,api-schema.ts,users.ts,user-status.ts}`
- `frontend/src/app/(app)/admin/users/{users-list.tsx,users-list.test.tsx}`
- `docs/DECISIONS.md`

## Data / contracts

- **Columns:**
  - `deleted_at TIMESTAMPTZ NULL`: NULL means not deleted. This column is the only "is deleted" signal.
  - `deleted_by BIGINT NULL` FK `users(id) ON DELETE SET NULL`.
  - `deleted_reason TEXT NULL`, at most 500 characters. Empty after trimming is stored as NULL.
  - Restore sets all three back to NULL. The history lives in the audit rows.
  - `status` is untouched by delete and restore.
- **`DELETE /admin/users/:id`:**
  - Body `{"reason": string?}` (optional; validated `max=500`; trimmed server-side).
  - 200 `UserResponse` with `deleted_at` set.
  - 400 bad id or body, 401, 403 (not admin, no `users:write`, or self: "You cannot delete your own account."), 404, 409 "At least one active admin is required.", 422 `{reason: …}`.
  - Already deleted: 200, no write, no audit.
- **`POST /admin/users/:id/restore`:**
  - No body. 200 `UserResponse` without `deleted_at`.
  - 400, 401, 403, 404.
  - Not deleted: 200, no write, no audit.
- **`GET /users?status=`:**
  - Values are `all` or empty, `active`, `inactive`, `disciplinary`, `deceased`, or `deleted`. Anything else is 422 `{status}`.
  - Only `deleted` includes deleted users.
- **`UserResponse`:** gets `deleted_at` (RFC 3339, omitted when not deleted).
- **Audit:**
  - `DELETE_USER` details `{reason: string|null, sessions_revoked: int}`.
  - `RESTORE_USER` details `{deleted_at, deleted_by, deleted_reason}` from before the restore.
  - The target is named by email, as in `user.AuditLog`. There are never secrets in details.
- **Login:** a deleted user gets 401 "Invalid email or password." whatever password is sent. It records no failed attempt and starts no session.
- **Create:** a deleted user's email gets 409 "This email belongs to a deleted user. Restore them instead."
- **Concurrency:** delete locks the active admins (id order) before the last-admin check, as D-A10 does. `RevokeSessions` keeps its users-row-first lock order.

## Testing

- Backend: `make test` (unit, `-race`) and `make test-integration` (Docker, testcontainers).
- Frontend: `npm run lint`, `npm run typecheck` and `npm test`.
- No e2e change: `npm run e2e` exists, but the Vitest list tests cover the UI states at proportionate cost.
- No live-browser or deployed-database evidence is claimed. Production migration runs as the separate `/app/api migrate` step (AGENTS.md).

## Notes for the AI

- Mirror `ChangeStatus` and `setStatus` for structure, error mapping and godoc. Mirror `status-section.tsx` for the dialog pattern, and use the plain `Button` rather than `AlertDialogAction`.
- A deleted user must look exactly like an unknown email at login. Don't let the lockout or status messages leak through for them.
- Keep the authorization at the route (`RequireRole("admin")` group plus `perm("users:write")`). Don't add checks in the service beyond self and last-admin.
- Every place that scans `userColumns` must scan `deleted_at` too (`scanUser`, `List`, `ListWithPagination`). `GetWithRole` has its own column list.
- Don't start the dev server.
