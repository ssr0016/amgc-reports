# Fix: Admin panel refresh, loading, toasts and retry, and report form help text

**Type:** Fix
**Status:** verified
**Branch:** fix/admin-panel-refresh-loading-toasts-and-retry

## The problem

Owner request, 2026-10-22: the admin panel should show changes without a reload, and handle loading, success and errors the same way on every tab. Today:

1. **Stale Tracker and stats.**
   - Approving or unapproving a report refreshes Reports and the stats cards, but not the Tracker.
   - Creating, editing or deleting a report refreshes only the reports lists.
   - Creating, editing, changing the status of, deleting or restoring a user refreshes only the user queries, though the stats count users and the Tracker lists them.
2. **Loading.** Every tab shows "Loading…" as plain text, with no spinner.
3. **Toasts.** Approve and Unapprove toast "Approved September 2026"; an Excel download that succeeds shows nothing.
4. **No Retry on some errors.** The stats cards, the Reports tab, the Users tab and the Excel preview show a failed fetch with no Retry (the Tracker and Logs have one).

5. **Report form wording** (owner, same day): the long-text fields have short placeholders and no help text; the buttons read "Create Report" and "Update Report".

Overlaps backlog item 31 (admin panel only).

## The fix

- **Refreshing:** `refreshAdminViews(queryClient)` in `src/lib/query.tsx` invalidates `["admin-stats"]` and `["tracker"]`.
  - It's called after every report or user mutation listed above, next to the invalidations those sites already make.
  - Invalidation marks the queries stale, so the Tracker refetches when its tab mounts. No polling, no refetch on focus: `query.tsx` keeps those off to protect the rate limit.
- **`Loading`** (`src/components/loading.tsx`): a `role="status"` line with a lucide `Loader2` icon (`motion-safe:animate-spin`, `aria-hidden`) and the text.
  - It replaces the plain "Loading…" lines in the stats cards, Reports, Tracker, Users, Logs and the Excel preview.
- **`ErrorRetry`** (`src/components/error-retry.tsx`): a `role="alert"` message and a Retry button that calls the query's `refetch`.
  - Used on those same six surfaces. The tabs that already have Retry switch to it.
- **Toasts:**
  - Approve and Unapprove say "Report approved: September 2026" and "Report unapproved: September 2026". The server's answer still decides which one shows.
  - A finished download says "Downloaded <file name>".
  - Errors stay as they are.
- **Report form:** a help line under each long-text field (`id="<name>-hint"`, added to the textarea's `aria-describedby` next to any error), with the owner's wording:
  - Names: "e.g., Juan dela Cruz, Maria Santos (Separate names with commas)."
  - Narrative Report: "Describe what happened this month in your ministry. Include key events, milestones, or testimonies."
  - Challenges: "What challenges or difficulties did you encounter this month?"
  - Prayer Request: "List your prayer requests here so the team can pray for you."
  - Placeholders become short examples that don't repeat the help. The submit button reads "Submit Report" on New and "Save Changes" on Edit.
- **Not built: "Save Draft."** Reports have no draft state. A saved report is already filed and counts as Pending in the Reports tab, the Tracker and the stats. A draft needs a product decision and a schema change.
- Must not break: the existing 403 messages, the empty states, the busy buttons during approve and download, and `keepPreviousData` (a filter change keeps the old rows and shows no spinner).

## Build steps

- [x] 1. Add `Loading`, `ErrorRetry` and `refreshAdminViews`, with tests. Use them in `stats-cards.tsx`, `admin-reports.tsx`, `tracker.tsx`, `users/users-list.tsx`, `audit-logs.tsx` and `excel-preview-dialog.tsx`. Call `refreshAdminViews` from approve, report create, edit and delete, and user create, edit, status, delete and restore. Change the toasts. Update the affected tests.
  - **Done when:**
    - Vitest shows each of the six surfaces renders the spinner while pending, and a Retry that refetches after an error
    - approve invalidates `["tracker"]` and `["admin-stats"]`
    - a download toasts its file name
    - `npm test`, `npm run lint` and `npm run typecheck` pass
    - the `a11y.spec.ts` and `tracker.spec.ts` browser tests pass

- [x] 2. Report form help text, placeholders and button labels as above, with the `report-form` and new/edit page tests updated.
  - **Done when:**
    - Vitest shows each long-text field is described by its help line, and by its error when there is one
    - New shows "Submit Report" and Edit shows "Save Changes"
    - `reports.spec.ts` and `a11y.spec.ts` pass

## Verify

On `/admin`, approve a report on Reports, then open Tracker: that worker shows Approved with no reload. Stop the API and switch tabs: each tab shows its error with Retry. Start the API again and click Retry: the data loads.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":4925,"specSha256":"def4d316bc3fe5fe0ef803811e46e2a80851cd3b53b3aa83d0fd5351a2300f40","branch":"refs/heads/fix/admin-panel-refresh-loading-toasts-and-retry","head":"2589c3ea3a282670ee54674a2ae6850b57ab0c9d","baseRef":"refs/heads/main","baseCommit":"2589c3ea3a282670ee54674a2ae6850b57ab0c9d","sourceTree":"31206923d7c2d38eb1485eccd2c0b332286537dd","landing":"local-merge","absentOptional":[]} -->

## Try it now

Run or do:

With this branch checked out: in `backend/`, `make db-start` then `make dev`; in `frontend/`, `npm run dev`. Sign in at http://localhost:3000 as admin, open **http://localhost:3000/admin**, and on **Reports** click **Approve** on a Pending report. Then open the **Tracker** tab for that report's month.

Expected:

A toast reads **"Report approved: <Month YYYY>"**. The Tracker shows that worker as **Approved** and its Approved count up by one, with no page reload. While each tab loads, a spinning icon sits next to "Loading…".

This proves:

An approval now refreshes the Tracker and the stats cards, and the approve toast uses the new wording.

## Deeper checks

1. **Retry:** stop the API (`Ctrl+C` on `make dev`), switch between Reports, Tracker, Users and Logs, and open an Excel preview. Each shows an error with a **Retry** button. Start the API again and click **Retry**: the data loads.
2. **Download toast:** on Reports, click **Excel**, then **Download** in the preview. Expect **"Downloaded <file name>"**.
3. **User changes:** create, deactivate, delete or restore a user on **Users**, then open **Tracker**: the list and the stats cards reflect it without a reload.
4. **Report form:** open **/reports/new**. Under Names, Narrative Report, Challenges and Prayer Request sits a help line in the owner's wording; the button reads **Submit Report**. Editing a report shows **Save Changes**.

Watch for: a filter change on any tab showing a spinner instead of keeping the old rows (`keepPreviousData` broken); a 403 tab showing Retry instead of its permission message; "Approved September 2026" (old toast wording).

Gaps: the guide assumes the dev seed has an admin and at least one Pending report for the chosen month.
