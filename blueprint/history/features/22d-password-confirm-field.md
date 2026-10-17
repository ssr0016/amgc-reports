# Feature: Password confirm field

**From build-plan:** feature 22d
**Build attempt:** 1
**Branch:** feature/password-confirm-field
**Status:** verified

## Goal

Every form that sets a new password asks for it twice, and the server enforces it: a `confirm_password` that is missing or different from the new password is a 422 on `confirm_password`. This stops a typo from locking a church worker out of an account an admin just created or a user just changed. It covers the create-user form, the change-password form, and the public reset-token request. D-A14 records the rule.

## In scope

### Server

- `model.CreateUserRequest` (`POST /admin/users`): add `ConfirmPassword string` `json:"confirm_password" validate:"required,eqfield=Password"`.
- `model.ChangePasswordRequest` (`POST /auth/change-password`): add `ConfirmPassword` with `validate:"required,eqfield=NewPassword"`.
- `handler.ResetPasswordRequest` (`POST /auth/reset-password`): add `ConfirmPassword` with `validate:"required,eqfield=NewPassword"`.
- `validator.message` gains an `eqfield` case that returns "does not match" (today it falls through to "invalid"). Missing stays "is required".
- The comparison is exact: no trimming, no case folding. Validation happens in `c.Validate` before any service call, so a mismatch never reaches `AuthService.ChangePassword` (which would otherwise count a wrong current password toward lockout, and take the bcrypt cost) or `UserService.Create` or the token lookup.
- A mismatch is reported together with the other field errors (a weak `new_password` and a mismatch both appear). The 422 body never holds a submitted value (the validator already guarantees it).
- Not stored, not logged, not in an audit row: `confirm_password` is read and dropped.

### Frontend

- **Change password** (`/account/password`, `change-password.tsx`): a third `AuthField` "Confirm password" (`name="confirm_password"`, `autoComplete="new-password"`) after "New password". Empty is "Confirm password is required"; different from the new password is "Confirm password does not match", checked before the request (no request spent). The server's 422 on `confirm_password` shows the same way. Focus goes to the first invalid field (`useFocusFirstError`); the form clears after success as today. `changePassword(current, next, confirm)` in `lib/auth.ts` sends `confirm_password`.
- **Create user** (`/admin/users/new`, `user-form.tsx`, create mode only): "Confirm password" after "Password", same rules. `UserFormValues` gains `confirm_password`; `createUser` input and `new-user.tsx` send it. Edit mode has no password field and is unchanged. A failed save keeps what was typed.
- **Reset-token form:** the page for it does not exist yet (build-plan 25), so this feature changes only the server request and docs for it. Item 25 must add the field when it builds the page.
- Labels are exactly "Confirm password" (not "Confirm new password"), so existing `getByLabel("New password")` selectors stay unambiguous.

### Contract and docs (same commit)

- Swagger annotations follow the structs; `make swagger` then `npm run gen:api` (CI fails on drift in `frontend/src/lib/api-types.ts`); `lib/api-schema.ts` if it lists the request fields.
- `docs/API.md`: the three requests and the 422 shape. `docs/DECISIONS.md`: **D-A14 Password confirmation** (all password forms, server-enforced, exact match, 422 on `confirm_password`, the reset-token page is item 25's, the deploy-order note below) with an index row.

## Out of scope

- The admin "Reset password" button, dialog, generator and `ADMIN_RESET_PASSWORD` audit (separate feature, D-A12).
- Removing the editor role (D-A13), the permissions UI, and any password-strength change.
- Building the forgot-password and reset-token pages (build-plan 24 and 25), email delivery, and a "Show password" label change on `AuthField` (its toggle already exists).
- A change to the login form (one password, no confirm).

## Build loop

`workflow.stepReview` is `feature`: review once after all steps. `checkpointCommits` is disabled; `/complete` makes the final commit. Test-first inside each step. No migration and no new audit action. **Deploy order:** the API now rejects a create-user or change-password body without `confirm_password`, and the old frontend does not send it. Push both in one go and expect a short window where those two forms fail (422) until Vercel finishes. Railway has no "Wait for CI".

## Build steps

- [x] 1. **Server validation (tests first).** Write the cases first. Validator: `eqfield` fails with "does not match" on the right field name, an empty confirm is "is required". Handler tests for `POST /admin/users`, `POST /auth/change-password` and `POST /auth/reset-password`: matching passwords reach the service with the same arguments as before; a missing or different `confirm_password` is 422 with `details.confirm_password` and the service is **not** called (fake service call count 0); a weak password plus a mismatch returns both fields; a case-only or trailing-space difference is a mismatch. Then add the field to the three structs and the `eqfield` message. Update the existing tests and helpers that post these bodies (`change_password_test.go` `cpBody`, `change_password_routes_test.go`, `bodylimit_test.go`, `admin_user_write_test.go`, `admin_users_routes_test.go`, the reset-password handler tests, and `testutil` or integration helpers that create users through the API).
  Done when: `make test` shows the new cases red first and then green, `make test-integration` passes, and `make lint` and `make vet` pass.
- [x] 2. **Contract.** Update the Swagger comments, run `make swagger` then `npm run gen:api`, and check `lib/api-schema.ts`. Update `docs/API.md` for the three requests.
  Done when: `make swagger` then `npm run gen:api` leave no further diff in `frontend/src/lib/api-types.ts`, and `npm run typecheck` fails only where the frontend still calls the old signatures (fixed in step 3).
- [x] 3. **Forms (tests first).** Vitest cases first: change-password shows "Confirm password" after "New password"; an empty confirm shows "Confirm password is required" and focuses it; a mismatch shows "Confirm password does not match" and sends **no request**; matching values call `changePassword(current, next, confirm)`; a server 422 on `confirm_password` shows on that field; success clears all three fields. Create-user: the same four cases through `UserForm`, edit mode shows no confirm field, a failed save keeps the typed values. `lib/auth.test.ts` and `lib/users.test.ts` assert `confirm_password` in the request body. Then implement `change-password.tsx`, `user-form.tsx`, `new-user.tsx`, `lib/auth.ts`, `lib/users.ts`.
  Done when: `npm test` shows the new cases red first and then green, and `npm run lint` and `npm run typecheck` pass.
- [x] 4. **Browser checks, docs and full suites.** Add `confirm_password` to every e2e call that creates a user or changes a password through the API (`e2e/setup.ts`, `user-status.spec.ts`, `a11y.spec.ts`, any other found by searching `"password"`), and to the forms in `account.spec.ts`. New e2e: a mismatch on `/account/password` shows the error on Confirm password and keeps you signed in; creating a user with a mismatch shows the error and creates nothing; both pages pass axe, including the focus and the error association (`aria-describedby`). Record D-A14 in `docs/DECISIONS.md` and update `docs/ARCHITECTURE.md` only if it lists these request shapes.
  Done when: `make test`, `make test-integration`, `make lint`, `npm test`, `npm run lint`, `npm run typecheck`, `npm run build` and `npm run e2e` pass, or any that cannot run is named in the summary with the reason.

## Files / areas

- Edit (backend): `internal/model/user.go`, `internal/handler/password_reset_handler.go`, `internal/validator/validator.go` (+ test), `internal/handler/auth_handler.go` and `admin_user_handler.go` (Swagger comments), the tests named in step 1.
- Edit (frontend): `src/app/(app)/account/password/change-password.tsx` (+ test), `src/components/user-form.tsx`, `src/app/(app)/admin/users/new/new-user.tsx`, `src/lib/auth.ts`, `src/lib/users.ts` (+ tests), `src/lib/api-schema.ts`, `src/lib/api-types.ts` (generated), `e2e/setup.ts`, `e2e/account.spec.ts`, `e2e/user-status.spec.ts`, `e2e/a11y.spec.ts`.
- Docs: `docs/API.md`, `docs/DECISIONS.md`, `backend/docs/swagger.*` (generated).
- Read only: `components/auth-field.tsx` (reused unchanged), `lib/auth-form.ts`, `lib/form-focus.ts`.

## Data / contracts

- **Requests:** `confirm_password` is a required string in `POST /admin/users`, `POST /auth/change-password` and `POST /auth/reset-password`. It must equal `password` (create) or `new_password` (the other two), byte for byte.
- **Errors:** 422 with the existing `apperror` shape, `details.confirm_password` of "is required" or "does not match". Other statuses are unchanged (400 bad body, 401, 403, 429).
- **Order of checks:** body bind (400), then struct validation including the match (422), then the service. The 72-byte limit and "same as current" rules stay in the service and are untouched.
- **Secrets:** no password or confirmation value is logged, audited, echoed in an error, or kept in a query cache. The frontend clears the fields after success.
- **Persisted data:** none. No migration, no new column, no new audit action.

## Testing

- Go: `make test`, `make test-integration` (Docker), `make lint`, `make vet`. Vitest: `npm test`. Playwright with axe: `npm run e2e` (the e2e API starts itself; the test Postgres must be up, `make db-start`). Never run e2e against production.
- `AGENTS.md` declares no single Verify command, so the done-when lists above are the gate.
- Not claimed unless run: any visual match, any production check, and any check of the not-yet-built reset-token page.

## Notes for the AI

- Keep it small: one struct field per request, one validator message, one `AuthField` per form. No shared "PasswordPair" component or helper unless the same eight lines appear three times.
- Use `eqfield`, the validator's built-in. Do not write a custom validator or compare in the handler.
- Keep the client mismatch check in addition to the server one; the server is authoritative.
- Match the surrounding style: `fieldError`/`apperror`, `AuthField`, `useFocusFirstError`. Do not edit `frontend/AGENTS.md`.
- Do not touch the admin reset, the editor role or the permissions UI. Do not commit or push; the owner pushes.

## Open questions

None block the build. Choices made where the request was silent, all reversible: the confirm field is required by the API for every caller, not only the UI (the owner chose both); the label is "Confirm password" to keep selectors unambiguous; a mismatch is checked in the browser first and again on the server; the reset-token form is left to build-plan 25.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":10846,"specSha256":"834bed029e3c6b5ca580965652f57644984213721269c459263594c68a7cb3aa","branch":"refs/heads/feature/password-confirm-field","head":"f08842eca1b6e3ed59e0f9f117e6a22596c2f5d2","baseRef":"refs/heads/main","baseCommit":"bff1b3edecc1f87c08b3b0b55e7258d3480b947b","sourceTree":"67f58f5e3daae4b006b04d14f27923992e70bfdc","landing":"local-merge","absentOptional":[]} -->

## Verification

- `make test`, `make vet`, `make lint`: pass (lint: 0 issues). `make test-integration` (Docker): pass, all 13 packages.
- `npm test`: 27 files, 258 tests passed. `npm run lint` and `npm run typecheck`: exit 0. `npm run build`: exit 0 (dev server stopped first).
- `npm run e2e`: 77 passed (Playwright with axe, light and dark), including the new `password-confirm.spec.ts` and the confirmation-error axe case.
- `make swagger` then `npm run gen:api`: run twice, no further diff in `frontend/src/lib/api-types.ts`.
- Red first: the new Go tests failed on the missing field and the `eqfield` message before the code; the new Vitest cases and the typecheck failed before the forms; the e2e setup failed against the real API with `confirm_password: is required` before the helpers were updated.
- Not run: `/check`, a builder `/audit`. Nothing was checked against production, and no visual comparison was taken. The reset-token form has no page yet (build-plan 25), so only its server request is covered.
- Carried forward, still open in `blueprint/context/findings.md` (P3, not archived because they are not resolved): F-01 and F-02 (from 22a) and F-03 (`lib/users.test.ts` does not assert that `createUser` sends `confirm_password`; the e2e create-user test covers the path).
- Deploy order: the API now rejects create-user and change-password bodies without `confirm_password` and the old frontend does not send it. Deploy both together (D-A14).

## Independent review

**Status:** passed
**Target commit:** f08842eca1b6e3ed59e0f9f117e6a22596c2f5d2
**Base commit:** bff1b3edecc1f87c08b3b0b55e7258d3480b947b
**Base ref:** main
**Spec hash:** 834bed029e3c6b5ca580965652f57644984213721269c459263594c68a7cb3aa
**Prepared by:** claude
**Builder model:** claude-sonnet-5-5
**Requested reviewer:** claude
**Requested model:** runtime default (exact model not known until reviewer starts)
**Requested execution:** automatic
**Requested at:** 2026-10-17T03:22:30Z
**Workflow:** regular
**Check required:** no
**Reviewer adapter:** claude
**Reviewer model:** claude-sonnet-5-5
**Reviewer context:** fresh subagent
**Actual execution:** automatic
**Reviewed at:** 2026-10-17T03:40:00Z
**Scope:** current
**Lenses:** quality, security, performance, tests
**Verdict:** passed
**Check result:** not-required

### Handoff

Review the active spec and the complete `<base>..<target>` delta in a fresh
session or isolated subagent without the builder conversation. Run all Audit lenses from scratch.
Run Check when required above. Do not edit product code, accept findings, or
reuse the existing findings as the review scope.

### Commands

- `git rev-parse HEAD`, `git merge-base main HEAD`, `sha256sum blueprint/context/current-feature.md`, `git status --short`: pass (all match the request; only review.md differs)
- `git diff bff1b3e..f08842e` (32 files read: backend, frontend, e2e, docs, generated files): pass
- `cd backend && go vet ./...`: pass
- `cd backend && go test ./internal/...` (no Docker): pass
- `cd frontend && npx vitest run`: pass (27 files, 258 tests)
- `cd frontend && npm run typecheck`: pass

### Evidence

- Only CreateUserRequest (`eqfield=Password`), ChangePasswordRequest and handler.ResetPasswordRequest (`eqfield=NewPassword`) gained `confirm_password` with `required,eqfield=...`; targets are correct. The validator maps `eqfield` to "does not match" and keeps "is required" for missing.
- Validation runs in `c.Validate` before the service call; new handler tests assert 422, service call count 0, and for change-password that a wrong current password is not reported when the confirmation is wrong (no lockout strike). Router test shows a matching reset body reaches the token lookup (400) while a mismatch stays 422.
- Error bodies carry only the message string; the create test asserts the password is absent from the body. `confirm_password` is not read by any service, log or audit call.
- Frontend: both forms check the mismatch client-side and map a server 422 to the same field; the error message id matches `aria-describedby`, `aria-invalid` is set and `useFocusFirstError` moves focus; change-password calls `form.reset()` on success; edit mode renders no confirm field; no password is placed in a query cache. `lib/auth.ts` and `new-user.tsx` send `confirm_password`; `edit-user.tsx` only seeds an empty value.
- swagger.json/yaml/docs.go and `api-types.ts` match the structs. docs/API.md and D-A14 are accurate, including the deploy-order note and that the reset-token page belongs to build-plan 25.
- No other caller of the three endpoints lacks the field. The remaining bodies without it are intentionally invalid validation cases and the removed `/auth/register` test.
- Existing fixtures were extended (cpBody, createBody, routes and integration bodies) and no assertion was weakened or deleted.

### Findings

- F-03 [P3] open: `lib/users.test.ts` does not assert that `createUser` sends `confirm_password` (spec step 3). F-01 and F-02 are carried from 22a and were not re-reviewed.

### Remaining risk

- Not run: `make test-integration` (needs Docker), `make lint`, `npm run lint`, `npm run build`, `npm run e2e` (Playwright, not permitted). The new e2e spec and the axe case were read, not executed.
- Not run: red-first checks. That the new tests fail without the code was judged from reading them, not by reverting the code.
- Deploy-order window (documented in D-A14): the old frontend gets 422 on create-user and change-password until the new frontend is live.

## Try it now

1. Start the stack (`make db-start` and `make dev` in `backend/`, `npm run dev` in `frontend/`) and sign in as an admin.
2. Open **Change password** from the avatar menu. There are three fields: Current password, New password, **Confirm password**. Type a different value in Confirm and press **Change password**: "Confirm password does not match" appears under it, focus moves there, and no request is sent.
3. Open `/admin/users/new`. Fill email, name and password, and a different Confirm password. Press **Create User**: the same message appears and nothing is created. Make them equal and the user is created and listed in the Users tab.
4. Open any user's Edit page: there is no password or confirm field there.

Unaffected regression path: the login page still has one Password field and signing in works.
