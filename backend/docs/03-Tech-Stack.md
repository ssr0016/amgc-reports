# Tech Stack
## Pastoral Reports Backend

**Version:** 1.0
**Date:** 2026-10-11
**Status:** Draft
**Repository:** github.com/ssr0016/pastoral-reports/backend
**Commit:** f40e94c (application code is unchanged since; later commits only add documentation)

---

> **Method.** Versions were read from `go.mod`, `Dockerfile`, `docker-compose*.yaml`, `.github/workflows/ci.yml`, `Makefile`, `.air.toml` and `.golangci.yml`. "Why" statements are marked **[ADR]** when the repository records the decision in `docs/adr/`, otherwise they are **[Inferred]** from how the code is built. Where nothing in the repository explains a choice, the document says so.

---

## 1. Executive Summary

The backend is a conventional, dependency-light Go service: Go 1.26 with the Echo v4 web framework, PostgreSQL 15.3 accessed through pgx v5 and the Squirrel query builder, goose for schema migrations, and server-side sessions (alexedwards/scs stored in PostgreSQL) instead of JWTs. There is no ORM and no code generation other than Swagger annotations.

Operational concerns are covered with the standard library and a few well-known packages: `log/slog` for structured logs, Prometheus for metrics, three health endpoints, a custom CSRF double-submit middleware, an in-memory rate limiter, bcrypt password hashing and an entropy-based password strength rule. The application is packaged as a multi-stage Docker image and checked by a GitHub Actions pipeline (lint, test, frontend, build, gosec).

The stack is sound for an authentication-and-RBAC foundation. The main weaknesses are in operations rather than language or framework choices: connection pooling is untuned, migrations are manual in production, several configuration values are loaded but ignored, request metrics and logs record wrong status codes for application errors, and there is no dependency vulnerability scan or deployment pipeline. Sections 19 and 20 list these in detail.

## 2. High-Level Architecture

```mermaid
flowchart LR
    CL[Client<br/>Next.js frontend / curl] -->|HTTPS, cookies,<br/>X-CSRF-Token| SCS[scs LoadAndSave<br/>session wrapper]
    SCS --> ECHO[Echo v4 router]

    subgraph MW[Global middleware]
        direction TB
        RID[RequestID] --> LOG[SlogLogger] --> PM[Prometheus] --> CSRF[CSRF] --> REC[Recover] --> CORS[CORS]
    end
    ECHO --> MW
    MW --> RM[Route middleware<br/>RateLimit, RequireAuth, RequireRole]
    RM --> H[Handlers<br/>validator, apperror]
    H --> S[Services<br/>auth, verification, reset]
    S --> R[Repositories<br/>Squirrel + pgx]
    H -.direct.-> R
    R --> PG[(PostgreSQL 15.3<br/>pgxpool)]
    SCS -.session rows.-> PG

    PROM[Prometheus server] -->|scrape| MET[/metrics]
    DEV[Developer] --> SW[/swagger/*]
    LB[Orchestrator / LB] --> HC[/health /ready /live]
    MET --- ECHO
    SW --- ECHO
    HC --- ECHO

    MIG[goose migrations<br/>embedded] -->|startup, non-prod| PG
    SEED[seed.go<br/>roles, permissions, admin] -->|startup| PG
```

Startup sequence in [cmd/api/main.go](../cmd/api/main.go): load `.env` (godotenv) then `config.Load()`, build the slog logger, open the pgx pool and ping, run goose migrations unless production, run seed, create the session manager, wire repositories/services/handlers, register middleware and routes, start `http.Server`, wait for SIGINT/SIGTERM, graceful shutdown (10 s).

## 3. Programming Language

### 3.1 Go Version

| Item | Value | Source |
|---|---|---|
| Language version | `go 1.26.0` | [go.mod](../go.mod) |
| Docker build image | `golang:1.26-alpine` | [Dockerfile](../Dockerfile) |
| CI | `GO_VERSION: '1.26'` | `.github/workflows/ci.yml` |
| Local toolchain observed | go1.26.8 | `go version` |

**Why this version.** The repository does not record a rationale. **[Inferred]** it is the newest stable line at project start, and the Dockerfile, CI and `go.mod` are aligned to it. The Docker tag `1.26-alpine` floats across patch releases, so image builds are not strictly reproducible.

**Language features in use.**
- Generics: `session.Get[T any]` ([internal/session/session.go](../internal/session/session.go)), `pagination.Response[T]` and `NewResponse[T]` ([pkg/pagination/pagination.go](../pkg/pagination/pagination.go)).
- `embed` for migrations; `log/slog`; `errors.Is/As` with wrapped errors; `context` propagation; `sync` for the rate limiter.

### 3.2 Go Toolchain

| Concern | Tool | Notes |
|---|---|---|
| Build | `go build`, `make build` | `CGO_ENABLED=0`, `-ldflags="-s -w"`, output `bin/api` |
| Modules | Go modules, `go mod tidy` (`make tidy`), `go mod verify` in CI | `go.sum` has 319 lines |
| Formatting | `gofmt -w -s` (`make fmt`); golangci-lint also enables `gofmt` and `goimports` formatters | |
| Static checks | `go vet` (`make vet`), golangci-lint (`make lint`) | |
| Dev tools | `make tools` installs air, swag and goose with `@latest` and golangci-lint pinned to v2.13.2 (`GOLANGCI_LINT_VERSION` in the Makefile, matching CI) | air, swag and goose are unpinned, so their versions drift between developers |

## 4. Web Framework: Echo v4

### 4.1 Why Echo
No ADR exists for this choice. **[Inferred]** Echo gives a small, explicit API (groups, middleware chain, centralised error handler, built-in validator hook) on top of `net/http`, with few dependencies. Section 19 compares it with Gin, Fiber and Chi.

### 4.2 Key Features Used

| Feature | Where | Notes |
|---|---|---|
| Routing and groups | `internal/router/router.go`, `main.go` | `/api/v1`, `/auth`, `/users`, `/admin` groups |
| Middleware | `main.go` | RequestID, Recover, CORS from Echo; custom SlogLogger, Prometheus, CSRF, RateLimit, auth, RBAC |
| Context | handlers | `c.Bind`, `c.Validate`, `c.JSON`, `c.Param`, `c.QueryParam`, `c.RealIP` |
| Custom validator | `e.Validator = validator.New()` | go-playground/validator (section 7) |
| Custom error handler | `e.HTTPErrorHandler = apperror.ErrorHandler` | Uniform `{error, message, details}` JSON |
| Server | `http.Server` wrapping `sm.LoadAndSave(e)` | Read 15 s, write 15 s, idle 60 s |

### 4.3 Version and Compatibility
`github.com/labstack/echo/v4 v4.15.4`. Companion modules: `labstack/gommon v0.5.0`, `valyala/fasttemplate`, `swaggo/echo-swagger v1.5.2`.

## 5. Database Layer

### 5.1 PostgreSQL 15.3

| Item | Value |
|---|---|
| Engine | PostgreSQL 15.3 |
| Image | `postgres:15.3-alpine` (dev and prod compose) |
| Admin UI (dev) | `dpage/pgadmin4:6` |

**Why PostgreSQL.** **[Inferred]** relational integrity (foreign keys for users, roles, permissions), unique constraints (used for duplicate prevention and token uniqueness), transactional DDL for goose, and a good fit for the planned reports data (aggregation, JSONB if chosen). Sessions are also stored here **[ADR 0002]**.

**Concern.** `15.3` is a May 2023 minor release; later 15.x minors contain security and bug fixes. The tag should move to the latest 15.x (or to a supported newer major).

### 5.2 Driver: pgx v5

| Item | Value |
|---|---|
| Package | `github.com/jackc/pgx/v5 v5.11.0` |
| Pool | `pgxpool` (`puddle/v2 v2.2.2` indirect) |
| Used for | All queries, session store, goose (via `pgx/stdlib`) |

**pgx vs `database/sql`.** pgx is used natively (not through `database/sql`), giving typed errors (`pgconn`), better performance and Postgres features. The `database/sql` bridge is used only for goose (`stdlib.OpenDBFromPool`).

**Pool configuration.** `pgxpool.New(ctx, dsn)` with **no explicit settings** ([internal/database/db.go](../internal/database/db.go)): max connections defaults to `max(4, NumCPU)`, no `MaxConnLifetime`, no `HealthCheckPeriod` override, no statement timeout. Connection settings can only be changed through DSN parameters (`pool_max_conns`, etc.).

### 5.3 Query Builder: Squirrel v1.5.4

**Why Squirrel (no ORM).** **[Inferred]** explicit SQL, no reflection magic, easy to review, composes dynamic filters (e.g. `email` filter, pagination) without string concatenation. Trade-off: SQL is not type-checked at compile time (unlike sqlc) and results are scanned manually.

Placeholder format is `$1, $2` via `sq.StatementBuilder.PlaceholderFormat(sq.Dollar)`.

Example from `internal/repository/user_repo.go` (lockout counter):

```go
query, args, err := r.db.Builder.
    Update("users").
    Set("failed_login_attempts", sq.Expr("failed_login_attempts + 1")).
    Set("locked_until", sq.Expr("CASE WHEN failed_login_attempts + 1 >= ? THEN NOW() + ?::interval ELSE locked_until END", maxAttempts, lockDuration.String())).
    Set("updated_at", sq.Expr("NOW()")).
    Where(sq.Eq{"id": userID}).
    ToSql()
_, err = r.db.Pool.Exec(ctx, query, args...)
```

Repositories are defined as interfaces in `internal/repository/interfaces.go` with a mock in `mock_repo.go`, which keeps services testable.

### 5.4 Migrations: Goose v3.28.0

| Item | Detail |
|---|---|
| Package | `github.com/pressly/goose/v3 v3.28.0` |
| Embedding | `//go:embed migrations/*.sql` in [internal/database/migrate.go](../internal/database/migrate.go) |
| Location | `internal/database/migrations/` (9 files, `YYYYMMDDHHMMSS_name.sql`) |
| Format | `-- +goose Up/Down` with `StatementBegin/End` |
| Auto-run | `RunMigrations` at startup when `!cfg.IsProduction()` |
| Manual | `make migrate-up | migrate-down | migrate-status` (goose CLI, `DATABASE_URL`) |

**Why goose.** **[Inferred]** simple SQL-first migrations, embeddable, CLI plus library. See section 19.

**Notes.** In production nothing runs migrations automatically (risk R-03 in the PRD). The Dockerfile also copies the migration files to `/app/migrations`, which the binary never reads because they are already embedded.

## 6. Authentication & Sessions

### 6.1 Session Manager: alexedwards/scs v2.9.0

| Item | Value |
|---|---|
| Packages | `alexedwards/scs/v2 v2.9.0`, `alexedwards/scs/pgxstore` (2025-10-02 pseudo-version) |
| Store | PostgreSQL table `sessions(token, data, expiry)` |
| Cookie | name `SESSION_COOKIE_NAME` (default `app_session`), `HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` when `APP_ENV=production` |
| Lifetime | 24 h (`SESSION_LIFETIME`, default 86400 s) |
| Idle timeout | 30 min, **hardcoded** in `session.go`; `SESSION_IDLE_TIMEOUT` is parsed but ignored |
| Fixation defence | `RenewToken` at login |

**Why sessions over JWT. [ADR 0001, 0002]** Server-side sessions can be revoked immediately (logout destroys the row), keep no user data in the browser, avoid JWT signing-key and refresh-token complexity, and fit a first-party browser frontend. Cost: one database lookup per request and no native support for third-party/mobile token clients.

**Config bypass.** `session.New` reads `SESSION_COOKIE_NAME`, `SESSION_LIFETIME` and `APP_ENV` directly with `os.Getenv`, not from the validated `Config`.

### 6.2 Password Hashing: bcrypt

- `golang.org/x/crypto v0.57.0` (`bcrypt`), `bcrypt.DefaultCost` = **10** used in registration, reset and seeding.
- **Why. [ADR 0004]** mature, adaptive, built into the extended standard library, no parameters to mis-tune. Cost 10 is the library default; current guidance often suggests 12 or more for new systems, and bcrypt truncates input at 72 bytes (the reset DTO caps at 72, registration relies on entropy rules).

### 6.3 Password Strength: wagslane/go-password-validator v0.3.0

- Entropy threshold **60** bits, wired as the `strongpass` validator tag in [internal/validator/validator.go](../internal/validator/validator.go).
- Applied to registration. The reset-password endpoint only enforces `required,min=8,max=72`.

### 6.4 CSRF Protection

- Custom middleware [internal/middleware/csrf.go](../internal/middleware/csrf.go) **[ADR 0003]**: double-submit cookie.
- Cookie `csrf_token`: `HttpOnly`, `SameSite=Strict`, `Secure` over HTTPS, max age 24 h. Header `X-CSRF-Token` required on non-GET/HEAD/OPTIONS.
- Because the cookie is HttpOnly, `GET /api/v1/auth/csrf` returns the value in JSON for the browser.
- **Why custom.** **[Inferred]** Echo's built-in CSRF middleware exists; the custom one is small and exposes the token through the endpoint. Note that the double-submit cookie is not signed or bound to the session.

## 7. Validation

### 7.1 go-playground/validator v10.30.4

| Aspect | Detail |
|---|---|
| Hook | `echo.Validator` implemented by `CustomValidator` |
| Tags used | `required`, `email`, `min`, `max`, `omitempty`, custom `strongpass` |
| Where | request DTOs in `internal/model/*.go` and handler-local structs |
| Failure handling | `apperror.Validation(err.Error())` returns HTTP 422 `VALIDATION_ERROR` |

**Why.** **[Inferred]** the de-facto Go struct-tag validator, integrates with Echo's `Validate`. **Gap:** the error message is the raw validator string (struct and field names), not structured per-field `details`.

## 8. Configuration

### 8.1 godotenv v1.5.1
- `_ = godotenv.Load()` at the start of `run()`; a missing `.env` is silently fine.
- `config.Load()` ([internal/config/config.go](../internal/config/config.go)) reads variables with `getEnv`, `getEnvDuration` (seconds or Go duration), `getEnvSlice` (comma separated), derives `Stage` from `APP_ENV` (`prod|production`, `dev|development`, else `local`), then `Validate()`.
- **Why.** **[Inferred]** twelve-factor style, no config framework needed.

### 8.2 Environment Variables

| Variable | Purpose | Required | Default |
|---|---|---|---|
| `APP_ENV` | Stage: local/dev/prod; production hides 5xx messages, sets `Secure` cookie, skips auto-migrate | No | `local` |
| `APP_PORT` | HTTP listen port | No | `8080` |
| `APP_URL` | Public base URL (logs, docs) | No | `http://localhost:8080` |
| `DATABASE_URL` | PostgreSQL DSN used by the app | **Yes** | none |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` | Parsed into config; **not used by the application** (used by compose/Makefile) | No | `localhost`, `5432`, `postgres`, empty, `app_db` |
| `SESSION_COOKIE_NAME` | Session cookie name | No (must be non-empty) | `app_session` |
| `SESSION_LIFETIME` | Absolute session lifetime | No | 24 h (86400) |
| `SESSION_IDLE_TIMEOUT` | Parsed but **ignored** (30 min hardcoded) | No | 30 min |
| `LOG_LEVEL` | debug, info, warn, error | No | `info` |
| `LOG_FORMAT` | `json` or `text` | No | `text` |
| `CORS_ALLOWED_ORIGINS` | Comma-separated allow-list | No (must be non-empty) | `http://localhost:3000` |
| `ADMIN_EMAIL` | Seed admin email; must be set with password | No | empty (no admin) |
| `ADMIN_PASSWORD` | Seed admin password, min 12 chars | No | empty |
| `ADMIN_NAME` | Seed admin display name | No | `Admin` |

Other variables appear only in `.env.example`, compose or Makefile: `TEST_DATABASE_URL`, `TEST_DB_PORT`, `TEST_DB_NAME`, `PGADMIN_PORT`, `PGADMIN_EMAIL`, `PGADMIN_PASSWORD`.

## 9. Logging

### 9.1 log/slog (Go stdlib)
- **Why.** **[Inferred]** standard library since Go 1.21: structured, leveled, zero dependencies, pluggable handlers.
- Format: `JSON` handler when `LOG_FORMAT=json`, otherwise `Text`. Output: stdout.
- Levels: debug, info, warn, error (`LOG_LEVEL`, default info).
- `AddSource` is enabled only at debug level. Set as the default logger (`slog.SetDefault`).
- Configured in [internal/logger/logger.go](../internal/logger/logger.go). Request ID helpers: `WithRequestIDContext`, `WithContext`.

### 9.2 Request Logging Middleware
[internal/middleware/logger.go](../internal/middleware/logger.go) emits one line per request with `request_id, method, uri, status, latency_ms, remote_ip, user_agent, bytes_in, bytes_out` and `error` when present. Level by status: info (<400), warn (400-499), error (>=500).

**Status resolution (fixed in a5ed763, C-2).** The middleware runs before `apperror.ErrorHandler` writes the response, so it derives the final status with `apperror.StatusOf(err)` (handles `*AppError`, `*echo.HTTPError` and unknown errors as 500). If the response is already committed it uses the status on the wire. 401, 403, 422 and 429 now log at `warn` with the real status.

## 10. Observability

### 10.1 Prometheus Metrics
- `github.com/prometheus/client_golang v1.24.1` (used directly in `pkg/metrics` and `main.go`; `go.mod` was tidied so it is now a direct dependency).
- Endpoint `GET /metrics` via `promhttp.Handler()`, unauthenticated.

| Metric | Type | Labels | State |
|---|---|---|---|
| `http_requests_total` | counter | method, path, status | Updated (status derived via `apperror.StatusOf`; fixed in a5ed763, C-1) |
| `http_request_duration_seconds` | histogram | method, path | Updated |
| `http_requests_in_flight` | gauge | none | Updated |
| `db_connections_active` | gauge | none | Defined, never updated |
| `db_connections_idle` | gauge | none | Defined, never updated |

Go runtime/process collectors from the default registry are also exposed. The `path` label is the route pattern, but for unmatched routes it falls back to the raw URL path (unbounded cardinality).

### 10.2 Health Checks

| Endpoint | Checks | Failure |
|---|---|---|
| `GET /health` | Process answers; returns `{"status":"ok"}` | n/a |
| `GET /ready` | `pool.Ping` with 3 s timeout; `{"status":"ready","db":"connected"}` | 503 with the error text |
| `GET /live` | Process alive; `{"status":"alive"}` | n/a |

The Dockerfile `HEALTHCHECK` and compose healthcheck call `/health`. `/ready` exposes the raw driver error to the caller.

### 10.3 Request ID
Echo `middleware.RequestID` sets/propagates `X-Request-ID`; `SlogLogger` copies it into a per-request logger stored in the context (`request_id` attribute). It is not forwarded to the database or returned in error bodies.

## 11. API Documentation

### 11.1 Swagger / OpenAPI
- `swaggo/swag v1.16.6` (CLI and annotations), `swaggo/echo-swagger v1.5.2` (UI), generated into `docs/` (`docs.go`, `swagger.json`, `swagger.yaml`) with `make swagger` (`swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`).
- UI at `GET /swagger/index.html`, served in **all** environments including production.
- **Why.** **[Inferred]** annotations live next to handlers; no separate spec to maintain. The generated files are committed and can drift if `make swagger` is not re-run.
- `swag/v2 v2.0.0-rc4` and `sv-tools/openapi` appear as transitive dependencies.

## 12. Rate Limiting

### 12.1 golang.org/x/time v0.16.0
- Token-bucket `rate.Limiter` per client IP in a mutex-guarded map, cleaned every 3 minutes ([internal/middleware/ratelimit.go](../internal/middleware/ratelimit.go)).
- Applied to `POST /auth/register` and `POST /auth/login`, sharing one bucket per IP: **5 per minute, burst 3** (`rate.Limit(5.0/60.0), 3`). Exceeding returns 429 `RATE_LIMIT_EXCEEDED`.
- **Why.** **[Inferred]** zero extra infrastructure. **Trade-off:** state is in memory, so it resets on restart and is not shared across instances.

### 12.2 Gaps
- Not applied to `forgot-password`, `reset-password`, `verify-email` or any other route.
- Keyed on `c.RealIP()`, which trusts forwarded headers unless trusted proxies are configured, so the limit can be evaded by spoofing headers.

## 13. Testing

### 13.1 Test Frameworks

| Library | Version | Status |
|---|---|---|
| stdlib `testing` + `net/http/httptest` | Go 1.26 | Used in all tests |
| `stretchr/testify` | v1.12.1 | **Used** in `user_repo_integration_test.go` (marked `// indirect`) |
| `testcontainers-go` | v0.44.0 | Used in `internal/testutil/db.go` (marked `// indirect`) |
| `testcontainers-go/modules/postgres` | v0.44.0 | Used for the integration database (marked `// indirect`) |

### 13.2 Test Files

| File | Type |
|---|---|
| `internal/apperror/apperror_test.go` | Unit |
| `internal/config/config_test.go` | Unit (env loading, admin validation) |
| `internal/handler/csrf_handler_test.go` | Unit (httptest + middleware) |
| `internal/service/auth_service_test.go` | Unit (mock repositories) |
| `internal/repository/user_repo_integration_test.go` | Integration, build tag `integration`, needs Docker |

### 13.3 Coverage (measured 2026-10-11)

| Package | Coverage |
|---|---|
| `internal/config` | 85.0% |
| `internal/apperror` | 38.2% |
| `internal/service` | 30.3% |
| `internal/handler` | CSRF handler only (not re-measured) |
| middleware, repository (unit), router, session, validator, database, logger, `pkg/*` | 0% / no unit tests |

A live end-to-end script (43 checks) was run against a real server and database during analysis but is **not committed** to the repository.

### 13.4 Running Tests
- `make test`: `go test ./cmd/... ./internal/... ./pkg/... $(shell test -d api && printf './api/...') -race -cover` (the `./api/...` fragment only matters if an `api` directory exists).
- Unit only: `go test -race -cover ./internal/...`.
- Integration: `go test -tags=integration ./internal/repository/...` (requires Docker).
- CI runs `go test -race -coverprofile=coverage.out -covermode=atomic ./internal/...` and uploads to Codecov (`continue-on-error: true`); the integration tag is **not** run in CI.

## 14. Build & Development

### 14.1 Makefile

| Group | Targets |
|---|---|
| Run/build | `dev` (air), `run`, `build`, `clean`, `install`, `tools` |
| Database | `db-start`, `db-stop`, `db-logs`, `db-reset` |
| Migrations | `migrate-up`, `migrate-down`, `migrate-status` |
| Quality | `test`, `lint`, `fmt`, `vet`, `tidy` |
| Docs | `swagger` |
| Misc | `help` (default) |

`.env` is auto-included and exported by the Makefile.

### 14.2 Hot Reload: Air
[.air.toml](../.air.toml): `go build -o ./tmp/main ./cmd/api/main.go`, watches `go` and `sql` files, excludes `tmp`, `volumes`, `docs`, `bin`, 500 ms debounce, `stop_on_error = true`.

### 14.3 Linting: golangci-lint
[.golangci.yml](../.golangci.yml) (v2 format, 5 m timeout, tests included):
- Enabled: `errcheck, govet, ineffassign, staticcheck, unused, misspell, revive, bodyclose, noctx, rowserrcheck, sqlclosecheck, unconvert, unparam, gocritic, gocyclo (15), dupl (100), gosec`.
- Disabled: `exhaustruct, varnamelen, wrapcheck, depguard`.
- Formatters: `gofmt`, `goimports`.
- Exclusions: generated `docs/`, `dupl` in handlers and two repositories, `gosec`/`dupl`/`gocyclo` in tests, `G117` globally.
- CI and `make tools` both pin golangci-lint v2.13.2; `GOLANGCI_LINT_VERSION` in the [Makefile](../Makefile) must be kept in sync with `ci.yml`. Before the pin, `make tools` installed `@latest` from the v1 module path, so local and CI results differed (the `noctx` rule failed in CI on a test that passed locally).

## 15. Containerization

### 15.1 Dockerfile
[Dockerfile](../Dockerfile), multi-stage:

| Stage | Base | Notes |
|---|---|---|
| builder | `golang:1.26-alpine` | `go mod download` layer, `CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/api ./cmd/api` |
| runtime | `alpine:3.19` | `ca-certificates tzdata wget`; non-root user `app`; `EXPOSE 8080`; `HEALTHCHECK` via `wget /health`; `CMD ["/app/api"]` |

- Migrations are embedded in the binary, so no external files are required (the extra `COPY ... /app/migrations` is unused).
- **Concerns:** `alpine:3.19` is past its upstream end-of-life window and should be verified and upgraded; images are not pinned by digest; no `.dockerignore` review was done beyond its presence.

### 15.2 Docker Compose (Dev)
[docker-compose.yaml](../docker-compose.yaml), project name `template-go-echo-squirrel-db`:

| Service | Image | Host port | Persistence |
|---|---|---|---|
| `pg` | `postgres:15.3-alpine` | `${DB_PORT:-5432}` | `./volumes/databases/pg/data` |
| `test-pg` | `postgres:15.3-alpine` | `${TEST_DB_PORT:-5433}` | `./volumes/databases/test-pg/data` |
| `pg-admin` | `dpage/pgadmin4:6` | `${PGADMIN_PORT:-5050}` | `./volumes/databases/pgadmin` |

Healthchecks on both Postgres services. Container names still carry the template prefix (`template-go-*`).

### 15.3 Docker Compose (Prod)
[docker-compose.prod.yaml](../docker-compose.prod.yaml): services `app` (built from the Dockerfile) and `pg`, bridge network `template-net`, named volume `pg_data`, `restart: unless-stopped`, app depends on `pg` healthy, app healthcheck on `/health`.

> **Security concern:** `pg` publishes `"${DB_PORT:-5435}:5432"` to the host, exposing the database. The `DB_PASSWORD` default is `postgres`, and `sslmode=disable` is hardcoded in `DATABASE_URL`. Names still refer to the template (`template-go-prod`, `template-net`).

## 16. CI/CD

### 16.1 GitHub Actions
[.github/workflows/ci.yml](../../.github/workflows/ci.yml) (repository root), triggers: push and pull request to `main`, `develop`. Backend jobs use `working-directory: backend`.

| Job | Steps |
|---|---|
| `lint` | setup-go, `go mod download`, golangci-lint-action v7 (v2.13.2) |
| `test` | `go mod verify`, `go build ./...`, `go test -race -coverprofile ... ./internal/...`, coverage report, Codecov upload (non-blocking) |
| `frontend` | setup-node 20, `npm ci`, `npm run lint`, `npm run build` in `frontend/` |
| `build` | needs lint + test; static binary build; uploads `api-binary` artifact (7 days) |
| `security` | gosec via `securego/gosec@master`, SARIF upload (non-blocking, `-no-fail`) |

### 16.2 Security Scanning
- **gosec** performs static analysis for Go security issues (hardcoded credentials, weak crypto, SQL string building, unsafe file permissions, etc.). It is configured with `-no-fail`, so findings never fail the pipeline, and it is referenced at `@master` (unpinned).
- golangci-lint also enables `gosec`, and this one **does** fail the lint job.
- Not present: dependency vulnerability scanning (`govulncheck`, Dependabot, `npm audit` gate), container image scanning, secret scanning in CI.

## 17. Deployment

### 17.1 Target: Railway
- **Per README only** (lines 13-16 and "Deployment" section: "Deployed on Railway… Auto-deploy from GitHub"). The README URLs belong to the previous template's deployment.
- **Not verified in the repository:** there is no Railway configuration file; deployment settings live outside the repo.
- Production environment variables to provide: `APP_ENV=production`, `DATABASE_URL`, `CORS_ALLOWED_ORIGINS`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, optional session and logging variables.
- There is no CD workflow; "auto-deploy" is a platform feature, not a pipeline in this repository.

### 17.2 Migration Strategy

| Environment | Behaviour |
|---|---|
| local / dev | `database.RunMigrations` at every startup (goose up) |
| production | **Not run automatically.** Run `make migrate-up` (or goose CLI) against `DATABASE_URL` before or with each deploy |

Seed data (roles, permissions, optional admin) runs on **every** start in all environments and is idempotent. Rollbacks use `make migrate-down`; no zero-downtime or locking strategy is defined.

## 18. Dependencies Summary Table

Direct dependencies (`go.mod` first `require` block) plus runtime components. "Imported directly" reflects source usage, not the `// indirect` marker.

| Category | Package | Version | Purpose |
|----------|---------|---------|---------|
| Language | Go | 1.26.0 | Language and toolchain |
| Web Framework | github.com/labstack/echo/v4 | v4.15.4 | HTTP routing, middleware |
| Database | PostgreSQL | 15.3 | Primary datastore and session store |
| DB Driver | github.com/jackc/pgx/v5 | v5.11.0 | Driver and `pgxpool` |
| Query Builder | github.com/Masterminds/squirrel | v1.5.4 | Composable SQL |
| Migration | github.com/pressly/goose/v3 | v3.28.0 | Embedded schema migrations |
| Session | github.com/alexedwards/scs/v2 | v2.9.0 | Server-side sessions |
| Session Store | github.com/alexedwards/scs/pgxstore | 2025-10-02 | Postgres session persistence |
| Crypto | golang.org/x/crypto | v0.57.0 | bcrypt |
| Validation | github.com/go-playground/validator/v10 | v10.30.4 | Struct validation |
| Password | github.com/wagslane/go-password-validator | v0.3.0 | Entropy-based strength |
| Config | github.com/joho/godotenv | v1.5.1 | `.env` loading |
| API Docs | github.com/swaggo/swag | v1.16.6 | Spec generation |
| Swagger UI | github.com/swaggo/echo-swagger | v1.5.2 | Interactive docs |
| Rate Limit | golang.org/x/time | v0.16.0 | Token bucket |
| Metrics | github.com/prometheus/client_golang | v1.24.1 | Imported directly; marked `// indirect` |
| UUID | github.com/google/uuid | v1.6.0 | Transitive only; not imported by project code |
| Testing | github.com/stretchr/testify | v1.12.1 | Imported in one integration test; marked `// indirect` |
| Testing | github.com/testcontainers/testcontainers-go | v0.44.0 | Integration DB; marked `// indirect` |
| Testing | testcontainers-go/modules/postgres | v0.44.0 | Postgres module; marked `// indirect` |
| Container | postgres:15.3-alpine / alpine:3.19 / golang:1.26-alpine | as listed | Runtime and build images |
| Dev tooling | air, swag CLI, goose CLI, golangci-lint | `@latest` (air, swag, goose) / v2.13.2 (golangci-lint, pinned in CI and Makefile) | Reload, docs, migrate, lint |

`go.mod` also contains about 100 transitive modules (OpenTelemetry, containerd, docker client libraries via testcontainers; logrus; yaml libraries), which are not shipped in the production binary beyond what the application imports.

## 19. Technology Decisions (ADR-style)

Recorded ADRs: 0001 session auth, 0002 Postgres session store, 0003 CSRF double-submit, 0004 bcrypt cost 10, 0005 account lockout. All other rows below are **[Inferred]** rationale; the repository does not document them.

| Decision | Chosen | Alternatives | Rationale and trade-off |
|---|---|---|---|
| Web framework | **Echo v4** | Gin, Fiber, Chi | Echo: explicit middleware and centralised error handling, standard `net/http` compatibility. Gin: larger ecosystem and similar model. Fiber: fasthttp-based, faster but not `net/http` compatible (breaks many libraries such as scs and promhttp). Chi: minimal router, would need more hand-wiring. Low switching cost today because handlers are thin. |
| DB access | **pgx v5 (native)** | `database/sql` + lib/pq, GORM | pgx is the actively maintained Postgres driver with typed errors and pooling. `database/sql` is portable but loses Postgres features. GORM adds an ORM, hiding SQL and adding reflection. |
| SQL layer | **Squirrel** | sqlc, GORM, raw SQL | Squirrel: dynamic filters without code generation, SQL still visible. sqlc: compile-time checked SQL and types, better for fixed queries, needs generation step. GORM: fastest start, least control. |
| Auth | **scs sessions** **[ADR 0001]** | JWT, opaque tokens | Revocable, no token handling in JS, simple for a first-party frontend. Cost: DB lookup per request, no native third-party/mobile token flow. |
| Session store | **Postgres (pgxstore)** **[ADR 0002]** | In-memory, Redis | No extra infrastructure, survives restarts and works across instances. Redis would be faster at high concurrency. |
| Logging | **slog** | zap, zerolog, logrus | Standard library, structured, no dependency. zap/zerolog are faster at extreme volume; logrus is in maintenance mode (and present only transitively). |
| Migrations | **goose** | golang-migrate, atlas | Single binary and library, SQL-first, easy embedding. golang-migrate is similar; atlas adds declarative diffing at higher complexity. |
| Password hash | **bcrypt cost 10** **[ADR 0004]** | argon2id, scrypt | Mature and simple. Argon2id is the current first recommendation by OWASP but needs parameter tuning. |
| Lockout | **5 attempts / 15 min** **[ADR 0005]** | Progressive delay, CAPTCHA | Simple and predictable; susceptible to deliberate account lockout (denial of service) by an attacker who knows an email. |
| CSRF | **Custom double-submit** **[ADR 0003]** | Echo CSRF middleware, SameSite only | Fully controlled token endpoint; unsigned and not session-bound. |
| Metrics | **Prometheus** | OpenTelemetry metrics, StatsD | Standard scrape model, trivial with `promhttp`. |

## 20. Gaps & Recommendations

| # | Gap | Evidence | Recommendation | Priority |
|---|---|---|---|---|
| 1 | `go mod tidy` needed | prometheus, testify, testcontainers imported directly but marked `// indirect` | Run `go mod tidy`; commit | Low |
| 2 | Connection pool not configured | `pgxpool.New(ctx, dsn)` only | Use `pgxpool.ParseConfig`; set max conns, lifetimes, health check period; add statement timeout | Medium |
| 3 | Prometheus/log status bug | AppError responses recorded as 200 | Convert errors with `c.Error(err)` before reading status, or read the status from the error handler | High (blinds monitoring) |
| 4 | DB gauges never updated | `db_connections_*` unset | Update from `pool.Stat()` on scrape or on an interval | Low |
| 5 | Low test coverage | service ~30%, handlers/middleware/repository untested | Add handler and middleware tests; run the integration tag in CI; commit the E2E script | High |
| 6 | Config loaded but ignored | `DB_*`, `SESSION_IDLE_TIMEOUT`, session vars read via `os.Getenv` | Pass `cfg` into `session.New`; drop or use unused fields | Medium |
| 7 | Linter in CI | Present (golangci-lint v2.13.2); `make tools` pins the same version | Keep `GOLANGCI_LINT_VERSION` in the Makefile in sync with `ci.yml` | Low |
| 8 | No dependency vulnerability scanning | No govulncheck / Dependabot / image scan | Add `govulncheck ./...` job, Dependabot or Renovate, and a Trivy/Grype image scan | High |
| 9 | gosec is non-blocking and unpinned | `-no-fail`, `@master` | Pin a version; fail on high severity | Medium |
| 10 | No CD pipeline | Only CI; deploy by platform auto-deploy | Add a release workflow (build image, run migrations, deploy) or document platform steps | Medium |
| 11 | Production migrations manual | `!IsProduction()` guard | Add a release step running `goose up`; consider advisory locks | High |
| 12 | Prod compose exposes Postgres | `ports: "${DB_PORT:-5435}:5432"` | Remove or bind to 127.0.0.1; require a strong password; use TLS | High |
| 13 | Outdated base images | `postgres:15.3`, `alpine:3.19` | Move to latest 15.x or supported major; current Alpine; pin digests | Medium |
| 14 | Swagger and metrics public | `/swagger/*`, `/metrics` unauthenticated in all envs | Restrict in production (network policy, auth, or disable) | Medium |
| 15 | Unpinned dev tools | `@latest` in `make tools` for air, swag and goose (golangci-lint is pinned) | Pin the remaining versions in Makefile or `tools.go` | Low |
| 16 | No HTTP client or email provider | Not present | Introduce an abstraction before adding email | Medium |
| 17 | Naming leftovers from the template | `template-go-*`, `template-net`, README URLs | Rename | Low |
| 18 | Token logged in plaintext | `verification_service.go` | Tracked for the implementation plan; do not log tokens | High (deferred by owner) |

## 21. Appendix

### 21.1 References
- Repository: github.com/ssr0016/pastoral-reports (backend in `backend/`)
- Analysis: `docs/00-Repository-Analysis.md`
- PRD: `docs/01-PRD.md`
- App Flow: `docs/02-App-Flow.md`
- ADRs: `docs/adr/0001` to `0005`
- Configuration files read: `go.mod`, `go.sum`, `Dockerfile`, `docker-compose.yaml`, `docker-compose.prod.yaml`, `Makefile`, `.github/workflows/ci.yml`, `.air.toml`, `.golangci.yml`, `cmd/api/main.go`, `internal/config/config.go`, `internal/database/db.go`

### 21.2 Glossary

| Term | Meaning |
|---|---|
| Echo | Go HTTP web framework |
| pgx / pgxpool | PostgreSQL driver and its connection pool |
| Squirrel | Fluent SQL query builder for Go |
| goose | SQL migration tool |
| scs | Server-side session manager by Alex Edwards |
| pgxstore | scs store backed by PostgreSQL via pgx |
| bcrypt | Adaptive password hashing function |
| Entropy | Measure of password unpredictability, in bits |
| CSRF | Cross-Site Request Forgery |
| Double-submit cookie | CSRF defence comparing a cookie value with a request header |
| slog | Go standard-library structured logging package |
| Prometheus | Metrics system that scrapes `/metrics` |
| gosec | Static security analyser for Go |
| govulncheck | Official Go dependency vulnerability scanner |
| ADR | Architecture Decision Record |
| CI / CD | Continuous Integration / Continuous Delivery |
| Air | Live-reload tool for Go |
| Testcontainers | Library that starts throwaway Docker containers in tests |
| SARIF | Static analysis results interchange format |
