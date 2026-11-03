# Admin Overview and Analytics Separation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an Admin Overview destination for operational KPI cards and make Analytics clearly about ministry trends and deeper analysis.

**Architecture:** Reuse the existing `StatsCards` component and stats query. Add `overview` to the existing admin section registry and render it as the first allowed section. Keep Analytics data, charts, filters, exports, permissions, and API contracts unchanged.

**Tech Stack:** Next.js, React, TypeScript, Tailwind, TanStack Query, Vitest, Testing Library.

**Spec:** Approved conversation request for Approach 1, Admin Overview and Analytics separation.

## Global Constraints

- Do not add dependencies, backend endpoints, migrations, or API changes.
- Keep sidebar label exactly `Analytics`.
- Keep KPI labels exactly `Total Users`, `Expected Reports`, `No Report`, and `Total Churches`.
- Do not commit or push unless explicitly requested.
- Run only focused frontend tests; do not run the full test suite.

## Review Focus

- Missing or unknown `?tab=` must now resolve to Overview when permitted. Test in `admin-panel.test.tsx`.
- Users without `tracker:read` must not fetch or render Overview cards. Preserve the existing `stats-cards.test.tsx` permission test.
- Analytics must not render the shared operational KPI cards. Test in `admin-panel.test.tsx`.
- Sidebar fallback must select Overview for a permitted user whose requested tab is unavailable. Test in `sidebar.test.tsx`.
- Overview, Analytics, and Users must retain distinct accessible region names. Test in `stats-cards.test.tsx` and `analytics.test.tsx`.

## UX Copy

- Sidebar: `Overview`, `Analytics`.
- Admin page title on Overview: `Admin / Overview`.
- Overview description: `Operational snapshot of users, reporting coverage, and churches.`
- Overview region label: `Overview`.
- Analytics page title remains the global `Admin / Analytics`.
- Analytics content heading: `Ministry Insights & Trends`.
- Remove the Analytics content heading's existing descriptive paragraph. The heading itself supplies the page context; filters and chart section descriptions remain.

### Task 1: Add Overview to the Admin section model and route rendering

**Files:**
- Modify: `frontend/src/lib/admin-navigation.ts`
- Modify: `frontend/src/app/(app)/admin/admin-panel.tsx`
- Test: `frontend/src/app/(app)/admin/admin-panel.test.tsx`

**Interfaces:**
- `adminSections()` produces `{ value: "overview", label: "Overview", allowed: can("tracker:read") }` before Reports.
- `AdminPanel` maps `overview` to `<StatsCards />` and uses Overview as the first permitted fallback.

- [ ] Add the `overview` section before `reports` in `adminSections`, using the existing `tracker:read` permission.
- [ ] Add `overview: <StatsCards />` to the `panels` map.
- [ ] Render `StatsCards` only for `overview`; retain the `users` variant only on Users and retain `RoleStats` only on Roles.
- [ ] Prevent Analytics from receiving the shared `StatsCards` row.
- [ ] Keep Overview outside the generic inner `Card` wrapper so KPI cards retain their current spacing and card presentation.
- [ ] Pass the exact Overview description through the existing `PageHeader` when `tab === "overview"`.
- [ ] Update tests for default and unknown tabs, the Overview heading, Overview content, permission fallback, and the absence of shared stats on Analytics.
- [ ] Run: `cd frontend && npm test -- --run src/app/\(app\)/admin/admin-panel.test.tsx`

### Task 2: Update sidebar labels and navigation expectations

**Files:**
- Modify: `frontend/src/app/(app)/sidebar.test.tsx`
- Test: `frontend/src/lib/admin-navigation.ts` indirectly through sidebar tests

**Interfaces:**
- Sidebar consumes the existing `adminSections()` output. No sidebar component API changes are needed.

- [ ] Update expected admin links to include `/admin?tab=overview` before Reports.
- [ ] Assert visible labels include `Overview` and `Analytics`.
- [ ] Update current-link expectations so `/admin` and unknown tabs select `Admin Overview`.
- [ ] Add or update the restricted-permission fallback assertion so `tracker:read` selects `Admin Overview`.
- [ ] Keep the Analytics link href exactly `/admin?tab=analytics` and label exactly `Admin Analytics` in accessible-name assertions.
- [ ] Run: `cd frontend && npm test -- --run src/app/\(app\)/sidebar.test.tsx`

### Task 3: Re-label the shared stats component for Overview ownership

**Files:**
- Modify: `frontend/src/app/(app)/admin/stats-cards.tsx`
- Modify: `frontend/src/app/(app)/admin/stats-cards.test.tsx`

**Interfaces:**
- `StatsCards({ users?: boolean })` keeps its existing prop and data source.
- Default cards remain `Total Users`, `Expected Reports`, `No Report`, and `Total Churches`.
- Users mode keeps `Total Users`, `Roles Breakdown`, and `Statuses Breakdown`.

- [ ] Change the default KPI section accessible name from `Statistics` to `Overview`.
- [ ] Give Users mode a distinct accessible name such as `User statistics`, without changing its card data.
- [ ] Keep loading, error, retry, zero values, permission behavior, and `admin-stats` query key unchanged.
- [ ] Update tests to assert the Overview region and preserve all KPI values and notes.
- [ ] Update Users-mode tests for its new region name.
- [ ] Run: `cd frontend && npm test -- --run src/app/\(app\)/admin/stats-cards.test.tsx`

### Task 4: Clarify Analytics heading and verify separation

**Files:**
- Modify: `frontend/src/app/(app)/admin/analytics.tsx`
- Modify: `frontend/src/app/(app)/admin/analytics.test.tsx`

**Interfaces:**
- `Analytics` keeps all existing filters, charts, exports, query behavior, permissions, and internal section labels.

- [ ] Replace the inner `Analytics` heading with `Ministry Insights & Trends`.
- [ ] Remove the adjacent redundant descriptive paragraph only.
- [ ] Do not rename the sidebar item, global Admin page title, analytics scope label, chart headings, or export actions.
- [ ] Update the analytics test to assert `Ministry Insights & Trends` and absence of the removed paragraph text.
- [ ] Keep existing KPI tests for analytical metrics such as attendance and giving; these are Analytics insights, not the shared operational cards.
- [ ] Run: `cd frontend && npm test -- --run src/app/\(app\)/admin/analytics.test.tsx`

### Task 5: Focused integration verification

**Files:**
- No source changes expected.

- [ ] Run the four focused test files together:
  `cd frontend && npm test -- --run src/app/\(app\)/admin/admin-panel.test.tsx src/app/\(app\)/sidebar.test.tsx src/app/\(app\)/admin/stats-cards.test.tsx src/app/\(app\)/admin/analytics.test.tsx`
- [ ] Confirm Overview is the default Admin destination, Analytics has no shared operational KPI row, and the sidebar still exposes both labels.
- [ ] Confirm no backend, generated API type, migration, dependency, or unrelated UI files changed.

No commit is planned until the user explicitly requests one.
