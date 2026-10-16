# Feature: Stats endpoint and cards

**From build-plan:** feature 20b
**Build attempt:** 1
**Branch:** feature/stats-endpoint-and-cards
**Status:** verified

## Goal

Give the admin panel its four statistics cards: an admin-only `GET /api/v1/admin/stats` that returns counts for one month, and a row of cards above the tabs on `/admin` that show them. Counts only: no report text, no names. No schema change, no migration.

## In scope

- **Backend:** `GET /api/v1/admin/stats` in the existing `/admin` group (already `RequireAuth` + `RequireRole admin`), behind the read rate limiter.
- Optional query `month` (English month name, case-insensitive, as `GET /reports`) and `year` (positive int). Both or neither: only one of them is a 422 validation error. With neither, the period is the **previous month in Asia/Manila** (D-A7, D-F8).
- Response (JSON, snake_case): `month` (string, canonical casing from `model.Months`), `year` (int), `reports_total`, `reports_approved`, `reports_pending`, `users_total`, `admins_total` (all non-negative ints).
- Definitions: reports = rows in `reports` with `deleted_at IS NULL` for that `month`+`year`; approved = `completed`; pending = not `completed`; users = every row in `users`; admins = users whose role name is `admin`. `users_total` and `admins_total` do not depend on the period.
- Swagger annotations on the handler; `make swagger` and `npm run gen:api` in the same commit as the handler, in step 3b (CI fails on drift in `frontend/src/lib/api-types.ts`).
- **Frontend:** `lib/stats.ts` (typed fetch), `StatsCards` component rendered above the tabs in `AdminPanel`, showing four cards: **Reports** (total, with the period label e.g. "September 2026"), **Approved**, **Pending**, **Users** (with "N admins" as its description).
- Tests (Go unit, Go integration, Vitest, one e2e/axe update) and a short `docs/API.md` entry.

## Out of scope

- Month/year picker on the cards, or tying them to the Reports tab filters (20c decides; the optional query params keep that possible without an API change).
- Tracker counts (Phase B), Logs, user status/active counts (Phase C).
- Caching, materialised counts, a new dependency, a new permission, any migration.
- Per-church or per-area breakdowns.
- Changing `GET /reports`, the `Reports`/`Users` tab placeholders, or the header.

## Build loop

`workflow.stepReview` is `feature`: review once after all steps. `checkpointCommits` is disabled; `/complete` makes the final commit. Test-first within each step. Stop any dev server before `npm run build`. Backend commands run from `backend/`, frontend from `frontend/`.

## Build steps

- [x] 1. **Repository counts (tests first).** Add a `StatsRepository` interface (one method taking `ctx`, month and year, returning the five counts) and `StatsRepo` in `backend/internal/repository/`, using the project's statement builder and a read on the pool (not a transaction). Add a small mock beside `mock_report_repo.go` if the service test needs it, and the compile-time check in `interfaces.go`. Write the integration test first (`//go:build integration`, testcontainers, like `report_repo_integration_test.go`).
  Done when: `make test-integration` passes cases for an empty database (all zeros), mixed approved/pending reports in the period, a soft-deleted report not counted, a report in another month/year not counted, and admins counted by role name versus all users.
- [x] 2. **Stats service with the default period (tests first).** `StatsService.Get(ctx, month *string, year *int)` in `backend/internal/service/`: both-or-neither rule, month name validated against `model.Months` (case-insensitive, canonical casing returned), default = previous month in `Asia/Manila` computed from an injectable clock (same approach as `ReportService.previousMonth`; do not couple the two services). Return a stable validation error the handler maps to 422.
  Done when: `make test` passes cases for January (previous month is December of last year), an explicit period, a lower-case month name, only-month and only-year (error), an unknown month (error), and a clock just before and just after midnight at the Manila month boundary.
- [x] 3a. **Handler and route (tests first).** `StatsHandler` with a `StatsServiceInterface` (like `ReportHandler`), parse `month`/`year` the way `ListReports` does (`year` positive int, else 400), errors via `apperror`. Wire in `cmd/api/main.go` and `router.Setup` as `admin.GET("/stats", ..., limits.Read)`.
  Done when: `make test` handler tests pass (200 body shape, bad year 400, service validation 422); a router-level check proves a non-admin gets 403 and an anonymous caller 401 (use the existing router test pattern, otherwise an integration case); `make lint` passes.
- [x] 3b. **Swagger and generated types.** Add godoc annotations to the handler (`@Tags admin`, `@Security CookieAuth`, 200/400/401/403/422), run `make swagger`, then `npm run gen:api`.
  Done when: `git diff` shows `backend/docs/*` and `frontend/src/lib/api-types.ts` changed only for the stats route and schema, `npm run typecheck` still passes, and running both generators again leaves no further diff.
- [x] 4. **Frontend fetch and cards (tests first).** Add a `Stats` type alias in `lib/api-schema.ts` (required fields via the existing `Always`/`Required` helpers), `lib/stats.ts` `getStats()`, and `app/(app)/admin/stats-cards.tsx` using `Card` and TanStack Query (`enabled: isAdmin`, short `staleTime`, same style as `users-list.tsx`). States: loading (`role="status"`), error (`role="alert"`, the API message), data (four cards with numbers; each number has a visible text label, not colour alone; zero renders as `0`, not blank). Render it in `AdminPanel` above the tabs, only for admins.
  Done when: `npm test` passes cases for loading, error, the four values with the period label, zeros, and that it is not fetched for a non-admin; `npm run lint` and `npm run typecheck` pass.
- [x] 5. **Browser check and docs.** Extend `e2e/a11y.spec.ts` so the existing `/admin` axe and 320 px reflow cases wait for the cards to load before scanning. Add `GET /admin/stats` to `docs/API.md`. Run the full suites.
  Done when: `npm test`, `npm run lint`, `npm run typecheck`, `npm run build` (dev server stopped first), `npm run e2e`, `make lint`, `make test` and `make test-integration` pass, or any that cannot run is named in the summary with the reason.

## Files / areas

- Backend new: `internal/repository/stats_repo.go` (+ integration test, mock if needed), `internal/service/stats_service.go` (+ test), `internal/handler/stats_handler.go` (+ test), a small `model` type for the response.
- Backend edit: `internal/repository/interfaces.go`, `internal/router/router.go`, `cmd/api/main.go`, `backend/docs/swagger.{json,yaml}` and `docs.go` (generated).
- Frontend new: `src/lib/stats.ts`, `src/app/(app)/admin/stats-cards.tsx` (+ test).
- Frontend edit: `src/lib/api-schema.ts`, `src/lib/api-types.ts` (generated), `src/app/(app)/admin/admin-panel.tsx` (+ its test), `e2e/a11y.spec.ts`.
- Docs: `docs/API.md`.
- Read only: `internal/service/report_service.go` (period logic), `internal/handler/report_handler.go` (query parsing), `internal/repository/report_repo.go` (query style), the `users` and `roles` migrations.

## Data / contracts

- No schema or migration. Reads `reports` (`month`, `year`, `completed`, `deleted_at`), `users`, `roles`.
- Route: `GET /api/v1/admin/stats?month=<Name>&year=<int>`; admin role required (the role gate is the authorization; the service takes no user id). Rate limit: `limits.Read` after the role check.
- Response:
  ```json
  {"month":"September","year":2026,"reports_total":0,"reports_approved":0,"reports_pending":0,"users_total":0,"admins_total":0}
  ```
  `reports_total = reports_approved + reports_pending` always. Errors use the existing `apperror` JSON shape.
- Counts carry no narrative text, names or emails, so no D-18 redaction applies. No audit row: it is a read.
- Month matching uses the same case-insensitive comparison as `ReportRepo.List`; the response returns the canonical name from `model.Months`.
- Frontend: the Swagger type lists fields as optional, so `api-schema.ts` marks them required (the server always sends them).
- Accessibility: cards are a labelled group; the loading and error messages are announced; numbers are text; the layout reflows at 320 px (e2e check).

## Testing

- Go: `make test` (unit), `make test-integration` (needs Docker), `make lint`, `make swagger` then check `git diff` for generated files.
- Frontend: `npm test`, `npm run lint`, `npm run typecheck`, `npm run build`, `npm run e2e` (the e2e backend starts itself against a fresh database; the repo's test Postgres must be up on :5440).
- Not claimed unless run: any production or live-data check.

## Notes for the AI

- Layering is handler → service → repository (`backend/docs/ARCHITECTURE.md`); the service here is thin but holds the period rule, so keep it out of the handler and the SQL.
- Reuse `apperror`, the statement builder, and the `ReportHandler` interface pattern. No new dependency, no generic "stats framework".
- One query (or a few cheap `COUNT`s) is enough; do not paginate or cache.
- Next.js 16 differs from older versions: read `frontend/AGENTS.md` before frontend work; do not edit it.
- Never run the Playwright suite against production.

## Open questions

- None. Resolved by the owner: the four cards are Reports, Approved, Pending and Users (with "N admins" as its description), and `month`/`year` stay optional with the previous Manila month as the default.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":9581,"specSha256":"20f719d93ca56e1a52f0f9f64d0a4616b82065c8cf240de9bbaa2d2586901a5e","branch":"refs/heads/feature/stats-endpoint-and-cards","head":"88d989906eed00d6c4e5593d55282c4e7f1e084c","baseRef":"refs/heads/main","baseCommit":"5357e68d77dbc38268d99a0da473e97a77e1b02e","sourceTree":"a48bde09e6499a4dec7a1ec1f81943c23bfce369","landing":"local-merge","absentOptional":[]} -->
