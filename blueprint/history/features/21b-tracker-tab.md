# Feature: Tracker tab

**From build-plan:** feature 21b
**Build attempt:** 1
**Branch:** feature/tracker-tab
**Status:** verified

## Goal

Admins see, for one month, which church workers have filed their report and which have not: counts (expected, submitted, approved, for review, not submitted), a progress bar, and one row per worker, in a new **Tracker** tab of `/admin` (D-A3).

## In scope

- **API:** admin-only `GET /admin/tracker?month=&year=`. Without month and year the period is the previous month in Asia/Manila (same rule and same 400/422 errors as `GET /admin/stats`).
- **Who is expected (D-A3):** every account with the `user` role (`accesscontrol.DefaultRoleName`) that is not soft-deleted. Admins and editors are not tracked. Account status (inactive, disciplinary, deceased) does not remove an account; see Open questions.
- **Status per account (D-A3):** from its one active (not soft-deleted) report for the month: `approved` when completed, `for_review` when it exists and is not completed, `not_submitted` when none. No new report state.
- **Tracker tab:** tab order Reports, Tracker, Users, Logs; `?tab=tracker`. Month and Year selects (default `defaultPeriod()`, the previous Manila month, as the Reports tab), a search box (worker name, email or church, case-insensitive, client-side), a Status filter (All, Approved, For Review, Not Submitted, client-side). Counts and progress bar always describe the whole month, not the filtered rows. Table: Worker (name, email underneath per D-A2, plus the user-status badge when not active), Church, Status badge.
- **States:** loading (`role="status"`), API error (`role="alert"` with a Retry), 403 ("You are not allowed to view the tracker."), no expected accounts ("No church workers to track."), no rows match the search or filter ("No workers match."), an "N of M workers" line.
- **Docs:** Swagger, `make swagger`, `npm run gen:api`, `docs/API.md`, D-A3 status "built in 21b".

## Out of scope

- Linking a row to the report or the preview modal (20e is not built).
- Server-side search, filtering or pagination (one month is about 24 accounts).
- Reminders or emails to workers who have not submitted; a Tracker sidebar item; changing the stats cards; aligning the Excel Report Dashboard's account set (D-A16) with D-A3.
- No migration, no audit row (a read, like `GET /admin/stats`).

## Build loop

`workflow.stepReview` is `feature`: build all steps, then one review at the end. `workflow.checkpointCommits` is `disabled`: no commits during the build; `/complete` makes the feature commit. Tests first in every step. Run `make lint` to 0 issues before presenting.

## Build steps

- [x] 1. **Store query (tests first).** In `backend/internal/stats/model.go` add `TrackerRow` and `Tracker` (Data / contracts) and the three status constants. Add `Tracker(ctx, month string, year int) ([]TrackerRow, error)` to `stats.Store` and implement it in `statsimpl/store.go` as one query: `users u JOIN roles ro ON ro.id = u.role_id AND ro.name = ?` (`accesscontrol.DefaultRoleName`) `LEFT JOIN reports r ON r.created_by = u.id AND r.deleted_at IS NULL AND r.year = ? AND lower(r.month) = lower(?)`, `WHERE u.deleted_at IS NULL`, ordered by `lower(COALESCE(u.church_name, ''))`, `lower(u.name)`, `u.id`; status from `r.id` and `r.completed`, as `reportimpl.store.Dashboard` does. A nullable `church_name` scans to `""`; check the column's nullability in the schema. Return `[]TrackerRow{}`, never nil.
  - **Done when:** `store_integration_test.go` passes (`make test-integration`) with: a `user` with an approved, a pending and no report give the three statuses; admin and editor accounts and a soft-deleted user are absent; a soft-deleted report and a report of another month count as `not_submitted`; an inactive user is present with its `user_status`; order is by church then name.
- [x] 2. **Service (tests first).** In `statsimpl/service.go` move the period rule out of `Get` into a private `period(q GetStatsQuery) (string, int, error)` and use it in `Get` and a new `Tracker(ctx, q) (stats.Tracker, error)` on `stats.Service`. Add `Tracker` to any stats mock/fake the tests use.
  - **Done when:** `service_test.go` passes (`make test`): no period gives the previous Manila month at a fixed clock (January gives December of the year before); a lowercase month is canonicalised; only one of month and year, or a bad month, is `ErrPeriod`; the existing `Get` tests still pass unchanged.
- [x] 3. **Handler, route, Swagger (tests first).** `StatsHandler.GetTracker` in `stats/rest/stats_handler.go`, parsing month and year exactly as `GetStats` (share the parsing in a small private func) and mapping `ErrPeriod` to 422 and an invalid year to 400. Route in `router/router.go`: `admin.GET("/tracker", statsHandler.GetTracker, perm("reports:read_any"), limits.Read)`. Godoc with `@Tags admin/stats`, `@Router /admin/tracker [get]`. Run `make swagger`, then `npm run gen:api` in `frontend/`; re-export `Tracker` and `TrackerRow` from `frontend/src/lib/api-schema.ts` as the other types are.
  - **Done when:** `stats_handler_test.go` covers 200 with rows, 400 bad year, 422 month without year; `make test`, `make lint` (0 issues) pass; `api-types.ts` has the new path with no other drift.
- [x] 4. **Frontend data and tab (tests first).** `frontend/src/lib/tracker.ts`: `getTracker(month, year)`, `TRACKER_STATUSES` (code, label) and a pure `trackerCounts(rows)` returning expected, submitted (approved + for_review), approved, for review, not submitted. `frontend/src/app/(app)/admin/tracker.tsx`: the tab in In scope, built like `audit-logs.tsx` (TanStack Query with `keepPreviousData`, key `["tracker", month, year]`, `staleTime: 30_000`, `Label`/`NativeSelect`/`Input`/`Table`/`Badge`, `UserStatusBadge` for non-active accounts). Progress is a native `<progress value={submitted} max={expected}>` with a visible label "S of E submitted" bound by `aria-labelledby`; hide it when expected is 0. Status badges print their label, so colour is never the only cue; use the light-tint/dark-text tones from `audit-logs.tsx` (approved green, for review yellow, not submitted red). In `admin-panel.tsx` accept `tracker` in the tab param and add the trigger and content between Reports and Users.
  - **Done when:** `lib/tracker.test.ts` (counts, including zero rows) and `admin/tracker.test.tsx` (loading, rows and counts, search by name/email/church, status filter, counts unchanged by filters, "No workers match.", "No church workers to track.", error with Retry, 403 message, changing month refetches with the new period) and the updated `admin-panel.test.tsx` (`?tab=tracker` selects Tracker, tab order) pass with `npm test`; `npm run lint` and `npm run typecheck` pass.
- [x] 5. **E2E, a11y, docs.** Add the Tracker tab to `frontend/e2e/a11y.spec.ts` (axe, no violations) and a short `frontend/e2e/tracker.spec.ts`: an admin opens `/admin?tab=tracker`, picks the period of a report the seed or test creates, and sees that worker with the right status and the counts. Update `docs/API.md` and D-A3's status in `docs/DECISIONS.md`.
  - **Done when:** `npm run e2e` passes for those specs (needs the e2e backend; report honestly if it cannot run here), and the docs name the endpoint and its rules.

## Files / areas

- Backend: `backend/internal/stats/{stats.go,model.go}`, `backend/internal/stats/statsimpl/{store.go,service.go}` and their tests, `backend/internal/stats/rest/stats_handler.go` and test, `backend/internal/router/router.go`, generated `backend/docs/swagger*`.
- Frontend: `frontend/src/lib/{tracker.ts,tracker.test.ts,api-schema.ts,api-types.ts}`, `frontend/src/app/(app)/admin/{tracker.tsx,tracker.test.tsx,admin-panel.tsx,admin-panel.test.tsx}`, `frontend/e2e/{a11y.spec.ts,tracker.spec.ts}`.
- Docs: `docs/API.md`, `docs/DECISIONS.md` (D-A3).

## Data / contracts

`GET /admin/tracker` → 200:

```json
{
  "month": "September",
  "year": 2026,
  "rows": [
    { "user_id": 7, "name": "Juan Cruz", "email": "juan@example.org", "church_name": "Cavite", "user_status": "active", "status": "for_review" }
  ]
}
```

- `month` is the canonical English name, `year` an int; `rows` is always an array (`[]` when no accounts).
- `status` is exactly one of `approved`, `for_review`, `not_submitted`. `user_status` is the account status string as stored. `church_name` is `""` when blank.
- No report text, no IP, no report id.
- Errors through `apperror`: 400 `invalid year`; 422 with `details.month` (`stats.ErrPeriod`); 401/403 from the admin group and `reports:read_any`; 500 "failed to get tracker".
- Swagger names follow the stats types' pattern (`// @name ...` pinned) so `gen:api` produces `Tracker` and `TrackerRow`.

## Testing

- Go: `make test` (service, handler), `make test-integration` (store, needs Docker), `make lint`.
- Frontend: `npm test`, `npm run lint`, `npm run typecheck`, `npm run e2e` (tracker and a11y specs).
- CI checks `api-types.ts` drift; regenerate after the Swagger change.

## Notes for the AI

- Put it in `stats`, not `report`: `stats` already owns the admin period rule (previous Manila month, `ErrPeriod`), and `report.Store.Dashboard` follows D-A16's different account set (not admin, non-blank church) which must not change.
- Worker names, emails and church names are user-controlled: render them as React text only.
- The tab's default period is `defaultPeriod()` from `lib/reports.ts` (previous month), not `currentPeriod()` as Logs uses: workers file last month's report.
- `shortcut:` comment on the client-side search/filter and no pagination: fine for about 24 accounts; move to the server if the account count grows into the hundreds.
- No migration: production deploy is a plain push (H-1 manual migrate step not needed).

## Decisions (owner, 2026-10-22)

1. Inactive, disciplinary and deceased workers are tracked: they show their status badge and count as Not Submitted when they have no report.
2. The `user` role only; editors are excluded.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":9976,"specSha256":"3081287d6fe0bc07c37626853304d5ccb57ceef6f84bc5b41c3055558fb8d5db","branch":"refs/heads/feature/tracker-tab","head":"c0707d0bd0dee431f0b3bbb0e16837d58416d799","baseRef":"refs/heads/main","baseCommit":"7559d69becbd24b0d55547c8b39308bd823fec60","sourceTree":"1f978e31374463c9a441f4780a3d4bc787a7d360","landing":"local-merge","absentOptional":[]} -->

## Independent review

**Status:** passed
**Target commit:** c0707d0bd0dee431f0b3bbb0e16837d58416d799
**Base commit:** 7559d69becbd24b0d55547c8b39308bd823fec60
**Base ref:** main
**Spec hash:** 3081287d6fe0bc07c37626853304d5ccb57ceef6f84bc5b41c3055558fb8d5db
**Prepared by:** claude
**Builder model:** claude-opus-5-5
**Requested reviewer:** claude
**Requested model:** claude-opus-5-5
**Requested execution:** automatic
**Requested at:** 2026-10-22T10:24:27Z
**Workflow:** regular
**Check required:** no
**Reviewer adapter:** claude
**Reviewer model:** claude-opus-5-5
**Reviewer context:** fresh subagent
**Actual execution:** automatic
**Reviewed at:** 2026-10-22T10:27:01Z
**Scope:** current
**Lenses:** quality, security, performance, tests
**Verdict:** passed
**Check result:** not-required

### Commands

- `git rev-parse HEAD`, `git merge-base main HEAD`, `sha256sum blueprint/context/current-feature.md`, `git status --porcelain --untracked-files=all`: pass (preconditions match; only review.md dirty)
- `make lint` (backend): pass, 0 issues
- `make test` (backend): pass
- `go test -tags integration -count=1 ./internal/stats/...` (backend, Docker): pass
- `npm run lint` (frontend): pass
- `npm run typecheck` (frontend): pass
- `npm test` (frontend): pass, 35 files, 380 tests
- `npm run e2e`: unavailable (not run by instruction; needs the e2e backend)

### Evidence

- Delta 7559d69..c0707d0 reviewed in full: 28 files (stats store/service/handler/route, Swagger and generated types, Tracker tab and lib, unit/integration/e2e tests, docs).
- Store query binds role name, year and month as parameters; `chk_reports_month` (exact casing) plus `idx_reports_unique_per_user` (created_by, month, year WHERE deleted_at IS NULL) give at most one joined report per account, so rows cannot duplicate; `users.name`/`status` are NOT NULL and `church_name` is COALESCEd.
- Route is in the admin group with `perm("reports:read_any")`; `TestTrackerRoute_AdminWithReadAny` covers 401, plain user, editor with read_any, admin without read_any (403) and admin (200).
- Response carries no report text, IP or report id; the UI renders names, emails and churches as React text only.
- Integration test covers approved/pending/none, admin/editor/soft-deleted user excluded, soft-deleted and other-period reports as not_submitted, inactive user present, case-insensitive church order.
- Frontend tests cover every state in the spec (loading, counts, progress, search, filter, counts unchanged by filters, no match, empty, refetch on month change, error with Retry, 403) and tab order/`?tab=tracker`.

### Findings

- F-12 [P3] open: migration-procedure doc change leaves backend/SETUP.md and project-overview.md saying pre-deploy (non-blocking)

### Remaining risk

- `npm run e2e` (tracker.spec.ts, a11y.spec.ts Tracker scans, 320 px reflow) not run here: axe contrast of the status badges and the progress bar in both themes is unverified in a browser.
- Check was not required and was not run.
- Dashboard activity (`blueprint/.state/run.json`) was not published: the reviewer was limited to writing findings.md and review.md.

## Try it now

Run or do:

In `backend/`: `make db-start`, then `make dev`. In `frontend/`: `npm run dev`. Sign in at http://localhost:3000 as the admin (your local `ADMIN_EMAIL` / `ADMIN_PASSWORD`) and open **http://localhost:3000/admin?tab=tracker**.

Expected:

The tabs read **Reports · Tracker · Users · Logs** with **Tracker** selected; Month and Year show the previous Manila month; five counts (**Expected, Submitted, Approved, For Review, Not Submitted**), an "*S* of *E* submitted" label with a progress bar, an "*N* of *M* workers" line, and a **Worker / Church / Status** table with one row per `user`-role account, each **Approved**, **For Review** or **Not Submitted**. Admin and editor accounts are not listed.

This proves:

The Tracker tab is wired to `GET /admin/tracker` and shows the D-A3 set (user role only, previous month by default) with each account's status.
