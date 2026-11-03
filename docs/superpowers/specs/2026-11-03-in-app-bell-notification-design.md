# In-App Bell Notification Design

## Intent

When a worker submits a report, every active account holding `reports:approve` gets an unread in-app notification. Admins can see unread items from the global top bar, mark one read, and open the reports list.

## Scope

Goals:

- Persist report-submitted notifications in Postgres.
- Create all recipient rows atomically with report creation, metrics, and audit.
- Expose unread notifications and an ownership-safe read action.
- Show a polling bell for users with `reports:approve`.

Non-goals:

- WebSockets or SSE.
- Email, push, notification preferences, or a mark-all-read action.
- Notification history after an item is read.
- Deep-linking to a specific report row; Phase 1 navigates to the reports list.

## Approaches

1. Direct application insert inside the existing report transaction. Recommended. It matches the requested in-app reliability model, uses the existing `InTransaction` boundary, and needs no worker.
2. Database trigger. Rejected. Permission resolution and user-facing message construction would move into SQL and become harder to test and evolve.
3. Outbox plus worker. Rejected for Phase 1. It is appropriate for email delivery, but adds delay and machinery for a local database write.

## Decisions

- Recipient eligibility is permission-based: active, non-deleted users whose role grants `reports:approve`; role name `admin` is not special.
- Notification type is `report_submitted`.
- Metadata is an object containing `report_id`.
- Unread results are newest first. Read rows are excluded from `GET`.
- The API prefix remains `/api/v1`; requested routes become `/api/v1/notifications` and `/api/v1/notifications/:id/read`.
- “Real-time” means React Query polling every 30 seconds while the authenticated app shell is mounted. No new transport infrastructure is added.
- The bell is hidden unless the current session has `reports:approve`.
- Clicking an item marks it read, then navigates to `/admin?tab=reports`.

## Security and failure behavior

- `user_id` is always taken from the authenticated request.
- Mark-read updates with both notification ID and current user ID; another user's ID is indistinguishable from not-found.
- Notification creation failure rolls back the report transaction, preventing silent loss of admin alerts.
- Deleted users do not receive new notifications. Existing rows use `ON DELETE CASCADE`.

## Review questions resolved by this proposal

- Recipient set: `reports:approve`, including custom roles.
- Delivery model: direct insert in the report-create transaction.
- Navigation target: reports list, not a new report-detail route.
- Refresh model: 30-second polling.
