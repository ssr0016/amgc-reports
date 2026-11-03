# Daily Digest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Send each eligible admin one idempotent daily email at 8:00 AM Asia/Manila with the current-month pending-report and no-report account counts, unless opted out.

**Architecture:** Extend the existing report notification outbox with a `daily_digest` event and JSONB payload. The existing polling email worker schedules due Manila-day digests, then delivers them through the current leasing, retry, and mailer path. Add one authenticated preference endpoint and one Profile toggle.

**Tech Stack:** Go, Echo, PostgreSQL/goose, pgx/Squirrel, existing email renderer and Resend mailer, Next.js/React, generated Swagger TypeScript types.

**Spec:** This plan implements the approved Daily Digest requirement in the user request.

## Global Constraints

- Digest time is 8:00 AM `Asia/Manila`.
- Missing count equals the existing Admin Overview `NoReport` KPI: active `user` accounts with a church and no report for the current month.
- Recipients must be active, non-deleted admins with `reports:approve` permission and `daily_digest` enabled.
- The outbox idempotency key must prevent duplicate digest rows across restarts and multiple API processes.
- Do not add a second email worker, scheduler process, or dependency.
- Do not modify an already-applied migration; add a new goose migration.

## Review Focus

- Two API processes schedule the same Manila day: one recipient row must survive, pinned by the unique idempotency key test in the outbox task.
- Worker starts after 8:00: the current Manila day's due slot must be scheduled once, pinned by the scheduler clock test.
- `notification_prefs` is NULL or lacks `daily_digest`: treat it as enabled, pinned by migration/store tests.
- A digest delivery fails: the existing retry and lease state machine must process it like report notifications, pinned by worker tests.
- User is not an active admin or lacks `reports:approve`: no digest row is created, pinned by scheduler integration tests.

---

### Task 1: Normalize notification preferences and extend the outbox schema

**Files:**
- Create: `backend/internal/database/migrations/20261103150000_add_daily_digest.sql`
- Modify: `backend/internal/database/notification_schema_integration_test.go`

**Interfaces:**
- Produces `users.notification_prefs` with default `{"daily_digest": true}` for new rows and backfilled existing rows.
- Produces `report_notification_outbox.payload JSONB NOT NULL DEFAULT '{}'::jsonb`.
- Allows outbox event type `daily_digest` while preserving existing report event types.

- [ ] **Step 1: Write migration-schema tests**

Assert the users column default, existing NULL/missing preference behavior after migration, payload default, and `daily_digest` event constraint.

- [ ] **Step 2: Run the focused integration test and verify it fails**

Run: `cd backend && go test -tags=integration ./internal/database -run 'TestNotificationSchema' -count=1`

- [ ] **Step 3: Add the goose migration**

Set the column default, backfill NULL or missing `daily_digest` to `true`, add `payload`, and extend the event CHECK constraint. Down migration must reverse only this migration's changes.

- [ ] **Step 4: Run the focused integration test**

Run: `cd backend && go test -tags=integration ./internal/database -run 'TestNotificationSchema' -count=1`

Expected: PASS.

### Task 2: Add digest data contracts, persistence, and scheduling

**Files:**
- Modify: `backend/internal/report/outbox/queue.go`
- Modify: `backend/internal/report/outbox/store.go`
- Modify: `backend/internal/email/worker/worker.go`
- Modify: `backend/internal/email/worker/worker_integration_test.go`
- Modify: `backend/internal/report/outbox/queue_integration_test.go`
- Modify: `backend/internal/stats/stats.go` only if a reusable digest result type is needed
- Modify: `backend/internal/stats/statsimpl/store.go` only if the scheduler needs a direct shared count method

**Interfaces:**
- Add a digest scheduling interface separate from `outbox.Queue`, so existing queue fakes remain small.
- Add `outbox.Job.Payload` as raw JSON and preserve existing report fields.
- Add a worker option for the digest scheduler and a `Now`-driven Manila clock.
- Add a digest payload containing `month`, `year`, `pending_count`, `missing_count`, and `reports_url`.

- [ ] **Step 1: Write failing outbox tests**

Cover enqueueing one digest per `(Manila date, recipient user)`, duplicate enqueue as a no-op, payload round-trip through `ClaimNext`, and filtering recipients to active non-deleted admins with `reports:approve` and enabled preference.

- [ ] **Step 2: Write failing worker scheduling tests**

Cover 07:59 no enqueue, 08:00 enqueue, 08:05 catch-up enqueue, second poll same day no duplicate, next Manila day new key, and a UTC time whose Manila date differs from the server date.

- [ ] **Step 3: Implement the scheduler query**

Use the existing `stats.Store.Counts(month, year)` semantics for pending and no-report counts. Insert one outbox row per eligible recipient using idempotency key `daily-digest:<YYYY-MM-DD>:<user_id>`. Store month, year, counts, and the configured Admin Reports URL in `payload`.

- [ ] **Step 4: Implement worker integration**

At each existing poll, compute the current Manila time and the current day's 08:00 slot. If the slot is due, call the scheduler before draining jobs. Keep the unique key as the once-per-day guarantee. Decode `daily_digest` payloads and render them; keep report notification behavior unchanged.

- [ ] **Step 5: Run focused tests**

Run: `cd backend && go test -tags=integration ./internal/report/outbox ./internal/email/worker -run 'Digest|Worker' -count=1`

Expected: PASS.

### Task 3: Render the digest email

**Files:**
- Create: `backend/internal/email/templates/daily_digest.html.tmpl`
- Create: `backend/internal/email/templates/daily_digest.txt.tmpl`
- Modify: `backend/internal/email/renderer.go`
- Modify: `backend/internal/email/renderer_test.go`
- Modify: `backend/internal/email/worker/worker.go`

**Interfaces:**
- Add `email.DailyDigestData` with `ReportMonth`, `ReportYear`, `PendingCount`, `MissingCount`, and `ReportsURL`.
- Add `email.RenderDailyDigest(data)` returning the existing `email.Message`.
- Subject: `Daily report digest`.

- [ ] **Step 1: Write renderer tests**

Assert subject, HTML escaping of the URL, both counts, month/year, Admin Reports link, and plain-text fallback.

- [ ] **Step 2: Implement templates and renderer**

Use the existing email typography and restrained green accent. Show `N pending reports` and `M missing church reports`, then one clear link to `/admin?tab=reports`. Keep HTML and text content equivalent.

- [ ] **Step 3: Run focused renderer and worker tests**

Run: `cd backend && go test ./internal/email ./internal/email/worker -run 'Digest|Renderer|Worker' -count=1`

Expected: PASS.

### Task 4: Add frontend URL configuration and wire the worker

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_app.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/.env.example`
- Modify: `backend/cmd/api/app/server.go`

**Interfaces:**
- Add `Config.App.FrontendURL`, loaded from `FRONTEND_URL` and trimmed of a trailing slash.
- Pass `outbox.NewStore(db)` as both the existing delivery queue and digest scheduler, plus `cfg.App.FrontendURL` to the scheduler.

- [ ] **Step 1: Write config tests**

Assert `FRONTEND_URL=https://reports.example.com/` becomes `https://reports.example.com` and remains distinct from API `APP_URL`.

- [ ] **Step 2: Implement config and server wiring**

Fail startup validation when digest email delivery is enabled but `FRONTEND_URL` is empty, or use the existing local frontend default only in non-production. Keep production explicit.

- [ ] **Step 3: Run focused config tests**

Run: `cd backend && go test ./internal/config -run 'FrontendURL|Load' -count=1`

Expected: PASS.

### Task 5: Expose the current preference and update endpoint

**Files:**
- Modify: `backend/internal/user/model.go`
- Modify: `backend/internal/user/user.go`
- Modify: `backend/internal/user/userimpl/store.go`
- Modify: `backend/internal/user/userimpl/mock.go`
- Modify: `backend/internal/auth/model.go`
- Modify: `backend/internal/auth/auth.go`
- Modify: `backend/internal/auth/authimpl/auth.go`
- Modify: `backend/internal/auth/rest/auth_handler.go`
- Modify: `backend/internal/auth/rest/auth_handler_test.go`
- Modify: `backend/internal/user/userimpl/user_profile_integration_test.go` or a new focused user preference integration test
- Modify: `backend/internal/router/router.go`

**Interfaces:**
- Add `DailyDigestEnabled bool` to the live `/auth/me` response.
- Add `auth.UpdateNotificationPreferences(ctx, userID, dailyDigest bool) error` or equivalent service method.
- Add authenticated `PUT /auth/me/notification-preferences` with body `{ "daily_digest": boolean }`, returning the updated preference.
- Treat NULL, missing, or invalid preference key as enabled when reading; accept only a boolean on writes.

- [ ] **Step 1: Write failing store/service/handler tests**

Cover default-enabled reads, disabling and re-enabling, authenticated caller ownership, invalid body rejection, and no exposure of another user's preference.

- [ ] **Step 2: Implement preference persistence**

Load the JSONB preference in the existing user scan path. Update only `daily_digest`, preserving unrelated future keys. Use the existing request validation and error conventions.

- [ ] **Step 3: Implement `/auth/me` and PUT route**

Register the route beside `GET /auth/me`, protected by existing auth and write/CSRF middleware. Do not add an audit action for this personal delivery preference unless project policy later requires it.

- [ ] **Step 4: Run focused backend tests**

Run: `cd backend && go test ./internal/auth/... ./internal/user/userimpl -run 'Preference|Me' -count=1`

Expected: PASS.

### Task 6: Add the Profile toggle

**Files:**
- Create: `frontend/src/app/(app)/account/profile/notification-preferences.tsx`
- Modify: `frontend/src/app/(app)/account/profile/page.tsx`
- Modify: `frontend/src/lib/auth.ts`
- Modify: `frontend/src/lib/api-schema.ts`
- Modify: `frontend/src/lib/session.tsx` only if the session type or local update helper needs a new field
- Test: `frontend/src/app/(app)/account/profile/notification-preferences.test.tsx`

**Interfaces:**
- Add `updateDailyDigest(enabled: boolean): Promise<Me>` or a focused preference response helper using `PUT /auth/me/notification-preferences`.
- Render an accessible switch labeled `Daily digest`, enabled from `user.daily_digest`.
- Explain: `Receive the current month's report status every morning at 8:00 AM.`
- On success update local session state and show the existing toast. On failure restore the previous switch state and show an error.

- [ ] **Step 1: Write the component test**

Assert the switch reflects `/auth/me`, sends the boolean body, updates after success, exposes a keyboard-accessible label, and restores state after a failed request.

- [ ] **Step 2: Implement the smallest Profile section**

Place it below Account details and above Profile picture. Reuse existing Card, PanelSection, Label, Button/switch primitives, loading, error, and toast patterns. Do not add a new settings page.

- [ ] **Step 3: Regenerate API types and run focused frontend tests**

Run: `cd backend && make swagger`; then `cd frontend && npm run gen:api && npm run typecheck && npm test -- --run src/app/(app)/account/profile/notification-preferences.test.tsx`

Expected: generated types have the new response field and route, typecheck passes, and the focused test passes.

### Task 7: Final verification and operational handoff

**Files:**
- Modify: `backend/docs/API.md` if the repository keeps this endpoint documented manually
- Modify: `docs/ARCHITECTURE.md` or the relevant operational section
- Modify: `backend/.env.example` if Task 4 did not already update it

- [ ] **Step 1: Verify focused backend gates**

Run the migration, outbox, worker, renderer, config, auth, and user tests from Tasks 1-5. Do not run the full suite.

- [ ] **Step 2: Verify focused frontend gates**

Run `npm run typecheck` and the Profile preference test. Run the existing Profile tests if the component composition changed their imports.

- [ ] **Step 3: Confirm deployment prerequisites**

Document `FRONTEND_URL`, the migration requirement, and the fact that only one digest row per recipient/day is created. Before production rollout, apply the migration to Neon manually per project guardrails.

## Explicitly Not Included

- No digest history screen.
- No per-role or per-recipient digest customization beyond the requested boolean.
- No separate scheduler service.
- No changes to the existing report notification preference keys.
- No full-suite test run, commit, push, or production migration execution in this planning pass.
