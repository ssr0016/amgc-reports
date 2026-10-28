# Project Plan — AMGCReports

> Adopted from the existing codebase on 2026-10-16. Facts come from the survey; intent comes from the owner interview. Items marked `TODO (confirm)` are inferences.

## Problem

Pastoral reporting across **24 churches** is collected piecemeal. AMGCReports gives church workers one place to submit weekly pastoral reports and gives admins one place to review and approve them.

## Users

| Role | Access |
|------|--------|
| admin | Full access, including approve and late-filing for a previous month |
| editor | `write_any` + `delete_any`; cannot approve (migration `20261014120000_revoke_editor_approve`) |
| user | Plain account with `read_any`; sees every report's metadata and counts but not the free text or tithes (D-18) |

Church workers submit weekly reports. Admins review and approve them.

## Features (shipped)

- **Reports:** one report per month, 5 weeks, 21 metrics per week.
  - Worker, area and church are prefilled from the user profile.
  - Status runs Pending → Approved or Needs Revision; a report exists only once submitted (drafts and server autosave removed, D-A25). A Needs Revision report returns to Pending when the worker saves it.
  - Browser backup of unsent work, one current report per filing period (`GET /reports/current`), admin notes and the worker's revision notes.
  - Soft delete, plus an audit log.
- **Previous-month rule (D-F8):** the month is evaluated in PHT. Only admin can late-file.
- **Auth:** session login with CSRF, `/auth/me`, email verification tokens, password reset (API), and change-password (API + UI).
- **RBAC:** roles, permissions and role-permission assignment, enforced by `RequirePermission` and `RequireRole`.
- **Admin users:** admin CRUD for user accounts, status (active, inactive, disciplinary, deceased), soft delete and restore, admin password reset.
- **Admin panel:** stats cards; Reports tab with bulk approve and unapprove; Tracker (submission and review status); Users; Roles (custom roles, D-A21); Logs (audit trail); Analytics (attendance charts, period comparison, church filter, submission compliance by church and month, CSV, Excel and PDF export).
- **Community Reports:** every worker sees the community's reports (D-18 redaction applies), separate from their own.
- **Profile pictures:** avatar upload (`POST /users/me/avatar`, R2 storage).
- **Platform:** rate limiting, body limit, trusted-proxy client IP, Prometheus metrics, health endpoint, Swagger/OpenAPI docs.
- **Excel export:** single and bulk download in the legacy layout with a read-only preview (`reports:export`; `cf0fcb6`, F-05 fix `d9e32a3`).
- **Audit:** 23 actions: `CREATE_REPORT`, `UPDATE_REPORT`, `SUBMIT_REPORT` (42e), `DELETE_REPORT`, `APPROVE_REPORT`, `UNAPPROVE_REPORT`, `CREATE_USER`, `UPDATE_USER`, `CHANGE_PASSWORD`, `DOWNLOAD_SINGLE_EXCEL`, `DOWNLOAD_BULK_EXCEL`, `CHANGE_USER_STATUS` (22a), `DELETE_USER` and `RESTORE_USER` (22b), `ADMIN_RESET_PASSWORD` (22e), `LOGIN`, `LOGOUT` and `LOGIN_FAILED` (21a), `CREATE_ROLE`, `UPDATE_ROLE`, `DELETE_ROLE` and `CHANGE_USER_ROLE` (37a), `UPDATE_CHURCH_REPORTING` (D-A31). `SUBMIT_REPORT` is history only (D-A25).

## Data

- PostgreSQL, migrated with goose (`backend/internal/database/migrations/`). 38 migrations. Latest: `20261028150200_add_reporting_default`. Added since F-1: `20261016120000_add_user_status` (22a), `20261017120000_allow_admin_reset_password_audit_action` (22e), `20261021100000_create_refresh_tokens`, `20261021120000_add_user_soft_delete` (22b), `20261022120000_allow_login_audit_actions` (21a), `20261022200000_allow_role_audit_actions` (37a), `20261023120000_drop_duplicate_indexes`, `20261024120000_add_user_avatar_url` (33), `20261026120000_add_report_status`, `20261026130000_add_report_admin_notes`, `20261026140000_add_report_revision_tracking` and `20261027120000_add_report_drafts` (42), `20261027140000_remove_report_drafts` (D-A25), `20261028120000_create_login_failures`, `20261028130000_purge_plaintext_password_resets`, `20261028150000_add_person_led_to_christ_metric`, `20261028150100_create_churches` and `20261028150200_add_reporting_default` (D-A31, D-A32).
- Core tables: users (status, soft delete, avatar URL), roles, permissions, role_permissions, reports (soft delete; status `pending`, `approved` or `needs_revision`; admin and user notes), report metrics, churches (reporting months, D-A31), audit log, login failures, verification tokens, password resets, sessions, refresh tokens.
- Seed data runs on API start (permissions, roles, admin account).

## Tech

| Area | Choice |
|------|--------|
| Backend | Go 1.26, Echo, pgx, goose, swaggo, golangci-lint v2.13.2, air |
| Frontend | Next.js 16.4, React 19.3, TypeScript 5, Tailwind 4, shadcn/Radix, TanStack Query |
| API contract | Swagger → OpenAPI → `openapi-typescript` (`npm run gen:api`); CI fails on drift |
| Tests | Go unit + testcontainers integration; Vitest (575) + Playwright (110, incl. axe a11y) |
| CI | `.github/workflows/ci.yml`: lint, test, integration, build, gosec, frontend, e2e |
| Hosting | Render (backend), Neon (Postgres); Frontend: Vercel (app.<domain>); Docker compose for local/prod |

Stack is intentional; nothing is legacy.

## Monetization

None. Internal tool. `TODO (confirm)`

## UI/UX

- shadcn components over Radix, Tailwind 4, Sonner toasts.
- Accessibility is a stated bar: skip link, focus management, announced list updates, axe checks, reduced motion.
- Content guidelines live in `backend/docs/04-Content-Guidelines.md`.

## Constraints and deferred decisions

- **D-F7:** the Cloudflare proxy breaks IP-based rate limits; deferred, not set up yet.
- **E-1:** email delivery is not built. Password recovery and reset-password UI are blocked on it.
- **Domain:** `amgreports.com` is taken, so `amgcreports.com` was chosen (build plan #16); F0 domain setup is deferred.

## Open decisions

- None open. Finding #1 is resolved by D-18 (report field visibility).

## Existing docs

`docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, `docs/API.md`, `backend/docs/00..08` (PRD, App Flow, Tech Stack, Content Guidelines, Backend Schema, Implementation Plan, Reports Decisions, Frontend Plan), `backend/docs/ARCHITECTURE.md` (layering and testing), `backend/ROADMAP.md`. These stay the detailed source of truth.
