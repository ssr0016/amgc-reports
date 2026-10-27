# Report Sidebar and Delete Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Keep worker report creation limited to one report for the previous Manila month, show the correct action in both navigation surfaces, and make every user-facing delete immediately return to a report list with the requested success feedback.

**Architecture:** Reuse the existing `GET /reports/current` query and its `['current-report', userId]` cache key. Add one small shared action-mapping helper so the sidebar and community reports header cannot disagree; delete mutations invalidate that key plus report-list/admin keys before redirecting.

**Tech Stack:** Next.js 16, React 19, TypeScript, TanStack Query, Vitest/Testing Library, Sonner.

**Spec:** User request in the conversation; existing previous-month rule in `docs/ARCHITECTURE.md` and `backend/docs/07-Reports-Decisions.md` remains unchanged.

## Global Constraints

- Preserve the previous-month filing rule in Asia/Manila; do not change backend period validation or API contracts.
- Do not add dependencies, migrations, Swagger changes, or generated API changes.
- Success copy is exactly `Report deleted successfully.`
- Do not commit or push until the user approves the diff and verification passes.

## Review Focus

- No current-period report: both sidebar and `/reports` header show `New Report`.
- Pending/approved report: both surfaces show `View Report` and link to the report view.
- Needs-revision report: both surfaces show `Revise Report` and link to edit.
- Draft report: preserve the existing `Continue Draft` behavior.
- Delete success, 404-as-already-deleted, and delete failure: success redirects/invalidation happen only for success or accepted 404; failures keep the existing error state.

### Task 1: Shared action mapping and creation entry points

**Files:**
- Modify: `frontend/src/lib/reports.ts`
- Modify: `frontend/src/app/(app)/sidebar.tsx`
- Modify: `frontend/src/app/(app)/reports/reports-list.tsx`
- Test: `frontend/src/app/(app)/sidebar.test.tsx`
- Test: `frontend/src/app/(app)/reports/reports-list.test.tsx`

- [ ] Add a pure `reportAction(report)` mapping for null, draft, needs_revision, pending, and approved reports, returning the label and destination.
- [ ] Replace the sidebar’s local nested status expression with the shared mapping and use the returned destination instead of always linking to `/reports/new`.
- [ ] Query the same `['current-report', userId]` key in `ReportsList` and render the mapped action in the header; keep the header absent while the user lacks worker write permission.
- [ ] Add tests for all status labels/destinations and the no-report header state. Keep previous-month calculation in `defaultPeriod()` unchanged.

### Task 2: Community and admin list delete behavior

**Files:**
- Modify: `frontend/src/app/(app)/reports/reports-list.tsx`
- Modify: `frontend/src/app/(app)/admin/admin-reports.tsx`
- Test: `frontend/src/app/(app)/reports/reports-list.test.tsx`
- Test: `frontend/src/app/(app)/admin/admin-reports.test.tsx`

- [ ] On successful community deletion, invalidate `['current-report', userId]` and existing report/admin views, show `Report deleted successfully.`, and replace the route with `/reports`.
- [ ] Preserve the existing 404-as-deleted behavior and pagination correction.
- [ ] Apply the same success copy and invalidations to admin deletion, then replace with `/admin?tab=reports`.
- [ ] Assert toast text, redirect, and current-report invalidation in focused tests; retain failure-dialog coverage.

### Task 3: Worker edit/new delete flow

**Files:**
- Modify: `frontend/src/app/(app)/reports/[id]/edit/edit-report.tsx`
- Modify: `frontend/src/app/(app)/reports/new/new-report.tsx`
- Test: `frontend/src/app/(app)/reports/[id]/edit/edit-report.test.tsx`
- Test: `frontend/src/app/(app)/reports/new/new-report.test.tsx`

- [ ] Add the existing alert-dialog pattern around an owner-authorized delete action on the edit page; on success invalidate the report, current-report, reports, and admin views, show the exact success toast, and replace with `/reports`.
- [ ] Keep 404 handling as the existing deleted state, without claiming a second successful deletion.
- [ ] Align the `/reports/new` draft replacement delete with the same cache invalidation and success copy, then return to `/reports`; the user can click the now-visible `New Report` action to start over.
- [ ] Add focused tests for delete confirmation, API call, toast, redirect, and current-report invalidation.

### Task 4: Verification and handoff

- [ ] Run the focused Vitest files after each test-first change.
- [ ] Run the requested full frontend suite, typecheck, lint, and production build from `frontend/`.
- [ ] Inspect the final diff and working tree; only after approval and green gates create the requested commit and push to `main`.

