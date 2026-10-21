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

### F-04 [P3] open - A login that verified the old password just before a reset can still start a session after the reset

**File:** backend/internal/service/user_service.go:211-259, backend/internal/service/auth_service.go:96-134
**Found:** 2026-10-17 by /audit independent (scope: current; lens: security)
**Why it matters:** The reset revokes every session that exists when its transaction runs. `AuthService.Login` reads the hash, runs bcrypt (about 100 ms), and only then does the handler create the session. A login with the old password that passed the compare just before the reset commits can create a session just after the revoke, so "signs them out everywhere" is not strictly true for that window. The same applies to a failed-login record that lands after `ClearLockout`. Reaching it needs the old password and exact timing, which is the case an admin reset is meant to close (a leaked password), so the impact is low. `ChangePassword` has the same window today.
**Suggested fix:** Optional: accept and note the residual window in D-A12 (as D-A10 may for F-02), or have session creation re-check that the password hash is unchanged. No current requirement is lost by only documenting it.
**Resolution:**

### F-05 [P2] open - Some church names make the Excel export fail with a 500, and one such report blocks the whole month

**File:** backend/internal/report/reportimpl/excel.go:219-226, backend/internal/report/reportimpl/excel.go:234-239
**Found:** 2026-10-21 by /audit independent (scope: current; lens: quality)
**Why it matters:** `sheetNames` cuts names to 31 runes, but excelize v2.11.0 `checkSheetName` counts UTF-16 units, and it also rejects a name that starts or ends with `'`. The leading/trailing `'` is trimmed before the cut, so the cut can leave a new trailing `'`. A probe against `workbookBytes` (run through a `go test -overlay`, no repo file added) showed that 20 × "🙏" fails with "the sheet name length exceeds the 31 characters limit", and `"a"×30 + "'b"` fails with "the first or last character of the sheet name can not be a single quote". `church_name` is free text up to 150 characters, so an admin gets a 500 for that report. The bulk export then fails for the whole month, because every report shares one workbook. No audit row is written, which is correct. `TestSheetNames` only covers ASCII names.
**Suggested fix:** Truncate by UTF-16 units (or loop on excelize's own length check), trim `'` after truncating, including after the ` (n)` suffix step, and add the two cases above to `TestSheetNames`.
**Resolution:**

### F-06 [P3] open - filename* leaves some characters unescaped that RFC 5987 does not allow

**File:** backend/internal/report/rest/report_handler.go:284
**Found:** 2026-10-21 by /audit independent (scope: current; lens: security)
**Why it matters:** `url.PathEscape` escapes `'`, `"`, `;`, spaces, controls and non-ASCII, so header injection and breaking the parameter are not possible. Together with `exportFilename`, which strips CR/LF and controls, the header is safe. But PathEscape leaves `= : @ & + $` unescaped, and these are not RFC 5987 `attr-char`s. A church name such as `A=B` or `Zion: East` therefore produces a technically invalid `filename*`. Browsers and the frontend's own regex in `api.ts` are lenient, so the impact is cosmetic or limited to strict clients.
**Suggested fix:** Percent-encode every byte outside RFC 5987 `attr-char`, or document the leniency. Optional.
**Resolution:**

### F-07 [P3] open - No unit test covers saveFile's object URL lifecycle

**File:** frontend/src/lib/api.ts:150-157, frontend/src/app/(app)/admin/admin-reports.test.tsx:26
**Found:** 2026-10-21 by /audit independent (scope: current; lens: tests)
**Why it matters:** The spec requires the download to go through an object URL that is then revoked. `admin-reports.test.tsx` mocks `saveFile`, and `api.test.ts` tests only `downloadFile`, so nothing checks that `createObjectURL` is called, that the anchor gets `download=filename`, or that `revokeObjectURL` runs. The Playwright spec proves a download happens, but not that the URL is revoked. The code looks correct (revoked after 1 s), so this is a coverage gap only.
**Suggested fix:** Add a Vitest case with fake timers that stubs `URL.createObjectURL`/`revokeObjectURL` and checks the click, the download name, and the revoke.
**Resolution:**
