# Feature: Users tab
**From build-plan:** feature 20d
**Build attempt:** 1
**Branch:** none (built on `main` outside the `/feature` flow; archived afterwards)
**Status:** verified
**Commit:** be31ad5

> Archived after the fact. There was no spec in `current-feature.md` while this was built, so the steps below are reconstructed from the commit and the session.

## Goal
Fill the Users tab of the admin panel (`/admin`) with the existing users list, add a "You" badge on the signed-in user's row, and add a Role filter. Frontend only: no API, schema or Swagger change.

## In scope
- `Users` is split into `Users` (the `/admin/users` page: `<main>`, `<h1>`, then the panel) and `UsersPanel` (search, Role filter, "New user", table), so the page and the tab share one component. The standalone page stays as it was.
- "You" badge next to the name on the row where `u.id === user?.user_id`.
- Role filter: `NativeSelect` labelled "Role", default "All roles", options from `listRoles`. Client-side (D-19).
- "New user" moves from beside the `<h1>` into the panel toolbar, so it is the same on the page and in the tab (owner's call).
- `AdminPanel` mounts `UsersPanel` in place of the placeholder.

## Out of scope
- A server-side role filter (a `role_id` query parameter, Swagger and `gen:api`). Upgrade when the user count exceeds `LIMIT` (100).
- Any backend change; preview modal and Avg column (20e); sidebar and profile menu (20f).

## Build steps
- [x] 1. **`UsersPanel` (tests first).** In `users-list.test.tsx`: You badge only on the signed-in row, Role filter narrows the loaded rows, a role with no loaded users shows "No users match.", and the panel has no `main` landmark or level-1 heading. Red first (4 failing), then the split, badge and filter.
- [x] 2. **Tab (tests first).** In `admin-panel.test.tsx` the Users tab shows `UsersPanel` instead of the placeholder. Red first (1 failing), then `admin-panel.tsx`.
- [x] 3. **Docs.** D-19 in `docs/DECISIONS.md`; 20d checked in `blueprint/build-plan.md`.

## Decisions
- **D-19:** the Role filter is client-side. It only sees the first `LIMIT` users. See `docs/DECISIONS.md`.

## Files / areas
- Edit: `frontend/src/app/(app)/admin/users/users-list.tsx` (+ `users-list.test.tsx`), `frontend/src/app/(app)/admin/admin-panel.tsx` (+ `admin-panel.test.tsx`), `docs/DECISIONS.md`, `blueprint/build-plan.md`.

## Verification
- `npm test`: 23 files, 206 tests passed (6 in `users-list.test.tsx`, 7 in `admin-panel.test.tsx`).
- `npm run typecheck` (`tsc --noEmit`) and `eslint` on `src/app/(app)/admin`: exit 0.
- `npm run e2e`: 51 passed (Playwright with axe).
- Not run: `npm run build`.

## Known limitations
- Role filter is client-side (D-19).
- **Axe gap:** `e2e/a11y.spec.ts` covers `/admin` and the standalone `/admin/users`, but never opens the Users tab, so the tab's filter and badge are covered by unit tests only. Follow-up: add an axe check on the Users tab.
