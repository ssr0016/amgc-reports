# Feature: Seed idempotency

**From build-plan:** feature 19
**Build attempt:** 1
**Branch:** feature/seed-idempotency
**Archive:** blueprint/history/features/19-seed-idempotency.md
**Status:** verified

## Goal

Prove that running the existing database seed twice with identical valid input succeeds without duplicating or changing the seeded roles, permissions, role-permission grants or admin account. Exercise the real migrated PostgreSQL schema as `app_user`, the application's database role.

## In scope

- Reuse `SeedData` and the existing `TestSeed_Idempotent` integration test, introduced in commit `f5e5e5c`.
- Verify initial seed contents, the admin's usable bcrypt password and verified/admin state, and exact persisted-row equality after the second call.
- Run the focused seed integration tests, including role defaults and privilege-isolation checks, and record observed evidence.
- Change a test only if inspection or a failing assertion demonstrates a gap in this contract; an acceptance-only result with no application-code changes is valid.

## Out of scope

- New seeding implementation, abstractions, dependencies, migrations or seed permissions.
- Password rotation, overwriting existing users, grant reconciliation, concurrent seed execution, sequence-counter stability or transaction redesign.
- Browser/frontend work, API contract changes, new audit actions, production database access and deployment.
- Full repository test suites or changes to unrelated in-progress documentation.

## Build loop

- `/implement` creates or uses `feature/seed-idempotency`; this spec does not create the branch.
- `workflow.stepReview: feature`: verify each step, then present one combined review packet.
- `workflow.checkpointCommits: disabled`: no intermediate commits. `/complete` owns final acceptance, archive and build-plan bookkeeping; commits and landing still require authorization.
- Regular audit/check gates are manual. Independent review is `when-sensitive`; if implementation would change seed credentials, grants or persisted-data behavior, assess that gate before completion. Test-only verification does not authorize a production behavior change.

## Build steps

- [x] 1. Confirm the existing regression test covers the acceptance contract.
  - Reuse a fresh testcontainers PostgreSQL database, `MigrateWithGoose` and `AppUserPool`.
  - First seed: three built-in roles, populated permissions/grants, exactly one admin account; verify admin name, role, email verification and bcrypt password match.
  - Snapshot complete rows in deterministic order for `roles`, `permissions`, `role_permissions` and `users`. After the identical second seed, require success and exact snapshot equality, including IDs, timestamps and the stored password hash.
  - Keep the current test unchanged if it already proves these assertions. If a gap requires a test edit, use existing helpers and keep it in this test file.
  - **Done when:** `TestSeed_Idempotent` runs and passes against real PostgreSQL as `app_user`, with all assertions above present; a compile-only run or skipped test is insufficient.

- [x] 2. Verify seed regressions and prepare acceptance evidence.
  - Run all `TestSeed_` tests with the integration tag, race detector and cache disabled.
  - Confirm report/admin-area permissions, built-in role defaults, user privilege isolation and admin-only approval checks still pass. Existing tests without admin credentials must continue to succeed.
  - Record command, exit status, executed test names and any environmental blocker. Review the final diff for unrelated edits and secrets.
  - **Done when:** the focused seed integration group passes with no skips or race reports, the feature's acceptance evidence is recorded, and the final review packet explains whether any code changes were necessary. Keep build-plan #19 unchecked until `/complete`.

## Files / areas

- `backend/internal/database/seed.go`: existing implementation; read-only reference unless a demonstrated defect is separately reviewed.
- `backend/internal/database/seed_integration_test.go`: existing acceptance and permission regression coverage; sole expected code-edit location if a test gap exists.
- `backend/internal/testutil/db.go`: existing disposable PostgreSQL, migration and app-role helpers; reuse unchanged.
- `backend/cmd/api/app/server.go`: startup caller of `SeedData`; reference only.
- `blueprint/context/current-feature.md`: implementation progress and verification evidence.
- `blueprint/build-plan.md`, feature archive: completion bookkeeping only through `/complete`.

## Data / contracts

- Caller supplies `AdminSeed{Email, Name, Password}`. Empty email or password skips admin creation; role/permission seeding still runs. Existing credential-free tests exercise that path.
- Roles and permissions conflict on `name`, grants on `(role_id, permission_id)`, and admin users on `email`; the implementation uses `ON CONFLICT DO NOTHING`.
- The admin password is bcrypt-hashed; tests use synthetic credentials and must not print password hashes or real credentials on failure. Snapshot values stay in the disposable test database/test process; review assertion diagnostics before retaining logs.
- Scope is sequential repeat execution with the same valid input. Existing stored account and grant values remain intact on the second call. Consuming PostgreSQL sequence values is not a duplicate-row failure.
- Seed runs on API startup. Production migrations remain a separate operation and are not exercised against Neon for this feature.
- No client/server request, authorization or UI contract changes.

## Testing

No umbrella Verify command is declared in `AGENTS.md`. The implementation pass verified the existing tests and built the API; no source or test changes were necessary.

Run from `backend/` with Docker/testcontainers available:

```bash
set -o pipefail
go test -tags=integration -race ./internal/database -run '^TestSeed_Idempotent$' -count=1 -v 2>&1 | tail -c 4000
go test -tags=integration -race ./internal/database -run '^TestSeed_' -count=1 -v 2>&1 | tail -c 4000
```

The second command includes the first test; run the isolated command only when developing or diagnosing an assertion. A single successful group run is enough when no test edits are needed. Container startup/database errors are blockers, not passing or skipped evidence. Do not substitute a production database.

### Observed verification — 2026-10-24

- Branch: `feature/seed-idempotency`, based on `f5e5e5c`.
- From `backend/`: `set -o pipefail; rtk go test -tags=integration -race ./internal/database -run '^TestSeed_' -count=1 -v 2>&1 | tail -c 4000` exited 0. Output: `Go test: 6 passed in 1 packages`. All six existing seed tests executed; no skips or race failures were reported.
- Executed tests: `TestSeed_ReportsPermissionsExist`, `TestSeed_RoleDefaults`, `TestSeed_NoPrivilegeLeakToUser`, `TestSeed_OnlyAdminApproves`, `TestSeed_AdminAreaPermissions`, `TestSeed_Idempotent`.
- `TestSeed_Idempotent` uses a fresh migrated PostgreSQL container, connects as `app_user`, verifies the initial admin and bcrypt password, then compares complete deterministic snapshots after reseeding. The unchanged test covers the approved contract.
- From `backend/`: `set -o pipefail; go build -o /tmp/amg-seed-idempotency-api ./cmd/api 2>&1 | tail -c 4000` exited 0 without build errors. Output binary is outside the repository.
- `git diff --check` passed. Source, tests, seed grants and credential behavior remain unchanged.
- Regular audit and live Check are manual and were not requested. Independent review was not selected: this pass changes verification documentation only, with no sensitive implementation changes; no prior independent-review request exists.
- Try-guide classification (`when-testable`): skipped. This is acceptance of existing internal behavior, with no new direct observation path. A test/build command is not substituted as a manual product demonstration.
- Findings handoff: 12 open P3 findings and one fixed P3 finding awaiting re-review; no P0/P1 blockers. Findings remain unchanged and are not closed by this verification.
- Unrelated `blueprint/context/project-overview.md` changes were preserved. Build-plan #19 stays unchecked pending `/complete`. No commit, merge, push, production database access or deployment occurred.

## Notes for the AI

- Critique narrowed this feature to verifying existing behavior; do not rewrite working seed logic or duplicate the recently committed test.
- Use repository Go/Echo/pgx/testcontainers conventions. Generic Prisma/Clerk/Server Actions guidance in Blueprint standards does not apply.
- Treat the current overview edit as separate documentation work; preserve it and do not silently include it in the eventual feature commit.
- Historical test commits and the earlier frontend E2E pass are not fresh seed integration evidence.
- If verification exposes a real production seed defect, report its root cause and propose a reviewed spec revision before changing persisted-data behavior.
- No product decision remains unresolved for this bounded sequential-idempotency contract.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":9149,"specSha256":"68c5ea5769cd9fe4c2d437062dfd7591ab823d377e2d5eb290accfe3db60ead8","branch":"refs/heads/feature/seed-idempotency","head":"feb685c0e066e0577ed55af9af61d39d5ca8e473","baseRef":"refs/heads/main","baseCommit":"f5e5e5c5d374af77380f54054c4625173b98916f","sourceTree":"a51c5089d795f27035dbf44688499a0feaa5d298","landing":"local-merge","absentOptional":[]} -->
