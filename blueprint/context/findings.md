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

### F-08 [P3] open - Names that differ only by Unicode case folding merge two reports into one sheet

**File:** backend/internal/report/reportimpl/excel.go:218, backend/internal/report/reportimpl/excel.go:226-230
**Found:** 2026-10-22 by /audit independent (scope: current; lens: quality)
**Why it matters:** `sheetNames` dedupes on `strings.ToLower`, but excelize's `NewSheet` matches existing sheets with `strings.EqualFold` (`GetSheetIndex`, sheet.go:428-439). When a name collides it returns the existing index with no error and creates nothing. Some names are equal under `EqualFold` but not under `ToLower`, such as `ſ`/`s` or `ς`/`σ`. In that case `buildWorkbook` writes the second report into the first one's sheet. A probe showed `["Zion ſ", "Zion s"]` gives one sheet and no error. The export then silently drops a report, while the bulk audit row still counts both. This predates the delta (feature 22): the delta kept the dedupe unchanged, as the spec asked. Church names using these characters are unlikely, so this is P3.
**Suggested fix:** Dedupe with `strings.EqualFold` over the names already chosen. A slice scan is fine at the 100-report cap. Add the `ſ`/`s` pair to `TestSheetNames`.
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

### F-10 [P3] open - The dialog test doesn't show that Escape alone closes it

**File:** frontend/src/app/(app)/admin/excel-preview-dialog.test.tsx:93
**Found:** 2026-10-22 by /audit independent (scope: current; lens: tests)
**Why it matters:** "Cancel and Escape close without downloading" clicks Cancel, then presses Escape, then checks `onClose` was called at least once. The Cancel click already satisfies that, so the Escape path (`onOpenChange(false)` while not downloading) is never shown on its own. Nothing checks that Escape is ignored while a download is in flight either. The behavior is plain Radix and looks right, so this is a coverage gap only.
**Suggested fix:** Split it into two cases, Cancel then `onClose` once, and Escape then `onClose` once, each with `onDownload` not called. Optionally add one that checks Escape during "Downloading…" leaves the dialog open.
**Resolution:**

### F-11 [P3] open - The dashboard order test can't tell case-insensitive order or the id tiebreak

**File:** backend/internal/report/reportimpl/store_integration_test.go:334-340
**Found:** 2026-10-22 by /audit independent (scope: current; lens: tests)
**Why it matters:** Spec step 1 asks the integration test to cover the order (`lower(u.church_name), u.id`). The fixture's churches are Abba, Bethel, Calvary, Redo and zion. The only lowercase one, "zion", also sorts last by byte order and by a locale collation, so dropping `lower()` from `OrderBy` would still pass. No two accounts share a church name, so the `u.id` tiebreak is never exercised either. The query is correct today; this is a coverage gap only.
**Suggested fix:** Add an account whose lowercase church sorts before an uppercase one by bytes (e.g. "abel" next to "Bethel"), and two accounts with the same church name in different case, then assert their order by id.
**Resolution:**

### F-12 [P3] fixed - Migration-procedure doc change leaves SETUP.md and project-overview.md saying pre-deploy

**File:** AGENTS.md:48, docs/ARCHITECTURE.md:25, backend/SETUP.md:157-158, backend/SETUP.md:186-190, blueprint/context/project-overview.md:107
**Found:** 2026-10-22 by /audit independent (scope: current; lens: quality)
**Why it matters:** The delta rewrites the production-migration rule in `AGENTS.md`, `docs/ARCHITECTURE.md` and `docs/DECISIONS.md` (Render Free tier runs neither Pre-Deploy nor Shell; run `make migrate-up` against Neon before pushing). That change is outside the Tracker spec's Files / areas. `backend/SETUP.md` (the runbook ARCHITECTURE links to) still tells the reader to set `/app/api migrate` as the Pre-Deploy Command, and `blueprint/context/project-overview.md` still says "run `/app/api migrate` as a separate pre-deploy step". So a deployer gets two different procedures. No code is affected and this feature has no migration, so it is P3.
**Suggested fix:** Update SETUP.md section 1 and section 3, and project-overview.md line 107, to match the D-H1 update of 2026-10-22. Or move the whole doc change out of the feature commit into its own docs commit.
**Resolution:** Fixed 2026-10-22 on `fix/migration-docs-for-render-free-tier-and-pending-in-the-tracker`: SETUP.md sections 1 and 3 and both runbooks, and project-overview.md:107, now describe the local migrate run against Neon before pushing `main`. Awaiting re-review.
