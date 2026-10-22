# Fix: Excel sheet layouts match the legacy workbook

**Type:** Fix
**Status:** verified
**Branch:** fix/report-dashboard-sheet-layout

## The problem

The Excel export's sheets don't look like the legacy workbook the owner works from:

- **"Report Dashboard"** (bulk only, 22h, D-A16): a plain table with columns Church, Worker, Area, Status, a bold header and thin borders everywhere.
- **Metric sheets** (the church tabs, single and bulk, D-A6): cell positions, labels, merges and formulas already match the template, but the styling doesn't:
  - the title isn't blue
  - there are no black band rows
  - every cell uses one Helvetica style with four-sided borders
  - the Narrative and Prayer rows are short

**References:** `blueprint/reference/image.png` (dashboard) and `blueprint/reference/image-report.png` (metric sheet). Both are screenshots of the legacy workbook `blueprint/reference/Church Growth Program Report September 2026.xlsx`, so the exact widths, fonts, fills and borders below are read from that file's `styles.xml` and sheet XML, not estimated from pixels.

The only exception is row heights: Google Sheets auto-grows rows for big or wrapped text, while Excel doesn't. Where a stored height of 15.75 pt would clip text, the height is measured from the screenshot instead, using 14 px per 15.75 pt.

All three files are ignored in `.gitignore`, because the images show real church names and the repo is public.

## The fix

All changes are in `backend/internal/report/reportimpl/excel.go`: the styles, `writeDashboard`, `writeSheet`, `writeMetrics` and `dashboardRows`.

Unchanged:
- cell positions, labels, merges and formulas (D-A6)
- the sheet order and the approved-only rule
- the cap and the audit count
- the single export's content
- the preview's 42 × 7 metric grid

The workbook's default font is Arial 10, as in the legacy file, and every styled cell names its font explicitly.

### Report Dashboard (owner, 2026-10-22, D-A17)

| Cells | Legacy style |
|---|---|
| A1:C1 merged, "Report Dashboard" | bold, single underline, 18 pt, `#1155CC`, fill `#D5A6BD`, centered, no border |
| D1 | fill `#D5A6BD`, empty |
| A2:D2: Churches, Status, Coordinator's Remark, Support Status | bold Arial 10, fill `#FFF2CC`, no border |
| A3…: church name | Arial 10, fill `#FCE5CD`, no border |
| B3…: Approved | bold white Arial 10, fill `#0000FF` |
| B3…: Pending, Not Filed | bold white Arial 10, fill `#FF0000`, which is the legacy red |
| C3…, D3…: blank | no style, no value |

- **Column widths:** A 21.88, B 12.63, C 20.38, D 12.63.
- **Row heights:** every row is 15.75 pt except row 1, which is 23.25 pt (from the screenshot; 18 pt text clips at 15.75 in Excel).
- **Worker and Area are removed** from the sheet, from `report.DashboardRow`, from the `Dashboard` query and from `MockAccount`, so the export no longer reads data it doesn't show (owner, 2026-10-22).
- **Blue for Approved:** the legacy blue marks the good state ("Checked", "Ready for pi"), and its red marks the rest ("-", "Review", "On Hold"). Pending is red like Not Filed (owner, 2026-10-22).

### Metric sheets (legacy church tab)

Widths are already the legacy values (A 26, B–G 12.63), and so is the default row height of 15.75 pt. Styles:

| Cells | Legacy style |
|---|---|
| A1:G2 merged title | Times New Roman 18, not bold, `#0000FF`, white fill, centered horizontally and bottom-aligned; right and bottom border |
| A3:G3 merged subtitle | bold Helvetica Neue 12, white fill, centered; left, right and bottom border |
| A4:A7 labels | Helvetica Neue 12, white fill; left, right and bottom border |
| B4:G4 merged month | bold Arial 10, white fill, `mmmm yyyy`, left; all four borders |
| B5:G7 merged values | Arial 10, white fill; all four borders |
| A8 "Weekly Attendance" | bold white Helvetica Neue 12 on black; left, right and bottom border |
| B8:G8, B29:G29 | black fill; right and bottom border |
| A29 "Other Statistics" | bold white Helvetica Neue 12 on black, like A8, so the label stays readable (the template leaves it blank) |
| A9, B9:G9 headers | bold Helvetica Neue 12, top-aligned; A has left, right and bottom border, B–G right and bottom |
| B30:G30 "Week 1…Average" | Helvetica Neue 12, **not bold**, top-aligned; right and bottom border |
| Main labels, A10–A20, A24, A27, A31–A37 | Helvetica Neue 12, top-aligned; left, right and bottom border |
| Sub and blank labels, A21–A23, A25–A26, A28, A30, A38, A39 | Arial 10, bottom-aligned; left, right and bottom border (the small "Leadership Conference" and "Family Day" text in the image) |
| B10:G28, B31:G36 values | Arial 10, bottom-aligned; right and bottom border; tithes keep `#,##0.00` |
| B37:G39 merged names | Arial 10, white fill, wrapped, top-aligned; right and bottom border |
| A40:A42 labels | Helvetica Neue 12, black, white fill, wrapped, top-aligned; left, right and bottom border |
| B40:G42 merged text | Helvetica Neue 12, white fill, wrapped, top-aligned; right and bottom border |

- **Row heights:** 40 (Narrative) 56.25 pt, 41 (Challenges) 29.25 pt, 42 (Prayer) 45 pt, measured from the screenshot. The file stores 43.5, 15.75 and 15.75, and Sheets grows them.
- **Kept on purpose:**
  - "Baptism" on row 19 uses the main-label style. D-A6 gave the template's blank row 19 that label.
  - Averages in G keep their current number format. The template's `0` format would round them, which changes D-A5's averages, not just their look.

### Preview

Its rows are unchanged, except that the dashboard is read as title, header and accounts, with the merged title's tails B1:C1 blank:
- `[["Report Dashboard","","",""],["Churches","Status","Coordinator's Remark","Support Status"],["Bethel","Approved","",""], ...]`

The response types are unchanged.

## Build steps

- [x] 1. **Report Dashboard.**
  - Restyle `writeDashboard` per the table and narrow `DashboardRow` (the store query, mock and tests).
  - Update `dashboardRows` for the title and header rows.
  - Done when `excel_test.go` and `report_test.go` assert each of these, and `make test`, `make lint` (0 issues) and `make test-integration` pass:
    - the A1:C1 merge and its title font
    - the D1, A2, A3 and status fills
    - C and D blank and unstyled
    - formula-like text staying text
    - the preview rows for 0 and 3 accounts
- [x] 2. **Metric sheets.**
  - Restyle `writeSheet` and `writeMetrics` per the table, using one small border-sides helper instead of a style per cell.
  - Done when `excel_test.go` asserts these, and `make test` and `make lint` (0 issues) pass:
    - the title font and color
    - the A8 and B29 black fills
    - A21 in Arial 10 next to A20 in Helvetica Neue 12
    - B30 not bold
    - the row 40–42 heights
    - B10's borders being right and bottom only
  - The existing layout and preview tests still pass.
- [x] 3. **Docs.**
  - Update the `ExportReports` and `PreviewExcel` godoc descriptions, then run `make swagger` and `npm run gen:api`.
  - Update `docs/API.md`: the dashboard bullet and the preview example.
  - Add **D-A17** to `docs/DECISIONS.md`: both sheets follow the legacy styling, amending D-A16's columns, with the color mapping, the blank remark columns and the two kept differences above.
  - Update the dialog test fixture in `excel-preview-dialog.test.tsx` and the comment at `excel-preview-dialog.tsx:25`.
  - Done when `make test`, `make lint`, `npm test`, `npm run lint` and `npm run typecheck` pass, and `api-types.ts` has no drift.

## Verify

- `cd backend && make test && make lint && make test-integration`
- `cd frontend && npm test && npm run lint && npm run typecheck`
- `npm run e2e -- excel-export` with the local stack. The existing "Visionaires Christian Church … Not Filed" assertion must still pass.
- **By hand:** bulk-download a month with one approved and one pending report, then compare the "Report Dashboard" tab with `image.png` and a church tab with `image-report.png`.

## Notes for the AI

- No migration, no new endpoint, no permission change. The preview still writes no audit row.
- Every cell value is still written with `SetCellStr`/`SetCellFloat`, so formula-like text stays text.
- Don't commit the reference images or the legacy workbook. Check `git status` before any `git add -A`.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":8270,"specSha256":"05a20759911e21ade26fcd2990dc2f5cd513aafa6af8cd72312389baa95125bf","branch":"refs/heads/fix/report-dashboard-sheet-layout","head":"9206955f8a909ff98c407e8f9e9d15b4e95fc773","baseRef":"refs/heads/main","baseCommit":"9206955f8a909ff98c407e8f9e9d15b4e95fc773","sourceTree":"06918964f1fcd66386289d745dfcd9d4db37e707","landing":"local-merge","absentOptional":[]} -->

## Try it now

Run the stack locally (`cd backend && make dev`, `cd frontend && npm run dev`) and sign in as an admin.

1. Open **Admin**, then the **Reports** tab, and pick a month with one approved and one pending report.
2. Click **Download Bulk**, then **Download**, and open the `.xlsx`.
3. The "Report Dashboard" tab matches `blueprint/reference/image.png`: a pink merged title over A1:C1, a yellow header (Churches, Status, Coordinator's Remark, Support Status), peach church cells, Approved in blue, Pending and Not Filed in red, and blank remark columns.
4. A church tab matches `blueprint/reference/image-report.png`: a blue Times New Roman title, black "Weekly Attendance" and "Other Statistics" bands, smaller Arial sub-items ("Leadership Conference", "Family Day"), and tall Narrative, Challenges and Prayer rows.
5. In the preview modal, the Report Dashboard shows the title row, then the header, then one row per church.
