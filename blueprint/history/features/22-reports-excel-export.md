# Feature: Reports Excel export

**From build-plan:** feature 22
**Build attempt:** 1
**Branch:** feature/reports-excel-export
**Status:** verified

## Goal

Let admins download reports as Excel, using the legacy "Church Growth Program Report" sheet layout: one report as a workbook, or every report for a month as one workbook with a sheet per report. Each download writes its audit row on the server (D-A6, D-10). This is the last unfinished part of Phase C (22a–22e are done).

## Design reference

`blueprint/reference/Church Growth Program Report September 2026.xlsx`. Every church sheet uses the same layout. The empty `SUMMARY` sheet is the blank form. The file holds real worker names, narratives, prayer requests and tithes, and the repository is **public**, so it must never be committed (step 1 adds an ignore rule). The layout below is the contract, so the code doesn't need the file.

Per-report sheet (columns A–G; width A 26, B–G 12.63; default row height 15.75; row 40 height 43.5):

| Row | A (label) | B–F | G |
|---|---|---|---|
| 1–2 | `ANG MANANAMPALATAYANG GUMAWA` (merged A1:G2, Times New Roman 18, centered) | | |
| 3 | `NATIONAL WORKERS' MONTHLY MINISTRY REPORT` (merged A3:G3, bold 12, centered) | | |
| 4 | `Month of:` | B4:G4 merged: first day of the month as a date, format `mmmm yyyy` | |
| 5 | `Worker` | B5:G5 merged: `worker` | |
| 6 | `Area of Assignment` | B6:G6 merged: `area_assignment` | |
| 7 | `Name of Church/Outreach:` | B7:G7 merged: `church_name` | |
| 8 | `Weekly Attendance` (section) | | |
| 9 | `Activities` | `Week 1` … `Week 5` | `Average` |
| 10–17 | Worship Service, Sunday School, Prayer Meeting, Bible Studies, Mens Fellowships, Womens Fellowships, Youth Fellowships, Children Fellowships | weeks 1–5 | average formula |
| 18 | Outreach | weeks | avg |
| 20 | Training/ Seminars | weeks | avg |
| 21 | Leadership Conference | weeks | avg |
| 22 | Leadership Training | weeks | avg |
| 24 | Other | weeks | avg |
| 25 | Family Day | weeks | avg |
| 27 | Tithes & Offering (number format `#,##0.00`) | weeks | avg |
| 29 | `Other Statistics` (section) | | |
| 30 | | `Week 1` … `Week 5` | `Average` |
| 31–36 | Home Visited, Bible Study Group Led, Sermon/Message Preached, Person Newly Contacted, Person Followed-up, Person Led To Christ | weeks | avg |
| 37 | `Names:` | B37:G39 merged, wrapped: `names` | |
| 40 | `Narrative Report:` | B40:G40 merged, wrapped, top: `narrative_report` | |
| 41 | `Challenges/Problem Encountered` | B41:G41 merged, wrapped: `challenges` | |
| 42 | `Prayer Request` | B42:G42 merged, wrapped: `prayer_request` | |

Average cells use the template formula `=IFERROR(AVERAGE(Bn:Fn),"")` on every metric row. It skips blanks, which matches D-A5. Print setup: portrait, gridlines, centered horizontally. Fonts: Helvetica Neue 12 for labels and text, with thin borders on the label and grid cells as in the template. Exact border styles are approximate; the cell positions, labels, merges, formats and formulas must be exact.

## In scope

- `excelize` (approved in D-A6) as the one new backend dependency.
- A workbook builder in `report/reportimpl` that renders one report into one sheet using the layout above. The bulk export reuses it to add one sheet per report.
- `POST /admin/reports/:id/excel`: one report → `.xlsx`. Writes `DOWNLOAD_SINGLE_EXCEL`.
- `POST /admin/reports/excel` with body `{ "month": "September", "year": 2026 }`: every report for that month that isn't soft-deleted, one sheet each → `.xlsx`. Writes `DOWNLOAD_BULK_EXCEL`.
- Admin Reports tab: enable "Download Bulk" (uses the selected month and year) and the per-row "Excel" button; both download the file.
- Ignore the legacy workbook in Git; update API docs, Swagger, generated API types, and the ARCHITECTURE/DECISIONS rows that call the Excel actions "Reserved".

## Out of scope

- The template's "Report Dashboard" and "SUMMARY" sheets (coordinator remark and support status don't exist in our data); see Resolved questions.
- Export for non-admins, the `/reports` page, CSV or PDF, a schema change, new audit actions, emailing exports, and the 20e preview modal.

## Build loop

Feature-level review (`workflow.stepReview: feature`); no checkpoint commits. Each step leaves the build green. Keep the unrelated `go.work.sum` change out of every commit. `/complete` makes the single feature commit.

## Build steps

- [x] **0. Answer the open questions.** Implementation starts only after the user answers them. Record the answers in this spec and in `docs/DECISIONS.md` (D-A6). **Done when:** the Open questions section is resolved.
- [x] **1. Guard the template and add excelize.** Add an ignore rule covering `blueprint/reference/*.xlsx`. `go get github.com/xuri/excelize/v2`. **Done when:** `git status` no longer lists the workbook; `make build` passes.
- [x] **2. Workbook builder.** In `backend/internal/report/reportimpl/excel.go`, add a pure function that turns `[]*report.Report`, with metrics loaded, into an `*excelize.File`: one sheet per report, using the layout above and the metric-to-row map in Data / contracts. Absent weeks stay blank (D-5); zero is written as 0. Numbers go in as numbers. Add a unit test that reopens the bytes with excelize and checks: labels, merges, B4 date and format, every mapped metric row, blank versus 0, the G formula, tithes number format, wrapped text, sheet naming and dedupe, and that text starting with `=` is stored as a string, not a formula. **Done when:** `make test` and `make lint` pass.
- [x] **3. Export service and endpoints.** Add service methods that load reports with metrics through the existing store; the bulk method uses the existing month/year filter with soft-deleted reports excluded. They build the workbook, write the bytes to a buffer, then insert the audit row and return the bytes. If the audit insert fails, the request returns 500 and sends no file. Add the two handlers under an `/admin/reports` group with `RequireRole(userRepo, "admin")`, `reports:read_any`, the CSRF check that existing POST routes use, and `limits.Write`. Response: `Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, `Content-Disposition: attachment` with an RFC 5987 `filename*`, `Cache-Control: no-store`. Add handler tests (200 with xlsx headers, 401, 403 for a non-admin, 404, 422 for an invalid month or year, 422 over the cap) and integration tests (one audit row per download, with `details` exactly `{month, year, count}`; reports whose owner is deleted or deceased are included; soft-deleted reports are excluded; no audit row when the request fails). Then `make swagger`. **Done when:** `make test`, `make test-integration` and `make lint` pass.
- [x] **4. Admin UI.** `npm run gen:api`. Add a `downloadFile` helper in `frontend/src/lib/api.ts`, next to `api<T>`: same credentials, CSRF and 401-refresh handling; it returns a `Blob` and the filename from the header. Add `exportReport(id)` and `exportReports(month, year)` beside `listReports`. Enable both buttons in `admin-reports.tsx`: while a download runs the button is disabled with "Downloading…" and `aria-busy`; on success the browser downloads the file through an object URL, which is then revoked; on error a toast shows the server's message. Keep `rowName(r)` in the per-row sr-only label. "Download Bulk" is disabled when the current list is empty. Add Vitest tests for busy, success and error states, and that the bulk request sends the selected month and year. **Done when:** `npm test`, `npm run lint` and `npm run typecheck` pass.
- [x] **5. Docs and full verification.** Update `docs/API.md` (both endpoints), `docs/ARCHITECTURE.md` (Excel actions are no longer Reserved) and `docs/DECISIONS.md` (D-A6 layout and the placeholder note). Add one Playwright test: an admin downloads bulk and single files (check the `download` event and filename), and axe passes on the Reports tab. Run the full backend and frontend suites, builds and e2e. **Done when:** all of them pass and `make swagger` followed by `npm run gen:api` leaves no diff.

## Files / areas

- `.gitignore`; `backend/go.mod`, `go.sum`
- `backend/internal/report/report.go` (Service interface), `reportimpl/excel.go` with its test, `reportimpl/report.go` (service), the existing store read methods
- `backend/internal/report/rest/` (handlers and tests); `backend/internal/router/router.go`; `backend/cmd/api/app` wiring if needed
- `backend/internal/audit` (existing `ActionDownloadSingle`, `ActionDownloadBulk`, `Insert`)
- `backend/docs` (generated Swagger); `frontend/src/lib/api-types.ts`
- `frontend/src/lib/api.ts`, the reports client module that has `listReports`, `frontend/src/app/(app)/admin/admin-reports.tsx` with its tests
- `frontend/e2e/` (one spec); `docs/API.md`, `docs/ARCHITECTURE.md`, `docs/DECISIONS.md`

## Data / contracts

**Metric → template row** (resolved questions 1–3):

| Row | Key | Row | Key |
|---|---|---|---|
| 10 | worship_service | 21 | leadership |
| 11 | sunday_school | 22 | *(blank)* |
| 12 | prayer_meeting | 24 | other |
| 13 | bible_studies | 25 | family_day |
| 14 | mens_fellowship | 27 | tithes_offering |
| 15 | womens_fellowship | 31 | home_visited |
| 16 | youth_fellowship | 32 | bible_study_group_led |
| 17 | children_fellowship | 33 | sermon_preached |
| 18 | outreach | 34 | person_newly_contacted |
| 19 | baptism (label `Baptism`, in the template's blank row) | 35 | person_followed_up |
| 20 | training | 36 | person_evangelized |

All 21 keys appear exactly once; the builder test fails if a key is missing or duplicated.

**Endpoints.** Both need an admin session and the CSRF header. Admins hold every permission, so D-18 redaction never applies and the export includes text and tithes.
- `POST /admin/reports/:id/excel`: a report that is missing or soft-deleted → 404 `Report not found.` A non-numeric id → 400 `invalid report id` (the existing `reportID` helper).
- `POST /admin/reports/excel` with `{month, year}`: `month` is required and must be an English month name (same rule as `CreateReportRequest`); `year` is required, 2000–2100; otherwise 422 on that field. No reports → 404 `No reports for that month.` More than the cap → 422.
- Errors use the existing `apperror` JSON shape. A success body is the xlsx bytes only.

**Sheets.** Bulk order is church name (case-insensitive), then id. Sheet name: `church_name`, else `worker`, else `Report <id>`. The characters `[]:*?/\` are replaced by `-`, names are cut to 31 characters, and duplicates get ` (2)`, ` (3)`… (cut so the result still fits 31). Every cell value is set as a string or number, never as a formula, so text that starts with `=` stays text.

**Filenames.** Single: `<church_name or Report id> <Month> <Year>.xlsx`. Bulk: `Church Growth Program Report <Month> <Year>.xlsx`. Path separators and control characters are removed.

**Audit (D-10).** One row per successful download. It's written from the trusted session actor, never from the request body. `target_type` is `report`. Single: `target_id` is the report id, `details` is `{month, year, count: 1}`. Bulk: no `target_id`, `details` is `{month, year, count: n}`. No narrative, prayer text or names in `details`.

## Testing

Go unit tests (builder and handlers), testcontainers integration tests (audit row, scope, rollback), Vitest (UI states), and one Playwright test with axe. All are existing runners. The only new dependency is excelize.

## Notes for the AI

- Follow the 22e admin-route pattern (`RequireRole` plus a permission) and `apperror`. Use Go/pgx, not the Prisma or Clerk examples in `coding-standards.md`.
- Build the workbook fully in memory before writing the audit row, so a failed build never leaves an audit row behind.
- Don't open, embed or commit the legacy workbook; the layout table is the source.
- Average formulas need no calculation in Go; Excel calculates them when the file opens.
- No commit or push without explicit approval.

## Resolved questions

Answered by the user on 2026-10-21 (recorded in `docs/DECISIONS.md` D-A6):

1. **Leadership rows:** `leadership` goes on row 21 (Leadership Conference). Row 22 (Leadership Training) keeps its label and stays blank: the database has no separate value for it. Only one legacy sheet ever used row 22.
2. **Baptism:** row 19 is labelled `Baptism` and holds `baptism`.
3. **Person Led To Christ:** row 36 holds `person_evangelized`; `names` goes in row 37.
4. **Bulk size cap:** at most 100 reports per file; above that, 422.
5. **Dashboard and SUMMARY sheets:** left out.
6. **Who can export:** admins only, single and bulk.

## Verification evidence

Ran on 2026-10-22:
- Backend: `make test`, `make test-integration` (testcontainers; includes the new export integration test), `make lint` (0 issues), `make vet` and `make build` all passed.
- Frontend: `npm run lint`, `npm run typecheck`, `npm test` (30 files, 320 tests), `npm run build` and `npm run e2e` (89 tests, including `excel-export.spec.ts` with axe) all passed.
- Running `make swagger` then `npm run gen:api` again produces identical output, and `git diff --check` passes.

Implementation notes and deviations:
- The legacy workbook is ignored by `.gitignore` (`blueprint/reference/*.xlsx`).
- excelize has no switch for printing gridlines, so the sheet uses thin cell borders instead. Portrait orientation and horizontal centering are set.
- A non-numeric id returns 400 `invalid report id`, matching the existing `reportID` helper. The spec was corrected in step 0.
- CORS now exposes `Content-Disposition` as well as `Retry-After`, so a cross-origin frontend can read the filename (`cmd/api/app/echo.go` and its test).
- `api.ts`: the existing `api<T>` flow (refresh on 401, ApiError) moved into a shared `call()`. `downloadFile` and `saveFile` use it. The existing `api.test.ts` tests pass unchanged.
- The first e2e run failed because the export test filed a report in the previous month, and `reports.spec.ts` expects that month to be empty. The export test now uses `earlierPeriod(5)` and soft-deletes its report afterwards. The rerun passed.
- `go.mod`: excelize v2.11.0 is a direct dependency; `go mod tidy` added only its indirect dependencies.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":14215,"specSha256":"5ca51b0ecca3807df43113760ef24e6ce3a581c4759311ce2fbdf26e034cd574","branch":"refs/heads/feature/reports-excel-export","head":"e97cfdaf5e0df75440fbcd0219113a656cdca83d","baseRef":"refs/heads/main","baseCommit":"e97cfdaf5e0df75440fbcd0219113a656cdca83d","sourceTree":"1e4bd10fd2ba54a7b90d1339d7a17ff799630c11","landing":"local-merge","absentOptional":[]} -->

## Independent review

**Status:** passed
**Target commit:** 417c2216a1a0b725fa30ea20d0f67e5657165355
**Base commit:** e97cfdaf5e0df75440fbcd0219113a656cdca83d
**Base ref:** refs/heads/main
**Spec hash:** 5ca51b0ecca3807df43113760ef24e6ce3a581c4759311ce2fbdf26e034cd574
**Prepared by:** claude
**Builder model:** claude-opus-5-5
**Requested reviewer:** claude
**Requested model:** claude-opus-5-5
**Requested execution:** automatic
**Requested at:** 2026-10-21T16:21:45Z
**Workflow:** regular
**Check required:** no
**Reviewer adapter:** claude
**Reviewer model:** claude-opus-5-5
**Reviewer context:** fresh subagent
**Actual execution:** automatic
**Reviewed at:** 2026-10-21T16:28:44Z
**Scope:** current
**Lenses:** quality, security, performance, tests
**Verdict:** passed
**Check result:** not-required

### Handoff

Review the active spec and the complete `<base>..<target>` delta in a fresh
session or isolated subagent without the builder conversation. Run all Audit lenses from scratch.
Run Check when required above. Do not edit product code, accept findings, or
reuse the existing findings as the review scope.

Note: `/complete` already archived this spec in the target commit, so
`current-feature.md` is the reset stub. The verified spec is the first 14215 bytes
of `blueprint/history/features/22-reports-excel-export.md` (SHA-256 above, recorded
in that archive's `blueprint:completion` annotation).

### Commands

- `git rev-parse HEAD` / `git merge-base main HEAD`: pass (417c2216…, e97cfdaf…)
- `head -c 14215 blueprint/history/features/22-reports-excel-export.md | sha256sum`: pass (matches Spec hash)
- `git status --porcelain`: pass (only blueprint/context/review.md and the pre-existing go.work.sum)
- `go test ./internal/report/... ./internal/router/... ./cmd/...` (backend/): pass (319 tests, 6 packages)
- `make lint` (backend/): pass (0 issues)
- `go test -tags integration -run Export -count=1 ./internal/report/reportimpl/` (backend/, Docker): pass (15 tests)
- `go test -overlay <scratch probe> -run TestProbeSheetNames ./internal/report/reportimpl/`: ran (shows the F-05 failures; no repo file added)
- `npx vitest run src/lib "src/app/(app)/admin"` (frontend/): pass (16 files, 194 tests)
- `npm run typecheck` (frontend/): pass

### Evidence

- Authorization: both routes sit in the `/admin` group (`RequireAuth` + `RequireRole(userRepo, "admin")`) with `perm("reports:read_any")` and `limits.Write` (backend/internal/router/router.go:137-139). `admin_reports_routes_test.go` covers 401, a non-admin holding read_any (403), an admin without read_any (403) and 200.
- CSRF: the global `CSRFProtection()` (backend/cmd/api/app/echo.go:42) exempts only GET/HEAD/OPTIONS, so both POSTs are covered. The frontend sends them through `call()`/`send`, which adds the X-CSRF-Token header (frontend/src/lib/api.ts; api.test.ts checks the header).
- Permissions on content: `ExportOne` uses `load(..., "reports:read_any")`, and `ExportMonth` scopes to the caller without read_any (reportimpl/report.go). Only admins reach these routes and admins hold every permission, so leaving D-18 redaction out matches the spec. `owner_status` is not written to the sheet.
- Content-Disposition: `exportFilename` strips controls and path separators. The ASCII fallback replaces non-ASCII, `"` and `;`, and `filename*` uses url.PathEscape, so header injection is not possible (minor RFC 5987 deviation: F-06).
- Formula injection: every text cell uses `SetCellStr` and numbers use `SetCellFloat`. Only the fixed G-column AVERAGE uses `SetCellFormula`. `TestBuildWorkbook_Layout` checks that `=HYPERLINK(...)` stays a string.
- Audit: the workbook is fully serialized in `workbookBytes` before `audit.Insert`, and an insert failure returns an error, so no file is sent. Details are `{month, year, count}` from the session actor. Bulk sets `TargetID` to nil. Unit and integration tests cover the counts, deceased or deleted owners being included, soft-deleted reports being excluded, and no row on failure. A row is written before the response bytes go out, so a client disconnect after the insert still leaves a row (accepted, as with other writes).
- Size bounds: `List` is called with `Limit: MaxExportReports` and `total > 100` → 422. "100" means non-soft-deleted reports for the month. Text fields are capped by validation (max 10000), so a 100-sheet workbook stays small and in memory. Tests cover exactly 100 passing and 101 failing.
- Sheet names: invalid characters are replaced, names are cut to 31 and deduplicated case-insensitively with ` (n)`, but non-BMP and trailing-quote cases fail (F-05).
- Frontend: a per-key busy Set, `aria-busy` and "Downloading…"; Bulk is disabled on an empty list; the error toast uses the server message via `retryText`; `saveFile` revokes the object URL after 1 s (untested: F-07).

### Findings

- F-05 [P2] open: excel.go:219-239, sheet-name truncation by runes, and a trailing `'` after the cut, make the export fail (500, whole month for bulk)
- F-06 [P3] open: report_handler.go:284, `filename*` leaves `= : @ & + $` unescaped
- F-07 [P3] open: api.ts:150-157, saveFile's object URL lifecycle has no unit test

### Remaining risk

- The spec was verified from the archive prefix (first 14215 bytes of blueprint/history/features/22-reports-excel-export.md), because `current-feature.md` is the reset stub after `/complete`.
- `go.work.sum` has a pre-existing uncommitted change outside this work item; it was not reviewed.
- Not run in this review: the full `make test`, `npm test`, `npm run lint`, `npm run build`, `npm run e2e` (Playwright and axe), and the swagger/gen:api drift check. The spec's verification evidence reports that they passed.
- Not verified: whether desktop Excel opens these files without a repair prompt, including a sheet named "History" (reserved in Excel; excelize accepts it).
