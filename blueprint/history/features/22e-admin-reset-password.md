# Feature: Admin reset password

**From build-plan:** feature 22e
**Build attempt:** 1
**Branch:** feature/admin-reset-password
**Status:** verified

## Goal

An admin can set a new password for another user from the Edit User page, because admins create the accounts (D-A2: no self-registration) and hand the credentials to church workers, and a worker who forgot theirs has no e-mail recovery yet (E-1 is later). The admin types a new password twice or generates one, copies it, and gives it to the worker. The reset ends all of that user's sessions, clears their login lockout, and is audited as `ADMIN_RESET_PASSWORD`. An admin never resets their own password this way (Change password is for that). D-A12 records the decision.

## In scope

### Database, model and audit (one migration, shipped in the same deploy as the code that writes the new action)

- A goose migration `2026101712xxxx_allow_admin_reset_password_audit_action.sql`: drop and re-add `chk_audit_logs_action` with `ADMIN_RESET_PASSWORD` added to the current 11 actions (as `20261016120000_add_user_status.sql` did). `Down` restores the 11-action list `NOT VALID`. No table or column change.
- `model.AuditAdminResetPassword = "ADMIN_RESET_PASSWORD"` added to `AuditActions` (now 12). The existing tests that compare that list with the CHECK constraint (`reports_constants_integration_test.go`, `reports_schema_integration_test.go` `auditActions`) and the count in `report_constants_test.go` (11 to 12) are updated.
- `model.AdminResetPasswordRequest`: `new_password` `required,strongpass` and `confirm_password` `required,eqfield=NewPassword` (the 22d rules, same messages). No `current_password`: an admin does not know it.

### Server

- `POST /admin/users/:id/reset-password` in the `admin` group (so `RequireRole("admin")`) with `perm("users:write")` and `limits.Write`, like the other admin writes and change-password. Answers **204** with no body. The target comes from the path, the actor from the session (`sm.GetInt64(..., "user_id")`), never from the body.
- `UserService.ResetPassword(ctx, actorID, ip, id, newPassword)`:
  1. Load the actor (`ErrUnauthorized` if the session's user is gone).
  2. `actorID == id` is `ErrCannotResetOwnPassword`, before any hashing or write.
  3. A new password over 72 bytes is `ErrPasswordTooLong` (bcrypt reads 72 bytes; the validator counts characters), as in `AuthService.ChangePassword`.
  4. Load the target (`GetByID`); missing is `ErrUserNotFound`.
  5. Hash with `bcrypt.DefaultCost`, then in one `tx.WithTx`: `UpdatePasswordKeepSession(ctx, q, id, hash, "")` (an empty keep-token revokes every session, since session tokens are never empty; a test proves it), clear the target's failed-login counter and `locked_until` on `q`, and `audit.Insert` the `ADMIN_RESET_PASSWORD` row. A failed audit insert rolls the whole reset back (500).
- New repository method `ClearLockout(ctx, q, userID)` on `UserRepository` (and `MockUserRepo`): `failed_login_attempts = 0, locked_until = NULL`. The existing `ResetLoginAttempts` runs on the pool, not on `q`, so it cannot join the transaction and is left unchanged.
- Audit row: action `ADMIN_RESET_PASSWORD`, target type `User`, target the user's email (`userAudit`), details `{"sessions_ended": N}` only. Never the password, its hash, the confirmation or a token. An actor name and role snapshot as for the other user actions.
- Any status and any role may be reset, including another admin and an inactive, disciplinary or deceased account (D-A10 already lets such an account reset by token; it still cannot sign in until Active). The reset does not change the status.
- Errors, keeping the existing `apperror` shape: not signed in 401; not admin or no `users:write` 403; **own account 403 "You cannot reset your own password. Use the Change Password page instead."**; unknown id 404 "user not found"; non-numeric id 400; weak or mismatched password 422 on `new_password` / `confirm_password`; over 72 bytes 422 on `new_password` "must be at most 72 bytes"; bad JSON 400. Order: bind (400), validate (422), then the service (403, 404). So a self-reset with a weak password answers 422 first; the 403 is for a valid body.
- Swagger annotations, then `make swagger` and `npm run gen:api` in the same commit (CI fails on drift in `frontend/src/lib/api-types.ts`). `lib/api-schema.ts` gains the input type.

### Edit User page (frontend)

- A **Reset password** section under Status in a new `reset-password-section.tsx`, rendered from `edit-user.tsx`: a heading, one sentence ("Sets a new password for {name} and signs them out everywhere."), and a **Reset password** button. When the page is the signed-in admin's own account the button is disabled with the note "You cannot reset your own password. Use the Change Password page instead." (the server still enforces it).
- The button opens an `AlertDialog` (the status-section pattern; Cancel and a plain `Button` for the action, so a failed reset keeps the dialog open) titled "Reset password for {name}". Inside, three controls built from the existing `AuthField`, so each password field has its own show/hide toggle: **New password**, **Confirm password** (22d labels and rules), and a **Generate random password** button.
- **Generate** fills both fields with a 20-character password drawn with `crypto.getRandomValues` from letters and digits without the look-alikes `0 O 1 l I`, with at least one lower case, one upper case and one digit, using rejection sampling (no modulo bias). The generator takes an optional random source so a test can fix it. 20 characters of that alphabet are far above the server's 60-bit rule and under 72 bytes. After Generate both fields show their value so the admin can read it; the toggles still work.
- A **Copy** button copies the current New password with `navigator.clipboard.writeText` and announces "Copied" (a toast). If copying is refused or unavailable, a visible line says "Could not copy. Select the password and copy it yourself." and the password stays readable.
- Before sending: an empty New or Confirm is "… is required", a mismatch is "Confirm password does not match", both with no request. The server's 422 on either field shows on that field (the change-password mapping, `toFormError`). Focus goes to the first invalid field. A 403, 404, 429 or other error shows its message in a `role="alert"` and leaves what was typed.
- **After a successful reset the dialog stays open in a done state:** "Password reset. {name} is signed out everywhere. This password will not be shown again." with the password in a visible read-only field, a **Copy** button and a **Close** button. This is the admin's last chance to copy it. Closing, or pressing Escape, clears every field and the stored password. The toast "Password reset for {name}" confirms it.
- The password lives only in component state while the dialog is open: not in a query cache, the URL, `localStorage`, a toast or a log. Queries `["users"]` and `["user", id]` are not refetched (nothing visible changes).

### Docs (same commit)

- `docs/API.md`: the endpoint, its body, its errors and the 204. `docs/ARCHITECTURE.md`: the new audit action and the migration in the history. `docs/DECISIONS.md`: **D-A12 Admin reset password** with an index row (the decision, who may do it, what it ends and clears, no self-reset, the audit details, why there is no "view password", e-mail delivery deferred to E-1, the generated password is shown once in the browser, the deploy-order note below).

## Out of scope

- E-mailing the new password or a reset link (E-1, build-plan 23 to 25), a "must change password at next sign-in" flag (it needs a new column), a "view password" action (the hash cannot be read back), and a password history.
- The public reset-token page (item 25) and the forgot-password flow (24).
- Removing the editor role and the permissions UI (D-A13, deferred), and any change to roles or statuses.
- A bulk reset, and resetting from the Users tab row (the Edit page only).
- Changing the login form or `ChangePassword` behaviour.

## Build loop

`workflow.stepReview` is `feature`: review once after all steps. `checkpointCommits` is disabled; `/complete` makes the final commit. Test-first inside each step. This feature carries a migration and writes an audit action, so the CHECK migration must ship in the same deploy as the code (Railway runs `/app/api migrate` before the new code starts, and the old code never writes the action, so the order is safe). The new route is unreachable from the old frontend, and the new frontend calls a route the old API lacks, so push once and expect a short window where the Reset password button gets a 404 until Railway finishes. Do not push; the owner pushes.

## Build steps

- [x] 1. **Migration, constant and repository (tests first).** Write the integration tests first (Docker): `ADMIN_RESET_PASSWORD` is accepted by `chk_audit_logs_action` and an unknown action is still rejected; the `AuditActions` list equals the CHECK list (12); `ClearLockout` zeroes `failed_login_attempts` and `locked_until` for one user and touches no other row, and on a missing id changes nothing; `UpdatePasswordKeepSession(ctx, q, id, hash, "")` revokes every session of that user, leaves another user's sessions alone, and returns the count. Then add the goose migration (with a `NOT VALID` `Down`), `AuditAdminResetPassword`, `ClearLockout` on the interface, `UserRepo` and `MockUserRepo`, and update the three audit-list tests (count 11 to 12).
  Done when: `make test` and `make test-integration` pass (new tests red first, then green) and `make lint` and `make vet` pass.
- [x] 2. **Service (tests first).** Unit tests with mocks for `UserService.ResetPassword`: the actor is the target is `ErrCannotResetOwnPassword` with nothing hashed or written; a missing actor is `ErrUnauthorized`; a password over 72 bytes (including a multi-byte one) is `ErrPasswordTooLong`; a missing target is `ErrUserNotFound`; success hashes with bcrypt (the stored hash verifies against the new password and never equals it), calls the keep-session update with an empty token, clears the lockout, and writes one audit row with action `ADMIN_RESET_PASSWORD`, the actor and role snapshot, the target email and details exactly `{"sessions_ended": N}` (assert the serialized details hold neither the password nor the hash); a failing audit insert or lockout clear returns an error and, in the integration test over a real database, leaves the old password, the sessions and the lockout unchanged. Integration (Docker): a locked-out user signs in with the new password after the reset and not with the old one; the target's open session stops working. Then implement `ResetPassword` and `ErrCannotResetOwnPassword`.
  Done when: `make test` and `make test-integration` pass (red first, then green) and `make lint` passes.
- [x] 3. **Endpoint and contract (tests first).** Handler tests with a fake service: 204 and the service gets the session user, the IP, the path id and the new password; 403 for the self error with exactly the message above; 404; 400 for a non-numeric id and for bad JSON; 422 for a weak password, a mismatch, a missing confirmation and a 72-byte overflow (field names `new_password` / `confirm_password`), with the service not called and the password absent from the body; 401 without a session. Router tests: a signed-out caller gets 401, a non-admin 403 (an editor holding `users:write` is still 403), an admin without `users:write` 403, an admin 204. Check that `limits.Write` is the same limiter change-password uses and that the request logger does not log bodies. Then add the handler method, the `UserAdminService` method (and the two fakes), the route, the Swagger comments; run `make swagger` then `npm run gen:api`; add the input type to `lib/api-schema.ts`; update `docs/API.md`.
  Done when: `make test` passes (red first, then green), `make swagger` then `npm run gen:api` leave no further diff in `frontend/src/lib/api-types.ts`, and `make lint` and `npm run typecheck` pass.
- [x] 4. **Dialog and generator (tests first).** Vitest cases first. Generator: with a fixed random source the output is the expected 20 characters; every character is in the alphabet (none of `0 O 1 l I`); at least one lower case, upper case and digit; two default calls differ; the byte length is under 72. Section: the button is disabled with the note on the admin's own page; the dialog opens with the two fields and Generate; Generate fills both with the same value; an empty field and a mismatch each show their message, focus that field and send nothing; a valid submit calls `resetUserPassword(id, new, confirm)` once; a server 422 on each field shows on that field; a 403 and a 404 show in the alert and keep the typed values; the done state shows the password, Copy writes it to the clipboard (mocked) and announces it, a refused clipboard shows the fallback line; Close and Escape clear the fields, and reopening starts empty; no request is sent while pending and Cancel is disabled then. `lib/users.test.ts` asserts the POST path and a body holding both `new_password` and `confirm_password`. Then implement `lib/users.ts` `resetUserPassword`, `lib/password.ts` (the generator), `reset-password-section.tsx`, and render it from `edit-user.tsx` under `StatusSection`.
  Done when: `npm test` shows the new cases red first and then green, and `npm run lint` and `npm run typecheck` pass.
- [x] 5. **Browser checks, docs and full suites.** `e2e/admin-reset-password.spec.ts` with its own account created by the admin API (a reset ends that account's sessions, so it must not reuse the shared saved sessions; `confirm_password` in the create call): a mismatch in the dialog shows the error and resets nothing; Generate fills both fields, Copy works (grant the clipboard permission in the test) and the real server accepts the generated password; after the reset the old password is refused with the generic message and the new one signs in; an open session of that account stops working; a locked-out account (five wrong sign-ins) signs in with the new password; on the admin's own Edit page the button is disabled and `POST /admin/users/{me}/reset-password` is a 403 with the message. Axe (`a11y.spec.ts`) covers the Edit page with the section and the open dialog in both its states, in both themes. Record D-A12 in `docs/DECISIONS.md` and update `docs/ARCHITECTURE.md`.
  Done when: `make test`, `make test-integration`, `make lint`, `npm test`, `npm run lint`, `npm run typecheck`, `npm run build` and `npm run e2e` pass, or any that cannot run is named in the summary with the reason.

## Files / areas

- New: `backend/internal/database/migrations/2026101712xxxx_allow_admin_reset_password_audit_action.sql`; `frontend/src/app/(app)/admin/users/[id]/edit/reset-password-section.tsx` (+ test); `frontend/src/lib/password.ts` (+ test); `frontend/e2e/admin-reset-password.spec.ts`; Go test files beside the code they test.
- Edit (backend): `internal/model/report_constants.go` (+ test), `internal/model/user.go`, `internal/repository/interfaces.go`, `user_repo.go`, `mock_repo.go`, `internal/service/user_service.go`, `internal/handler/admin_user_handler.go` (and `userError`), `internal/router/router.go`, the fakes in `handler/admin_user_write_test.go` and `router/admin_users_routes_test.go`, `database/reports_constants_integration_test.go` and `reports_schema_integration_test.go`.
- Edit (frontend): `src/lib/users.ts` (+ test), `src/lib/api-schema.ts`, `src/lib/api-types.ts` (generated), `src/app/(app)/admin/users/[id]/edit/edit-user.tsx` (+ its test), `e2e/a11y.spec.ts`.
- Docs: `docs/API.md`, `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, `backend/docs/swagger.*` (generated).
- Read only: `repository/user_repo.go` `UpdatePasswordKeepSession` (the revoke pattern), `service/auth_service.go` `ChangePassword` (the 72-byte and bcrypt pattern), `status-section.tsx` (the dialog pattern), `components/auth-field.tsx` (reused unchanged), `lib/auth-form.ts`, `lib/form-focus.ts`.

## Data / contracts

- **Request:** `POST /admin/users/{id}/reset-password`, body `{"new_password": "...", "confirm_password": "..."}`. **Response:** 204, no body. Errors as listed above.
- **Password rules:** the same as everywhere else: `strongpass` (60 bits), at most 72 bytes, `confirm_password` equal to `new_password` byte for byte.
- **Audit:** action `ADMIN_RESET_PASSWORD`, target type `User`, target the user's email, details `{"sessions_ended": N}`. Written in the same transaction as the password update, the session revoke and the lockout clear.
- **Trusted actor:** the signed-in admin is read from the session, never from the body; the target comes from the path.
- **Atomicity:** password update, session revoke, lockout clear and audit row are one transaction; the hash is computed before it and holds no lock. No admin-row locking is needed because the reset changes neither status nor role.
- **Secrets:** the password is never logged, audited, returned, cached or put in a URL. The only place it is shown is the admin's own browser, once, in the dialog; it is cleared on close.
- **Rendering:** names and emails render as text only. The generated password is rendered as an input value or text node, never as HTML.

## Testing

- Go: `make test`, `make test-integration` (Docker, testcontainers), `make lint`, `make vet`. Vitest: `npm test`. Playwright with axe: `npm run e2e` (the e2e API starts itself; the test Postgres must be up, `make db-start`). Never run e2e against production.
- `AGENTS.md` declares no single Verify command, so the done-when lists above are the gate.
- Not claimed unless run: any visual match to a design, any production or deployed check, the clipboard on a real device or an insecure origin, and any e-mail delivery.

## Notes for the AI

- Keep it small: one endpoint, one service method, one repository method, one migration, one dialog section, one generator function. No new dependency (the Web Crypto API is native), no password-pair component, no new table or column.
- Reuse `UpdatePasswordKeepSession` with an empty token rather than a new revoke query; reuse `AuthField`, the 22d messages, `toFormError`, `useFocusFirstError` and the `AlertDialog` pattern of `status-section.tsx`.
- The self check comes before hashing; the 72-byte check before hashing; never log or audit the password.
- A new audit action needs its CHECK migration in the same deploy as the code (`AGENTS.md` guardrail). Do not edit `frontend/AGENTS.md`. Do not commit or push; the owner pushes.
- Match the surrounding style: Squirrel builders, `apperror`, `userAudit`, `fieldError`, `NativeSelect`/`AlertDialog` on the frontend.

## Open questions

None block the build. Choices made where the request was silent, all reversible: an admin may reset another admin and any non-active account (the admin role already carries full access, and it is audited); the endpoint answers 204 with no body; the password stays visible in a done state until the dialog closes (so a forgotten Copy is not fatal) and exists nowhere else; a self-reset with an invalid body answers 422 before 403; `limits.Write` guards the route; the generator is client-side with a 20-character alphabet without look-alikes; a "must change at next sign-in" flag is not built because it needs a column.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":19356,"specSha256":"55f38f7e6ebcda57227fd18dcbe9f9804b4d80ae26d662e9dfc460989d017d11","branch":"refs/heads/feature/admin-reset-password","head":"bac66a1655166aba87a3ecf25fb4363b618ddc43","baseRef":"refs/heads/main","baseCommit":"bad17046cd950d4d4338ef749529c76c971d2889","sourceTree":"e4c9c00c2a3480c24625855cf86eed2fd6e469f3","landing":"local-merge","absentOptional":[]} -->

## Verification

- `make test`, `make vet`, `make lint`: pass (lint: 0 issues). `make test-integration` (Docker): pass, all 13 packages, including the new audit-CHECK, repository, service and rollback tests.
- `npm test`: 29 files, 285 tests passed. `npm run lint` and `npm run typecheck`: exit 0. `npm run build`: exit 0 (dev server stopped first).
- `npm run e2e`: 85 passed (Playwright with axe, light and dark), including the new `admin-reset-password.spec.ts` (9 tests against the real server: mismatch, Generate and Copy and Reset, old password refused and new one signs in, an open session ends, a locked-out account signs in, own-account button off and API 403, 422 and 404, plain user 403, and axe on the section and the dialog in its form, error, generated and done states).
- `make swagger` then `npm run gen:api`: run repeatedly, no further diff.
- Red first: the new Go tests failed on the real CHECK constraint (`chk_audit_logs_action`) and on missing symbols before the migration, the repository method and the service existed; the handler and router tests got a 404 before the route existed; the Vitest cases failed on missing modules before the generator, the call and the dialog.
- Side fix, its own commit on the branch (`46bbccb`): `e2e/support.ts` `clientIp()` used signed shifts, which gave invalid addresses for some seeds so those tests shared one login bucket. It now uses `>>>`.
- Not run: `/check`, a builder `/audit`. Nothing was checked against production, there was no real-device or insecure-origin clipboard check, and no mutation checks (code was not reverted to watch the tests fail).
- Deviations from the spec: `components/auth-field.tsx` gained an optional `defaultShown` prop (default false) so Generate can show both fields; the axe coverage for the dialog went into the new e2e spec instead of `a11y.spec.ts`.
- Carried forward, still open in `blueprint/context/findings.md` (P3, not archived because they are not resolved): F-01 and F-02 (22a), F-03 (22d) and F-04 (this feature: a login that passed its password check just before a reset can still start a session just after the revoke, the same short window as change-password).
- Deploy order: the migration only widens a CHECK and the old code never writes the action, so the order is safe. The new frontend calls a route the old API lacks, so the Reset password button gets a 404 until Railway finishes.

## Independent review

**Status:** passed
**Target commit:** bac66a1655166aba87a3ecf25fb4363b618ddc43
**Base commit:** bad17046cd950d4d4338ef749529c76c971d2889
**Base ref:** main
**Spec hash:** 55f38f7e6ebcda57227fd18dcbe9f9804b4d80ae26d662e9dfc460989d017d11
**Prepared by:** claude
**Builder model:** claude-sonnet-5-5
**Requested reviewer:** claude
**Requested model:** runtime default (exact model not known until reviewer starts)
**Requested execution:** automatic
**Requested at:** 2026-10-17T04:46:00Z
**Workflow:** regular
**Check required:** no
**Reviewer adapter:** claude
**Reviewer model:** claude-sonnet-5-5
**Reviewer context:** fresh subagent
**Actual execution:** automatic
**Reviewed at:** 2026-10-17T05:10:00Z
**Scope:** current
**Lenses:** quality, security, performance, tests
**Verdict:** passed
**Check result:** not-required

### Commands

- `git rev-parse HEAD`, `git merge-base main HEAD`, `sha256sum blueprint/context/current-feature.md`, `git status --short`: pass (all match the request; only review.md differed)
- `git diff bad1704..bac66a1` (37 files, read for backend, docs, generated types, frontend component, generator, e2e): pass
- `cd backend && go vet ./...` and `go vet -tags integration ./...`: pass
- `cd backend && go test ./internal/...`: pass
- `cd frontend && npx vitest run`: pass (29 files, 285 tests)
- `cd frontend && npm run typecheck`: pass
- `cd frontend && npm run lint`: pass

### Evidence

- ResetPassword order in backend/internal/service/user_service.go:218-259: actor (ErrUnauthorized), self-refusal, `len()` in bytes against 72, target lookup, bcrypt, then one WithTx with UpdatePasswordKeepSession(..., ""), ClearLockout(ctx, q, id), audit insert; any error rolls back all. ErrNotFound inside the tx maps to 404.
- user_repo.go:381-407: the revoke is `WHERE user_id = $1 AND token <> $2 AND revoked_at IS NULL`; an empty keep-token ends every live session of that user only. ClearLockout runs on the passed Querier. The integration tests assert both, including rollback.
- Audit: ADMIN_RESET_PASSWORD, actor name and role, target email via userAudit, details `{"sessions_ended": N}` only. Go AuditActions (12) matches the migration CHECK list; the Down restores the 11-action list NOT VALID. Migration and writer are in one commit.
- Logging: SlogLogger logs method, redacted URI, status, sizes and the error text, never bodies. Validator errors never include submitted values. The 500 path hides its cause. 204 has an empty body.
- Route: admin group (RequireRole admin) + perm users:write + limits.Write, the same limiter as change-password. Actor from the session, id parsed with ParseInt and > 0. Handler order is bind, validate, service. Status is untouched (tests assert no UpdateStatus call).
- Generated types: api-types.ts has AdminResetPasswordRequest and the 204; docs (API.md, ARCHITECTURE.md, D-A12) match the code.
- Frontend: password.ts uses crypto.getRandomValues, rejection sampling, an alphabet without 0 O 1 l I, one of each class, a correct Fisher-Yates shuffle, 20 chars (ASCII, under 72 bytes). The section keeps the password only in state, clears it on every close path, blocks close while pending, has a clipboard failure path, and disables self-reset. AuthField `defaultShown` defaults to false, so existing callers are unchanged.
- support.ts: `>>>` is correct; hashes below 2^31 give identical addresses, larger ones now give valid addresses instead of falling back to 127.0.0.1, so other tests' buckets only get more isolated.
- Test quality: new service, handler, router and repository tests assert the behaviour above (empty keep-token, byte-count cases, rollback on audit failure in the integration test, no password in audit or body). No weakened or deleted assertions in the existing tests. The e2e spec uses its own accounts, runs under the single-worker config, and checks old and new password, open session and lockout.
- Callers: UserAdminService has the real service and the two test fakes updated; UserRepository has UserRepo and MockUserRepo updated. go vet with the integration tag compiles.

### Findings

- F-04 [P3]: a login that passed the bcrypt compare just before a reset can still start a session just after the revoke (same window as ChangePassword).
- F-01, F-02, F-03 carried from earlier work, untouched and not re-reviewed.

### Remaining risk

- Not run (out of this reviewer's allowed scope): `make test-integration` and all `//go:build integration` tests (Docker), `npm run e2e` (Playwright, servers), `npm run build`, `make lint` (golangci-lint), and `make swagger` / `npm run gen:api` drift checks (they rewrite files). The generated swagger and api-types were compared by reading only.
- Spec Files list mentions an `e2e/a11y.spec.ts` edit; axe coverage was put in admin-reset-password.spec.ts instead. Not a defect, but not run.
- Reviewer model and fresh-context status are declared metadata.

## Try it now

1. Start the stack (`make db-start` and `make dev` in `backend/`, `npm run dev` in `frontend/`) and sign in as an admin.
2. Open another user's Edit page from the Users tab. Under **Status** there is a **Reset password** section with a **Reset password** button.
3. Press it. In the dialog, type different values in **New password** and **Confirm password** and press **Reset password**: "Confirm password does not match" appears and nothing is sent. Press **Generate random password**: both fields fill with the same 20-character password and show it. Press **Copy password**: a "Copied" toast appears.
4. Press **Reset password**. The dialog stays open and shows the password once, "This password will not be shown again". Press **Close**: the password is gone from the page.
5. In a private window, sign in as that user with the new password: it works. The old password gets the generic "Invalid email or password." If that user had another window open, it is signed out.
6. Open your own Edit page: the button is off, with "You cannot reset your own password. Use the Change Password page instead."

Unaffected regression path: **Change password** in the avatar menu still asks for the current, new and confirm password.
