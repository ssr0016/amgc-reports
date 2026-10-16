# Feature: Sidebar navigation and profile menu

**From build-plan:** feature 20f
**Build attempt:** 1
**Branch:** feature/sidebar-navigation-and-profile-menu
**Status:** verified

> **Revised by the owner after the build (D-A9):** the sidebar has **no Users item**. Wherever this spec says Reports, Admin, Users, read Reports and Admin; Admin is marked on `/admin` and everything under it. `/admin/users` redirects to `/admin?tab=users` (`AdminPanel` reads `?tab=users` to pick the tab), `/admin/users/new` and `/edit` stay, and the unused standalone `Users` page component was removed.

## Goal

Move the app's navigation from the top bar to a left sidebar on every signed-in page, slim the top bar to the app name and the user, and move "Change password" into a menu on the user's avatar. Frontend only: no API, schema or route change.

## In scope

- **Sidebar** (`<nav aria-label="Main">`) on the left of every page under `frontend/src/app/(app)/`, so `/reports`, `/reports/new`, `/reports/:id/edit`, `/admin`, `/admin/users` (and its new/edit pages) and `/account/password`.
  - **Items and who sees them:** *Reports* → `/reports` for everyone. *Admin* → `/admin` and *Users* → `/admin/users` for the admin role only (`isAdmin`). Order: Reports, Admin, Users. A plain user or editor sees only *Reports*.
  - **Tracker and Logs are not rendered at all** until Phase B (item 21) builds their pages; no disabled placeholder, no route.
  - The current page is marked `aria-current="page"` and styled so it does not rely on colour alone. Matching: *Reports* for `/reports` and anything under it; *Admin* for `/admin` exactly; *Users* for `/admin/users` and anything under it. (`/account/password` marks none.)
  - Hiding an item is a convenience, not authorization: the pages and the API still enforce the role.
- **Top bar:** the app name "AMGCReports" (links to `HOME`, `/reports`) on the left. On the right: the **profile menu** trigger (the avatar with the user's name and role, as today) and the "Sign out" button. "Admin", "Users" and "Account" are removed from it.
- **Profile menu:** the avatar and name are one button that opens a menu (Radix `DropdownMenu`, already in the installed `radix-ui`; new shadcn-style wrapper `components/ui/dropdown-menu.tsx`). Its only item is **"Change password"** → `/account/password`. Keyboard: Enter, Space or ArrowDown opens it, arrows move, Escape closes it and returns focus to the trigger; the item is a real link, so it works as a link too. The trigger keeps the text `{name} · {role}` and the title with the email, as today.
- **Small screens:** below Tailwind's `md` breakpoint (768 px, so at 320 px too) the sidebar is hidden behind a **"Menu" button** in the top bar (`aria-expanded`, `aria-controls` the nav). Opening it shows the same nav as a block under the top bar, above the page content; choosing an item closes it. At `md` and up the sidebar is always visible and the button is hidden.
- **Routes unchanged:** `/admin` (Reports and Users tabs, stats cards, Approve/Unapprove), `/reports` (Edit/Delete), `/admin/users`, `/account/password` and their titles.
- **Skip link and landmarks:** the skip link stays the first tab stop and still moves focus to `main#main-content`; the sidebar comes after the header and before the page's `<main>`; the page keeps one `<main>`.
- Tests for the new pieces, the existing tests that name the old header updated on purpose (listed below), new e2e cases, and docs.

## Out of scope

- The Tracker and Logs pages, their routes and data (Phase B); the Users tab of the admin panel (20d); the preview modal (20e).
- A profile page, avatar upload, or anything else in the profile menu beyond "Change password".
- Any change to the contents of the pages, to `/login`, or to the session and permission code.
- Remembering the open or closed state of the small-screen menu, a collapsible desktop sidebar, icons, or a theme switch.
- Any backend change.

## Build loop

`workflow.stepReview` is `feature`: review once after all steps. `checkpointCommits` is disabled; `/complete` makes the final commit. Test-first inside each step. Stop any dev server before `npm run build`, and ask before stopping one you did not start (or build with a separate `NEXT_DIST_DIR` and restore `tsconfig.json` afterwards). Frontend commands run from `frontend/`.

## Build steps

- [x] 1. **Profile menu (tests first).** Add `components/ui/dropdown-menu.tsx` (wrapper over `DropdownMenu` from `radix-ui`, in the style of `tabs.tsx` and `alert-dialog.tsx`) and `app/(app)/profile-menu.tsx`, with `profile-menu.test.tsx` written first: the trigger is a button showing `{name} · {role}` with the email as its title; clicking or pressing Enter opens a menu with one item, "Change password", whose `href` is `/account/password`; Escape closes it and focus returns to the trigger; a user with no role reads "no role", as the header does today.
  Done when: `npm test` shows the new tests red first and then green, and `npm run lint` and `npm run typecheck` pass.
- [x] 2. **Sidebar (tests first).** Add `app/(app)/sidebar.tsx` and `sidebar.test.tsx` first (mock `next/navigation`'s `usePathname` and `@/lib/session`): an admin sees Reports, Admin, Users in that order, each a link with the right `href`; a non-admin sees only Reports; there is no Tracker or Logs for anyone; `aria-current="page"` is on the matching item for `/reports`, `/reports/new`, `/reports/5/edit`, `/admin` (only Admin), `/admin/users` and `/admin/users/new` (only Users), and on none for `/account/password`; the nav is named "Main".
  Done when: `npm test` passes these cases (red first, then green), with lint and typecheck passing.
- [x] 3. **Layout, top bar and the Menu button (tests first).** Update `layout.test.tsx` and `layout.header.test.tsx` first (see below), adding: the header has the app name, the profile menu and Sign out, and no "Admin", "Users" or "Account" link in it; the sidebar renders inside the layout for an admin and for a non-admin; a "Menu" button exists with `aria-expanded="false"` and `aria-controls` pointing at the nav; clicking it sets `aria-expanded="true"`, clicking a nav link or the button again closes it; the skip link is still the first link and points at `#main-content`; while the session loads there is no header, skip link or sidebar. Then edit `layout.tsx`: remove the three links, add the profile menu and the sidebar, wrap the page in a row layout (`md:flex-row`) whose content column is `min-w-0 flex-1`, and add the Menu button (visible below `md` only; the nav is hidden below `md` unless open, always shown from `md`).
  Done when: `npm test`, `npm run lint` and `npm run typecheck` pass, and `npm run build` passes (every page uses the layout and `usePathname`).
- [x] 4. **Browser checks, docs and full suites.**
  - `e2e/account.spec.ts`: reach the password page through the avatar menu ("Change password") instead of the "Account" link.
  - `e2e/auth.spec.ts`: a plain user's sidebar has only "Reports" (no "Users", no "Admin"); `e2e/a11y.spec.ts` "page titles": navigate with the sidebar's "Users" link (inside the "Main" navigation); the new `/admin/users` and `/reports` pages show the sidebar for the admin.
  - New e2e: the admin sidebar shows Reports, Admin, Users and the current page is marked; the editor and the user see only Reports; the avatar menu opens, "Change password" goes to the password page; at 320 px the sidebar is hidden, the "Menu" button opens it and a link in it navigates and closes it; at 1280 px the Menu button is not visible. The axe and 320 px reflow cases keep passing with the sidebar and the open menu.
  - Docs: record the navigation decision (D-A9) in `docs/DECISIONS.md` and update the "app header" mention in `backend/docs/08-Frontend-Plan.md`.
  Done when: `npm test`, `npm run lint`, `npm run typecheck`, `npm run build` and `npm run e2e` pass, or any that cannot run is named in the summary with the reason.

## Existing tests that change on purpose (they assert the old top bar)

- `app/(app)/layout.header.test.tsx`: "links an admin to the admin panel" (the link moves to the sidebar) and the 320 px wrap comment and checks.
- `app/(app)/layout.test.tsx`: "does not show the Admin link to a non-admin" (now asserted for the sidebar) and the layout mock (it must also mock `usePathname`).
- `e2e/account.spec.ts` (clicks "Account"), `e2e/a11y.spec.ts` ("page titles" clicks the header "Users" link), `e2e/auth.spec.ts` (expects no "Users" link).
- Everything else, including the stats cards, the Approve/Unapprove flow and the Edit/Delete on `/reports`, must pass unchanged.

## Files / areas

- New: `frontend/src/components/ui/dropdown-menu.tsx`, `frontend/src/app/(app)/profile-menu.tsx` (+ test), `frontend/src/app/(app)/sidebar.tsx` (+ test).
- Edit: `frontend/src/app/(app)/layout.tsx`, `layout.test.tsx`, `layout.header.test.tsx`, `frontend/e2e/account.spec.ts`, `frontend/e2e/auth.spec.ts`, `frontend/e2e/a11y.spec.ts`, `docs/DECISIONS.md`, `backend/docs/08-Frontend-Plan.md`.
- Read only: `frontend/src/lib/session.tsx` (`isAdmin`, `signOut`), `frontend/src/lib/auth-form.ts` (`HOME`), `frontend/src/components/ui/avatar.tsx`, `frontend/src/components/ui/tabs.tsx` (wrapper style), `frontend/src/app/layout.tsx`.

## Data / contracts

- No API, Swagger or route change. Navigation uses `next/link` and `usePathname` from `next/navigation`.
- Visibility rule: *Reports* always; *Admin* and *Users* only when `isAdmin` (`user.role === "admin"`). The server and each page still enforce the role.
- Link targets: `/reports`, `/admin`, `/admin/users`, `/account/password`. The app name links to `HOME` (`/reports`).
- Accessibility: one `nav` named "Main"; `aria-current="page"` on the current item; the Menu button has `aria-expanded` and `aria-controls`; the profile menu trigger is a labelled button and the menu follows the Radix keyboard pattern; focus returns to the trigger on close; the skip link and the single `<main id="main-content">` are unchanged; reflow at 320 px (WCAG 1.4.10) still holds with no sideways scroll.
- The profile menu's trigger text must stay `{name} · {role}` in one element: `e2e/auth.spec.ts` looks for it.

## Testing

- Vitest with Testing Library: `npm test`, `npm run lint`, `npm run typecheck`, `npm run build`. jsdom has no layout or media queries, so the small-screen behaviour is tested as state (`aria-expanded`, which classes are present) and proven for real by the Playwright cases at 320 px and 1280 px.
- Playwright + axe: `npm run e2e` (the e2e backend starts itself against a fresh database; the test Postgres must be up on :5440). Never run it against production.
- Not claimed unless run: any visual match to a screenshot (none is in the repo) and any production check.

## Notes for the AI

- Keep it small: no context, store or hook for navigation, no icon library, no remembered state. Two small components and a wrapper, plus the layout edit.
- Next.js 16 differs from older versions: read `frontend/AGENTS.md` and the relevant `node_modules/next/dist/docs/` page on `usePathname` before the layout edit; do not edit `AGENTS.md`.
- Pages render their own `<main id="main-content" className="mx-auto max-w-5xl …">`; do not change them. The layout only supplies the row around them.
- Do not touch `/login` (outside the `(app)` group).
- Never run the Playwright suite against production.

## Open questions

- None block the build. Choices made where the request was silent, all reversible: the small-screen breakpoint is `md` (768 px) rather than exactly 320 px; the opened menu pushes the content down instead of overlaying it; the profile menu holds only "Change password". Say so if you want any of them different.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":11732,"specSha256":"7b685cc6d624c6214825fa1b45eaf2878bb6ee03a264ee4e69ad4b59da3d857d","branch":"refs/heads/feature/sidebar-navigation-and-profile-menu","head":"695aa1c9464ad911ae5805b7f7193cd305a2c951","baseRef":"refs/heads/main","baseCommit":"695aa1c9464ad911ae5805b7f7193cd305a2c951","sourceTree":"2895b784427cfa312971a2262a3e284097b8796a","landing":"local-merge","absentOptional":[]} -->

## Verification

- `npm test`: 25 files, 223 tests passed. `tsc --noEmit` and `npm run lint`: exit 0.
- `npm run build` (separate `NEXT_DIST_DIR`, dev server untouched): exit 0.
- `npm run e2e`: 61 passed (Playwright with axe), including the new `navigation.spec.ts` and the 320 px reflow case with the Menu open.
- Not run: independent review (not required: navigation only, the server still enforces roles), `/check`, `/audit`. No screenshot or visual comparison was taken by the agent, and no browser check by the owner is recorded.

## Try it now

1. Start the stack (`make db-start` and `make dev` in `backend/`, `npm run dev` in `frontend/`) and sign in as an admin.
2. At 768 px or wider, the left sidebar lists **Reports** and **Admin**. The current page is bold and underlined, and carries `aria-current="page"`.
3. Press the avatar and name in the top bar: a menu with **Change password** opens. Escape closes it and focus returns to the button.
4. Open `/admin/users`: it redirects to `/admin?tab=users` with the Users tab selected, the "You" badge on your row and the Role filter.
5. Narrow the window below 768 px: a **Menu** button replaces the sidebar. Open it, choose an item, and it closes.
6. Sign in as a plain user: the sidebar shows only **Reports**.

Unaffected regression path: Approve and Unapprove still work in the Reports tab of `/admin`.
