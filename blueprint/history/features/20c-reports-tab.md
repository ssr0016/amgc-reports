# Feature: Reports tab

**From build-plan:** feature 20c
**Build attempt:** 1
**Branch:** feature/reports-tab
**Status:** verified

> Revised three times at the owner's request. (1) The tab no longer reuses `ReportsList`; it has its own component, `AdminReports`. (2) Layout redesign: the admin table filters only by Month and Year, gains a "Download Bulk" button, and owns Approve/Unapprove, which are removed from `/reports`. (3) Column order and the filter row: `/reports` and `/admin` share one column order, and "Download Bulk" sits on the right of the filter row.

## Goal

Make the Reports tab of the admin panel (`/admin`) the one place where reports are approved and where an admin sees who reported for a chosen month and year. `/reports` stays the place to read, edit and delete reports, with Status shown read-only. Frontend only: no API, schema or Swagger change. The Excel downloads (bulk and per row) are visible placeholders for Phase C.

## In scope

### `/admin`, Reports tab (new component `AdminReports`, `frontend/src/app/(app)/admin/admin-reports.tsx`)

- **Filters: Month and Year dropdowns only.** No Worker, Area or Church boxes and no Clear button.
  - They start on the previous month in Asia/Manila (`defaultPeriod()`: in October 2026 that is September 2026, in January 2027 it is December 2026).
  - The Year list runs from `FIRST_YEAR` (2026) to the current calendar year, so in 2027 an admin can still go back to 2026. Any month can be picked with any listed year. The lists are computed when the tab renders, so they stay correct as time passes.
  - The Month list has no "Any" option: the period is always one month, so "Download Bulk" has one clear meaning. (See Open questions.)
  - Changing either returns to page 1.
- **"Download Bulk" button** in the same row as the dropdowns, on its **right** side (the dropdowns stay on the left). **A placeholder:** disabled, with a `title` saying the export comes later. No endpoint, no request.
- **Table columns, in order:** Worker, Month, Year, Area of Assignment, Church, Status, Action, Download.
  - Worker, Area of Assignment and Church show "—" when empty. Status is the badge "Completed" or "Pending" (the words `/reports` uses).
  - **Action:** a **Approve** button on a Pending row and an **Unapprove** button on a Completed row, each shown only with `reports:approve`; without it the cell is "—". No Edit, View or Delete here.
  - **Download:** a per-row "Excel" control on every row. **A placeholder:** a disabled button (not a link: there is no URL), named "Excel" plus the row's name for screen readers, with a `title` saying the export comes later.
- **Header:** "N total reports" (N = the API's total for the selected month and year), above the table, as a status message so a change is announced. N=1 reads "1 total reports", as the owner wrote it.
- **Pagination:** Previous / Next with "Page X of Y", 20 per page (the API default), only when there is more than one page.
- **States:** loading, API error, empty ("No reports match these filters."), and not allowed (no `reports:read`, or a 403): "You are not allowed to view reports."
- **Approve / Unapprove flow:** `PATCH /reports/:id/complete` (it toggles, so the row's button is disabled while the call is in flight), then refetch the list and the stats cards. The toast uses the server's answer ("Approved September 2026" or "Unapproved September 2026"). A 404 says "<period> no longer exists." and refetches; any other error shows the API message with its retry hint (`retryText`).

### `/reports` (`ReportsList`, `frontend/src/app/(app)/reports/reports-list.tsx`)

- **Columns, in this order:** Worker, Month, Year, Area of Assignment, Church, Status, Actions. The first five show "—" when empty. "Actions" (plural) is a visible heading; the cell holds Edit (or View) and Delete only. (Today the order is Month, Year, Worker and the heading reads "Area"; the text box that filters by area keeps its label "Area".)
- **Remove the Approve and Unapprove buttons** and the code only they used (the in-flight set, the optimistic row patch, the toggle handler and its `toggleComplete` import). The Status column stays, read-only. Edit, View and Delete, the filters, pagination and the "New report" button are unchanged.

### Docs

- `docs/DECISIONS.md`: record that approval moved from the reports list to the admin panel; update the Approve rows in `backend/docs/08-Frontend-Plan.md` that say it is in the list.

## Out of scope

- Any other change to the `/reports` filters, rows or pagination beyond the column order, the "Area of Assignment" heading, the visible "Actions" heading and the removal above.

- Making any Excel download work: no endpoint (not even one that answers 501), no library, no CSV, no audit row (Phase C, item 22, D-A6, D-10).
- Any other change to `/reports`, `lib/reports.ts`, or the API. `toggleComplete` stays in `lib/reports.ts` (the admin component uses it).
- Filters or page in the URL, and tab state in the URL (decided in 20a).
- Tying the stats cards to the Month/Year dropdowns: the cards stay on the previous month (20b). (See Open questions.)
- The Users tab (20d), the preview modal and the Avg column (20e), Tracker ("who has not reported") and Logs (Phase B).
- Any backend change.

## Build loop

`workflow.stepReview` is `feature`: review once after all steps. `checkpointCommits` is disabled; `/complete` makes the final commit. Test-first inside each step. Stop any dev server before `npm run build`, and ask before stopping one you did not start. Frontend commands run from `frontend/`.

## Build steps

- [x] 1. **`/reports` loses Approve and Unapprove (tests first).** In `reports-list.test.tsx`, replace the approve cases (the "approve" describe and the "Approve shows only with reports:approve" case) with one case: even for a user holding `reports:approve`, no button named Approve or Unapprove appears on a Pending or a Completed row, while Status still reads Completed/Pending and Edit/View/Delete still follow the permission rules. Drop the `toggleComplete` mock if nothing uses it. Then remove the buttons and the code only they used from `reports-list.tsx`.
  Done when: `npm test` shows the new case red first and then green, every other existing reports-list test unchanged and passing, and `npm run lint` and `npm run typecheck` pass (no unused import or variable left behind).
- [x] 2. **`AdminReports`: Month and Year only, and Download Bulk (tests first).** In `admin-reports.test.tsx`, remove the Worker/Area/Church/Clear cases and add: only the Month and Year selects exist (no text boxes); Month has no "Any"; the Year list is `FIRST_YEAR` through the current year (use a fixed clock) and a past year can be chosen; the first request is the previous month and year; changing either refetches with it and with page 1; a "Download Bulk" button sits in the filter row, disabled, with a `title`, and clicking it sends nothing. Then change `admin-reports.tsx`.
  Done when: `npm test` passes these cases (red first, then green) and the table, header, state and pagination cases from before still pass; `npm run lint` and `npm run typecheck` pass.
- [x] 3. **`AdminReports`: Approve and Unapprove (tests first).** Replace the "no Unapprove" expectations with: a Pending row offers Approve and a Completed row offers Unapprove, named for the row, each only with `reports:approve`, and the cell is "—" without it; a click calls `toggleComplete(id)` once and disables that button until the list has refetched; success shows the server's answer, refetches the list and invalidates `["admin-stats"]`; a 404 and another error behave as in In scope; no Edit, View or Delete appears.
  Done when: `npm test` passes these cases (red first, then green).
- [x] 4. **Browser checks, docs and full suites.**
  - `e2e/reports.spec.ts`: the admin case "approves and unapproves a report in the list" moves to `/admin`: open the tab, Approve the E2E report (row reads Completed, the Approved card +1 and Pending −1), Unapprove it (cards return), and reload to confirm the server kept the state. Add to it that `/reports` shows no Approve or Unapprove button for the admin. Keep the existing user and editor cases ("no button, API 403").
  - `e2e/a11y.spec.ts`: the `/admin` axe and 320 px reflow cases keep waiting for the table.
  - Docs as in In scope.
  Done when: `npm test`, `npm run lint`, `npm run typecheck`, `npm run build` (dev server stopped first) and `npm run e2e` pass, or any that cannot run is named in the summary with the reason.
- [x] 5. **`/reports` column order and headings (tests first).** In `reports-list.test.tsx`, add a case that the column headers read Worker, Month, Year, Area of Assignment, Church, Status, Actions in that order, with "Actions" visible (not screen-reader-only), and that a row's cells follow the same order (worker, month, year, area, church, status badge, then the actions). Then reorder the `TableHead` and `TableCell` entries in `reports-list.tsx` and rename the heading.
  Done when: `npm test` shows the new case red first and then green with every other reports-list test passing, and `npm run lint` and `npm run typecheck` pass. No `/reports` behavior changes: Edit, View, Delete, filters and pagination still pass their tests.
- [x] 6. **`AdminReports`: Download Bulk on the right (tests first).** In `admin-reports.test.tsx`, assert that "Download Bulk" comes after the Month and Year selects in the filter row and carries the right-alignment class (`ml-auto`: jsdom has no layout, so the class and the DOM order are what can be checked). Then move it in `admin-reports.tsx`. Re-run everything: `npm test`, `npm run lint`, `npm run typecheck`, `npm run build` (dev server stopped first) and `npm run e2e`, and finish step 4 (the build is still owed).
  Done when: those pass, or any that cannot run is named in the summary with the reason.

## Files / areas

- Edit: `frontend/src/app/(app)/reports/reports-list.tsx` (+ `reports-list.test.tsx`), `frontend/src/app/(app)/admin/admin-reports.tsx` (+ `admin-reports.test.tsx`), `frontend/e2e/reports.spec.ts`, `frontend/e2e/a11y.spec.ts`, `docs/DECISIONS.md`, `backend/docs/08-Frontend-Plan.md`.
- Already in place and unchanged by this revision: `admin-panel.tsx` (mounts `AdminReports`) and its test.
- Read only: `frontend/src/lib/reports.ts` (`listReports`, `toggleComplete`, `defaultPeriod`, `MONTHS`, `FIRST_YEAR`), `frontend/src/app/(app)/admin/stats-cards.tsx` (the `["admin-stats"]` key), `frontend/src/lib/auth-form.ts` (`retryText`).

## Data / contracts

- No API, Swagger or `api-types.ts` change. The admin component calls `GET /reports` with `month`, `year`, `page` (and the default `limit`) and `PATCH /reports/:id/complete`, through the existing `listReports` and `toggleComplete`. `/reports` keeps calling `GET /reports`; it no longer calls the approve endpoint.
- Fields used from `Report`: `id`, `worker`, `month`, `year`, `area_assignment`, `church_name`, `completed`; from the page meta: `total`, `page`, `total_pages`.
- The approve endpoint **toggles**. If the row was changed elsewhere, the click does the opposite of what the label said; the toast reports what the server did and the list is refetched, so the screen shows the truth.
- Query keys: the admin table uses `["reports", filters]` (the same prefix as `/reports`) and the cards `["admin-stats"]`; a successful approve or unapprove invalidates both prefixes.
- Authorization: the panel shows only to the admin role; Approve/Unapprove also need `reports:approve`; the server enforces both. Only the admin role holds `reports:approve`, so after this change nobody else has a place to approve (it already was admin-only).
- Accessibility: headers are `<th>`; row controls carry the row's name for screen readers; "N total reports", loading and empty are `role="status"`; the error is `role="alert"`; each disabled Excel control is explained by its `title` and its name; the page still has one `<h1>`.

## Testing

- Vitest with Testing Library: `npm test`, `npm run lint`, `npm run typecheck`, `npm run build`.
- Playwright + axe: `npm run e2e` (the e2e backend starts itself against a fresh database; the test Postgres must be up on :5440). Run the whole `reports.spec.ts`: the admin case depends on fixtures from earlier cases, and after an Unapprove the row flips before the server answers, so reload before reading it back.
- Not claimed unless run: any production or live-data check, and any visual match to a screenshot: the owner described the layout in writing and mentioned two screenshots, but none reached the session or the repo.

## Notes for the AI

- Remove, do not hide: the Approve/Unapprove code in `ReportsList` goes entirely, leaving no dead state or unused import.
- The Excel controls are placeholders and make no request. Do not add an endpoint, a library, a CSV, a URL or an audit row for them.
- Keep `AdminReports` small: no URL syncing, no optimistic update, no shared hook with `ReportsList`.
- Next.js 16 differs from older versions: read `frontend/AGENTS.md` before frontend work. Do not edit it.
- Never run the Playwright suite against production.

## Open questions

- **Excel placeholder: which one?** Recommended and specified above: disabled controls that make no request. The alternatives, with their cost:
  - **A 501 endpoint** (admin route, handler, Swagger, `gen:api`, tests): about a day of backend and generated-file churn for a button nobody can use; it reserves the URL shape early, which Phase C may want to change. Safe, but not faster.
  - **A basic CSV:** not Excel, and not safe as the "Excel" feature: it needs a server endpoint to write the `DOWNLOAD_*` audit row (D-10); worker, area and church are user-typed text that can start with `=`, `+`, `-` or `@` and run as a formula when opened (CSV injection) unless escaped; and its columns would be invented, which D-A6 says to avoid until the legacy template is provided.
  Say so if you want either instead.
- **Month "Any".** This design drops it, so every view and every Bulk download is one month. Say so if you want "Any" back (Bulk would then need a rule for it).
- **Stats cards and the dropdowns.** The four cards stay on the previous month, so after the admin picks another month the cards and the table describe different months. The API already accepts a month and year, so following the dropdowns is a small frontend change: say so and I will add it.
- **Layout source.** The screenshot is not in the repo, so this follows the written description. Put it in `blueprint/reference/` if the result should match it exactly.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":14619,"specSha256":"18b90a3de04c3a9c119199df212a24ebb38f2608afe654b64900440f7bc659ad","branch":"refs/heads/feature/reports-tab","head":"ca18cd09f6cc0639d7e8764452ac908d2098744e","baseRef":"refs/heads/main","baseCommit":"ca18cd09f6cc0639d7e8764452ac908d2098744e","sourceTree":"4f7d3e7a0bea9d2ceca4b7ab12a279a63666eb71","landing":"local-merge","absentOptional":[]} -->
