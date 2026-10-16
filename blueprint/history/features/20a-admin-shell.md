# Feature: Admin shell

**From build-plan:** feature 20a
**Build attempt:** 1
**Branch:** feature/admin-shell
**Status:** verified

## Goal

Add the `/admin` page: an admin-only screen with an accessible tab list (arrow-key navigation), a page title, and a header link. It is the frame that 20b (stats cards), 20c (Reports tab), 20d (Users tab) and 20e (preview modal) fill in. Frontend only, no schema or API change.

## In scope

- Route `/admin` (`frontend/src/app/(app)/admin/page.tsx`) with `metadata.title = "Admin"` and an `<h1>Admin</h1>`.
- A client component `AdminPanel` with two tabs, **Reports** and **Users**, built on a new shadcn-style Radix Tabs wrapper (`components/ui/tabs.tsx`; `radix-ui` is already installed, so no new dependency). Each tab panel holds a short placeholder line until 20c and 20d fill it.
- Guard: only `isAdmin` sees the tabs. Anyone else sees "You are not allowed to view the admin panel." (same pattern as `users-list.tsx`). The server still enforces the role on every admin endpoint.
- Header link "Admin" to `/admin`, shown only to admins.
- Unit tests, one e2e a11y entry, and a title test entry.

## Out of scope

- The stats cards and endpoint (20b), tab contents (20c, 20d), preview modal (20e).
- The Tracker and Logs tabs: they belong to Phase B (the plan's design shows four tabs, but only Reports and Users exist in Phase A).
- Removing the existing header "Users" link or the `/admin/users` routes. 20d decides what happens to them.
- URL-synced tab state (`?tab=`). The tab lives in component state; add deep links only if the owner asks.
- Any backend change, new permission, or new API call.

## Build loop

`workflow.stepReview` is `feature`: review once after all steps, not after each. `checkpointCommits` is disabled, so no commits until `/complete`. Work test-first. Do not start the dev server; stop it before any build.

## Build steps

- [x] 1. **Tabs wrapper.** Add `frontend/src/components/ui/tabs.tsx` exporting `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` over `Tabs as TabsPrimitive` from `radix-ui`, in the style of `alert-dialog.tsx` (`data-slot`, `cn`). Visible focus ring on triggers; the active tab is distinguishable without colour alone.
  Done when: `npm run typecheck` and `npm run lint` pass with the new file (it is exercised by step 2's tests).
- [x] 2. **AdminPanel and page (tests first).** Write `admin-panel.test.tsx` beside the component first, then `admin-panel.tsx` and `page.tsx` (same `instant = false` Server Component page pattern as `admin/users/page.tsx`). Add `["./(app)/admin/page", "Admin"]` to `app/titles.test.ts`.
  Done when: `npm test` passes with cases for: admin sees `role="tablist"` with tabs named Reports and Users, Reports selected first; ArrowRight/ArrowLeft/Home/End move selection and focus; only the active panel is shown; a non-admin sees the not-allowed message and no tablist; the page title is "Admin".
- [x] 3. **Header link.** In `app/(app)/layout.tsx`, add an `Admin` link to `/admin` beside the admin-only `Users` link. Extend `layout.header.test.tsx` (admin sees it) and `layout.test.tsx` (a non-admin does not).
  Done when: those tests pass and the header row still has `flex-wrap` (320 px reflow test unchanged).
- [x] 4. **Browser a11y.** Add `["admin", "/admin", /Admin/]` to the signed-in list in `e2e/a11y.spec.ts` (heading, no sideways scroll at 320 px, axe scan). If the e2e stack can run here, run that spec; if not, say so in the result.
  Done when: the case passes locally, or the summary states it was not run and why.

## Files / areas

- New: `frontend/src/components/ui/tabs.tsx`, `frontend/src/app/(app)/admin/page.tsx`, `frontend/src/app/(app)/admin/admin-panel.tsx`, `frontend/src/app/(app)/admin/admin-panel.test.tsx`.
- Edit: `frontend/src/app/(app)/layout.tsx`, `layout.test.tsx`, `layout.header.test.tsx`, `frontend/src/app/titles.test.ts`, `frontend/e2e/a11y.spec.ts`.
- Read only: `frontend/src/lib/session.tsx` (`isAdmin`), `admin/users/users-list.tsx` (guard pattern).

## Data / contracts

- No API, Swagger, schema or `api-types.ts` change.
- `isAdmin` is `user.role === "admin"` from `/auth/me`; it only hides UI. Hiding is not authorization.
- Tab order is fixed: Reports, Users. 20c and 20d fill the existing panels and must not reorder them.
- Accessibility: Radix provides `tablist`/`tab`/`tabpanel`, roving tabindex and arrow keys; the page has one `<h1>`; the title changes on navigation (WCAG 2.4.2); the skip link and `#main-content` landmark come from the layout.

## Testing

- Vitest with Testing Library (mock `@/lib/session` as the existing layout tests do). Checks: `npm test`, `npm run lint`, `npm run typecheck`.
- Playwright + axe entry in `a11y.spec.ts` (needs the e2e backend; see `e2e/start-backend.sh`). Not claimed unless run.
- `npm run build` once at the end, with the dev server stopped.

## Notes for the AI

- Next.js 16 differs from older versions: read `frontend/AGENTS.md` before writing the page. Do not edit that file.
- Match the surrounding code: `"use client"` components, `@/` imports, the not-allowed copy from `users-list.tsx`. No new dependency, no abstraction beyond the wrapper.
- Do not run the Playwright suite against production.
- Placeholders in the panels are plain text, not a "coming soon" feature surface.

## Open questions

- None blocking. Chosen defaults to confirm: two tabs only (Reports, Users) in this phase; tab state in memory, not the URL; keep the existing "Users" header link next to the new "Admin" link.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":5525,"specSha256":"5d5e3a9fc606b4efce2c79ae04e9b1ac45dbc9da43a6d895161f4df26b523549","branch":"refs/heads/feature/admin-shell","head":"9215139409c2007f055f23fb59c5d0725a0db7d4","baseRef":"refs/heads/main","baseCommit":"796112388a45f3834d93843ebb8115ea190c1673","sourceTree":"22a632bdd623ae3c4b7170d4f4db886d7d6c68a6","landing":"local-merge","absentOptional":[]} -->
