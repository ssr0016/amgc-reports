# Feature: Reports of deleted and deceased users

**From build-plan:** feature 22c
**Build attempt:** 1
**Branch:** feature/reports-deleted-deceased-users
**Status:** verified

## Goal

Keep reports visible when their owner is deleted or deceased, and show the current owner state beside the worker in both report lists (D-A11).

## In scope

- Reports API: `owner_status` and optional `owner_deleted_at`, sourced from the current users row, on list, detail, create and update responses.
- Both the admin Reports tab and /reports show `Deleted user` for a deleted owner and `Archived` for a deceased owner. If both apply, show both badges. Preserve Completed/Pending and all existing actions.
- Refetch report queries after successful user status, delete and restore changes so owner badges update immediately.
- Preserve pagination, report soft-delete exclusions, ownership permissions, D-18 redaction and approval metadata responses.

## Out of scope

- Hiding reports, new report filters, changing approval/edit/delete permissions, schema changes, audit actions or hard deletion.

## Build loop

Feature-level review; no checkpoint commits. The user requested implementation and the full test suite, followed by the diff before committing. Preserve the unrelated go.work.sum change.

## Build steps

- [x] **1. API owner state.** Add report model/response fields and load current owner state with the report reads. Create/update return the same enriched response. Add model and store regression tests for active, deleted, deceased, both, restore and status reversal, pagination and report soft deletion. **Done when:** focused unit and database integration tests pass.
- [x] **2. Report badges and cache refresh.** Add one shared badge component using existing Badge styles, integrate into both Worker cells, and invalidate report queries on owner state mutations. Add UI tests for every badge combination and mutation invalidation. Regenerate Swagger and API types. **Done when:** focused frontend tests, lint and typecheck pass.
- [x] **3. Full verification and review.** Run backend unit/integration tests, lint, vet and build; frontend unit tests, lint, typecheck, build and Playwright. Review the diff against scope. **Done when:** all checks pass or external blockers are explicitly recorded; show the diff without committing.

## Files / areas

- backend/internal/report/model.go, model_test.go, reportimpl/store.go and integration tests
- backend/docs generated Swagger outputs; frontend/src/lib/api-types.ts
- frontend/src/components/report-owner-badges.tsx and tests
- frontend/src/app/(app)/admin/admin-reports.tsx and tests
- frontend/src/app/(app)/reports/reports-list.tsx and tests
- User status, delete and restore UI mutation success paths and tests

## Data / contracts

`owner_status` is the owner's current account status, independent of report approval. `owner_deleted_at` is the owner's deletion timestamp (omitted when not deleted), independent of the report's own hidden deleted_at. Neither field is accepted from report request bodies. No deletion reason, actor or private owner details are exposed. Badge conditions are independent. Owner deletion never excludes their reports; a deleted report itself remains excluded.

## Testing

Use existing Go, testcontainers, Vitest and Playwright runners. Test report content, totals, metrics and permission/redaction behavior alongside the new owner metadata. Use existing text badges for accessible labels in light/dark themes. No dependencies added.

## Notes for the AI

Follow 22b patterns and AGENTS.md (Go/pgx, not the stale Prisma/Clerk examples in coding-standards.md). Keep report metadata responses minimal. Preserve the 22b spec in history when advancing the active spec. No commit or push during this task.

## Verification evidence

- Backend: `make test` (race detector and coverage), `make test-integration` (Docker/testcontainers, race detector), `make lint` (0 issues), `make vet`, `make build`: all passed.
- Frontend: `npm run lint`, `npm run typecheck`, `npm test` (30 files, 313 tests), `npm run build`, `npm run e2e` (88 tests): all passed. Final lint ran after Playwright to avoid a race with its output directory cleanup.
- `make swagger` followed by `npm run gen:api` regenerates identical outputs. `git diff --check` passes.
- Fresh working-tree code review found no blocking correctness or security issues. Its low-severity frontend required-field finding was fixed and rechecked; final typecheck/tests/build pass. This is not a Blueprint immutable-checkpoint review receipt.
- Full browser coverage observes live status changes, delete, restore, independent badges in both lists, approval availability, and D-18 redaction of narrative and tithes while nonfinancial metrics remain visible. Axe found no violations in the report panel with both badges.
- The existing four P3 ledger findings remain outside this feature. No new unresolved findings. Configured manual Audit/Check gates were not invoked; the user-requested complete automated suite ran. No auth or sensitive-content authorization rules changed. A separate manual try guide is omitted because there is no known pre-existing owner-state fixture in the user's running app; the browser test creates and exercises its own isolated fixture.
- The completed 22b spec is preserved unchanged in history. Build-plan completion/feature commit is deferred until the user's diff review. Nothing was staged, committed, merged or pushed. The pre-existing go.work.sum change is excluded from the review patch.
