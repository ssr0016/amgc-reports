# Plan: Audit Logs UI

**Spec:** [spec.md](spec.md) · **Tickets:** [tasks.md](tasks.md)

## Approach

The table, the indexes, the snapshot columns and 14 of the 17 actions already exist. This feature adds three pieces:

1. **A guard at the single write path.** `audit.Store.Insert` is the only way a row is written, so the details allow-list is checked there. It fails closed: a disallowed key returns an error, and the change it audits rolls back.
2. **Sign-in auditing in the auth service.** It adds three actions and a CHECK migration, shipped in the same deploy as the code that writes them.
3. **A read slice.** A read-only `audit.Reader` and an admin-only `GET /admin/audit-logs` endpoint, followed by a Logs tab built the same way as the Reports and Users tabs.

## Architecture

### Backend (`backend/internal/audit/`)

| Layer | Change |
|---|---|
| `audit/model.go` | Add `ActionLogin`, `ActionLogout` and `ActionLoginFailed` (stored as `LOGIN`, `LOGOUT`, `LOGIN_FAILED`), bringing `Actions` to 17. Add `DetailKeys map[string][]string`, the allow-list per action, taken from the current writers (see below). Add `(Log).CheckDetails() error`. Add the response DTO `LogResponse`, which has **no IP field**, and `ToResponse`. Add the failed-login reasons `ReasonUnknownAccount`, `ReasonWrongPassword`, `ReasonLocked`; a refused status reuses the status value. |
| `audit/audit.go` | Keep `Store` (Insert). Add `Reader` with `List(ctx, ListParams, pagination.Params) ([]Log, int64, error)` and `Service` with `List(ctx, ListQuery, pagination.Params)`. `ListQuery` has `Month string`, `Year int` and `Action *string`. |
| `auditimpl/store.go` | `Insert` calls `CheckDetails` first. `List` selects every column except `ip_address`, filters `created_at >= $from AND created_at < $to` (bounds computed in Asia/Manila), optionally filters `action = $a`, and orders by `created_at DESC, id DESC`. The count shares the same WHERE clause. It runs on the existing `idx_audit_logs_created_at` and `idx_audit_logs_action` indexes, so no new index is needed. |
| `auditimpl/mock.go` | `Insert` also calls `CheckDetails`, so every service unit test that writes an audit row checks the allow-list (spec success criterion 6). |
| `auditimpl/service.go` (new) | Loads Asia/Manila once (and panics without tzdata, as `statsimpl` does). Validates the month name, using the same month-name parsing as `statsimpl.canonicalMonth`, moved to a small shared spot rather than copied. Validates the year (2000 to 2100) and the action against `Actions`. Returns `ErrPeriod` or `ErrAction`, which become 422 errors. |
| `audit/rest/audit_handler.go` (new) | `GET /admin/audit-logs?month=&year=&action=&page=&limit=`. When `limit` is absent it defaults to **50**; otherwise `FromQuery`'s cap of 100 applies. Responds with `pagination.Response[audit.LogResponse]`. Has swaggo annotations. |
| `router/router.go`, `cmd/api/app/server.go` | Register the route in the `admin` group (which already requires the admin role) with `limits.Read`. Wire the store, service and handler. |
| Migration `YYYYMMDDHHMMSS_allow_login_audit_actions.sql` | Drop and re-add `chk_audit_logs_action` with the 14 current actions plus the 3 new ones. The Down migration uses `NOT VALID`, as in the earlier audit-action migrations. |

**Allow-list (from the writers in the code today):**

| Action | Keys |
|---|---|
| The 5 report actions, `DOWNLOAD_SINGLE_EXCEL`, `DOWNLOAD_BULK_EXCEL` | `month`, `year`, `count` |
| `CREATE_USER` | `role_id` |
| `UPDATE_USER`, `LOGIN`, `LOGOUT` | none |
| `CHANGE_USER_STATUS` | `from`, `to`, `sessions_revoked` |
| `DELETE_USER` | `reason`, `sessions_revoked` |
| `RESTORE_USER` | `deleted_at`, `deleted_by`, `deleted_reason` |
| `CHANGE_PASSWORD`, `ADMIN_RESET_PASSWORD` | `sessions_ended` |
| `LOGIN_FAILED` | `reason` |

A deny-list test also asserts that no allowed key contains `password`, `hash`, `token`, `secret`, `cookie`, `csrf`, `narrative`, `prayer`, `challenges` or `names`.

### Auth (`backend/internal/auth/`)

- **`Login(ctx, email, password, ip)`** gains the IP. Each refusal path writes one `LOGIN_FAILED` row on a best-effort basis: there's no surrounding change to roll back, and the response must not change, so a failed insert is logged as a warning and the usual error is returned.
  - **Unknown or deleted email:** `PerformedBy` empty, `TargetType` `User`, `TargetID` and `TargetName` empty, `reason=unknown_account`. The email isn't stored.
  - **Locked account:** the target is the user, `reason=locked`.
  - **Wrong password:** the target is the user, `reason=wrong_password`.
  - **Refused by status:** the target is the user, `reason=<status>`.
- **`StartSession(ctx, userID, ip, ua)`** keeps its signature (built differently from the first draft, which passed the user in): inside one `s.tx.InTransaction` it loads the user with `GetWithRole` (the row needs the name and role), stores the refresh token and inserts `LOGIN` (`user.AuditLog`, actor and target the user). If the audit row fails, no refresh token survives and the handler answers 500 (spec behavior 6). No caller changed.
- **`EndSession(ctx, raw, ip)`**: inside the existing transaction, once `t != nil`, it loads the user and inserts `LOGOUT`. An unknown or already-revoked token writes nothing. `GetForUpdate` must return revoked tokens as "no live session": check this in T3.
- **Not touched:** `Refresh` and `rotate` (the spec's non-goals).

### Frontend (`frontend/src/`)

| File | Change |
|---|---|
| `lib/audit-logs.ts` (new) | `listAuditLogs({month, year, action, page})` sends `limit=50`. `ACTION_LABELS` maps the 17 codes to labels and badge variants. `formatDetail(action, key, value)` turns a value into words (reason codes, `from → to`, "Sessions ended: N"). Types come from `api-types.ts`. |
| `app/(app)/admin/audit-logs.tsx` (new) | `AuditLogs` follows the `AdminReports` pattern: TanStack Query, Month and Year `NativeSelect` (current Manila month; years from `FIRST_YEAR` to now), an Action select, "N total logs" as a `role="status"`, the house `Table`, Previous and Next, and the loading, error, empty and 403 states. Each row has a "View details" `Button` with `aria-expanded` and `aria-controls` that toggles a full-width row underneath (a `<tr>` with a `<td colSpan>`) holding a `<dl>`. It doesn't need a new component or dependency. |
| `app/(app)/admin/admin-panel.tsx` | Add a third `TabsTrigger`/`TabsContent` "logs". Widen the `?tab=` parse to `reports`, `users` or `logs`. |
| Manila current month | Reuse whatever `defaultPeriod()` builds on in `lib/report-form.ts` (Asia/Manila). Add a `currentPeriod()` next to it if needed, rather than a new date library. |

## Key decisions

| Decision | Why |
|---|---|
| Enforce the allow-list in `Insert` (fail closed) rather than only in tests | Insert is the single write path, so no future writer can skip it. The allow-list is built from today's writers, so nothing that works now starts failing. The mock store runs the same check, so drift fails a unit test before it reaches production. |
| Failed-login rows are best-effort, the `LOGIN` row is transactional | A failed login has no change to tie the row to, and a 500 there would be a different response, which the spec rules out. A login creates a session, which is a change, so D-10 applies. |
| No stored email for an unknown account | People sometimes type a password into the email box, and D-A11 requires a deleted account to look the same as an unknown one. |
| IP excluded in the SQL SELECT and the DTO, not just hidden in the UI | The spec requires that it isn't in the API response either (D-A4). |
| Inline expandable row, not a side sheet | The repo has no Sheet or Dialog wrapper beyond `AlertDialog`, and an inline row needs no focus trap. |
| Default page size of 50 set in the handler, not by changing `pagination.DefaultLimit` | The other list endpoints keep their default of 20. |
| No new index | `created_at DESC` and `action` are already indexed, and one month at a time keeps each scan bounded. |

## Risks

- **Migration order (P0 lesson from F-1).** T3 writes `LOGIN`, so T3's code and T2's migration ship in the same deploy, and production runs `/app/api migrate` before the new code serves traffic. The plan says so in T3's done-when.
- **Fail-closed `Insert`.** A writer outside the allow-list would now answer 500. To guard against that, T1 runs every existing service test through the checking mock before merge.
- **Failed-login volume.** Rows are bounded by the existing login rate limiter. Retention is still deferred (D-10).
- **`StartSession` costs one more query per login** (`GetWithRole`), in exchange for no signature change.

## Verification

- Backend: `make lint` (0 issues), `make test`, `make test-integration`, `make swagger`.
- Frontend: `npm run gen:api` (no drift), `npm run lint`, `npm run typecheck`, `npm test`, `npm run e2e` (including axe and 320 px).
- Stop the dev server before `npm run build`.
