# Report Admin Notes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Persist an optional admin note when a report is unapproved or sent for revision, and show it to report readers.

**Architecture:** Add `admin_notes` to the existing reports row/model/query scan and pass it through a small optional PATCH body on the existing status endpoints. Keep the admin dialog inline in `AdminReports`, and render the note in the existing `ReportPreviewDialog`.

**Tech Stack:** Go, Echo, pgx/Squirrel, goose, Next.js, React, TypeScript, Vitest, Go tests.

**Spec:** User request in the conversation: default status remains `pending`; add nullable `admin_notes`; accept notes for Unapprove/Request Revision; show the note in the report preview.

## Global Constraints

- Do not rewrite legitimate `needs_revision` rows; the current status migration already defaults new rows to `pending`.
- Do not add dependencies or change the existing status transition semantics.
- Regenerate `frontend/src/lib/api-types.ts` after Swagger changes.
- Run focused tests only; do not run the full suites.

## Review Focus

- Empty or omitted note: status change still succeeds and stores NULL/empty safely — backend handler and frontend action tests.
- Note with text: both PATCH endpoints pass it to the store — handler/service tests.
- Existing report reads: nullable note scans and appears in API response/preview — model/store/frontend tests.
- Existing pending creation: remains `pending` — migration/model coverage and migration inspection.

### Task 1: Backend persistence and status API

**Files:**
- Create: `backend/internal/database/migrations/20261026130000_add_report_admin_notes.sql`
- Modify: `backend/internal/report/model.go`, `backend/internal/report/report.go`, `backend/internal/report/rest/report_handler.go`, `backend/internal/report/reportimpl/store.go`, `backend/internal/report/reportimpl/report.go`, `backend/internal/report/reportimpl/mock.go`
- Test: focused report handler/service/model tests

- [ ] Add a nullable text column and include it in the report model, response, report column list, scanner, and metadata response.
- [ ] Change `ToggleComplete` and `RequestRevision` signatures to accept an optional note; trim empty input to NULL and persist it only on the unapprove/revision transition.
- [ ] Bind and validate the optional PATCH request body in both handlers, preserving empty-body compatibility.
- [ ] Add failing tests for note forwarding/persistence and run them to confirm failure.
- [ ] Implement the minimum backend changes and rerun focused Go tests.

### Task 2: Frontend dialog and preview

**Files:**
- Modify: `frontend/src/lib/reports.ts`, `frontend/src/components/use-report-actions.ts`, `frontend/src/app/(app)/admin/admin-reports.tsx`, `frontend/src/app/(app)/admin/report-preview-dialog.tsx`, `frontend/src/lib/api-schema.ts`
- Test: `frontend/src/app/(app)/admin/admin-reports.test.tsx` and relevant report tests

- [ ] Add optional note bodies to the two API helpers and action mutations.
- [ ] Add an inline controlled AlertDialog with textarea for Unapprove and Request Revision; submit the selected action with the note and keep existing busy/error behavior.
- [ ] Render a prominent note alert when the fetched report status is `needs_revision` and `admin_notes` is present.
- [ ] Add failing UI assertions, verify red, then implement and verify green.

### Task 3: Generated contract and verification

**Files:**
- Modify: `backend/docs/swagger.yaml`, `backend/docs/swagger.json`, `backend/docs/docs.go`, `frontend/src/lib/api-types.ts`

- [ ] Regenerate Swagger and frontend API types using the repository commands.
- [ ] Run focused Go and Vitest tests, plus frontend typecheck/lint if time permits.
- [ ] Review the final diff and git status for secrets/unrelated files.
- [ ] Commit and push the verified change as requested.
