# Fix: Report Dashboard remarks for approved reports

**Type:** Fix
**Status:** implemented, awaiting owner check
**Amends:** D-A19 (`blueprint/history/fixes/report-dashboard-with-month-and-year-columns.md`)

## Problem

The owner reviewed the bulk export against `blueprint/reference/image.png` (2026-10-22) and asked for three changes to the Report Dashboard sheet:

1. Approved goes back to blue `#0000FF`, not green `#00AA00`.
2. An Approved row fills the remark columns: "Checked" in Coordinator's Remark and "Ready for Pick Up" in Support Status. Pending and Not Filed rows leave them blank.
3. The filled remark cells use the reference's style: blue fill `#0000CC`, white text, centered; both bold (owner, 2026-10-22).

## Change

- `writeDashboard` (`backend/internal/report/reportimpl/excel.go`): `dashApproved` fill is `0000FF`; new `dashRemarkSet` style; Approved rows write the two remark strings. The values derive from the status, so there is no store, model, API type or migration change.
- The preview reads the sheet back, so it shows the remarks with no extra code.

## Unchanged

- Only approved reports get a metric sheet (D-A16); pending and not-filed churches appear on the dashboard alone, with blank remarks.

## Tasks

- [x] Styles and values in `writeDashboard`.
- [x] `excel_test.go`: blue Approved, `#0000CC` remark fills, bold "Checked" and "Ready for Pick Up", blank unfilled remarks on Pending and Not Filed rows, preview rows.
- [x] `report_test.go` dashboard fixture and the frontend preview dialog fixture.
- [x] D-A19 and `docs/API.md`.

## Verify

- `cd backend && make test && make lint` (done, 0 issues).
- **By hand:** bulk-export a month with one approved and one pending report, open it in LibreOffice and compare with `blueprint/reference/image.png`.
