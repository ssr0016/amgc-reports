# Product Requirements Document (PRD)
## Pastoral Reports Backend

**Version:** 1.0
**Date:** 2026-10-11
**Status:** Draft
**Repository:** github.com/ssr0016/pastoral-reports/backend
**Stack:** Go 1.26 + Echo v4 + PostgreSQL 15.3
**Author:** Reverse-engineered by Claude Code (from commit `f40e94c`)

---

> **Reading guide.** This PRD documents what the code does **today**. It is not a wish list.
> Every requirement carries a status: **Implemented** (works and was exercised), **Partial** (exists but incomplete or not enforced), or **Missing** (not in the code).
> The most important finding is in section 2: the repository currently contains an authentication and access-management platform, but **no pastoral-report domain code at all**.

---

## 1. Executive Summary

The Pastoral Reports Backend is a Go HTTP/JSON API (Echo v4, PostgreSQL 15.3, pgx v5) intended to serve a church or organization's pastoral reporting system. At the time of this analysis it implements the **foundation** of that system: user registration and login with server-side sessions, email verification and password reset flows, account lockout, CSRF protection, rate limiting, role-based access control with a permission catalogue, and operational endpoints (health, readiness, Prometheus metrics, Swagger).

The service is a reusable backend starter kit that was renamed for this project. It has **no pastoral report entities, endpoints, tables or business rules yet**. Everything that makes it a *pastoral reporting* system (report creation, pastoral metrics, review/approval, listing and filtering) is still to be designed and built on top of the existing foundation.

Intended users are church administrators, editors/reviewers, and reporters (pastors, workers) who will authenticate through a Next.js frontend (`frontend/` in the same monorepo, currently an unmodified scaffold) using cookie-based sessions.

## 2. Background & Context

**Why it exists.** The owner is building a pastoral reporting web application. The backend was started from a production-style Go template (`template-go-echo-squirrel`) so authentication, RBAC, observability and deployment plumbing would not need to be rebuilt.

**What "pastoral reporting" means here.** The code does not define it. No table, model, route, field or permission refers to reports, pastors, members, congregations or metrics (verified by searching all `.go` and `.sql` files). Any description of report content in this document would be speculation, so section 5.4 records the capability as **Missing** and section 10 lists the questions that must be answered before it can be specified.

**What is real today.**

| Area | State |
|---|---|
| Identity (register, login, logout, sessions) | Implemented |
| Account protection (lockout, rate limit, CSRF, password strength) | Implemented |
| Email verification / password reset (token logic) | Partial (no email delivery) |
| RBAC data model and admin management APIs | Implemented |
| RBAC **enforcement** by permission | Missing (see FR-RBAC-04) |
| Pastoral reports | Missing |
| Audit trail | Missing |

**Relevant constraints from the owner (not verifiable from code).** Target hosting is Railway with Docker. The repository contains no Railway configuration; the only Railway URLs in the README belong to the previous template's deployment.

## 3. Goals & Objectives

### 3.1 Primary Goals
- Provide secure, session-based authentication for a browser frontend.
- Provide role-based access control that administrators can manage at runtime (roles, permissions, assignments).
- Provide a stable, observable, containerised API foundation on which the pastoral report features can be built.

### 3.2 Secondary Goals
- Consistent error format and request correlation (`apperror`, request IDs).
- Self-documenting API (Swagger at `/swagger/index.html`, `docs/API.md`).
- Automated quality gates (golangci-lint, `go test -race`, gosec, CI on GitHub Actions).
- Repeatable local development (`make dev`, docker compose, goose migrations).

### 3.3 Non-Goals (Out of Scope)
These are **not** provided by the current design; some may become goals later.

| Non-goal | Note |
|---|---|
| JWT / bearer tokens / API keys | Session cookies are by design; non-browser clients are not supported |
| OAuth / SSO / social login | Not implemented |
| Email delivery | No mail client or provider integration exists |
| File upload, PDF/Excel export | No code present |
| Multi-tenancy / per-organisation data isolation | No tenant model |
| Real-time features (WebSockets, push) | Not implemented |
| Background jobs / schedulers | None (token cleanup functions exist but nothing schedules them) |
| Outbound HTTP integrations | No HTTP client abstraction |

## 4. Target Audience & User Personas

### 4.1 Admin User
Role `admin`. Holds all nine seeded permissions and is the only role that can reach `/api/v1/admin/*` (guarded by `RequireRole("admin")`).
Can: list/create/update/delete roles; list/create/delete permissions; assign and revoke permissions on roles; view any user with role; change any user's role. Created only by seeding (`ADMIN_EMAIL`/`ADMIN_PASSWORD`) or by another admin changing a role.

### 4.2 Editor User
Role `editor`, seeded with `users:read` and `users:write`. **No route currently checks these permissions**, so today an editor has exactly the same API access as a regular user. The persona exists in data only.

### 4.3 Regular User (Reporter/Worker)
Role `user` (default for self-registered accounts), seeded with `users:read`. Can: register, verify email, log in/out, request and complete password reset, view own session identity (`/auth/me`), list users and view any user by ID. Cannot reach admin routes (403).

### 4.4 System Integrations

| Consumer | Interface |
|---|---|
| Next.js frontend (`frontend/`, same repository) | Cookie session + `X-CSRF-Token`, CORS allow-list (`CORS_ALLOWED_ORIGINS`). No integration code written yet |
| Prometheus scraper | `GET /metrics` (unauthenticated) |
| Orchestrator / load balancer | `GET /health`, `/ready`, `/live` |
| API explorers | `GET /swagger/*` |

## 5. Functional Requirements

Priority: Must / Should / Could. Status: Implemented / Partial / Missing.

### 5.1 Authentication & Session Management

| ID | Requirement | Priority | Status | Evidence / Notes |
|---|---|---|---|---|
| FR-AUTH-01 | Users register with name, email and password; default role `user` | Must | Implemented | `POST /api/v1/auth/register`; duplicate email returns 409 |
| FR-AUTH-02 | Users log in with email and password | Must | Implemented | `POST /api/v1/auth/login`; session token renewed on login |
| FR-AUTH-03 | Sessions are server-side, stored in PostgreSQL (`sessions` table), delivered in an HttpOnly cookie (`Secure` in production, `SameSite=Lax`) | Must | Implemented | `internal/session/session.go`; lifetime 24 h |
| FR-AUTH-04 | Idle session timeout is configurable via `SESSION_IDLE_TIMEOUT` | Should | **Partial** | Variable is parsed by config but ignored; 30 minutes is hardcoded in `session.go` |
| FR-AUTH-05 | Users can log out, destroying the session | Must | Implemented | `POST /api/v1/auth/logout` |
| FR-AUTH-06 | Current-user endpoint | Should | Partial | `GET /api/v1/auth/me` returns only `{"user_id": n}`; no profile or role |
| FR-AUTH-07 | Account locks for 15 minutes after 5 consecutive failed logins | Must | Implemented | Returns 403 while locked, even with the correct password (`auth_service.go`) |
| FR-AUTH-08 | CSRF protection on all non-GET/HEAD/OPTIONS requests (double-submit cookie) | Must | Implemented | Global middleware; token readable via `GET /api/v1/auth/csrf` |
| FR-AUTH-09 | Password strength validation (entropy >= 60 bits) | Must | Implemented | `strongpass` validator; HTTP 422 on failure |
| FR-AUTH-10 | Passwords stored as bcrypt hashes (cost 10) | Must | Implemented | `bcrypt.DefaultCost` |
| FR-AUTH-11 | Email verification: token issued on registration (24 h lifetime), consumed via `GET /api/v1/auth/verify-email?token=` | Must | **Partial** | Token logic works; token is only written to the server log (no email). Login does **not** require a verified email |
| FR-AUTH-12 | Password reset: `POST /auth/forgot-password` then `POST /auth/reset-password` with a single-use token (1 h lifetime) | Must | **Partial** | Logic works and does not reveal whether an email exists; no email delivery, so users cannot receive the token |
| FR-AUTH-13 | Rate limiting on register and login (5 per minute, burst 3, per IP) | Must | Implemented | In-memory; returns 429. **Not applied** to forgot-password or reset-password |
| FR-AUTH-14 | Authenticated users can change their own password | Should | Missing | No endpoint |
| FR-AUTH-15 | Resend verification email | Should | Missing | No endpoint |
| FR-AUTH-16 | Invalidate other sessions on password reset | Should | Missing | Not implemented |

### 5.2 Authorization & RBAC

| ID | Requirement | Priority | Status | Evidence / Notes |
|---|---|---|---|---|
| FR-RBAC-01 | Every user has one role; roles `admin`, `editor`, `user` are seeded | Must | Implemented | `internal/database/seed.go` |
| FR-RBAC-02 | Permission catalogue (`resource:action`), nine seeded: `users`, `roles`, `permissions` x `read`/`write`/`delete` | Must | Implemented | Seeded at startup |
| FR-RBAC-03 | Role to permission mapping managed at runtime | Must | Implemented | `POST/DELETE /admin/roles/:id/permissions` |
| FR-RBAC-04 | Routes enforce **permissions** | Must | **Missing** | `RequirePermission` is implemented in `middleware/rbac.go` but never applied to a route; only `RequireRole("admin")` is used |
| FR-RBAC-05 | Admin routes require role `admin` | Must | Implemented | 403 for other roles; 401 when unauthenticated |
| FR-RBAC-06 | Admin can change a user's role | Must | Implemented | `PUT /api/v1/admin/users/:id/role` |
| FR-RBAC-07 | Admin can create, update, delete roles and permissions | Must | Implemented | Delete returns 204; duplicate names return 409 |
| FR-RBAC-08 | Protect against removing the last admin or deleting built-in roles | Should | Missing | Not checked |

### 5.3 User Management

| ID | Requirement | Priority | Status | Evidence / Notes |
|---|---|---|---|---|
| FR-USER-01 | List users with optional `email` filter and `limit` (default 20, max 100) | Should | Partial | `GET /api/v1/users`; no offset/page, no total, no sorting |
| FR-USER-02 | Get a user by ID | Should | Partial | `GET /api/v1/users/:id`; available to **any authenticated user** for **any user** |
| FR-USER-03 | Admin view of a user including role | Should | Implemented | `GET /api/v1/admin/users/:id` |
| FR-USER-04 | Create / update / delete users (admin) | Should | Missing | Only self-registration and role change exist |
| FR-USER-05 | Profile management (name, email) | Should | Missing | |
| FR-USER-06 | User status tracking | Could | Partial | Columns `email_verified`, `email_verified_at`, `failed_login_attempts`, `locked_until` exist; no active/disabled flag; no admin unlock |

User fields: `id, email, name, role_id, email_verified, email_verified_at, created_at, updated_at` (password hash never serialised).

### 5.4 Pastoral Reports

| ID | Requirement | Priority | Status | Notes |
|---|---|---|---|---|
| FR-REP-01 | Create, read, update, delete pastoral reports | Must | **Missing** | No model, table, route or permission |
| FR-REP-02 | Report content (pastoral metrics, narrative, period, author) | Must | **Missing** | Undefined; see section 10 |
| FR-REP-03 | Listing with filters and pagination | Must | **Missing** | A reusable `pkg/pagination` exists (used by roles and permissions) |
| FR-REP-04 | Review / approval / completion workflow | Should | **Missing** | |
| FR-REP-05 | Report-level access rules (own vs. all reports) | Must | **Missing** | |
| FR-REP-06 | Aggregations / summaries / exports | Could | **Missing** | |

### 5.5 Audit Logging

| ID | Requirement | Priority | Status | Notes |
|---|---|---|---|---|
| FR-AUD-01 | Persistent audit trail of security-relevant and data-changing actions | Should | **Missing** | No audit table or code |
| FR-AUD-02 | Structured request log (method, path, status, latency, request ID) | Must | Implemented | `middleware.SlogLogger` to stdout |
| FR-AUD-03 | Operational log lines for token generation, resets and seeding | Could | Partial | The email-verification token is logged in plaintext (see risk R-11) |

### 5.6 Metrics & Monitoring

| ID | Requirement | Priority | Status | Notes |
|---|---|---|---|---|
| FR-MON-01 | Prometheus endpoint `GET /metrics` | Must | Implemented | Unauthenticated |
| FR-MON-02 | HTTP request count, duration histogram, in-flight gauge | Must | Implemented | `pkg/metrics`, `middleware/metrics.go` |
| FR-MON-03 | Database pool gauges (`db_connections_active`, `db_connections_idle`) | Should | **Partial** | Defined but never updated |
| FR-MON-04 | Business metrics (registrations, logins, reports) | Could | Missing | |
| FR-MON-05 | Liveness / readiness / health | Must | Implemented | `/live`, `/ready` (pings DB, 503 on failure), `/health` |

### 5.7 System Operations

| ID | Requirement | Priority | Status | Notes |
|---|---|---|---|---|
| FR-OPS-01 | Schema migrations with goose, embedded in the binary | Must | Implemented | Nine migrations; run automatically when `APP_ENV` is not production |
| FR-OPS-02 | Production migration procedure | Must | **Partial** | Manual `make migrate-up`; not run by the container |
| FR-OPS-03 | Seed roles, permissions and role mappings (idempotent) | Must | Implemented | Runs on every start |
| FR-OPS-04 | Seed initial admin from `ADMIN_EMAIL` / `ADMIN_PASSWORD` (min 12 chars) | Must | Implemented | No admin is created when unset (warning logged) |
| FR-OPS-05 | Graceful shutdown on SIGINT/SIGTERM (10 s) | Should | Implemented | Not covered by tests |
| FR-OPS-06 | Server timeouts (read 15 s, write 15 s, idle 60 s) | Should | Implemented | `main.go` |
| FR-OPS-07 | API documentation generated from annotations | Should | Implemented | `swag init`; `/auth/csrf` included |
| FR-OPS-08 | Container image and CI pipeline | Must | Implemented | Multi-stage Dockerfile; lint, test, frontend, build, security jobs |

### 5.8 Current API Surface

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/health`, `/ready`, `/live` | none | |
| GET | `/metrics` | none | |
| GET | `/swagger/*` | none | |
| GET | `/api/v1/auth/csrf` | none | Returns `{"csrf_token"}` |
| POST | `/api/v1/auth/register` | none | rate limited |
| POST | `/api/v1/auth/login` | none | rate limited |
| GET | `/api/v1/auth/verify-email` | none | `?token=` |
| POST | `/api/v1/auth/forgot-password` | none | not rate limited |
| POST | `/api/v1/auth/reset-password` | none | not rate limited |
| POST | `/api/v1/auth/logout` | session | |
| GET | `/api/v1/auth/me` | session | |
| GET | `/api/v1/users`, `/api/v1/users/:id` | session | any authenticated user |
| GET/POST | `/api/v1/admin/roles` | admin | |
| GET/PUT/DELETE | `/api/v1/admin/roles/:id` | admin | |
| POST/DELETE | `/api/v1/admin/roles/:id/permissions[/:pid]` | admin | |
| GET/POST | `/api/v1/admin/permissions` | admin | |
| GET/DELETE | `/api/v1/admin/permissions/:id` | admin | |
| GET | `/api/v1/admin/users/:id` | admin | |
| PUT | `/api/v1/admin/users/:id/role` | admin | |

## 6. Non-Functional Requirements

| ID | Area | Requirement (target) | Current state |
|---|---|---|---|
| NFR-PERF-01 | Performance | p95 latency for authenticated reads under 200 ms at expected load | **Unmeasured.** Each protected request does one session lookup in Postgres; bcrypt cost 10 dominates login time |
| NFR-PERF-02 | Performance | Concurrent-user capacity defined | **Undefined.** pgxpool uses driver defaults (no tuning) |
| NFR-SEC-01 | Security | HTTPS only in production; `Secure` cookies | Cookie `Secure` is set when `APP_ENV=production`; TLS termination is external (not in repo) |
| NFR-SEC-02 | Security | bcrypt, lockout, rate limit, CSRF, strong passwords | Implemented (see FR-AUTH) |
| NFR-SEC-03 | Security | No secrets or tokens in logs | **Not met**: verification token logged (R-11) |
| NFR-SEC-04 | Security | No default credentials | Met since `7158f34` (admin seeded from env only) |
| NFR-SEC-05 | Security | Least privilege on data access | **Not met**: any user can list/read all users (FR-USER-01/02) |
| NFR-SCAL-01 | Scalability | Stateless application tier | Met for sessions (stored in Postgres). **Not met** for rate limiting (in-memory per instance) |
| NFR-SCAL-02 | Scalability | Session store scales with traffic | Postgres `sessions` table with expiry index; cleanup is delegated to the store; untested under load |
| NFR-REL-01 | Reliability | Graceful shutdown, panic recovery | Implemented (`Shutdown`, `middleware.Recover`) |
| NFR-REL-02 | Reliability | Database backup / failover strategy | **Undefined** |
| NFR-MAINT-01 | Maintainability | Layered structure (handler, service, repository) | Met |
| NFR-MAINT-02 | Maintainability | Lint and CI on every change | Met (golangci-lint v2, gosec, `-race`) |
| NFR-MAINT-03 | Maintainability | Test coverage | **Low**: config 85%, service ~30%, apperror 38%; handlers, middleware, repositories largely untested (one integration test suite) |
| NFR-OBS-01 | Observability | Structured logs, request IDs, metrics | Implemented; DB gauges not wired |
| NFR-COMP-01 | Compliance | Protection of church-member / pastoral data | **Undefined.** No data classification, retention, deletion or consent features; user list exposes emails to all authenticated users |

## 7. Success Metrics

Proposed targets (not currently measured):

| Metric | Proposed target |
|---|---|
| Login success rate (excluding wrong credentials) | >= 99.5% |
| API latency p95 (reads / writes) | < 200 ms / < 500 ms |
| 5xx error rate | < 0.5% |
| Line coverage (service + handler + middleware) | >= 70% (currently ~30% in service, 0% in handlers) |
| Migration success on deploy | 100%, with rollback tested |
| Open high-severity security findings | 0 |
| Verified-account ratio (once email delivery exists) | >= 90% within 7 days |
| Active reporters per month (once reports exist) | Set by product owner |

## 8. Assumptions & Constraints

### 8.1 Assumptions
- A single organisation uses the system (no tenancy).
- The Next.js frontend and the API are served from the same registrable domain, so cookies and `SameSite` rules work.
- Self-registration is acceptable; there is no invite-only flow today.
- One role per user is sufficient.
- Email delivery will be added before verification/reset are considered usable.

### 8.2 Constraints
- **Technical:** Go 1.26, Echo v4, PostgreSQL 15.3, session cookies (no JWT), goose migrations, Docker.
- **Hosting:** Railway (owner-stated; no configuration in the repository).
- **Team / timeline / budget:** Not stated. The repository history shows a single contributor.
- **Repository:** Monorepo with `backend/` and `frontend/`; CI covers both.

## 9. Risks & Mitigations

| ID | Risk | Impact | Likelihood | Mitigation |
|---|---|---|---|---|
| R-01 | `SESSION_IDLE_TIMEOUT` env var ignored (30 min hardcoded) | Medium: operators cannot tune session policy; documented behaviour is wrong | Certain | Read `cfg.Session.IdleTimeout` in `session.New` |
| R-02 | SameSite inconsistency: session cookie `Lax`, CSRF cookie `Strict` | Medium: CSRF cookie not sent when frontend and API are on different sites, breaking all writes | Medium | Decide domain layout; align both cookies |
| R-03 | No automatic migration in production | High: deploy can run new code against old schema | High | Release step running `goose up`; document in runbook; consider advisory-lock migration at start |
| R-04 | `docker-compose.prod.yaml` publishes Postgres to the host (`${DB_PORT:-5435}:5432`) | High: database reachable from the network | Medium | Remove the `ports` mapping or bind to `127.0.0.1` |
| R-05 | Low test coverage on handlers, repositories, middleware | Medium: regressions in auth/RBAC unnoticed | High | Add handler tests with mocked repos; commit the end-to-end API test script |
| R-06 | Indirect dependencies not tidied (`prometheus`, `testify`, `testcontainers` used directly but marked `// indirect`) | Low | Certain | `go mod tidy` |
| R-07 | Graceful shutdown never verified by a test | Low | Medium | Add a shutdown test or document manual check |
| R-08 | No HTTP client abstraction for external calls | Low now, Medium when email/SMS/storage are added | Medium | Introduce a small client/interface with timeouts and retries before integrating providers |
| R-09 | Session store in Postgres is a scaling bottleneck (one query per request) | Medium at high concurrency | Low | Index on expiry exists; add caching layer or move to Redis if load requires; tune pool |
| R-10 | No JWT/API keys: mobile or third-party clients cannot integrate easily | Medium (if such clients are planned) | Low | Decide scope; add token auth only if a requirement appears |
| R-11 | Email-verification token is logged in plaintext (`verification_service.go`); same class of issue was fixed for password reset in `dc7bad0` | High: anyone with log access can verify arbitrary accounts | Certain | Stop logging the token; deliver it by email |
| R-12 | Permissions are not enforced (`RequirePermission` unused); the `editor` role has no effect | High for the product: RBAC model suggests protection that does not exist | Certain | Apply `RequirePermission` to routes, or remove the unused model |
| R-13 | Any authenticated user can list and read all users | Medium: privacy exposure of member data | Certain | Restrict to `users:read` permission and/or admin; add pagination |
| R-14 | forgot-password / reset-password are not rate limited | Medium: email enumeration pressure, token guessing, spam once email exists | Medium | Apply `authLimit` to both routes |
| R-15 | Existing databases still contain the old default admin (`admin@example.com` / `AdminPass123!`), including the previous template deployment | High where such databases exist | Medium | Rotate or delete that account everywhere; consider a startup warning |
| R-16 | No pastoral report functionality | Critical to the product goal | Certain | Define the domain (section 10) and add it |
| R-17 | Rate limiter is per instance and per IP only | Low-Medium behind proxies or multiple replicas | Medium | Use the real client IP; share state if scaling out |

## 10. Open Questions

1. **Domain:** What is a pastoral report? Which fields, metrics, period, and narrative sections? Who writes, reviews and approves?
2. **Data scope:** Does a report belong to a person, a church, a district? Is there a hierarchy (pastor, overseer, admin)?
3. **Access rules:** Should reporters see only their own reports? What should an `editor` be allowed to do?
4. **Data protection:** What personal or sensitive data will be stored? Retention, deletion, consent and legal obligations?
5. **Email:** Which provider will send verification and reset messages? Is verification mandatory for login?
6. **Hosting:** Railway project, environment variables, TLS and domain setup; who runs migrations on deploy?
7. **Domains:** Will frontend and API share a registrable domain (cookie `SameSite`)?
8. **Frontend:** Frontend lives in `frontend/` of the same monorepo; is that the final location (the brief mentioned a separate repo)?
9. **Operations:** Backup and restore plan, zero-downtime migration approach, retention of logs.
10. **Quality:** Target test coverage and which CI checks are required to merge.
11. **Integrations:** Are mobile apps or external systems planned (would require token-based auth)?
12. **Team:** Number of contributors, timeline and budget.

## 11. Appendix

### 11.1 Glossary

| Term | Meaning |
|---|---|
| PRD | Product Requirements Document |
| RBAC | Role-Based Access Control |
| CSRF | Cross-Site Request Forgery; mitigated here with a double-submit cookie |
| SCS | `alexedwards/scs`, server-side session manager |
| pgx / pgxpool | PostgreSQL driver and connection pool for Go |
| Squirrel | SQL query builder used by the repositories |
| goose | Database migration tool |
| slog | Go standard-library structured logger |
| bcrypt | Adaptive password hashing function |
| Entropy | Password strength measure used by `go-password-validator` (threshold 60 bits) |
| SameSite | Cookie attribute controlling cross-site sending |
| Echo | Go HTTP web framework |
| gosec | Static security analyser for Go |
| Prometheus | Metrics collection system scraping `/metrics` |

### 11.2 References
- Repository: github.com/ssr0016/pastoral-reports (backend in `backend/`)
- Analysis: `backend/docs/00-Repository-Analysis.md`
- Existing documentation: `backend/docs/API.md`, `backend/docs/ARCHITECTURE.md`, `backend/docs/adr/0001`-`0005`
- Commit analysed: `f40e94c` (test: use NewRequestWithContext in csrf handler tests)
