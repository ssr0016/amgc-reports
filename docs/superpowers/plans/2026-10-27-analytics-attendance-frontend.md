# Analytics Attendance Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a permission-aware Analytics tab that lets admins inspect approved or all-submitted annual attendance totals with charts and accessible tables.

**Architecture:** Add a small typed client wrapper for the existing attendance analytics endpoint, then render one client component in the existing admin panel. Reuse the existing `adminSections`, `NativeSelect`, `Card`, `Table`, session permission, query, loading, and error patterns; use Recharts only for visualization and keep the data tables as the non-visual reading path.

**Tech Stack:** Next.js 16.4, React 19, TypeScript, TanStack Query, Recharts, existing shadcn/Radix-inspired UI primitives, Vitest.

**Spec:** User request in the conversation and `docs/superpowers/plans/2026-10-27-analytics-attendance-backend.md`.

## Global Constraints

- The Analytics tab is available when `tracker:read` is available.
- Year defaults to the current browser year; metric defaults to `worship_service`; status defaults to `approved`.
- The API query refetches when year, metric, or status changes.
- Charts are summaries; the accessible tables below them expose the same trend and church data.
- Use existing metric labels, `FIRST_YEAR`, `NativeSelect`, table primitives, loading/error states, and API wrapper.
- Do not add a custom chart abstraction, decorative dashboard cards, or client-side aggregation.

## Review Focus

- 403 or missing permission: hide the tab and do not fetch analytics.
- Loading, error, and empty data: provide perceivable status text and retry behavior.
- Narrow viewport or zoom: controls and tables remain reachable without clipped text.
- Chart-only interpretation: the tables below charts contain equivalent values and headers.
- Filter changes: query keys include all three filters so stale results are not shown as current.

### Task 1: Dependency and typed API client

**Files:**
- Modify: `frontend/package.json`, `frontend/package-lock.json`
- Modify: `frontend/src/lib/api-schema.ts`
- Create: `frontend/src/lib/analytics.ts`
- Test: `frontend/src/lib/analytics.test.ts`

**Interfaces:**
- `AttendanceAnalytics`, `AttendanceTrendPoint`, and `AttendanceChurch` are exported from `api-schema.ts`.
- `getAttendanceAnalytics(year: number, metric: string, status: "approved" | "all")` calls `/admin/analytics/attendance` with URL-encoded query parameters.

- [ ] Install `recharts` and verify the package lock records it.
- [ ] Add the generated-schema aliases and client wrapper.
- [ ] Test the exact request path and response passthrough.

### Task 2: Navigation and Analytics panel

**Files:**
- Modify: `frontend/src/lib/admin-navigation.ts`
- Modify: `frontend/src/app/(app)/admin/admin-panel.tsx`
- Modify: `frontend/src/app/(app)/admin/admin-panel.test.tsx`
- Create: `frontend/src/app/(app)/admin/analytics.tsx`
- Test: `frontend/src/app/(app)/admin/analytics.test.tsx`

**Interfaces:**
- The tab value is `analytics`, label is `Analytics`, and it is placed immediately after `reports`.
- `Analytics` renders only for `tracker:read` and uses the existing panel card wrapper.

- [ ] Add the permission-gated navigation entry and panel mapping.
- [ ] Render current-year, Worship Service, and Approved defaults using native selects.
- [ ] Query the client wrapper with all filter values and expose loading, error, empty, and retry states.
- [ ] Render the weekly trend with a labelled Recharts line chart and the church totals with a labelled Recharts bar chart.
- [ ] Render equivalent accessible trend and church tables below the charts.
- [ ] Add tests for navigation order/permission, defaults, filter refetch, chart headings, table headers/data, and error/empty states.

### Task 3: Verification

**Files:**
- No additional files.

- [ ] Run the focused analytics and admin panel Vitest tests.
- [ ] Run `npm run typecheck` and `npm run lint`.
- [ ] Run the frontend build if the focused checks are clean.
