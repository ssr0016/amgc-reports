# Premium UX and Reliability Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the frontend resilient to deleted reports and consistently communicate loading, mutation, navigation, background-fetch, and empty states.

**Architecture:** Reuse existing React Query, Tailwind, Radix, and Lucide patterns. Add small shared UI primitives for skeletons, empty states, and global activity, then replace loading/action states at existing screen boundaries without changing API contracts.

**Tech Stack:** Next.js 16.4, React 19, TypeScript, TanStack Query 5, Tailwind 4, Vitest, Testing Library.

**Spec:** `docs/superpowers/specs/2026-10-27-premium-ux-reliability-design.md`

## Global Constraints

- No new runtime dependency.
- No API or database changes.
- Preserve access-control, error handling, and dirty-form behavior.
- Do not commit secrets or generated Next.js agent instructions.

## Review Focus

- A report deleted after load must never be submitted or saved as a new record; test the 404 mutation path.
- A 404 initial report load must provide a recovery link; test the user-visible deleted state.
- Skeletons must retain the main layout rather than only announcing text; test representative roles/classes.
- Empty states must retain access controls and provide only valid CTAs; test reports and admin data views.
- Global indicators must not block interaction or remain visible after navigation/fetch completion; test their visibility transitions.

### Task 1: Shared loading, empty, and global activity primitives

**Files:**
- Create: `frontend/src/components/ui/skeleton.tsx`
- Create: `frontend/src/components/empty-state.tsx`
- Create: `frontend/src/components/global-activity-indicator.tsx`
- Modify: `frontend/src/app/layout.tsx`
- Test: `frontend/src/components/ui/skeleton.test.tsx`
- Test: `frontend/src/components/empty-state.test.tsx`
- Test: `frontend/src/components/global-activity-indicator.test.tsx`

- [ ] Write failing component tests.
- [ ] Verify the focused tests fail for the missing components.
- [ ] Implement the primitives and root integration.
- [ ] Verify focused tests pass.

### Task 2: Deleted report protection

**Files:**
- Modify: `frontend/src/app/(app)/reports/[id]/edit/edit-report.tsx`
- Modify: `frontend/src/lib/query.tsx`
- Create: `frontend/src/app/(app)/reports/[id]/edit/edit-report.test.tsx`

- [ ] Write failing tests for initial 404, Save Draft 404, Submit 404, invalidations, and focus refetch configuration.
- [ ] Verify failure.
- [ ] Implement deleted-state UI and mutation/cache handling.
- [ ] Verify focused tests pass.

### Task 3: Login and mutation busy feedback

**Files:**
- Modify: `frontend/src/app/login/page.tsx`
- Modify: `frontend/src/components/report-form.tsx`
- Modify: `frontend/src/app/(app)/reports/new/new-report.tsx`
- Modify: `frontend/src/app/(app)/reports/reports-list.tsx`
- Modify: `frontend/src/app/(app)/admin/admin-reports.tsx`
- Modify: `frontend/src/components/use-report-actions.ts`
- Modify: `frontend/src/app/(app)/account/profile/profile-picture.tsx`
- Modify: focused existing tests in the affected paths.

- [ ] Add/adjust failing busy-state tests.
- [ ] Verify failure.
- [ ] Add `Loader2`, disabled states, and `aria-busy` without changing mutation semantics.
- [ ] Verify focused tests pass.

### Task 4: Layout-shaped skeletons and empty states

**Files:**
- Modify: `frontend/src/app/(app)/reports/reports-list.tsx`
- Modify: `frontend/src/app/(app)/admin/tracker.tsx`
- Modify: `frontend/src/app/(app)/admin/analytics.tsx`
- Modify: `frontend/src/app/(app)/admin/users/users-list.tsx`
- Modify: `frontend/src/app/(app)/account/profile/profile-picture.tsx`
- Modify: `frontend/src/app/(app)/admin/admin-reports.tsx`
- Modify: `frontend/src/app/(app)/admin/audit-logs.tsx`
- Modify: `frontend/src/app/(app)/admin/roles.tsx`
- Modify: `frontend/src/app/(app)/admin/roles/new/new-role.tsx`
- Modify: `frontend/src/app/(app)/admin/roles/[id]/edit/edit-role.tsx`
- Modify: `frontend/src/app/(app)/admin/users/new/new-user.tsx`
- Modify: `frontend/src/app/(app)/admin/users/[id]/edit/edit-user.tsx`
- Modify: `frontend/src/app/(app)/admin/report-preview-dialog.tsx`
- Modify: `frontend/src/app/(app)/admin/excel-preview-dialog.tsx`
- Modify: `frontend/src/app/(app)/layout.tsx`
- Modify: affected existing tests.

- [ ] Add representative failing assertions for skeleton layout and actionable empty states.
- [ ] Verify failure.
- [ ] Replace generic fetch loaders while preserving errors and permission states.
- [ ] Verify focused tests pass.

### Task 5: Route transition feedback and final verification

**Files:**
- Modify: `frontend/src/components/global-activity-indicator.tsx` as needed.
- Modify: affected global tests.

- [ ] Verify route-progress and background-fetch transitions.
- [ ] Run `cd frontend && npm test`.
- [ ] Run `cd frontend && npm run typecheck`.
- [ ] Run `cd frontend && npm run lint`.
- [ ] Run `cd frontend && npm run build`.
- [ ] Review the full diff, commit, and push `main`.
