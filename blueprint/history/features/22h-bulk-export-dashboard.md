# Feature: Bulk export dashboard and approved-only sheets

**From build-plan:** feature 22h
**Build attempt:** 1
**Branch:** feature/bulk-export-dashboard-and-approved-only-sheets
**Status:** verified

## Goal

The bulk Excel export (`POST /admin/reports/excel`) and its preview (`POST /admin/reports/excel/preview` with `{month, year}`) change in two ways (D-A16):

- They start with a "Report Dashboard" sheet. It lists every church account with its status for the selected month: Approved, Pending or Not Filed.
- After it come metric sheets for **approved** reports only, in the existing D-A6 layout. Pending reports get no sheet.

The single-report export and the single-report preview are unchanged.

## In scope

- **Dashboard rows:** one per account that has a non-blank `church_name`, is not soft-deleted, and whose role is not `admin`. Inactive, disciplinary and deceased accounts are included. Columns: Church (`users.church_name`), Worker (`users.name`), Area (`users.area_assignment`), Status. Rows are ordered by church name (case-insensitive), then account id.
- **Status:** for the selected month and year, using the account's one active (not soft-deleted) report:
  - "Approved" if `completed`
  - "Pending" if it exists and isn't `completed`
  - "Not Filed" if there is none
- **Metric sheets:** approved, not-soft-deleted reports of the month only, in the existing order (church name case-insensitive, then id) and layout. Admins' and deleted accounts' approved reports keep their sheet.
- **No approved reports:** the file has the dashboard sheet alone and returns 200, for both the export and the preview. The old 404 "No reports for that month." goes away for bulk.
- **Cap and audit:** the cap is more than 100 *approved* reports, which gives a 422. The `DOWNLOAD_BULK_EXCEL` audit `count` is the number of approved sheets, which can be 0.
- **Name clash:** a church sheet named "Report Dashboard" (any letter case) gets " (2)" through the existing duplicate rule.
- **Preview:** the dashboard sheet comes first. `count` is the number of approved metric sheets. The modal shows the dashboard and the approved sheets, and labels the count "N approved".
- **Docs:** Swagger descriptions, `npm run gen:api`, `docs/API.md`, and the D-A16 status.

## Out of scope

- A churches table or seed list (D-A16: the account is the church).
- The single-report export and preview: no dashboard, and pending reports still export.
- The Download Bulk button's disabled rule (still disabled when the month's list is empty).
- The template's "SUMMARY" sheet, styling beyond a bold header and borders, and the open findings F-01 to F-10.

## Build loop

- `workflow.stepReview: feature`: build every step, then stop once for review at the end.
- `workflow.checkpointCommits: disabled`: no commits during the build. `/complete` makes the feature commit.
- Each step leaves `make test` and `make lint` (backend) and `npm test` and `npm run typecheck` (frontend) passing for the code it touches.

## Build steps

- [x] 1. **Store: approved filter and dashboard rows.**
  - In `backend/internal/report/model.go`, add `Completed *bool` to `ListReportsParams` (nil means any) and a `DashboardRow{Church, Worker, Area string; Status string}` type, with `Status*` constants for "Approved", "Pending" and "Not Filed".
  - In `report.go`, add `Dashboard(ctx, month string, year int) ([]DashboardRow, error)` to `Store`.
  - In `reportimpl/store.go`: `List` adds `r.completed = ?` when `Completed` is set. `Dashboard` is one query: `users u LEFT JOIN roles ro ON ro.id = u.role_id LEFT JOIN reports r ON r.created_by = u.id AND r.deleted_at IS NULL AND r.year = ? AND lower(r.month) = lower(?)`, where `u.deleted_at IS NULL`, `btrim(coalesce(u.church_name, '')) <> ''` and `ro.name IS DISTINCT FROM 'admin'`, ordered by `lower(u.church_name), u.id`. The status comes from whether `r.id` is null and from `r.completed`. `idx_reports_unique_per_user` guarantees at most one report per row.
  - Mirror both in `reportimpl/mock.go`. The mock needs a way to seed accounts; check how `mock.go` sees users and add the smallest seam.

  Done when: `store_integration_test.go` covers each of these, and `make test-integration` passes:
  - `Completed` filtering
  - each status
  - admin exclusion
  - a deleted-account exclusion
  - an inactive or deceased account included
  - a blank `church_name` excluded
  - a soft-deleted report counted as Not Filed
  - another month's report ignored
  - the order
- [x] 2. **Workbook: dashboard sheet.** In `reportimpl/excel.go`, add `buildBulkWorkbook(rows []report.DashboardRow, reps []*report.Report)`:
  - Sheet 1 is "Report Dashboard": header row Church, Worker, Area, Status (bold, thin borders), one row per `DashboardRow` (thin borders), and sensible column widths. Every value is written with `SetCellStr`, so text starting with "=" stays text.
  - Then the metric sheets via the existing `writeSheet`.
  - `sheetNames` gains a reserved-names input, so "Report Dashboard" is taken before the reports are named. `buildWorkbook` (single) keeps its behavior.

  Done when: `excel_test.go` asserts these, and `make test` passes:
  - the sheet order
  - the dashboard header and rows
  - a formula-like church name stays text
  - a church named "report dashboard" becomes "report dashboard (2)"
  - an empty `reps` gives one sheet
  - `TestSheetNames` still passes
- [x] 3. **Preview reads variable sheets.** `previewSheets` reads A1:G42 for metric sheets. For "Report Dashboard" it reads A1:D(n+1), the header plus one row per account, so it doesn't truncate or pad. `PreviewSheet.Rows` is documented as "rows as shown; a metric sheet is always 42 × 7, the dashboard is 4 columns".
  Done when: a `TestPreviewSheets` case covers a dashboard with 0 and with 3 rows next to a metric sheet. `make test` passes.
- [x] 4. **Service.**
  - `monthForExport` lists with `Completed: ptr(true)`. `total > MaxExportReports` returns `ErrTooManyReports`. `total == 0` is no longer an error. It also returns `s.reports.Dashboard(ctx, month, year)`.
  - `ExportMonth` and `Preview` (month shape) use `buildBulkWorkbook`. `ExportMonth`'s audit `count` is `len(reps)`. `Preview.Count` is `len(reps)`.
  - Remove `ErrNoReports` and its handler mapping if nothing else uses it (today only the bulk path does).
  - `ExportOne` and the single preview are unchanged.

  Done when: `report_test.go` updates the bulk export and preview tests and checks these. Wrap each preview call in its own audit-count check, so the F-09 gap isn't repeated.
  - pending excluded
  - the dashboard first, with the right statuses
  - zero approved gives the dashboard only, a 200 path, and an audit `count` of 0 for the export
  - the cap counts approved reports only (101 approved gives 422; 100 approved plus pending passes)
  - the preview writes no audit row
- [x] 5. **Handler, Swagger, types, docs.**
  - Update the `ExportReports` and `PreviewExcel` godoc descriptions (no 404 for an empty month), then `make swagger` and `npm run gen:api`.
  - Update the bulk export and preview sections of `docs/API.md`.
  - Update handler tests that expected `ErrNoReports`.

  Done when: `make test`, `make lint` and `npm run typecheck` pass, and `api-types.ts` matches.
- [x] 6. **Preview modal.** In `excel-preview-dialog.tsx`:
  - Column headers come from each sheet's row width (A to D for the dashboard, A to G for metric sheets), not the fixed 7.
  - Bulk shows "N approved" ("1 approved" for one).
  - "Report Dashboard" is the first option in the Sheet select and is shown on open.
  - With 0 approved, the select has the dashboard only and Download stays enabled.

  Done when: `excel-preview-dialog.test.tsx` covers the dashboard columns, the label, and the 0-approved case. `admin-reports.test.tsx` still passes. `npm test`, `npm run lint` and `npm run typecheck` pass.
- [x] 7. **End to end and decision.**
  - In `frontend/e2e/excel-export.spec.ts`, approve the filed report through `PATCH /reports/:id/complete` before previewing.
  - Bulk preview: the selected sheet is "Report Dashboard". It shows the `e2e-user` account's church "Visionaires Christian Church" as "Not Filed" for that month. Selecting "E2E Export Church" shows its metric sheet.
  - Download still gives the bulk file. Axe passes on the open modal.
  - Mark D-A16 as built in 22h in `docs/DECISIONS.md`.

  Done when: `npm run e2e -- excel-export` passes, then the full `npm run e2e`.

## Files / areas

- `backend/internal/report/model.go`, `report.go`: `ListReportsParams.Completed`, `DashboardRow`, `Store.Dashboard`, and possibly removing `ErrNoReports`.
- `backend/internal/report/reportimpl/store.go` (+ `store_integration_test.go`), `mock.go`.
- `backend/internal/report/reportimpl/excel.go` (+ `excel_test.go`), `report.go` (+ `report_test.go`, `export_integration_test.go` if it asserts the bulk contents).
- `backend/internal/report/rest/report_handler.go` (+ test). `backend/docs/*` and `frontend/src/lib/api-types.ts` (generated). `docs/API.md`, `docs/DECISIONS.md`.
- `frontend/src/app/(app)/admin/excel-preview-dialog.tsx` (+ test), `frontend/e2e/excel-export.spec.ts`.

## Data / contracts

- **No schema change and no migration.** The dashboard reads `users`, `roles` and `reports` only.
- **`POST /admin/reports/excel` and `POST /admin/reports/excel/preview` (`{month, year}`)** keep their request shape, gate, limiter and audit behavior. Their response contracts change:
  - **Workbook:** sheet 1 is "Report Dashboard". Then one sheet per approved report, in the D-A6 layout and existing order. The file name is unchanged.
  - **Preview `sheets[0]`:** `{name: "Report Dashboard", rows: [["Church","Worker","Area","Status"], ...]}`, one row per account. Metric sheets stay 42 × 7.
  - **Preview `count`:** the number of approved metric sheets (0 or more).
  - **Errors:** 422 for more than 100 approved reports. A month with no reports, or no approved ones, returns 200 with the dashboard only.
- **Status strings** are exactly `Approved`, `Pending` and `Not Filed`.
- **Single shapes** (`POST /admin/reports/:id/excel`, preview `{report_id}`) are unchanged.

## Testing

- **Backend unit:** `make test`, covering steps 2 to 5.
- **Backend integration:** `make test-integration` for the store query in step 1. It needs Docker; if Docker isn't available, report it as not run.
- **Frontend:** `npm test`, `npm run lint`, `npm run typecheck`.
- **End to end:** `npm run e2e`, which needs the local stack.

## Notes for the AI

- The dashboard holds names and church data only, no free text or tithes. The endpoints keep `reports:read_any` in the admin group, `no-store` on the preview, and no audit on the preview.
- The dashboard query must not filter by user status (D-A16 includes inactive, disciplinary and deceased accounts). It only filters out deleted and admin accounts.
- Keep D-A6's layout code as the single source for metric sheets. The dashboard is a separate writer and the preview reads the built workbook.
- Render preview cells as text nodes only, as in 22f.

## Open questions

None. The owner settled D-A16 on 2026-10-22.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":11128,"specSha256":"8c90ffab155a2547c531dbd2f7d59a1f7c9e03e526c2afbd29e2e2df4be7581e","branch":"refs/heads/feature/bulk-export-dashboard-and-approved-only-sheets","head":"08d62b0cfebb760e73f6466488863840e00a946e","baseRef":"refs/heads/main","baseCommit":"e731282b5cb11890af285bd14e4ab1443f6581b5","sourceTree":"c64cda4f52d6c33783d18cf8074eea06167f0ebd","landing":"local-merge","absentOptional":[]} -->

## Findings

### 22h/F-09 [P2] closed - Preview's no-audit assertion skips every successful preview

**File:** backend/internal/report/reportimpl/report_test.go:584
**Found:** 2026-10-22 by /audit independent (scope: current; lens: tests)
**Why it matters:** Spec step 3 says the audit mock records zero entries in every case. `TestReportService_Preview` takes `before` at the start, then resets it with `before = len(f.audit.Entries())` inside the "missing or deleted" subtest. The reset is needed because that subtest's `Create` and `Delete` write audit rows, but it also hides any row written by the "one report" and "a month" previews that ran first. The final check at line 601 therefore covers only the empty-month and over-cap error paths, which return before any audit code could run. Today `Preview` never touches `s.audit`, so the code is correct. The test just wouldn't catch a regression that adds an audit write to the success path, which is the D-A15 contract.
**Suggested fix:** Assert `assert.Len(t, f.audit.Entries(), n)` around each `Preview` call (or snapshot before and after the call in each subtest) instead of using one shared `before`.
**Resolution:** Closed 2026-10-22 by /audit independent (22h, target 08d62b0). `TestReportService_Preview` now routes every call through a `preview` helper that snapshots `len(f.audit.Entries())` immediately before and asserts it is unchanged immediately after, and each subtest builds its own fixture, so the success paths (single report, month, empty month) are each checked. No shared `before` remains. `make test` passes.

## Independent review

**Status:** passed
**Target commit:** 08d62b0cfebb760e73f6466488863840e00a946e
**Base commit:** e731282b5cb11890af285bd14e4ab1443f6581b5
**Base ref:** main
**Spec hash:** 8c90ffab155a2547c531dbd2f7d59a1f7c9e03e526c2afbd29e2e2df4be7581e
**Prepared by:** claude
**Builder model:** claude-opus-5-5
**Requested reviewer:** claude
**Requested model:** claude-opus-5-5
**Requested execution:** automatic
**Requested at:** 2026-10-22T02:53:29Z
**Workflow:** regular
**Check required:** no
**Reviewer adapter:** claude
**Reviewer model:** claude-opus-5-5
**Reviewer context:** fresh subagent
**Actual execution:** automatic
**Reviewed at:** 2026-10-22T02:57:05Z
**Scope:** current
**Lenses:** quality, security, performance, tests
**Verdict:** passed
**Check result:** not-required

### Handoff

Review the active spec and the complete `e731282b5cb11890af285bd14e4ab1443f6581b5..08d62b0cfebb760e73f6466488863840e00a946e` delta in a fresh
session or isolated subagent without the builder conversation. Run all Audit lenses from scratch.
Run Check when required above. Do not edit product code, accept findings, or
reuse the existing findings as the review scope.

### Commands

- `git rev-parse HEAD`, `git merge-base main HEAD`, `sha256sum blueprint/context/current-feature.md`, `git status --porcelain --untracked-files=all`: pass (HEAD, merge base and spec hash match the request; only review.md differs)
- `cd backend && make test`: pass
- `cd backend && make lint`: pass (0 issues)
- `cd backend && go test -tags=integration -race -count=1 ./internal/report/...`: pass (189 tests, 3 packages; Docker/testcontainers)
- `cd frontend && npm test`: pass (31 files, 331 tests)
- `cd frontend && npm run typecheck`: pass
- `cd frontend && npm run lint`: pass
- `cd frontend && npm run e2e`: unavailable (needs the local stack; not started)
- `make swagger` / `npm run gen:api` drift check: unavailable (would write to the tree; checked by inspection only)

### Evidence

- Full 26-file delta e731282..08d62b0 read against the spec: store `Dashboard` query and `List` `Completed` filter (store.go:177-357), `renderWorkbook`/`writeDashboard`/`dashboardRows`/`sheetNames(reserved, ...)` (excel.go), `monthForExport`/`ExportMonth`/`Preview` (report.go), handler error map and godoc, mock seam, dialog column/label change, e2e approve step, API.md and D-A16.
- Dashboard query matches the spec: users LEFT JOIN roles and the month's active report, deleted/blank-church/admin accounts excluded, no status filter, ordered by `lower(church_name), id`; `church_name`/`name` scans are safe (non-null by WHERE / NOT NULL). Month values are validated `oneof` capitalized, so the case-sensitive unique index keeps one report per row.
- All dashboard cells use `SetCellStr` (formula-like church test asserts no formula); preview renders text nodes only; route gate, `no-store` and no-audit on preview unchanged.
- Cap counts approved only (`Completed: &true` in the DB count); audit `count` and preview `Count` are `len(reps)`, 0 allowed; `ErrNoReports` fully removed (no remaining references).
- Dashboard query is one round trip over ~24 accounts; no new N+1 or unbounded work.

### Findings

- F-11 [P3] open: dashboard order test can't detect a missing `lower()` or the id tiebreak (new)
- F-09 [P2] closed: preview no-audit assertion now wraps every call (re-examined in this delta)

### Remaining risk

- `npm run e2e` not run (needs the local stack), so the Playwright dashboard/approve flow and axe check on the modal are unverified here.
- Swagger/api-types drift not regenerated; `swagger.json` and `api-types.ts` both carry the new descriptions, and CI checks drift.
- Pre-existing F-08 (Unicode case-fold sheet-name merge) also applies to the reserved "Report Dashboard" name; still P3 and out of scope per the spec.

## Try it now

Run the stack locally (`cd backend && make dev`, `cd frontend && npm run dev`) and sign in as an admin.

1. Open **Admin**, then the **Reports** tab, and pick a month where at least one report is approved and one is still pending.
2. Click **Download Bulk**. The preview opens on **Report Dashboard**: columns A to D (Church, Worker, Area, Status), one row per church account, each marked Approved, Pending or Not Filed. The count reads "N approved".
3. Open **Sheet**. "Report Dashboard" is first, followed only by the approved churches; the pending church has no sheet.
4. Click **Download**. The `.xlsx` opens with "Report Dashboard" as its first tab and the same approved sheets after it.
5. Pick a month with reports but none approved and click **Download Bulk** again: the preview shows "0 approved" and the dashboard alone, and Download still works (no more "No reports for that month.").
6. A row's **Excel** button still exports that one report with no dashboard, even when it is pending.
