# Fix: Migration docs for Render Free tier, and Pending in the Tracker

**Type:** Fix
**Status:** verified
**Branch:** fix/migration-docs-for-render-free-tier-and-pending-in-the-tracker
**Fixes:** F-12

## The problem

The H-1 update of 2026-10-22 (`docs/DECISIONS.md`) says Render's Free tier runs neither the Pre-Deploy Command nor Shell, so production migrations are run from a local machine against Neon before pushing `main`. `AGENTS.md`, `docs/ARCHITECTURE.md` and `docs/DECISIONS.md` say so. Two docs still describe the old procedure:

- `backend/SETUP.md`:
  - section 1 (line 157) lists **Pre-Deploy Command** `/app/api migrate` as a service setting;
  - section 3 (lines 186-190) says to run it "as the service's Pre-Deploy Command where the plan allows it, or by hand", and to verify which in the dashboard;
  - the `migrate_user` and `app_user` runbooks (lines 231, 243, 254, 281) refer to "the pre-deploy" migrate run.
- `blueprint/context/project-overview.md:107` says "run `/app/api migrate` as a separate pre-deploy step".

## The fix

Docs only; no code, config or migration changes.

- **SETUP.md section 1:** replace the Pre-Deploy Command bullet with: not used, because the Free tier does not run it (H-1); see Migrations.
- **SETUP.md section 3:** production migrations run from your machine against Neon, before pushing `main`:
  - `cd backend && make migrate-up DATABASE_URL='<Neon owner-role URL>'`, then `make migrate-status DATABASE_URL='…'` to confirm nothing is pending.
  - Pass the URL **on the make command line**: the Makefile includes `backend/.env`, whose `DATABASE_URL` overrides a plain environment variable.
  - Use the owner (or `migrate_user`) URL, not `app_user`, which cannot run DDL.
  - The migration must work with the code still running until the push deploys (code first, or a backward-compatible migration).
  - Keep the existing notes on `/app/api migrate` (`MIGRATE_DATABASE_URL`, log lines, non-zero exit) as the alternative runner.
- **SETUP.md runbooks:** change "the pre-deploy migrate/command" to "the local migrate run", and "Redeploy" steps that relied on Pre-Deploy to "run the migration locally, then redeploy"; keep each runbook's steps otherwise unchanged.
- **project-overview.md:107:** match `AGENTS.md:48`. `/overview` regenerates the overview next anyway; this keeps the line correct if it does not.
- Do not touch historical docs that record the Railway era (`backend/docs/05-Backend-Schema.md`, `backend/docs/06-Implementation-Plan.md`, history archives); H-1 leaves older entries as written.

### Pending in the Tracker (owner, 2026-10-22)

The Tracker labels a filed, unapproved report "For Review"; the Reports tab, the stats cards and the Excel dashboard call the same state "Pending". Change only the displayed label to "Pending" (the Tracker's status badge, Status filter option and count card). The API code `for_review` and every backend name stay as they are, so there is no Swagger or `api-types.ts` change. Record the wording in D-A3.

## Build steps

- [x] 1. Update `backend/SETUP.md` (sections 1 and 3 and the two runbooks) and `blueprint/context/project-overview.md:107` as above.
  - **Done when:** `grep -n -i "pre-deploy" backend/SETUP.md blueprint/context/project-overview.md` returns only the line saying the Pre-Deploy Command is not used on the Free tier; section 3 shows the exact `make migrate-up DATABASE_URL=…` command; the four procedure docs (`AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/DECISIONS.md` H-1, `backend/SETUP.md`) describe the same procedure.

- [x] 2. In `frontend/src/lib/tracker.ts` label `for_review` "Pending"; in `frontend/src/app/(app)/admin/tracker.tsx` name the count card "Pending"; update `tracker.test.ts`, `tracker.test.tsx` and `e2e/tracker.spec.ts`; add the wording to D-A3 in `docs/DECISIONS.md`.
  - **Done when:** `grep -rn "For Review" frontend/src frontend/e2e` finds nothing; the Tracker shows **Pending** in its badge, filter and count card; `npm test`, `npm run lint`, `npm run typecheck` and `npx playwright test e2e/tracker.spec.ts` pass; `git diff` touches no backend file.

## Verify

- Read SETUP.md sections 1 and 3 next to H-1 in `docs/DECISIONS.md`: same procedure, same command.
- `cd backend && make -n migrate-status DATABASE_URL=postgres://example` prints the goose command with the URL from the command line, not from `.env` (a dry run; it connects to nothing).


<!-- blueprint:completion {"schemaVersion":2,"specBytes":4407,"specSha256":"909d2307b3dad61161d733eff84ddeae87a4a5f5a1d508be3aeadfea397273d8","branch":"refs/heads/fix/migration-docs-for-render-free-tier-and-pending-in-the-tracker","head":"d85e3a3e7cb5255be06432a1f794414220e980e1","baseRef":"refs/heads/main","baseCommit":"d85e3a3e7cb5255be06432a1f794414220e980e1","sourceTree":"13b5b534e37596ba43d2dfc55ab26213f4adaf98","landing":"local-merge","absentOptional":[]} -->

## Try it now

Run or do:

With this branch checked out: in `backend/`, `make db-start` then `make dev`; in `frontend/`, `npm run dev`. Sign in at http://localhost:3000 as admin and open **http://localhost:3000/admin?tab=tracker**.

Expected:

The count cards read **Expected, Submitted, Approved, Pending, Not Submitted**; the **Status** filter offers **All statuses, Approved, Pending, Not Submitted**; a worker whose report is filed but not approved shows a yellow **Pending** badge. "For Review" appears nowhere on the page.

This proves:

The Tracker now shows the filed-not-approved state as "Pending", the same word as the Reports tab.
