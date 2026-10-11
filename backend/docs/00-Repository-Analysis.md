# Repository Analysis - Pastoral Reports Backend (Go + PostgreSQL)

- **Repository:** github.com/ssr0016/pastoral-reports/backend
- **Analysis Date:** 2026-10-11
- **Purpose:** Baseline reference for the 6 Essential Development Documents

> **Point-in-time analysis.** Describes the code at commit `f40e94c` (2026-10-11), before any implementation work. It is not kept up to date: fixes made since are recorded in [06-Implementation-Plan.md](06-Implementation-Plan.md) and in git history.

> Read-only analysis of the code at commit `f40e94c`. Everything below was taken from the actual files; items that could not be found are marked "Not found".

---

## 1. Go Module Info

```
module github.com/ssr0016/pastoral-reports/backend
go 1.26.0
```

**Direct dependencies (13)**

| Package | Version | Role |
|---|---|---|
| `github.com/labstack/echo/v4` | v4.15.4 | Web framework |
| `github.com/jackc/pgx/v5` | v5.11.0 | PostgreSQL driver and pool |
| `github.com/Masterminds/squirrel` | v1.5.4 | SQL query builder |
| `github.com/pressly/goose/v3` | v3.28.0 | Database migrations |
| `github.com/alexedwards/scs/v2` | v2.9.0 | Session manager |
| `github.com/alexedwards/scs/pgxstore` | 2025-10-02 | Postgres session store |
| `golang.org/x/crypto` | v0.57.0 | bcrypt password hashing |
| `github.com/go-playground/validator/v10` | v10.30.4 | Request validation |
| `github.com/wagslane/go-password-validator` | v0.3.0 | Password strength |
| `github.com/joho/godotenv` | v1.5.1 | `.env` loading |
| `github.com/swaggo/swag`, `swaggo/echo-swagger` | v1.16.6, v1.5.2 | Swagger / OpenAPI docs |
| `golang.org/x/time` | v0.16.0 | Rate limiter |

**Notable indirect dependencies:** `prometheus/client_golang` v1.24.1, `google/uuid` v1.6.0, `stretchr/testify` v1.12.1, `testcontainers-go` v0.44.0 (+ postgres module), `jackc/puddle/v2` v2.2.2.

> Prometheus, testify and testcontainers are used directly but marked `// indirect`. `go mod tidy` will correct this.

---

## 2. Web Framework

- **Framework:** Echo v4.15.4
- **Initialized in:** `cmd/api/main.go` (`echo.New()`); routes registered in `internal/router/router.go`

```go
e := echo.New()
e.Validator = validator.New()
e.HTTPErrorHandler = apperror.ErrorHandler

api := e.Group("/api/v1")
auth := api.Group("/auth")
authLimit := ourmiddleware.RateLimit(rate.Limit(5.0/60.0), 3)
auth.GET("/csrf", handler.CSRFToken)
auth.POST("/register", authHandler.Register, authLimit)
auth.POST("/login", authHandler.Login, authLimit)
// ...
admin := api.Group("/admin",
    ourmiddleware.RequireAuth(sm),
    ourmiddleware.RequireRole(userRepo, sm, "admin"),
)
```

Routes outside `router.Setup` (registered in `main.go`): `/swagger/*`, `/metrics`, `/health`, `/ready`, `/live`, `GET /api/v1/auth/verify-email`, `POST /api/v1/auth/forgot-password`, `POST /api/v1/auth/reset-password`.

---

## 3. Database & Driver

- **Database:** PostgreSQL 15.3 (docker image `postgres:15.3-alpine`)
- **Driver / pool:** `pgx/v5` with `pgxpool`
- **Query builder:** Squirrel (`$` placeholders). No ORM.
- **Initialized in:** `internal/database/db.go`

```go
pool, err := pgxpool.New(ctx, dsn)
if err := pool.Ping(ctx); err != nil { ... }
builder := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
return &DB{Pool: pool, Builder: builder}, nil
```

- **Pool tuning:** None. pgxpool defaults are used (no max connections, lifetimes, or health-check settings).

---

## 4. Migrations

- **Tool:** goose v3, migrations embedded in the binary (`//go:embed migrations/*.sql` in `internal/database/migrate.go`)
- **Location:** `internal/database/migrations/`
- **Auto-run:** only when not production (`if !cfg.IsProduction()`); in production use `make migrate-up`

| # | File |
|---|---|
| 1 | `20250101000001_create_users.sql` |
| 2 | `20250101000002_create_sessions.sql` |
| 3 | `20250918130000_create_roles.sql` |
| 4 | `20250918130001_create_permissions.sql` |
| 5 | `20250918130002_create_role_permissions.sql` |
| 6 | `20250918130003_add_role_to_users.sql` |
| 7 | `20250919140000_add_email_verification.sql` |
| 8 | `20250919150000_create_password_resets.sql` |
| 9 | `20250919160000_add_account_lockout.sql` |

Latest migration:

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN failed_login_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN locked_until TIMESTAMPTZ;
CREATE INDEX idx_users_locked_until ON users(locked_until);
```

Roles, permissions and the optional admin user are seeded in code (`internal/database/seed.go`), not in migrations.

---

## 5. Authentication

| Item | Finding |
|---|---|
| JWT | Not found (session-based auth) |
| Password hashing | bcrypt, `bcrypt.DefaultCost` (10) |
| Sessions | `alexedwards/scs` + Postgres store; cookie `HttpOnly`, `SameSite=Lax`, `Secure` in production, lifetime 24h |
| OAuth | Not found |
| Account lockout | 5 failed logins, locked for 15 minutes |
| CSRF | Double-submit cookie; token exposed via `GET /api/v1/auth/csrf` |
| RBAC | `RequireRole` / `RequirePermission`; roles `admin`, `editor`, `user` |
| Initial admin | Seeded only when `ADMIN_EMAIL` and `ADMIN_PASSWORD` are set |

Auth middleware (`internal/middleware/auth.go`):

```go
func RequireAuth(sm *scs.SessionManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := sm.GetInt64(c.Request().Context(), UserIDKey)
			if userID == 0 {
				return apperror.Unauthorized("authentication required")
			}
			return next(c)
		}
	}
}
```

---

## 6. Configuration

- **Loading:** `godotenv.Load()` + `os.Getenv` through a custom `config.Load()` (`internal/config/config.go`). No viper or envconfig.
- **Validation:** `Config.Validate()` (DATABASE_URL, port, cookie name, CORS origins; admin email/password must be set together, password >= 12 chars).

**Environment variables referenced in code**

| Group | Variables |
|---|---|
| App | `APP_ENV`, `APP_PORT`, `APP_URL` |
| Database | `DATABASE_URL`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` |
| Session | `SESSION_COOKIE_NAME`, `SESSION_LIFETIME`, `SESSION_IDLE_TIMEOUT` |
| Logging | `LOG_LEVEL`, `LOG_FORMAT` |
| CORS | `CORS_ALLOWED_ORIGINS` |
| Admin seed | `ADMIN_EMAIL`, `ADMIN_PASSWORD`, `ADMIN_NAME` |

`.env.example` exists and documents all of the above, plus `TEST_DATABASE_URL`, `TEST_DB_PORT`, `TEST_DB_NAME` and the PgAdmin settings.

---

## 7. Validation

- **Library:** `go-playground/validator/v10`, wrapped in `internal/validator` and set as `e.Validator`
- **Custom rule:** `strongpass` (go-password-validator, entropy 60)
- **Applied:** struct tags on request models/handler DTOs (for example `validate:"required,email"`); handlers call `c.Validate(...)`. Failures return HTTP 422 with `VALIDATION_ERROR`.

---

## 8. Logging

- **Library:** `log/slog` (standard library)
- **Format:** `json` or `text` (default `text`); level from `LOG_LEVEL`; `AddSource` enabled at debug level
- **Configured in:** `internal/logger/logger.go`; set as default logger in `main.go`; request logging in `middleware.SlogLogger`

---

## 9. Middleware

Registered globally in `cmd/api/main.go`, in this order:

| # | Middleware | Purpose |
|---|---|---|
| 1 | `RequestID` | Request correlation ID |
| 2 | `SlogLogger` | Structured request logging |
| 3 | `PrometheusMiddleware` | HTTP metrics |
| 4 | `CSRFProtection` | Double-submit cookie check |
| 5 | `Recover` | Panic recovery |
| 6 | `CORS` | Allowed origins, credentials, `X-CSRF-Token` header |
| 7 | `sm.LoadAndSave` | Session loading, wraps the whole handler |

Per route: `RateLimit` (5 per minute, burst 3, per IP, in memory) on register and login; `RequireAuth`; `RequireRole`.

---

## 10. Testing

- **Libraries:** standard `testing`; testcontainers-go (Postgres) for integration tests; testify is a dependency but not used in current test files.
- **Test files (5):**
  - `internal/apperror/apperror_test.go`
  - `internal/config/config_test.go`
  - `internal/handler/csrf_handler_test.go`
  - `internal/service/auth_service_test.go`
  - `internal/repository/user_repo_integration_test.go` (build tag `integration`)
- **Coverage tooling:** `go test -race -cover` (`make test`); CI uploads `coverage.out` to Codecov.
- **Last measured coverage:** config 85%, service about 30%, apperror 38%; handler, middleware, repository, router, session and validator have little or none.
- **Live API check (2026-10-11):** 43 end-to-end checks against a real server and database; 42 passed, 1 was an incorrect expected status code in the test script.

---

## 11. Build & Deployment

- **Makefile targets:** `dev` (air), `run`, `build`, `clean`, `install`, `tools`, `db-start`, `db-stop`, `db-logs`, `db-reset`, `migrate-up`, `migrate-down`, `migrate-status`, `test`, `lint`, `fmt`, `vet`, `tidy`, `swagger`
- **Dockerfile:** multi-stage (`golang:1.26-alpine` to `alpine:3.19`), `CGO_ENABLED=0`, non-root user, `HEALTHCHECK` on `/health`
- **Docker Compose:** `docker-compose.yaml` (dev: pg, test-pg, pgadmin) and `docker-compose.prod.yaml` (app + pg)
- **CI:** `.github/workflows/ci.yml` at the repository root. Jobs: lint (golangci-lint v2.13.2), test, frontend (lint + build), build, security (gosec). Backend jobs use `working-directory: backend`.
- **Lint config:** `.golangci.yml` (errcheck, govet, staticcheck, revive, noctx, gosec, gocyclo 15, dupl, and others)
- **Hot reload:** `.air.toml`

---

## 12. Other Libraries

| Need | Library | Notes |
|---|---|---|
| UUID | `google/uuid` v1.6.0 | indirect |
| Metrics | `prometheus/client_golang` | `/metrics`, `pkg/metrics` |
| Pagination | custom | `pkg/pagination` |
| API docs | swaggo | generated into `docs/` |
| HTTP client | Not found | |
| Email sending | Not found | no email sender implemented |
| JWT / OAuth | Not found | |

---

## 13. Folder Structure

```
backend/
├── cmd/api/main.go              entrypoint, wiring, middleware
├── internal/
│   ├── apperror/                typed errors and HTTP error handler
│   ├── config/                  env config + stages
│   ├── database/                pgx pool, goose runner, seed, migrations/
│   ├── handler/                 HTTP handlers
│   ├── logger/                  slog setup
│   ├── middleware/              auth, csrf, logger, metrics, ratelimit, rbac
│   ├── model/                   entities and request/response DTOs
│   ├── repository/              data access (Squirrel + pgx)
│   ├── router/                  route registration
│   ├── service/                 business logic
│   ├── session/                 scs session manager
│   ├── testutil/                testcontainers helpers
│   └── validator/               validator wrapper
├── pkg/{metrics,pagination}/
├── docs/{adr/, API.md, ARCHITECTURE.md, swagger.*}
├── Dockerfile, docker-compose*.yaml, Makefile, .air.toml, .golangci.yml
└── README.md, SETUP.md, CUSTOMIZATION.md, CONTRIBUTING.md, ROADMAP.md
```

---

## 14. Summary Table

| Category | Technology | Version | Notes |
|---|---|---|---|
| Language | Go | 1.26.0 | |
| Web framework | Echo | v4.15.4 | |
| Database | PostgreSQL | 15.3 | docker image |
| DB driver | pgx / pgxpool | v5.11.0 | with Squirrel v1.5.4 query builder |
| Migration tool | goose | v3.28.0 | embedded; manual in production |
| Auth | scs sessions + bcrypt | v2.9.0 / x/crypto v0.57.0 | no JWT, no OAuth |
| Config | godotenv + os.Getenv | v1.5.1 | custom loader |
| Validation | validator | v10.30.4 | plus password-validator |
| Logging | slog | stdlib | JSON or text |
| Testing | testing + testcontainers | v0.44.0 | low coverage outside config/service |
| Deployment | Docker, GitHub Actions | alpine 3.19 | previous template deployed on Railway |

---

## 15. Notable Observations & Concerns

**Fixed during this session**

| Item | Resolution |
|---|---|
| Plaintext password-reset token (and email) written to logs | Removed (`dc7bad0`) |
| Hardcoded default admin `admin@example.com` / `AdminPass123!` seeded on every start | Admin now seeded only from `ADMIN_EMAIL` / `ADMIN_PASSWORD` (`7158f34`) |
| CSRF cookie is HttpOnly, so browser JS could not read the token | Added `GET /api/v1/auth/csrf` and `X-CSRF-Token` in CORS (`9ed0a57`) |
| `cmd/api/main.go` missing from the repository (ignored by the `api` pattern in `.gitignore`) | Pattern anchored to `/api`; file tracked (`9ed0a57`) |

**Still open**

1. **`SESSION_IDLE_TIMEOUT` is ignored.** `internal/session/session.go` hardcodes `30 * time.Minute`.
2. **Cookie SameSite mismatch.** Session cookie is `Lax`, CSRF cookie is `Strict`. A frontend and API on different sites will not receive the CSRF cookie.
3. **No automatic migration in production.** `migrate-up` must run before deploy. The Dockerfile copies `/app/migrations`, which is unused because migrations are embedded.
4. **Production compose exposes Postgres** on the host (`${DB_PORT:-5435}:5432`).
5. **No email delivery.** Password-reset and email-verification tokens are generated (verification on registration, 24 h; reset, 1 h) but cannot reach the user. The verification token is also written to the log in plaintext (`internal/service/verification_service.go`), the same issue that was fixed for password reset.
6. **Existing databases keep the old default admin.** Instances seeded before the fix (including the previous Railway deployment) still contain `admin@example.com` with the old password.
7. **Low test coverage** for handlers, repositories and middleware; no end-to-end suite is committed (the live check script lives outside the repo).
8. **Rate limiter is in-memory and per IP.** Not shared across instances and affected by proxies unless the real client IP is configured.
9. **Failed duplicate registrations burn user ID sequence values.** Cosmetic.
10. **Frontend not wired yet.** `frontend/` is a fresh Next.js scaffold with no API integration; `npm audit` reports 5 high-severity advisories in scaffold dependencies.
11. **Leftover template references.** Some README lines still point at the old template's Railway URLs, and `CUSTOMIZATION.md` describes renaming the template.

---

## 16. Open Questions

1. Where will this be hosted (Railway, VPS, other), and who runs migrations on deploy?
2. Which email provider/service will send verification and password-reset messages?
3. Will the frontend and API share a registrable domain (for example `app.` and `api.`)? This decides the cookie `SameSite` strategy.
4. Does the product need roles/permissions beyond `admin`, `editor` and `user` (for example parish or diocese scoping)?
5. What is the "report" domain model (entities, fields, approval workflow) that this backend must serve?
6. Is multi-tenancy or per-organisation data isolation required?
7. Should the old Railway deployment and the old GitHub repository be retired, and its database credentials rotated?
8. What coverage and CI quality gates should be enforced before merging?

---

## 17. Next Steps

The 6 documents to generate next. *This list is proposed from the planning templates that existed in this project's earlier workflow (`spec`, `plan`, `tasks`, `adr`, `checklist`, `notes`); confirm or replace before generating.*

1. **Product / Feature Specification** (`spec`): what the system must do and why.
2. **Technical Plan** (`plan`): how it will be built, mapped onto the stack in this report.
3. **Task Breakdown** (`tasks`): ordered, independently verifiable tickets.
4. **Architecture Decision Records** (`adr`): new decisions, building on `docs/adr/0001` to `0005`.
5. **Quality & Release Checklist** (`checklist`): test, security and deploy gates.
6. **Working Notes / Open Issues Log** (`notes`): running log of findings, including the open items in sections 15 and 16.
