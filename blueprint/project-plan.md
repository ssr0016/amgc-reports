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
  - Status runs Pending → Approved.
  - Soft delete, plus an audit log.
- **Previous-month rule (D-F8):** the month is evaluated in PHT. Only admin can late-file.
- **Auth:** session login with CSRF, `/auth/me`, email verification tokens, password reset (API), and change-password (API + UI).
- **RBAC:** roles, permissions and role-permission assignment, enforced by `RequirePermission` and `RequireRole`.
- **Admin users:** admin CRUD for user accounts.
- **Platform:** rate limiting, body limit, trusted-proxy client IP, Prometheus metrics, health endpoint, Swagger/OpenAPI docs.
- **Audit:** 10 actions — `CREATE_REPORT`, `UPDATE_REPORT`, `DELETE_REPORT`, `APPROVE_REPORT`, `UNAPPROVE_REPORT`, `CREATE_USER`, `UPDATE_USER`, `CHANGE_PASSWORD`, `DOWNLOAD_SINGLE_EXCEL`, `DOWNLOAD_BULK_EXCEL`.

> TODO (confirm): the Excel download actions exist in the audit enum; I did not confirm the download endpoints are shipped. The owner said exports belong to Phase C.

## Data

- PostgreSQL, migrated with goose (`backend/internal/database/migrations/`). Latest: `20261015100000_allow_change_password_audit_action`.
- Core tables: users, roles, permissions, role_permissions, reports (soft delete), audit log, verification tokens, password resets, sessions.
- Seed data runs on API start (permissions, roles, admin account).

## Tech

| Area | Choice |
|------|--------|
| Backend | Go 1.26, Echo, pgx, goose, swaggo, golangci-lint v2.13.2, air |
| Frontend | Next.js 16.4, React 19.3, TypeScript 5, Tailwind 4, shadcn/Radix, TanStack Query |
| API contract | Swagger → OpenAPI → `openapi-typescript` (`npm run gen:api`); CI fails on drift |
| Tests | Go unit + testcontainers integration; Vitest (144) + Playwright (47, incl. axe a11y) |
| CI | `.github/workflows/ci.yml`: lint, test, integration, build, gosec, frontend, e2e |
| Hosting | Railway (backend + Postgres); Frontend: Vercel (app.<domain>); Docker compose for local/prod |

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
- **Domain:** `amgreports.com` is taken; F0 domain setup is still open.

## Open decisions

- None open. Finding #1 is resolved by D-18 (report field visibility).

## Existing docs

`docs/ARCHITECTURE.md`, `docs/DECISIONS.md`, `docs/API.md`, `backend/docs/00..08` (PRD, App Flow, Tech Stack, Content Guidelines, Backend Schema, Implementation Plan, Reports Decisions, Frontend Plan), `backend/docs/ARCHITECTURE.md` (layering and testing), `backend/ROADMAP.md`. These stay the detailed source of truth.
