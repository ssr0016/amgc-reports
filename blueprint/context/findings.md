# Findings

> **Generated file.** The findings ledger: review findings raised by `/audit`
> against the work in progress, each with a durable ID, severity (P0-P3), and
> status. `/implement` marks repaired findings `fixed`, a later `/audit` pass
> moves them to `closed`, and `/complete` refuses to land while any P0 or P1
> finding is `open` or `fixed`, then archives resolved findings with the work
> and resets this file.

### F-01 [P3] open - Status badge prints the raw server string for an unknown status

**File:** frontend/src/lib/user-status.ts:19, frontend/src/components/user-status-badge.tsx:25
**Found:** 2026-10-17 by /audit (scope: current; lens: quality)
**Why it matters:** The spec says the status label comes from a fixed map, not from the server string. `statusLabel` falls back to the server value (`?? status`) and the badge also puts it in `data-status`. React escapes it, so there is no injection, and `chk_users_status` limits the stored values, so this is a spec deviation only.
**Suggested fix:** Fall back to a fixed label such as "Unknown" instead of echoing the server value, or record the echo as intended in the spec or a decision.
**Resolution:**

### F-02 [P3] open - Session created in a login/status-change race is not refused by RequireAuth-only routes

**File:** backend/internal/router/router.go:61-66
**Found:** 2026-10-17 by /audit (scope: current; lens: security)
**Why it matters:** If an admin deactivates a user between Login reading the user and the session being created, the revoke finds no row and a live session survives for an inactive user. RequireRole, RequirePermission and Me refuse it, but `POST /auth/change-password` and `POST /auth/logout` are behind RequireAuth only. Impact is small: the user can only change their own password, and the window is a few milliseconds.
**Suggested fix:** Optional: check status in ChangePassword (it already loads the user with GetWithRole), or accept the residual window and note it in D-A10.
**Resolution:**

### F-03 [P3] open - No test asserts that createUser sends confirm_password

**File:** frontend/src/lib/users.test.ts:42-44
**Found:** 2026-10-17 by /audit independent (scope: current; lens: tests)
**Why it matters:** Spec step 3 says `lib/auth.test.ts` and `lib/users.test.ts` assert `confirm_password` in the request body. `auth.test.ts` does; the `createUser` case still posts `{ email } as never` and checks only method and CSRF. `new-user.tsx` is the only place that maps `confirm_password`, and no unit test covers it, so dropping that line would pass Vitest, typecheck (the field is required on the generated type, but `as never` hides it in the test) and lint. The e2e spec would catch it, but it was not run here.
**Suggested fix:** Assert the parsed body of the `createUser` POST contains `confirm_password`, or add a `new-user` test that checks the `createUser` argument.
**Resolution:**
