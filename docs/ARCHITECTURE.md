# System architecture

Single source of truth for deployment, auth, permissions, audit actions and migration history.
Backend layering and testing rules are in [backend/docs/ARCHITECTURE.md](../backend/docs/ARCHITECTURE.md).
Decisions and their rationale: [DECISIONS.md](DECISIONS.md). API reference: [API.md](API.md).

## System overview

Three tiers:

| Tier | Tech | Where |
|---|---|---|
| Frontend | Next.js 16, React 19, TanStack Query | Vercel, `app.<domain>` (D-F6) |
| API | Go 1.26, Echo, pgx | Render, `api.<domain>` |
| Database | PostgreSQL | Neon |

The browser calls the API directly (client-side fetching). DNS is held by Cloudflare (DNS only; the proxy is deferred, D-F7).
Frontend API types are generated from `backend/docs/swagger.json` (`npm run gen:api`); CI fails if they drift.

> The production domain is not final: `amgreports.com` is taken (F0, D-F2). F14 (production smoke test) waits on it.

## Deployment

- **Render** builds `backend/` from its Dockerfile. Root Directory is `backend`. Health check: `/health` (verify in the Render dashboard).
- **Migrations in production** are not run on startup. `/app/api migrate` runs as a separate step, as the Pre-Deploy Command or by hand (verify in the Render dashboard).
  - Auto-deploy on push to `main`, and whether it waits for CI: verify in the Render dashboard.
  - `TRUSTED_PROXY_CIDRS` is required in production (`100.64.0.0/10` on Render); the server refuses to start without it or with a `/0` range (H-1).
- Runbook and environment variables: [backend/SETUP.md](../backend/SETUP.md).
- **Vercel** hosts the frontend (D-F6). TODO (confirm): current Vercel deployment state; F14 is the production smoke test.
- Local and dev run migrations at startup; production does not.

## Auth

- JWT sign-in (D-AR1), both tokens in HttpOnly cookies, `SameSite=Lax`, `Secure` in production. Access token (`access_token`, HS256, 10 min) carries only `sub`, `sid` (the sign-in) and `iat`; the role and permissions are read from the database on every request. Refresh token (`refresh_token`, path `/api/v1/auth`, opaque, only its SHA-256 stored in `refresh_tokens`) is single use: `POST /auth/refresh` rotates it inside one family, 24 h absolute and 30 min idle. A rotated token seen again after 10 s revokes the family (logged with `slog`, not audited).
- Revocation: password change, admin reset and a status change revoke the user's refresh tokens and set `users.tokens_valid_after`, so older access tokens are refused at once by `RequireRole`, `RequirePermission`, `/auth/me` and change-password. Change password keeps the current sign-in and renews its access token.
- Config: `JWT_SECRET` (>= 32 bytes, required by the server, not by `api migrate`), `JWT_ISSUER`, `JWT_AUDIENCE`, `JWT_ACCESS_TTL`, `REFRESH_TTL`, `REFRESH_IDLE_TTL`, `COOKIE_DOMAIN`. The old `sessions` table is unused and is dropped by a later migration (kept now so a rollback still works).
- Only an `active` account (D-A10) can sign in or keep a session: login checks the status after the password, a status change revokes the user's sessions, and `RequireRole`, `RequirePermission` and `/auth/me` answer 401 for a non-active user.
- CSRF: double-submit token. `GET /auth/csrf`, then `X-CSRF-Token` on POST/PUT/PATCH/DELETE.
- Passwords: bcrypt cost 10; `strongpass` validator (entropy 60); 72-byte limit checked in services.
- Lockout: 5 failed logins lock the account for 15 minutes.
- No self-registration: an admin creates every account.
- Session cookie `Domain=.<domain>` is planned (D-F1).
- ADRs: [0001](../backend/docs/adr/0001-session-auth.md) to [0005](../backend/docs/adr/0005-account-lockout.md).

## Permissions

16 permissions, seeded in `backend/internal/database/seed.go` (seed data, not migrations):

| Resource | Actions |
|---|---|
| users | read, write, delete |
| roles | read, write, delete |
| permissions | read, write, delete |
| reports | read, read_any, write, write_any, delete, delete_any, approve |

| Role | Grants |
|---|---|
| admin | all 16 |
| editor | users:read, users:write, reports:read, reports:write, reports:delete, reports:read_any, reports:write_any, reports:delete_any (no `reports:approve`; revoked by migration `20261014120000`) |
| user | users:read, reports:read, reports:read_any, reports:write, reports:delete |

`user` holds `reports:read_any`, so every signed-in user can list and open every report. D-18 limits what they see: without ownership, `write_any` or `delete_any`, the free text (`names`, `narrative_report`, `challenges`, `prayer_request`) and the `tithes_offering` metric are withheld.

**Duplicated rule:** the backend applies it in `ReportService.redact` (`backend/internal/service/report_service.go`) and the frontend repeats it in `isRestricted` (`frontend/src/lib/report-form.ts`) so the detail page can show "Restricted" instead of empty fields. Keep the two in sync; the API does not signal redaction.

**Known limitation:** the field-visibility rule is duplicated in `ReportService.redact` (backend) and `isRestricted` (frontend). They must be kept in sync; a drift would show the wrong view but not leak data, since the backend is authoritative. No automated test covers the sync.

## Layers

Handler → service → repository. Simple CRUD may skip the service. Rules: [backend/docs/ARCHITECTURE.md](../backend/docs/ARCHITECTURE.md).
Errors go through `apperror`. Rate limits: a per-IP global limit, a per-user limit on report writes (60/min), and a per-IP limit on token routes.

## Report lifecycle

1. A worker creates a report for a month. Only the previous month (Asia/Manila) is allowed, unless the caller has `reports:write_any` (D-F8).
2. The report holds 5 weeks × 21 metrics. Worker, area and church come from the profile.
3. Status starts Pending. Only `reports:approve` (admin) approves or unapproves.
4. Delete is a soft delete.
5. Each mutation writes one audit row in the same transaction (D-10, D-14).

## Audit actions

12 actions, enforced by the `chk_audit_logs_action` CHECK (DDL: [05-Backend-Schema.md](../backend/docs/05-Backend-Schema.md) §2.5). A new action needs a migration, shipped in the same deploy as the code that writes it.

| Group | Actions |
|---|---|
| Reports | `CREATE_REPORT`, `UPDATE_REPORT`, `DELETE_REPORT`, `APPROVE_REPORT`, `UNAPPROVE_REPORT` |
| Users | `CREATE_USER`, `UPDATE_USER`, `CHANGE_PASSWORD`, `CHANGE_USER_STATUS`, `ADMIN_RESET_PASSWORD` |
| Reserved | `DOWNLOAD_SINGLE_EXCEL`, `DOWNLOAD_BULK_EXCEL` (written only by a future server-side export, D-10). Reserved, Phase C; no endpoint is confirmed shipped |

## Migration history

goose, in `backend/internal/database/migrations/`. 22 migrations.

| Era | Migrations |
|---|---|
| Starter kit, 2025-01 | `20250101000001_create_users`, `…02_create_sessions` |
| RBAC, 2025-09 | `20250918130000_create_roles`, `…01_create_permissions`, `…02_create_role_permissions`, `…03_add_role_to_users` |
| Auth, 2025-09 | `20250919140000_add_email_verification`, `20250919150000_create_password_resets`, `20250919160000_add_account_lockout` |
| DB roles, 2025-10 | `20251012000000_create_app_user_role`, `…01_create_migrate_user_role`, `…02_add_user_link_to_sessions` |
| Reports, 2026-10-13 | `20261013140000_create_reports`, `…140100_create_report_metrics`, `…140200_create_audit_logs` |
| Users and audit, 2026-10-14 | `20261014100000_add_user_profile_fields`, `…100100_extend_audit_actions_users`, `…110000_drop_user_worker`, `…120000_revoke_editor_approve` |
| 2026-10-15 | `20261015100000_allow_change_password_audit_action` |
| 2026-10-16 | `20261016120000_add_user_status` (`users.status`, `CHANGE_USER_STATUS`) |
| Latest, 2026-10-17 | `20261017120000_allow_admin_reset_password_audit_action` (`ADMIN_RESET_PASSWORD`) |

## CI

`.github/workflows/ci.yml`: Go lint, tests, integration tests (testcontainers), build, gosec; frontend lint, `gen:api` drift check, typecheck, build; Playwright e2e. Whether a red CI blocks a deploy depends on Render's auto-deploy setting (verify in the Render dashboard).

## Reference material

- `backend/docs/reference/report-analysis.docx` — legacy snapshot of the
  Express/MongoDB report backend. Not authoritative: where it differs, the
  code and 07-Reports-Decisions.md win. **Contains no Excel template**
  (D-A6 is still waiting for the legacy workbook).
