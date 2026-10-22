# Fix: Report Dashboard with Month and Year columns

**Type:** Fix
**Status:** verified
**Branch:** fix/report-dashboard-with-month-and-year-columns

## The problem

The owner's new reference, `blueprint/reference/image.png` (replaced 2026-10-22 13:12, ignored in Git), shows a six-column "Report Dashboard": Churches, Month, Year, Status, Coordinator's Remark, Support Status. Today's dashboard (D-A17) has four columns and follows the old screenshot's styling. The new reference also restyles the sheet.

Values sampled from the image:

| Cells | New reference | Today (D-A17) |
|---|---|---|
| A1:F1 merged, "Report Dashboard" | bold Arial 18, blue `#0000FF`, not underlined, centered, lavender fill `#DDDDFF`, about twice a normal row's height | A1:C1, underlined `#1155CC` on pink `#D5A6BD` |
| Row 2 headers | bold, centered, gray fill `#E0E0E0`, thin borders | bold, yellow `#FFF2CC`, no borders |
| A: church | white, left-aligned, thin borders | peach `#FCE5CD`, no borders |
| B: month ("September") | centered, regular | (no column) |
| C: year ("2026") | centered, **bold** | (no column) |
| D: Approved | bold white on green `#00AA00`, centered | bold white on blue `#0000FF` |
| E, F: remarks | bold white on blue `#0000CC` when filled ("Checked", "Ready for Pick Up") | blank, unstyled |
| Grid | thin black borders on every cell, A2:F(n+2) | none |

Column widths, measured in pixels against a default column and converted to Excel units: A about 36, B 13, C 9.5, D 13, E 21, F 17.

**Already done, kept as is:** the bulk export and its preview already include approved reports only (D-A16, 22h). `monthForExport` lists with `Completed: true`, so pending reports get no metric sheet. Every church account still appears on the dashboard as Approved, Pending or Not Filed. This fix doesn't change that.

## The fix

All the sheet and preview code is in `backend/internal/report/reportimpl/excel.go`, plus the store, the model and the tests.

1. **Data:**
   - `report.DashboardRow` becomes `{Church, Month string; Year int; Status string}`, where Year 0 means no report.
   - `store.Dashboard` also selects `r.month` and `r.year` from the joined report. They are NULL for Not Filed, so they're scanned into pointers.
   - `MockStore.Dashboard` mirrors this.
2. **Month and Year values:**
   - Approved and Pending rows: the report's own month and year ("September", 2026).
   - Not Filed rows: both blank.
   - The year is written as a number with `SetCellInt`, so Excel doesn't flag "number stored as text". The month and every other value stay `SetCellStr`, so formula-like church names stay text.
3. **Layout:**
   - Title merged over **A1:F1**.
   - `dashboardHeader` becomes the six names.
   - Widths and styles as in the table: title row 30 pt; thin borders on A2:F(n+2); Month and Year centered, Year bold.
   - Status: Approved is green `#00AA00`; Pending and Not Filed stay red `#FF0000` (D-A17, owner). All three are bold white and centered.
   - The dashboard prints landscape, so all six columns fit on one page (owner, 2026-10-22).
4. **Remarks stay blank:**
   - Coordinator's Remark and Support Status are written empty with borders only, no fill (D-A17), because the app has no data for them.
   - The reference's "Checked" and "Ready for Pick Up" are typed values.
   - The blue `#0000CC` style is left for the feature that fills these columns.
5. **Preview:**
   - `dashboardRows` pads every row to 6 columns and blanks the merged title's tails B1:F1.
   - `sheets[0].rows` becomes `[["Report Dashboard","","","","",""],["Churches","Month","Year","Status","Coordinator's Remark","Support Status"],["Bethel","September","2026","Approved","",""],["Abba","","","Not Filed","",""], ...]`.
   - The response types are unchanged (`string[][]`).
6. **Frontend preview modal:**
   - No logic change: `excel-preview-dialog.tsx` already sizes its column letters from each sheet's row width, so it shows A to F.
   - Update the comment at line 25 and the dialog test fixture to six columns.
   - Add an assertion that the dashboard shows column letters A to F.
7. **Docs:**
   - **D-A19** in `docs/DECISIONS.md` amends D-A16's columns and D-A17's dashboard styling. The metric sheets keep D-A17.
   - Update the Swagger descriptions, then run `make swagger` and `npm run gen:api`.
   - Update `docs/API.md` (dashboard bullet and preview example).

The metric sheets, the single export, the sheet order, the cap and the audit count are unchanged.

## Build steps

- [x] 1. **Store and model.**
  - Add Month and Year to `DashboardRow`, the `Dashboard` query and the mock.
  - Done when `store_integration_test.go` expects Month and Year for the Approved and Pending rows and blanks for the Not Filed rows (including the soft-deleted-report row), and `make test-integration` passes.
- [x] 2. **Sheet and preview.**
  - Six-column layout and styles in `writeDashboard`, and 6-wide preview rows in `dashboardRows`.
  - Done when `excel_test.go` and `report_test.go` assert each of these, and `make test` and `make lint` (0 issues) pass:
    - the A1:F1 merge, the title's font and lavender fill
    - the gray header row
    - a white church cell with a border
    - Month centered, Year bold and stored as a number
    - the green Approved and red Pending and Not Filed fills
    - unfilled remark cells
    - formula-like text staying text
    - the preview rows for 0 and 3 accounts
- [x] 3. **Frontend and docs.**
  - The dialog fixture and an A–F test, the comment, D-A19, Swagger, `api-types.ts` and `API.md`.
  - Done when `npm test`, `npm run lint`, `npm run typecheck`, `make test` and `make lint` pass, and `api-types.ts` has no drift.

## Verify

- `cd backend && make test && make lint && make test-integration`
- `cd frontend && npm test && npm run lint && npm run typecheck`
- `npm run e2e -- excel-export` with the local stack. The "Visionaires Christian Church … Not Filed" assertion must still pass.
- **By hand:** bulk-export a month with one approved and one pending report, and compare the "Report Dashboard" tab with `blueprint/reference/image.png`. Also render it with LibreOffice before handoff.

## Notes for the AI

- No migration, no new endpoint, no permission change. The preview still writes no audit row.
- Row heights and widths are converted from a screenshot. Excel's own units are the source of truth for the tests.
- Don't commit `blueprint/reference/`. Check `git status` before any `git add -A`.

## Open questions

Each has a default the build uses unless the owner says otherwise.

1. **Pending rows' Month and Year:** the request says to fill them "for approved reports" and leave them blank for churches that haven't filed. A pending church has filed. The default fills them, since the report exists. Should they be blank instead?
2. **"N/A" or blank for Not Filed:** the default is blank, to keep the sheet clean. Should it say "N/A"?
3. **Pending and Not Filed colors:** the new reference only shows Approved, now green. The default keeps both red, as the owner chose for D-A17. Should either change?


<!-- blueprint:completion {"schemaVersion":2,"specBytes":7066,"specSha256":"867be139ee7f7358a2e50ebf4a7f3fe01102c11003072f0f7c686739175548e0","branch":"refs/heads/fix/report-dashboard-with-month-and-year-columns","head":"42171392759906ff46ef2516c10716300ea709d7","baseRef":"refs/heads/main","baseCommit":"3d0d219eb169a3f2f98cd721a76bb116999e525b","sourceTree":"ccada76313bc61c1fc3abafab18f5f1452d82670","landing":"local-merge","absentOptional":[]} -->

## Try it now

Run the stack locally (`cd backend && make dev`, `cd frontend && npm run dev`) and sign in as an admin.

1. Open **Admin**, then the **Reports** tab, and pick a month with one approved and one pending report.
2. Click **Download Bulk**. The preview opens on **Report Dashboard** with columns A to F: Churches, Month, Year, Status, Coordinator's Remark, Support Status. The approved and pending churches show the month and year; churches that haven't filed show blanks.
3. Click **Download** and open the `.xlsx`. The Report Dashboard tab matches `blueprint/reference/image.png`: a lavender title over A1:F1, a gray header, white bordered cells, Approved in green, Pending and Not Filed in red, and blank remark columns. Only approved churches get their own tab.
4. Print preview the Report Dashboard: it is landscape, and all six columns fit on one page.
