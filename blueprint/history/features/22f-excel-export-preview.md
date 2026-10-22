# Feature: Excel export preview

**From build-plan:** feature 22f
**Build attempt:** 1
**Branch:** feature/excel-export-preview
**Status:** verified

## Goal

Before an admin downloads an Excel file, show what it will contain. The row "Excel" and the "Download Bulk" buttons open a modal with the workbook's sheets as read-only tables. The data comes from a new `POST /admin/reports/excel/preview`, which builds the same workbook as the export and writes no audit row. Only the modal's "Download" calls the existing export, which builds the file again and writes the `DOWNLOAD_*_EXCEL` audit row. "Cancel" closes the modal with no download and no audit row (D-A15).

## In scope

- **Backend:** admin-only `POST /admin/reports/excel/preview` for one report (`report_id`) or one month (`month` + `year`). It returns each sheet's name and cell text as JSON and writes no audit row. The 100-report cap (D-A6) applies.
- **Shared loading:** the export and the preview pick and order reports with the same code, so the sheet names, sheet order and contents match.
- **Frontend:** the "Excel" (row) and "Download Bulk" buttons open a preview modal. It has loading, error, table and Download/Cancel states. For bulk it adds a sheet selector and shows the report count.
- **Docs and types:** Swagger, `npm run gen:api`, `docs/API.md`, and the D-A15 status in `docs/DECISIONS.md`.

## Out of scope

- 20e report preview modal (D-A5): separate feature.
- Cell styling, merged-cell spans, column widths and number formats beyond the displayed text. The preview is plain text, not a rendering of the spreadsheet.
- Preview tokens or a cached workbook: "Download" builds the file again (D-A15). A report edited after the preview opened gives a file that differs from the preview. That's accepted.
- Changes to the export endpoints, their audit rows or their file names.
- Email (E-1), Phase B.

## Build loop

- `workflow.stepReview: feature`: build all steps, then stop for one review at the end of the feature, not after each step.
- `workflow.checkpointCommits: disabled`: no commits during the build. `/complete` makes the feature commit.
- Each step leaves `make test` (backend) and `npm test` / `npm run typecheck` (frontend) passing for the code it touches.

## Build steps

- [x] 1. **Share report selection between export and preview.** In `backend/internal/report/reportimpl/report.go`, move ExportOne's report load (actor, `load` with `reports:read_any`) into `oneForExport`. Move ExportMonth's list, count checks (`ErrNoReports`, `ErrTooManyReports`) and church-name sort into `monthForExport`. ExportOne and ExportMonth call these helpers. Their behavior, audit rows and file names stay the same.
  Done when: existing export unit and integration tests pass unchanged (`make test`, and `make test-integration` for `export_integration_test.go`).
- [x] 2. **Preview builder.** In `backend/internal/report/reportimpl/excel.go`, add `previewSheets(f *excelize.File) ([]report.PreviewSheet, error)`. For every sheet in `f.GetSheetList()` order, it reads the A1:G42 grid as rows of strings: the cell text as excelize displays it, with the cell's number format applied. Column G's `IFERROR(AVERAGE(...),"")` formulas aren't cached in a new workbook, so compute them with `CalcCellValue`. A blank week row gives `""`.
  Check before coding: `GetCellValue` formats B4 as `mmmm yyyy` (for example `October 2026`) and row 27 as `#,##0.00`, and `CalcCellValue` returns the average, not the formula text. If either is not true, format the value in code to match the cell's format, and record that in Notes.
  Done when: `excel_test.go` builds a workbook from a fixture report and asserts: the sheet name; A1 is the title; B4 is the month text; a metric row's week cells and its G average; tithes shows money format; a missing week is `""`; report text starting with `=` stays literal text.
- [x] 3. **Service and contract.** In `backend/internal/report/model.go`, add `ExcelPreviewRequest`, `ExcelPreview` and `PreviewSheet` (see Data / contracts). In `report.go`, add `Preview(ctx, userID, req) (*ExcelPreview, error)` to the `Service` interface. Implement it with the step 1 helpers plus `buildWorkbook` and `previewSheets`. It doesn't take `ip` and never calls the audit store. Close the workbook after reading.
  Done when: `report_test.go` covers single, bulk (sheet order matches `sheetNames`, `count` is right), 404 for a missing or deleted report, 404 for an empty month, 422 for more than 100, and no `audit.Insert` call. The audit mock records zero entries in every case.
- [x] 4. **Handler and route.** In `backend/internal/report/rest/report_handler.go`, add `PreviewExcel`: bind, validate, call `svc.Preview`, then `c.JSON(200, …)`. Set `Cache-Control: no-store`, because the body holds free text and tithes. Write swaggo annotations in the same style as ExportReports. In `backend/internal/router/router.go`, add `admin.POST("/reports/excel/preview", reportHandler.PreviewExcel, perm("reports:read_any"), limits.Read)` next to the Excel downloads.
  Done when: handler tests cover 200 for both shapes, 400 for a malformed body, and 422 for each invalid shape. `admin_reports_routes_test.go` covers 401 signed out, 403 for a non-admin, and 403 for an admin without `reports:read_any`. It also shows that `/reports/excel/preview` reaches the preview handler, not `/reports/:id/excel`. `make test` and `make lint` pass.
- [x] 5. **Swagger and types.** Run `make swagger`, then `npm run gen:api`. Add the endpoint to `docs/API.md`.
  Done when: `api-types.ts` has the new path and schemas, and `npm run typecheck` passes.
- [x] 6. **Preview modal.** In `frontend/src/lib/reports.ts`, add `previewExcel(body)`, which calls `api<ExcelPreview>("/admin/reports/excel/preview", { method: "POST", body })`. Add `frontend/src/app/(app)/admin/excel-preview-dialog.tsx`, built on the house `AlertDialog` (the same pattern as `reset-password-section.tsx`). Its props are a target (`{ reportId, label }` or `{ month, year }`), `onDownload` and `onOpenChange`. Fetch with `useQuery` (key includes the target, `gcTime: 0`, so every opening refetches). States:
  - **loading:** "Loading preview…" in a `role="status"` region. Download is disabled.
  - **error:** the `retryText(err)` message in `role="alert"`. Download is disabled, Cancel is enabled.
  - **loaded:** a `Table` per sheet in a scrollable box, with every cell rendered as React text (escaped, never HTML).
  - **bulk:** a `NativeSelect` labeled "Sheet" lists sheet names and shows one sheet at a time, with "N reports" beside it.
  "Download" calls `onDownload` and shows "Downloading…" with `aria-busy`. On success the modal closes. On failure it stays open with the existing error toast. "Cancel" and Escape close the modal with no request. Focus returns to the button that opened it (Radix default).
  Done when: a Vitest file `excel-preview-dialog.test.tsx` covers loading, error, single table, bulk selector switching sheets, Download calling `onDownload` once, and Cancel not calling it.
- [x] 7. **Wire the buttons.** In `admin-reports.tsx`, the row "Excel" and "Download Bulk" buttons now open the modal for their target instead of downloading. The modal's `onDownload` reuses the existing `download(key, run)` with `exportReport` / `exportReports`. Button labels and disabled rules stay the same (bulk disabled with no rows).
  Done when: `admin-reports.test.tsx` shows that clicking either button opens the modal and makes no export call. Download triggers exactly one export call. Cancel triggers none. `npm test`, `npm run lint` and `npm run typecheck` pass.
- [x] 8. **End to end and docs.** Update `frontend/e2e/excel-export.spec.ts`:
  - Bulk: click "Download Bulk", see the modal with the sheet selector and the filed report's church in the table, click "Download", and get the same file name as today.
  - Single: click the row "Excel", click "Cancel", and expect no download event and the modal closed. Open it again, click "Download", and get the single file.
  - Run axe on the open modal.
  Mark D-A15 as built in 22f in `docs/DECISIONS.md` (the status column and the log entry).
  Done when: `npm run e2e -- excel-export` passes.

## Files / areas

- `backend/internal/report/model.go`: request and response types.
- `backend/internal/report/report.go`: `Service.Preview`.
- `backend/internal/report/reportimpl/report.go` (+ `report_test.go`): shared selection and `Preview`.
- `backend/internal/report/reportimpl/excel.go` (+ `excel_test.go`): `previewSheets`.
- `backend/internal/report/rest/report_handler.go` (+ test): `PreviewExcel`.
- `backend/internal/router/router.go`, `admin_reports_routes_test.go`: the route.
- `backend/docs/*` (generated), `frontend/src/lib/api-types.ts` (generated), `docs/API.md`, `docs/DECISIONS.md`.
- `frontend/src/lib/reports.ts`: `previewExcel`.
- `frontend/src/app/(app)/admin/excel-preview-dialog.tsx` (+ test), `admin-reports.tsx` (+ test).
- `frontend/e2e/excel-export.spec.ts`.

## Data / contracts

`POST /admin/reports/excel/preview`. It requires the admin role group and `reports:read_any`, goes through the CSRF check like the other admin POSTs, and uses the `limits.Read` limiter. It writes no audit row and returns `Cache-Control: no-store`.

Request `ExcelPreviewRequest`, exactly one shape:

```json
{ "report_id": 42 }
{ "month": "October", "year": 2026 }
```

- `report_id`: int64, > 0. Not allowed together with `month`/`year`.
- `month`: the same `oneof` list as `ExportReportsRequest`. `year`: 2000..2100. Each requires the other.
- Neither shape, or both: 422 with field errors through the existing validator.

Response 200 `ExcelPreview`:

```json
{ "count": 2, "sheets": [ { "name": "Grace Church", "rows": [["ANG MANANAMPALATAYANG GUMAWA", "", "", "", "", "", ""], ...] } ] }
```

- `count`: the number of reports (= the number of sheets).
- `sheets`: in workbook order, which is the export's order. `name` is the exact Excel sheet name (`sheetNames`).
- `rows`: always 42 rows × 7 strings (A1:G42). A merged range's text is in its top-left cell only.

Errors, the same mapping as the export (`reportError`): 400 malformed body, 401, 403, 404 (report missing/deleted, or `ErrNoReports`), 422 (validation, `ErrTooManyReports`), 500.

## Testing

- Backend unit: `make test` (steps 1–4). Backend integration: `make test-integration` for the shared-selection refactor (step 1). This needs Docker. If Docker isn't available, report it as not run.
- Frontend: `npm test`, `npm run lint`, `npm run typecheck`. E2E: `npm run e2e -- excel-export` (needs the local stack).
- No audit: asserted in the service tests (step 3) through the mock audit store.

## Notes for the AI

- Admins already see the full report, free text included (D-18 limits only non-owners without admin rights). The preview carries the same data as the file, so it has the same gate (`reports:read_any` in the admin group) and `no-store`.
- `limits.Read` was chosen because the preview has no side effects. Every download is preceded by a preview, so putting it on `limits.Write` would halve the export budget.
- Render cells only as text nodes. Report text can contain `<`, `=` and similar characters.
- Echo matches the static `/reports/excel/preview` path before `/reports/:id/excel`. The route test in step 4 is there to prove it.
- Keep the D-A6 layout code as the single source: the preview reads the built workbook and never duplicates the layout.

## Open questions

None.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":11483,"specSha256":"674af5aeb786ebceeb64d71eb6adbcddb6946130e017fcee6d9e003a8eb62f7e","branch":"refs/heads/feature/excel-export-preview","head":"fb9821cf996a0de81ecf2d177d371e340dc6cea5","baseRef":"refs/heads/main","baseCommit":"b14f2cf59c82953a08ad811c28a8729bf88300fe","sourceTree":"12e8b623459a05b994620e52986af65b4331ebf6","landing":"local-merge","absentOptional":[]} -->

## Independent review

**Status:** passed
**Target commit:** fb9821cf996a0de81ecf2d177d371e340dc6cea5
**Base commit:** b14f2cf59c82953a08ad811c28a8729bf88300fe
**Base ref:** refs/heads/main
**Spec hash:** 674af5aeb786ebceeb64d71eb6adbcddb6946130e017fcee6d9e003a8eb62f7e
**Prepared by:** claude
**Builder model:** claude-opus-5-5
**Requested reviewer:** claude
**Requested model:** runtime default (exact model not known until reviewer starts)
**Requested execution:** automatic
**Requested at:** 2026-10-22T02:00:29Z
**Workflow:** regular
**Check required:** no
**Reviewer adapter:** claude
**Reviewer model:** claude-opus-5-5
**Reviewer context:** fresh subagent
**Actual execution:** automatic
**Reviewed at:** 2026-10-22T02:06:03Z
**Scope:** current
**Lenses:** quality, security, performance, tests
**Verdict:** passed
**Check result:** not-required

### Handoff

Review the active spec and the complete `b14f2cf59c82953a08ad811c28a8729bf88300fe..fb9821cf996a0de81ecf2d177d371e340dc6cea5` delta in a fresh
session or isolated subagent without the builder conversation. Run all Audit lenses from scratch.
Run Check when required above. Do not edit product code, accept findings, or
reuse the existing findings as the review scope.

### Commands

- `git rev-parse HEAD`: pass (equals Target commit)
- `git merge-base refs/heads/main HEAD`: pass (equals Base commit)
- `sha256sum blueprint/context/current-feature.md`: pass (equals Spec hash)
- `git status --short`: pass (only blueprint/context/review.md modified)
- `cd backend && make test`: pass
- `cd backend && make lint`: pass (0 issues)
- `cd backend && make test-integration`: pass (Docker available; includes reportimpl and router)
- `cd frontend && npm test`: pass (31 files, 329 tests)
- `cd frontend && npm run lint`: pass
- `cd frontend && npm run typecheck`: pass
- `cd frontend && npm run e2e -- excel-export`: unavailable (not run; needs the local stack)

### Evidence

- Reviewed all 26 files in `b14f2cf..fb9821c`: model, Service.Preview, the oneForExport/monthForExport refactor, previewSheets/mergedTails, PreviewExcel handler, router, Swagger/api-types, admin-reports.tsx, excel-preview-dialog.tsx, reports.ts, unit, route, Vitest and e2e tests, docs/API.md, docs/DECISIONS.md.
- Security: the route sits in the admin group with `perm("reports:read_any")`, goes through the CSRF check like the other admin POSTs, and sets `Cache-Control: no-store`. Preview never references `s.audit`. Cells render as React text nodes only (Vitest checks `<b>` stays literal). Formula-like report text is written with SetCellStr and stays literal (excel_test B40).
- Validation: `ExcelPreviewRequest` tags reject neither, both, mixed (`report_id`+`year`), bad id, month, and year shapes. The handler test covers 7 invalid shapes with 422.
- Shared selection: ExportOne/ExportMonth and Preview use the same helpers. Export behavior is unchanged (existing unit and integration export tests pass).
- Layout: previewSheets reads the built workbook (no duplicated layout), A1:G42 covers every written cell, and the merged tails are blanked. Month and money formats and the computed G averages are asserted in TestPreviewSheets.
- Frontend: the dialog mounts only while open, Download stays disabled until the data loads and while it downloads, a failed download keeps the dialog open, and the existing `download()` toast path is reused.

### Findings

- F-09 [P2] open: Preview's no-audit assertion skips every successful preview
- F-10 [P3] open: The dialog test doesn't show that Escape alone closes it

### Remaining risk

- `npm run e2e -- excel-export` was not run (needs the local stack), so the axe check on the open modal and the real download and cancel flow aren't verified by this review.
- `make swagger` / `npm run gen:api` drift wasn't regenerated (the reviewer can't write those files). CI's drift check is the guard. `api-types.ts` contains the new path.
- Performance unverified: a 100-sheet preview does about 29k cell reads plus 2.2k `CalcCellValue` calls under the more lenient `limits.Read` limiter. That was a deliberate spec choice. It wasn't profiled.

## Try it now

Run the stack locally (`cd backend && make dev`, `cd frontend && npm run dev`) and sign in as an admin.

1. Open **Admin**, then the **Reports** tab, and pick a month that has reports.
2. Click **Download Bulk**. A modal titled "Excel preview: <Month> <Year>" opens and nothing downloads yet. Use **Sheet** to switch between churches; the count beside it matches the report count. The month shows as text ("October 2026"), tithes as money ("1,810.50"), and column G holds the averages.
3. Click **Cancel**. The modal closes, no file downloads, and focus returns to Download Bulk.
4. Click a row's **Excel** button, then **Download**. The button shows "Downloading…", the file `<Church> <Month> <Year>.xlsx` downloads, and the modal closes.
5. Optional: in the database, `audit_logs` has one `DOWNLOAD_SINGLE_EXCEL` row for step 4 and none for the previews or the cancel.
