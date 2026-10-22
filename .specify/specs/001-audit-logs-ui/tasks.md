# Tasks: Audit Logs UI

**Plan:** [plan.md](plan.md) · Board order: DB → repository → service → handler → swagger → UI. Every ticket is test-first.

## DAG

```
T1 (allow-list guard) ──┐
                        ├──► T3 (sign-in audit) ───────────┐
T2 (migration + 17) ────┤                                  ├──► T6 (e2e + axe)
                        └──► T4 (read API + swagger) ──► T5 (Logs tab) ──┘
                                     └──────────────► T7 (docs)
```

| Ticket | Type | blocked_by | Parallel with |
|---|---|---|---|
| T1 | backend | — | T2 |
| T2 | db + backend | — | T1 |
| T3 | backend | T1, T2 | T4, T5 |
| T4 | backend + swagger | T2 | T3 |
| T5 | frontend | T4 | T3, T7 |
| T6 | e2e | T3, T5 | — |
| T7 | docs | T3, T4 | T5 |

**Deploy rule:** T2 and T3 go out in the same deploy, with the migration run first (`/app/api migrate`). T2 alone is harmless because it only widens the CHECK.

---

## T1. Details allow-list at the single write path
**blocked_by:** none · **parallel:** yes (with T2)

- Add `audit.DetailKeys` (the table in plan.md), `(Log).CheckDetails()` and `ErrDetailKey`.
- Call `CheckDetails` in `auditimpl.store.Insert` and in `auditimpl.MockStore.Insert`.
- **Tests first:**
  - `model_test.go`: every action in `Actions` has an entry in `DetailKeys`. An unknown key is rejected. No allowed key matches the deny-list (password, hash, token, secret, cookie, csrf, narrative, prayer, challenges, names).
  - An integration test: Insert with a bad key writes nothing.

**Done when:** `make test` and `make test-integration` are green with every existing service test unchanged, which proves today's writers fit the allow-list, and `make lint` reports 0 issues.

## T2. Migration and the three new actions
**blocked_by:** none · **parallel:** yes (with T1)

- Add the goose migration `…_allow_login_audit_actions.sql`: the CHECK lists the 14 current actions plus `LOGIN`, `LOGOUT` and `LOGIN_FAILED`. The Down migration uses `NOT VALID`.
- Add `ActionLogin`, `ActionLogout` and `ActionLoginFailed` to `Actions`, and the reason constants.
- Add `DetailKeys` entries for the three new actions. If T1 hasn't merged yet, coordinate: T2 adds only the constants, and T1 or T3 adds their keys.
- **Tests:** `TestActions` expects 17. `TestModelConstants_MatchCheckConstraints` passes.

**Done when:** `make migrate-up`, then `make migrate-down`, then `make migrate-up` again runs cleanly against the dev database; `make test-integration` is green; and `make lint` reports 0 issues.

## T3. Sign-in, sign-out and failed sign-in audit
**blocked_by:** T1, T2 · **parallel:** yes (with T4 and T5)

- `Login(ctx, email, password, ip)` writes one best-effort `LOGIN_FAILED` row on each refusal path, using the reasons in plan.md. An unknown or deleted email gets no target ID or name and no stored email.
- `StartSession(ctx, usr, ip, ua)` writes the refresh token and the `LOGIN` row in one transaction.
- `EndSession(ctx, raw, ip)` writes `LOGOUT` only when it revokes a live session.
- Update the handler, the `auth.Service` interface and the mocks.
- **Tests first:**
  - **Unit:** each refusal path writes exactly one row with the right reason and target. For an unknown email, no email appears anywhere in the row. An Insert error on a refusal still returns the same error type. Each response's status code and message is unchanged (the existing `auth_handler_test` and `auth_login_status_test` stay green).
  - **Integration:** a login writes one `LOGIN` row. When Insert fails during login, the answer is 500 and the `refresh_tokens` count is unchanged. A logout writes one `LOGOUT` row; a second logout writes none. A refresh writes none.

**Done when:** all of the above are green and `make lint` reports 0 issues. The PR notes that it **must deploy with T2's migration**.

## T4. Read API: `GET /admin/audit-logs`
**blocked_by:** T2 · **parallel:** yes (with T3)

- **Repository:** `auditimpl.store.List`, with Manila month bounds, an optional action filter, `ORDER BY created_at DESC, id DESC`, the count, and no `ip_address` in the SELECT.
- **Service:** `auditimpl/service.go` validates the period (the shared month parser moved out of `statsimpl`) and the action, returning `ErrPeriod` or `ErrAction`.
- **Handler:** `audit/rest/audit_handler.go` defaults `limit` to 50. Errors map to 422 with `details.month`, `details.year` or `details.action`, and swaggo annotations are added.
- Register the route in the `admin` group with `limits.Read`, and wire it in `server.go`.
- **Tests first:**
  - **Store integration:**
    - A row at 23:30 Manila on the last day of a month lands in that month.
    - Ties on `created_at` page without repeats or gaps.
    - The action filter works.
    - The count matches the filters.
  - **Service:** a bad month, year or action gives a 422 error.
  - **Handler:**
    - The default limit is 50.
    - The JSON has no `ip_address` key.
    - A non-admin gets 403 through the router test.
    - Viewing the list writes no audit row.

**Done when:** `make test`, `make test-integration` and `make lint` (0 issues) pass, and `make swagger` has been run. Then run `npm run gen:api` in `frontend/` and commit `api-types.ts` in the same commit.

## T5. Logs tab
**blocked_by:** T4 · **parallel:** yes (with T3 and T7)

- Add `lib/audit-logs.ts` (the client, the 17 labels and badge variants, `formatDetail`) and `app/(app)/admin/audit-logs.tsx`.
- Add the third tab to `admin-panel.tsx`. `?tab=logs` must survive a reload.
- **Tests first (Vitest):**
  - **Filters and paging:**
    - The default is the current Manila month, and the Year list starts at 2026.
    - Changing a filter goes back to page 1.
    - Every request sends `limit=50`.
    - The count is announced.
  - **Rows:**
    - Each of the 17 actions renders its label, and an unknown code renders as-is.
    - A row with no actor and no target shows "—".
    - "View details" toggles `aria-expanded` and shows labelled values. No IP and no raw JSON appear, and "No further details." shows when the details are empty.
  - **States:** loading, error with retry, empty, and 403.
  - **Tab:** in `admin-panel.test.tsx`, the third tab works with arrow keys and `?tab=logs` opens it.

**Done when:** `npm run lint`, `npm run typecheck` and `npm test` pass.

## T6. End-to-end and accessibility
**blocked_by:** T3, T5 · **parallel:** no

- Add Playwright coverage:
  - The admin signs in, opens Logs, and sees their own "Logged in" row.
  - A wrong-password attempt by the seeded user shows up as "Failed login" with the reason "Wrong password".
  - An approve action shows up and its details expand.
  - The `user` role has no Logs tab.
  - axe passes on the tab with one row expanded, and the tab reflows at 320 px.

**Done when:** `npm run e2e` is green locally, and CI is green when it runs.

## T7. Docs
**blocked_by:** T3, T4 · **parallel:** yes (with T5)

- `docs/DECISIONS.md`:
  - Amend **D-A4**: failed logins are now logged (owner, 2026-10-22), the IP stays hidden, and the email of an unknown account isn't stored.
  - Amend **D-10**: there's now an admin-only read API, and the details allow-list is enforced in `Insert`.
- `docs/ARCHITECTURE.md`: list all 17 audit actions.
- `docs/API.md`: document `GET /admin/audit-logs`.
- `blueprint/build-plan.md`: tick the item when it ships.
- `backend/docs/08-Frontend-Plan.md` §10: update the Logs rows.
- Then re-run the jdocmunch `index_local`.

**Done when:** the docs match the shipped behavior, and the Obsidian dashboard and session note are updated (CLAUDE.md vault upkeep).
