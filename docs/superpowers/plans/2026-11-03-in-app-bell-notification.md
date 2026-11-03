# In-App Bell Notification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist and display unread `report_submitted` notifications for every active user with `reports:approve`, with atomic creation and read navigation.

**Architecture:** Add a focused `notification` domain and Postgres store. The report service calls the store's bulk recipient insert inside its existing transaction. Add authenticated notification routes, then a React Query-backed bell in `AppLayout` using 30-second polling.

**Tech Stack:** Go 1.26, Echo, pgx/Squirrel, goose, swaggo, Next.js 16.4, React 19, TanStack Query, Radix DropdownMenu, lucide-react, Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-11-03-in-app-bell-notification-design.md`

## Global Constraints

- Use `/api/v1` as the API prefix.
- Keep report, metrics, audit, and in-app notification inserts in one transaction.
- Resolve recipients by `reports:approve`, not by role name alone.
- Do not add WebSockets, SSE, workers, preferences, or mark-all-read.
- Do not commit `.env` files or secrets.
- After Swagger changes, run `make swagger` then `npm run gen:api`.
- Run focused tests only; do not run the full Go or frontend suite.

## Review Focus

- A custom role with `reports:approve` receives a row; a role without it does not.
- Inactive or soft-deleted users receive no new row.
- A notification ID cannot be read by another user.
- Notification insert failure rolls back report creation and its audit row.
- Empty, loading, failed, and polled bell states remain accessible and do not expose report text.

### Task 1: Add notification schema

**Files:**
- Create: `backend/internal/database/migrations/20261103130000_create_user_notifications.sql`
- Create: `backend/internal/database/notification_schema_integration_test.go`

**Interfaces:**
- Produces table `user_notifications` for later stores.

- [ ] **Step 1: Write the migration**

Create `user_notifications` with `BIGSERIAL id`, `BIGINT user_id NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `type VARCHAR(64) NOT NULL`, `message TEXT NOT NULL`, `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`, nullable `read_at TIMESTAMPTZ`, and `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`.

Add a check allowing `report_submitted`, plus index `idx_user_notifications_user_read_at` on `(user_id, read_at)`. Down migration drops the index and table.

- [ ] **Step 2: Add schema integration assertions**

Assert the table, columns, foreign key, type check, and `(user_id, read_at)` index after migrations. Assert down migration removes the table if the migration test suite already covers rollback.

- [ ] **Step 3: Run the focused migration tests**

Run: `cd backend && go test ./internal/database -run 'Notification|Migrate' -count=1`

Expected: PASS.

### Task 2: Add notification domain and store

**Files:**
- Create: `backend/internal/notification/notification.go`
- Create: `backend/internal/notification/model.go`
- Create: `backend/internal/notification/notificationimpl/store.go`
- Create: `backend/internal/notification/notificationimpl/store_integration_test.go`
- Create: `backend/internal/notification/notificationimpl/mock.go` if existing handler tests need a mock

**Interfaces:**
- `notification.Store.CreateReportSubmitted(ctx context.Context, reportID int64, message string) error`
- `notification.Store.ListUnread(ctx context.Context, userID int64) ([]notification.Notification, error)`
- `notification.Store.MarkRead(ctx context.Context, userID, id int64) error`

- [ ] **Step 1: Define domain types and store contract**

Define `Notification` with `ID`, `UserID`, `Type`, `Message`, `Metadata map[string]any`, `ReadAt *time.Time`, and `CreatedAt`. Define a response conversion that returns the same public fields except `UserID`.

- [ ] **Step 2: Test recipient selection and atomic writes**

Using the existing Postgres test helpers, create users with: `reports:approve`, no permission, inactive status, and soft-deleted status. Assert only active non-deleted approvers get `report_submitted` rows, with message and `metadata.report_id`.

- [ ] **Step 3: Implement `CreateReportSubmitted`**

Use one insert-select statement joining `users`, `roles`, `role_permissions`, and `permissions`, filtering permission name `reports:approve`, active status, and `deleted_at IS NULL`. Insert metadata with the report ID. The caller supplies the final message.

- [ ] **Step 4: Test unread listing and ownership-safe mark-read**

Assert unread results are newest first, read rows are excluded, marking one row read changes its `read_at`, and a different user cannot mark it read.

- [ ] **Step 5: Implement `ListUnread` and `MarkRead`**

`ListUnread` filters `user_id` and `read_at IS NULL`, ordering by `created_at DESC, id DESC`. `MarkRead` updates only `WHERE id = $1 AND user_id = $2 AND read_at IS NULL`; return not-found when no row changes.

- [ ] **Step 6: Run notification store tests**

Run: `cd backend && go test ./internal/notification/... -count=1`

Expected: PASS.

### Task 3: Trigger notifications from report creation

**Files:**
- Modify: `backend/internal/report/report.go`
- Modify: `backend/internal/report/reportimpl/report.go`
- Modify: `backend/cmd/api/app/server.go`
- Modify: `backend/internal/report/reportimpl/report_test.go`
- Modify or create: `backend/internal/report/reportimpl/store_integration_test.go`

**Interfaces:**
- Consumes `notification.Store` from Task 2.
- Produces `NewService(..., notifications notification.Store)` wiring and report-create side effect.

- [ ] **Step 1: Add a service-level unit test**

Extend the existing report service fixture with a notification spy. Assert `Create` calls `CreateReportSubmitted` with the created report ID and a message naming the submitting worker, and still writes the audit entry.

- [ ] **Step 2: Add rollback integration coverage**

Use a failing notification store or database failure inside `Create`. Assert no report, metrics, audit row, or notification row remains after the transaction fails.

- [ ] **Step 3: Implement the create side effect**

Inside the existing `s.tx.InTransaction` callback, create the report, call the notification store with `report_submitted`, then insert audit. Place all three writes before the callback returns.

- [ ] **Step 4: Wire the concrete store**

Construct `notificationimpl.NewStore(db)` in `server.go` and pass it to `reportimpl.NewService`. Keep the existing email `NotificationSink` separate; do not reuse its outbox contract.

- [ ] **Step 5: Run focused report tests**

Run: `cd backend && go test ./internal/report/reportimpl -run 'TestReportService_Create|Notification' -count=1`

Expected: PASS.

### Task 4: Add authenticated notification API

**Files:**
- Create: `backend/internal/notification/rest/handler.go`
- Create: `backend/internal/notification/rest/handler_test.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/cmd/api/app/server.go`
- Modify: `backend/docs/swagger.json` via `make swagger`
- Modify: `frontend/src/lib/api-types.ts` via `npm run gen:api`

**Interfaces:**
- `GET /api/v1/notifications` returns `notification.NotificationResponse[]`.
- `PATCH /api/v1/notifications/:id/read` returns `204 No Content`.

- [ ] **Step 1: Write handler tests**

Assert authenticated users receive only their unread rows, malformed IDs return 400, successful read returns 204, another user's ID returns 404, and unauthenticated requests are rejected by the route group.

- [ ] **Step 2: Implement handler methods**

Read current user ID from request context. Map not-found to 404 and store failures to the existing internal error shape. Return `[]`, never `null`, for no unread notifications.

- [ ] **Step 3: Mount routes and wire handler**

Add a notification handler to `router.Setup`. Mount routes under authenticated `/api/v1/notifications`; do not require `reports:approve` because the resource is user-scoped, though non-approvers naturally receive an empty list.

- [ ] **Step 4: Regenerate API artifacts**

Run: `cd backend && make swagger && cd ../frontend && npm run gen:api`

Expected: Swagger and `frontend/src/lib/api-types.ts` contain both notification routes and schemas.

- [ ] **Step 5: Run focused backend route tests**

Run: `cd backend && go test ./internal/notification/rest ./internal/router -run 'Notification' -count=1`

Expected: PASS.

### Task 5: Add frontend notification client and bell

**Files:**
- Create: `frontend/src/lib/notifications.ts`
- Create: `frontend/src/lib/notifications.test.ts`
- Create: `frontend/src/components/notification-bell.tsx`
- Create: `frontend/src/components/notification-bell.test.tsx`
- Modify: `frontend/src/app/(app)/layout.tsx`
- Modify: `frontend/src/app/(app)/layout.header.test.tsx`
- Modify: `frontend/src/app/(app)/admin/admin-reports.tsx`

**Interfaces:**
- `listUnreadNotifications(): Promise<Notification[]>`
- `markNotificationRead(id: number): Promise<void>`
- `NotificationBell` consumes `useSession`, `useQuery`, `useMutation`, and `useRouter`.

- [ ] **Step 1: Test API wrappers**

Assert GET uses `/notifications`, PATCH uses `/notifications/:id/read`, and the wrapper preserves generated notification fields.

- [ ] **Step 2: Implement API wrappers and types**

Reuse `api()` and generated OpenAPI types. Keep metadata typed as an object containing numeric `report_id` for this event.

- [ ] **Step 3: Test bell states and interaction**

Assert the bell is hidden without `reports:approve`; shows an accessible label and red unread dot with data; renders empty and loading states; marks an item read; invalidates unread data; and navigates to `/admin?tab=reports`.

- [ ] **Step 4: Implement `NotificationBell`**

Use the existing Radix `DropdownMenu` and `Bell` icon. Poll with `refetchInterval: 30_000`; query only when the user has `reports:approve`. Put the red dot inside the button with a screen-reader unread count. On item selection, mark read, close through Radix selection, invalidate `['notifications','unread']`, then navigate.

- [ ] **Step 5: Place bell in the global header**

Render `NotificationBell` immediately before `ProfileMenu` in `AppLayout`. Update header tests for order, permission visibility, unread dot, and accessible labels.

- [ ] **Step 6: Remove the obsolete disabled bell**

Delete the disabled “Email alerts coming soon” button from `admin-reports.tsx`; the global bell replaces that placeholder.

- [ ] **Step 7: Run focused frontend tests and checks**

Run: `cd frontend && npm test -- --run src/components/notification-bell.test.tsx src/lib/notifications.test.ts && npm run typecheck && npm run lint`

Expected: PASS.

### Task 6: Verify end-to-end behavior

**Files:**
- Create or modify: `frontend/e2e/notifications.spec.ts`
- Modify: existing test fixtures only if required by the current auth setup

- [ ] **Step 1: Add the browser scenario**

Submit a report as a worker, sign in as an approver, assert the bell dot and notification message, click it, assert read state disappears, and assert navigation to `/admin?tab=reports`.

- [ ] **Step 2: Run the focused browser test**

Run: `cd frontend && npm run e2e -- e2e/notifications.spec.ts`

Expected: PASS.

- [ ] **Step 3: Review the diff and stop**

Run: `git diff --check` and inspect only the notification files plus generated Swagger/API artifacts. Do not commit or push unless separately requested.
