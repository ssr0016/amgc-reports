# 07 – Reports Feature Decisions

**Status:** Approved by owner · **Date:** 2026-10-13 · **Baseline:** HEAD 081c1f6
**Inputs:** 05-Backend-Schema.md (Part 2, Part 7), 06-Implementation-Plan.md (§13.1, §13.2), 04-Content-Guidelines.md (§18, §19)
**Scope:** records the owner's picks for D-1..D-11 and D-14. Phase 1 (migrations and seed permissions, 2de41a4) is implemented; the rest is not. The schema (05) and plan (06) docs were synced to Phase 1 in the follow-up docs commit.

## 0. Numbering

Three numbering schemes exist. This doc uses the plan's D-numbers.

| Plan ID | Schema Part 7 | Note |
|---|---|---|
| D-1 | Q1 | |
| D-2 | Q2 | |
| D-3 | Q3 + Q9 | Q9 is the soft-delete lifecycle |
| D-4 | Q4 | |
| D-5 | Q5 | |
| D-6 | Q6 | |
| D-7 | Q7 | |
| D-8 | Q8 + Q13 | Q13 is empty string vs NULL |
| D-9 | none | Only in plan §13.2 |
| D-10 | Q10 | |
| D-11 | Q11 | |
| D-14 | Q14 | |

- Schema §2.1 has its own "Decision 1–7" (owner-approved). Those are not D-numbers.
- `05-Backend-Schema.md` §5.3 says filter semantics are "Part 7, question 12". The correct reference is Q11 (Q12 is `week_no`). Fixed in 05 §5.3.
- D-9 is in plan §13.2, the others in §13.1. D-12, D-13 and D-15..D-17 are unrelated to Reports and are out of scope.
- Not covered by a D-number: Q12 (week 5), Q15 (legacy import), Q16 (gate other endpoints, H-6), Q17 (data-driven seeds). Plan §13.1 treats them as non-blocking.

## D-1 Ownership scope
- **Decision:** Seven `reports:*` permissions: `read`, `read_any`, `write`, `write_any`, `delete`, `delete_any`, `approve`. Without a `_any` variant, a user sees, updates and deletes only own reports. For another user's report the API returns **404**, not 403, on get, update and delete. `approve` is global: permission-gated, with no `_any` variant. (Read scope changed 2026-10-15: every default role now holds `read_any`; see the note under Sub-questions and D-2.)
- **Rationale:** Keeps access permission-based (Decision 6, no hardcoded role). `admin` gets all seven through the seed cross join. 404 doesn't reveal that a report exists.
- **Implications:**
  - **Privacy (2026-10-15):** because the default `user` role holds `read_any`, every signed-in user can read the full text of every report, not only its status: the narrative, the names of new believers and the prayer requests. The 404 rule still hides a report from a role without `read_any`, and still stops anyone but the owner, an editor or an admin from changing one.
  - Schema: none. Seven `permissions` rows.
  - API: the service compares `reports.created_by` with the session user. The list query filters to own reports unless the caller holds `read_any`. The service needs the caller's permission set, but `RequirePermission` (`middleware/rbac.go:44`) only allows or denies, so a lookup helper is needed.
  - Tests: own/other × has/lacks `_any` for read, list, update and delete, expecting 404 or a filtered list. Approve works on a report owned by someone else.
- **Sub-questions (resolved):**
  - Read scope: `read` is own only; `read_any` is all. **Superseded 2026-10-15:** every default role now holds `read_any` (see D-2), so everyone reads every report. `read` alone, without `read_any`, still means own only, for a custom role.
  - Approve scope: global.
  - The 404 rule applies to GET as well as update and delete.
  - Edge: a custom role with `approve` but without `read_any` can approve any report by ID but cannot list them.
- **Feeds:** D-2.

## D-2 Default permissions
- **Decision:**
  - `user`: `read`, `read_any`, `write`, `delete`. Reads every report; changes only their own.
  - `editor`: `read`, `write`, `delete`, `read_any`, `write_any`, `delete_any`. No `approve`.
  - `admin`: all seven, via the cross join. The only role that approves.
  - **Changed 2026-10-15.** Before: `user` had no `read_any` and `editor` also held `approve`.
- **Rationale:** Everyone sees who has reported; editors correct; only the admin approves. Pastoral workers file their own reports. Ownership is enforced by D-1.
- **Implications:**
  - Seed: add the 7 names and extend the hardcoded `editor` and `user` lists in `seed.go`. Seeds only insert, so changing defaults later won't revoke rows already in prod. On 2026-10-15 the `user` role gained `read_any` from the seed on the next boot, and migration `20261014120000_revoke_editor_approve` deleted the editor's `approve` grant.
  - API: routes use `RequirePermission` (its first use). Plan §3.4 assumes H-6 produced the pattern.
  - Tests: seed assertions per role.
- **Sub-questions (resolved):** editor reach (editor gets `write_any` and `delete_any`) and approve scope (global).
- **Depends on:** D-1.

## D-3 `created_by` delete behavior and soft-delete lifecycle
- **Decision:** `ON DELETE RESTRICT`. Soft delete only. No restore, trash view or purge endpoint.
- **Rationale:** Matches the `users.role_id` RESTRICT precedent, and the app has no user-delete feature. Skipping restore avoids the 23505 clash with the partial unique index.
- **Implications:**
  - Schema: as proposed in 05 §2.2, no change.
  - API: DELETE sets `deleted_at`; the 7 endpoints stay as planned.
  - Tests: deleted rows are hidden from get and list; re-creating the same user/month/year succeeds.
- **Sub-questions:** DEFERRED: retention and purge. Deleted narratives stay stored. A future user-delete feature needs an anonymize or transfer path first.

## D-4 JSON form of NUMERIC(12,2)
- **Decision:** JSON number, backed by `json.Number` in and out. No trailing-zero normalization. Values with more than 2 decimal places are rejected with 422.
- **Rationale:** Exact end to end and matches the legacy number shape. Max value (9,999,999,999.99) is below 2^53, so JS clients are safe.
- **Implications:**
  - Schema: none.
  - API/DTO: `*json.Number` fields as in 05 §3.2. Read from `pgtype.Numeric` and emit via its text form. Postgres returns `120.00`, which is valid JSON.
  - Tests: round trip `1234.50`; negative values and more than 2 decimals rejected with 422.
- **Sub-questions (resolved):** the 2-decimal rule is confirmed.
- **Depends on:** wire shape with D-6, D-9.

## D-5 Absent vs zero metrics
- **Decision:** Sparse. A row exists only when a value was supplied. Absent means "not reported" and is not 0.
- **Rationale:** Keeps "reported zero" distinct from "not reported". Max 105 rows per report.
- **Implications:**
  - Schema: none.
  - API: update replaces the metric set. Responses omit absent weeks.
  - Tests: zero vs absent round trip.
- **Sub-questions:** DEFERRED: whether summaries count absent as 0 (Phase 5, aggregations).

## D-6 Metric key case
- **Decision:** `snake_case` metric keys, stored and on the wire. `week1..week5` stays as is.
- **Rationale:** Consistent with the rest of the API (snake_case JSON, Doc 04 §5). The week keys stay for legacy compatibility.
- **Implications:**
  - Schema: the `chk_report_metrics_metric` list uses these 21 keys: `worship_service`, `sunday_school`, `prayer_meeting`, `bible_studies`, `mens_fellowship`, `womens_fellowship`, `youth_fellowship`, `children_fellowship`, `tithes_offering`, `home_visited`, `bible_study_group_led`, `sermon_preached`, `person_newly_contacted`, `person_followed_up`, `person_evangelized`, `outreach`, `training`, `leadership`, `baptism`, `other`, `family_day`. `week_no` stays 1..5.
  - API: the DTO `oneof` list in 05 §3.2 changes to match. `MetricWeeks` keeps `week1..week5`.
  - Tests: one Go key list is the source of truth; a test compares it with the CHECK constraint. Adding a metric still needs a migration.
- **Sub-questions (resolved):** week keys stay. The legacy Express frontend and Excel templates need a mapping for metric names only (camelCase to snake_case).
- **Depends on:** wire shape with D-4, D-9.

## D-7 Length limits and year range
- **Decision:** `worker`, `area_assignment`, `church_name` ≤ 150; `names` ≤ 5,000; `narrative_report`, `challenges`, `prayer_request` ≤ 10,000. Year 2000–2100. No current-year+1 cap.
- **Rationale:** Accepts the Document 2 proposals. The 1 MiB body limit (`MaxRequestBodyBytes`) leaves room.
- **Implications:**
  - Schema: `chk_reports_text_len` and `chk_reports_year` as in 05 §2.2.
  - API: `validate` tags (`max=…`, `gte=2000,lte=2100`). Both count characters, matching `char_length`.
  - Tests: boundary values at limit and limit+1.

## D-8 Required fields and lookup tables
- **Decision:** `worker`, `area_assignment`, `church_name` stay nullable free text. No lookup tables. Empty string is stored as NULL. `worker` is not the author.
- **Rationale:** Matches Express. Lookups can be added later through a nullable FK.
- **Implications:**
  - Schema: none.
  - API: the service maps `""` to NULL. `worker` is independent of `created_by`.
  - Tests: empty to NULL; `omitempty` output.
- **Sub-questions:** DEFERRED: free-text drift ("St. Paul" vs "Saint Paul") weakens summaries. Revisit before Phase 5.
- **Depends on:** D-11.

## D-9 Response envelope
- **Decision:** Keep the current convention.
- **Rationale:** M-1 is an 8h breaking change that would widen scope.
- **Implications:**
  - API: single resource returns a bare `ReportResponse`; lists return `pagination.Response[ReportResponse]` (`{data, meta}`); errors via `apperror`.
  - Plan: M-1 drops to documentation only (0–1h). Settled before Phase 3.
  - Tests: list and single response shape.
- **Depends on:** wire shape with D-4, D-6.

## D-10 audit_logs
- **Decision:** Include `audit_logs`. The audit row is written in the same transaction as the mutation. No read API. `ip_address VARCHAR(64)`. Retention deferred. A migration per new `action` value is accepted.
- **Rationale:** Docs 01 and 02 require audit logging, and a same-transaction write can't diverge from the mutation. `VARCHAR` can't abort the transaction on a malformed IP.
- **Implications:**
  - Schema: 05 §2.5 as proposed. `details` holds `{month, year, count}` only, never narrative or prayer text.
  - API: no `audit_handler.go` (plan §3.4 task dropped, −2h).
  - Tests: each mutation writes exactly one audit row; a rolled-back mutation writes none (see D-14).
- **Sub-questions:**
  - Retention: DEFERRED. Revisit if the table grows.
  - `action` CHECK: ACCEPTED. A new action needs a migration.
  - `DOWNLOAD_SINGLE_EXCEL` and `DOWNLOAD_BULK_EXCEL`: RESOLVED (Option 2). The audit is written server-side where the download happens, not via a client-asserted `POST /reports/log-download`. That endpoint and the two actions are dropped from the API until a server-side Excel export endpoint exists, because client-asserted entries are forgeable and skippable and so prove nothing. Implementation is deferred until exports are built. The two action names stay in the `chk_audit_logs_action` CHECK and in `AuditActions` (harmless; removing them needs a migration), so the future export endpoint can use them without one.
- **Depends on:** D-14.

## D-11 Filter semantics
- **Decision:** Partial, case-insensitive match with `ILIKE` and escaped wildcards. Drop the church/area/worker btree indexes.
- **Rationale:** Express parity. A btree can't serve `%x%`, and a few thousand rows a year doesn't need one.
- **Implications:**
  - Schema: remove `idx_reports_church_name`, `idx_reports_area_assignment`, `idx_reports_worker` from 05 §2.6. Keep the others. No `pg_trgm`.
  - API: `escapeLike` as in 05 §5.3. Filters: `month`, `year`, `worker`, `area`, `church`.
  - Tests: wildcard characters (`%`, `_`, `\`) treated literally; case-insensitivity.
- **Depends on:** D-8.

## D-14 Transaction helper
- **Decision:** INCLUDE. Shape A confirmed: a `WithTx(ctx, pool, func(tx) error)` helper. The new report and audit repositories take a small querier interface satisfied by both the pool and a transaction.
- **Rationale:** Create, update, delete and approve each need one transaction (report + metrics + audit). Today all repositories use the pool directly (Doc 04 §19).
- **Implications:**
  - Narrowed: only the new report and audit repositories (and their mocks) take the querier. Existing repositories don't take part in these transactions and are not retrofitted.
  - The helper is Phase 2 work (plan §3.3, `internal/repository/helpers.go`). Phase 1 (2de41a4) adds none.
  - The helper commits on a nil return and rolls back on error or panic.
  - Tests: rollback test with a failing audit write, asserting no report or metric rows persist.
- **Sub-questions (resolved):** shape = A.

## Dependencies
- **Wire shape:** D-9 (envelope) + D-4 (number form) + D-6 (key case) together define the JSON contract. Settle before Phase 3 handlers.
- **D-8 → D-11:** free text means ILIKE filters. Lookup tables would have made them ID equality.
- **D-1 → D-2:** seed assignments follow the permission set.
- **D-10 → D-14:** same-transaction audit needs the helper (shape A, resolved).
- **First migration (plan §13.2):** D-3, D-6, D-7, D-10 and the D-11 index set are resolved. D-1/D-2 are resolved for the seed. D-9 and D-4 are resolved for Phase 3.

## Open items before implementation
None.

## Deferred / implementation-time
- D-3: retention and purge of soft-deleted narratives.
- D-5: whether summaries count absent as 0 (Phase 5). Decided only for the admin preview Avg (D-A5): weeks with a value, a dash when none.
- D-8: free-text drift in church, area and worker (revisit before Phase 5).
- D-10: audit retention.
- D-10: resolved as Option 2 (server-side audit, no `log-download`); implementation deferred until exports exist.

## Follow-up doc edits (done)
Applied to 05 (Part 2, §5.3, Part 4, Part 7 pointer, Appendix A) and 06 (§3.2 to §3.4, M-1, §13, change log 1.13) in the docs sync commit. Pending in 06: recompute of the effort totals (see change log 1.13).

## Frontend, deployment and admin-panel decisions
Moved to [docs/DECISIONS.md](../../docs/DECISIONS.md): D-F1, D-F2, D-F6, D-F7, D-12, D-A1..D-A7, F-1, L-3. Only D-F8, which shapes `ReportService`, stays here.

### D-F8 Previous-month rule
- **Decision:** `ReportService.Create` rejects any period but the previous month unless the caller holds `reports:write_any` (editor and admin, who may file late). Rejection is `ErrPeriodNotAllowed`, mapped to 422 with `details.month`, the key the form already maps to the month field.
- **Time zone:** months are counted in `Asia/Manila`, loaded once in `NewReportService` (it panics if tzdata is missing; the Dockerfile installs it). The browser's `defaultPeriod()` uses the same zone.
- **Year default:** a missing `year` is the previous month's year when the month asked for is the previous month (December filed in January lands in last year), else the current year in Manila.
- **Not covered:** `Update` is not restricted. The UI locks the period on edit, and the owner chose the smaller scope (2026-10-15). Revisit if the API must refuse a period change.
- **Tests:** service tests pin the clock around the Manila month boundary; the router ownership tests derive the month from the clock. Repository and schema integration tests do not use the service and are unchanged.
