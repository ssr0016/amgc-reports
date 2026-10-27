# Coding Standards

> AMGCReports conventions: a Go (Echo, pgx, goose) API and a Next.js 16 +
> TypeScript + Tailwind 4 frontend that calls it through TanStack Query.
>
> Run `/onboard` after installing the Blueprint. It tunes this file to the real
> project stack, along with `AGENTS.md`, `CLAUDE.md` when present,
> `ai-interaction.md`, `.gitignore`, and README placement. Review the result
> before `/overview`.

## TypeScript

- Strict mode enabled
- No `any` types - use proper typing or `unknown`
- Define interfaces for all props, API responses, and data models
- Use type inference where obvious, explicit types where helpful

## React

- Functional components only (no class components)
- Use hooks for state and side effects
- Keep components focused - one job per component
- Extract reusable logic into custom hooks

## Next.js

- Read `frontend/AGENTS.md` before using a Next.js API: Next.js 16 differs from older versions
- `page.tsx` files are Server Components: they export `metadata` (and `instant = false` when the screen needs the session) and render one Client Component screen
- Wrap a screen that calls `useSearchParams` in `<Suspense>` (`cacheComponents` is on)
- Screens that read or change data are Client Components (`'use client'`): the session lives in the browser (`useSession`, `/auth/me`), so pages do not fetch on the server
- No Server Actions and no route handlers: every read and write goes to the Go API
- `src/proxy.ts` only turns away visitors without the `signed_in` hint; the `(app)` layout guards private pages
- Dynamic routes for item pages (`reports/[id]`, `admin/users/[id]/edit`)

## File Organization

- Pages: `src/app/(app)/[route]/page.tsx`, with the screen beside it in kebab-case (`reports-list.tsx`)
- Shared components: `src/components/` (kebab-case files; shadcn primitives in `src/components/ui/`)
- API clients per domain: `src/lib/[domain].ts` (`reports.ts`, `users.ts`)
- API types: generated `src/lib/api-types.ts` (never edit by hand; `npm run gen:api`), re-exported through `src/lib/api-schema.ts`
- Tests next to the source (`*.test.ts(x)`); Playwright specs in `frontend/e2e/`
- Backend: `internal/<domain>/` (contracts, models), `<domain>impl/` (service, store), `rest/` (Echo handlers); see `backend/CONTRIBUTING.md`

## Naming

- Components: PascalCase (`ItemCard.tsx`)
- Files: Match component name or kebab-case
- Functions: camelCase
- Constants: SCREAMING_SNAKE_CASE
- Types/Interfaces: PascalCase (no prefix)

## Styling

- Tailwind CSS for all styling
- Tailwind v4: CSS-first config (`@theme` in `globals.css`), no `tailwind.config.js`
- Use shadcn/ui components where applicable
- No inline styles
- Dark mode first, light mode as option

## Database

- PostgreSQL through pgx; each domain's `store.go` builds SQL with squirrel and placeholders, never string-concatenated input
- Schema changes are goose migrations in `backend/internal/database/migrations/`, named `YYYYMMDDHHMMSS_description.sql`, each with Up and Down
- Run `make migrate-status` before committing to verify migrations are in sync
- Production does not auto-migrate: run `make migrate-up` against Neon before pushing `main` (Render Free has no Pre-Deploy Command)
- Permissions and role grants are seed data in `seed.go`, not migrations
- A new audit action needs its CHECK-constraint migration in the same deploy as the code that writes it

## Data Fetching

- Client components call the API through the typed clients in `src/lib/`; `api.ts` adds the CSRF token and turns failures into `ApiError`
- Wrap reads in TanStack Query `useQuery` and writes in `useMutation`; the shared `queryClient` (`src/lib/query.tsx`) does not retry or refetch on focus, so the per-user rate limit is not spent twice
- The Go API validates every input (`validator` tags, 422 with field `details`); client-side form checks (`src/lib/report-form.ts`) are for fast feedback only
- The API scopes every query by the session's user and permissions; never trust a client-supplied user id

## Error Handling

- Backend: return errors through `apperror`; handlers map service sentinels to status codes, and audit rows are written in the same transaction as the change
- Frontend: show `ApiError.details` on the matching form field and other failures in a Sonner toast; `queryClient` already toasts network and 5xx load failures

## Testing

The blueprint installs no test runner; testing is opt-in at the project level,
because the overlay can't know your stack. Adding unit testing is an explicit
setup task the AI can do through the normal workflow, either as a build-plan item
or with `/tests`. The setup should choose the stack-native runner, wire the
scripts or commands, add a small example test, and update the Commands section
of `AGENTS.md`.

When `AGENTS.md` declares a `Verify` command, treat it as the umbrella automated
gate. It combines only the checks this project actually has, in this order when
available: typecheck, tests, then build. The command does not enable an absent
test runner or replace focused evidence. It gives local work and optional CI one
exact command to run. `/ci` owns Verify and CI setup. `/tests` adds the real test
command to Verify when it already exists, but never creates CI only because
testing was configured.

**The opt-in switch is one signal: a `test` command in the Commands section of
`AGENTS.md`.** Declare one and **tests become a gate for logic-bearing steps**,
not an optional extra; leave it out and the loop verifies logic with the evidence
it already uses (run it, a screenshot, the build). Adding the runner is itself a
deliberate step, never a silent mid-step install. This is the single definition
of the switch; the skills and `ai-interaction.md` only point back here.

- **What to test (the scope rule):** pure logic where a wrong answer is possible -
  parsers, formatters, validators, id/slug builders, server actions. These have
  assertable inputs and outputs and real edge cases (empty, missing, malformed).
- **What not to test:** UI components and integration-level surfaces (render or
  export routes, anything driving a real browser or external service). Verify those
  with a screenshot and the build, not brittle unit tests.
- **The gate (when a runner is configured):** a build step that adds in-scope logic
  must ship a passing test in the same reviewable diff. The project's test command
  must be green before the step is approved, before any checkpoint commit, and
  before `/complete` merges. UI and integration-only steps are exempt and ride on
  screenshot plus build evidence.
- **When it's named:** the `/feature` spec's Testing section predicts the coverage,
  `/implement` writes the test with the step, and if a step surfaces logic the spec
  didn't foresee, add a focused test then.
- An empty suite should fail, not pass, so "no tests ran" never looks like "passed".
- Test files live next to source files (for example `feature.test.ts`).
- Run them via the project's test command (see Commands in `AGENTS.md`), not a
  hardcoded tool name.

Stack binding (swap for yours): a TypeScript app uses Vitest, `vi.mock()` for
external dependencies (the API clients in `src/lib/`, `next/navigation`), and `vi.useFakeTimers()` for
time-dependent logic; a Python app would use pytest; a Go app `go test`.

## Browser Verification

For UI and integration behavior, prefer real browser evidence over reading the
code and assuming it works.

- Browser automation is separately opt-in through `/tests browser`. That setup
  reuses a compatible runner or prefers Playwright for supported projects, then
  documents the exact command as `Browser tests` in `AGENTS.md`.
- When `Browser tests` is declared, add focused coverage for stable behavioral
  done-whens when it is proportionate, and run the documented command during
  `/check`. Do not assume it proves visual fidelity, real authenticated-profile
  behavior, browser chrome, or another claim the test does not observe.
- If no Browser tests command is declared, do not add a runner silently in the
  middle of an unrelated feature. Use the available dev server, browser
  screenshots, build output, API output, or manual evidence instead.
- Browser tests are not part of the default Verify command or CI unless the user
  separately chooses that slower gate.
- Browser evidence is especially important for flows that click, type, submit,
  navigate, download files, render complex layouts, or depend on client-side
  state.

## Code Quality

- No commented-out code unless specified
- No unused imports or variables
- Keep functions under 50 lines when possible

## Comments

Write code that explains itself; comment only what the code cannot say.
Over-commenting is a common AI tell, so resist it.

- Comment the **why**, not the **what**. Delete any comment that restates the code.
- No banner/header blocks, section dividers, or step-by-step narration of obvious
  code. A file does not need a comment announcing each region.
- A comment earns its place only when it captures something the code can't: a
  non-obvious decision, a gotcha or workaround, why a value is what it is, or a
  link to a spec or issue.
- Prefer self-documenting names and small functions over explanatory comments.
- Keep doc comments minimal: a one-line purpose on an exported type or function is
  plenty; don't write JSDoc that just repeats the signature.
- When in doubt, leave the comment out.

## Writing

- No em dashes (U+2014) in generated content: docs, comments, commit messages,
  READMEs, specs. They read as AI-generated.
- Use a hyphen for `term - description` separators; rephrase prose with commas,
  parentheses, or a colon. Avoid en dashes and the ellipsis character too.
