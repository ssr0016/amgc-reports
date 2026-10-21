# Content Guidelines
## Pastoral Reports Backend

**Version:** 1.0
**Date:** 2026-10-11
**Status:** Reference (Documents Existing Conventions)
**Repository:** github.com/ssr0016/pastoral-reports/backend
**Commit Reference:** f40e94c

> **Purpose:** This document captures the CONVENTIONS ALREADY IN USE in the codebase.
> It does NOT propose changes. It serves as a reference for new developers.
> Any improvements or fixes are tracked in `06-Implementation-Plan.md`.

---

> **Notation.** Every statement below was read from source files. Where the code is inconsistent with itself, the pattern that is used **most** is described as "the convention" and the exceptions are listed in section 18. Where nothing exists in the code, the document says **"No established convention"** and does not invent one.

---

## 1. How to Use This Document

- **New developers:** Read this before writing code
- **Reviewers:** Check PRs against these conventions
- **When in doubt:** Follow the existing code, then update this doc if patterns change

Quick lookup:

| I need to... | Go to |
|---|---|
| Return a response | section 2 |
| Return an error | sections 2.5, 3 |
| Add a request body | sections 4, 5, 6 |
| Add a list endpoint | section 9 |
| Add a route | section 10 |
| Add a handler / service / repository | sections 4, 12 |
| Add a migration | section 13 |
| Add Swagger annotations | section 14 |
| Write a test | section 16 |
| Write a commit message | section 17 |

---

## 2. Response Format (As Implemented)

> **Important:** the API does **not** wrap single resources in a `{"data": ...}` envelope. A resource is returned as the bare DTO object. Only paginated lists use `{"data": [...], "meta": {...}}`.

All responses are `application/json` produced with `c.JSON(status, value)` unless the status is 204.

### 2.1 Success: Single Resource

Bare DTO object, HTTP 200 (read/update/action) or 201 (create). Source: `UserHandler.GetUser`, `RoleHandler.Get/Create/Update`, `AdminUserHandler.ChangeRole`.

```json
{
  "id": 3,
  "email": "user1@example.com",
  "name": "User One",
  "role_id": 2,
  "email_verified": true,
  "email_verified_at": "2026-10-11T00:01:20.123456+08:00",
  "created_at": "2026-10-11T00:01:18.973842+08:00"
}
```

Role example (`RoleResponse`):

```json
{
  "id": 17,
  "name": "qa-lead",
  "description": "can edit",
  "permissions": [{ "id": 1, "name": "users:read", "resource": "users", "action": "read" }],
  "created_at": "2026-10-11T00:01:18.97+08:00",
  "updated_at": "2026-10-11T00:01:18.97+08:00"
}
```

Note: `RoleResponse.Permissions` has `omitempty`; Go's JSON encoder omits empty slices with `omitempty`, so a role without permissions has no `permissions` key.

### 2.2 Success: Paginated List

Used by `GET /admin/roles` and `GET /admin/permissions` (`pagination.NewResponse`). Source: `pkg/pagination/pagination.go`.

```json
{
  "data": [ { "id": 1, "name": "admin", "description": "...", "created_at": "...", "updated_at": "..." } ],
  "meta": { "page": 1, "limit": 20, "total": 3, "total_pages": 1 }
}
```

`data` is never `null` (nil slices are replaced by `[]`).

### 2.3 Success: Unpaginated List

`GET /users` returns a **bare JSON array** of `UserResponse` (`[]` when empty), with no `meta`.

```json
[ { "id": 1, "email": "admin@example.com", "name": "Admin", "role_id": 2, "email_verified": false, "created_at": "..." } ]
```

### 2.4 Success: Message Only

Actions that return no resource use a flat object with a single `message` key (`map[string]string`), HTTP 200:

| Endpoint | Body |
|---|---|
| `POST /auth/logout` | `{"message": "logged out"}` |
| `GET /auth/verify-email` | `{"message": "email verified successfully"}` |
| `POST /auth/forgot-password` | `{"message": "if the email exists, a reset link has been sent"}` |
| `POST /auth/reset-password` | `{"message": "password reset successful"}` |
| `POST /admin/roles/:id/permissions` | `{"message": "permission assigned"}` |

Other small shapes: `GET /auth/me` returns `{"user_id": <int>}`; `GET /auth/csrf` returns `{"csrf_token": "<string>"}`; health endpoints return `{"status": "ok" | "alive" | "ready"}` (`/ready` also `"db": "connected"`).

### 2.5 Error Response

Every error is produced by `apperror.ErrorHandler` (set as `e.HTTPErrorHandler`). Source: `internal/apperror/response.go`.

```json
{
  "error": "VALIDATION_ERROR",
  "message": "Key: 'RegisterRequest.Email' Error:Field validation for 'Email' failed on the 'email' tag",
  "details": { "field": "reason" }
}
```

| Field | Type | Notes |
|---|---|---|
| `error` | string | Machine-readable code from the table in section 3 |
| `message` | string | Human-readable; in production, messages of 5xx errors are replaced with `"internal server error"` |
| `details` | object (string to string) | Present only when `WithDetails(...)` was called; `omitempty`. **No handler currently sets it** |

### 2.6 No Content

Deletes return **204** with an empty body via `c.NoContent(http.StatusNoContent)`: `DELETE /admin/roles/:id`, `DELETE /admin/roles/:id/permissions/:pid`, `DELETE /admin/permissions/:id`.

### 2.7 Status Code Usage

| Status | Used for | Where |
|---|---|---|
| 200 | Reads, updates, actions, message-only | most handlers |
| 201 | Resource created | `Register`, `RoleHandler.Create`, `PermissionHandler.Create` |
| 204 | Deleted / revoked | role, permission, role-permission |
| 400 | Bad body, bad path id, invalid/expired token, missing `token` query | `BadRequest` |
| 401 | Not authenticated, bad credentials | `Unauthorized` |
| 403 | CSRF failure, locked account, insufficient role | `Forbidden` |
| 404 | Resource not found | `NotFound` |
| 409 | Duplicate email / role name / permission name | `Conflict` |
| 422 | Struct validation failure | `Validation` |
| 429 | Rate limit | `RateLimit` |
| 500 | Repository / unexpected error | `Internal` |

---

## 3. Error Handling Conventions

### 3.1 Error Types and Codes
Defined in [internal/apperror/apperror.go](../internal/apperror/apperror.go).

| Constructor | Code (`error` field) | HTTP |
|---|---|---|
| `apperror.Validation(msg)` | `VALIDATION_ERROR` | 422 |
| `apperror.BadRequest(msg)` | `BAD_REQUEST` | 400 |
| `apperror.NotFound(msg)` | `NOT_FOUND` | 404 |
| `apperror.Unauthorized(msg)` | `UNAUTHORIZED` | 401 |
| `apperror.Forbidden(msg)` | `FORBIDDEN` | 403 |
| `apperror.Conflict(msg)` | `CONFLICT` | 409 |
| `apperror.RateLimit(msg)` | `RATE_LIMIT_EXCEEDED` | 429 |
| `apperror.Internal(msg)` | `INTERNAL_ERROR` | 500 |

`AppError` fields: `Code`, `Message`, `Details`, `HTTPStatus`, `Err` (underlying error, never serialised). Builders: `WithDetails(map[string]string)`, `WithError(err)`.

### 3.2 How Handlers Return Errors
Handlers **return** the error; they never write the error response themselves.

```go
return apperror.BadRequest("invalid role id")
return apperror.Internal("failed to get role").WithError(err)
```

- Internal/unexpected errors always attach the cause with `.WithError(err)`; client-caused errors (400/404/409/422) do not.
- Non-`AppError` errors reaching the handler (for example Echo's own 404 for unknown routes) are converted: `echo.HTTPError` maps to the matching code via `fromHTTPError`, anything else becomes 500 `INTERNAL_ERROR`.

### 3.3 Message Wording
Messages are lowercase, no trailing period.

| Situation | Pattern | Examples |
|---|---|---|
| Malformed path parameter | `invalid <thing> id` | `invalid role id`, `invalid user id`, `invalid permission id` |
| Malformed body | `invalid request body` | all handlers |
| Not found | `<thing> not found` | `role not found`, `user not found` |
| Conflict | `<thing> <field> already exists` | `role name already exists`, `permission name already exists` |
| Repository failure | `failed to <verb> <thing>` | `failed to list roles`, `failed to update role` |
| Auth | plain statement | `authentication required`, `invalid email or password`, `account is locked, try again later` |

### 3.4 Service Layer Errors
Services expose sentinel errors (`errors.New`) that handlers map to `apperror` values:

| Sentinel | File | Handler mapping |
|---|---|---|
| `ErrEmailTaken` | `auth_service.go` | 409 `CONFLICT` |
| `ErrInvalidCredentials` | `auth_service.go` | 401 `UNAUTHORIZED` ("invalid email or password") |
| `ErrAccountLocked` | `auth_service.go` | 403 `FORBIDDEN` |
| `ErrInvalidToken` | `verification_service.go` | 400 `BAD_REQUEST` ("invalid or expired token") |
| `ErrInvalidResetToken` | `password_reset_service.go` | 400 `BAD_REQUEST` ("invalid or expired token") |

Internal failures are wrapped with `fmt.Errorf("<context>: %w", err)`.

### 3.5 "Not Found" at the Repository Boundary
Lookups return `(nil, nil)` when no row exists (`errors.Is(err, pgx.ErrNoRows)`); the handler checks `if x == nil` and returns `apperror.NotFound(...)`.

---

## 4. Request Handling Pattern

The sequence used by every handler that accepts a body:

```go
func (h *RoleHandler) Create(c echo.Context) error {
    var req model.CreateRoleRequest
    if err := c.Bind(&req); err != nil {
        return apperror.BadRequest("invalid request body").WithError(err)
    }
    if err := c.Validate(&req); err != nil {
        return apperror.Validation(err.Error())
    }
    // existence / conflict checks via repository
    // call repository or service
    return c.JSON(http.StatusCreated, role.ToResponse())
}
```

Order: parse path id (`strconv.ParseInt(c.Param("id"), 10, 64)`) then bind then validate then existence/conflict checks then write then respond with `ToResponse()` DTO. The request context is always `c.Request().Context()`.

---

## 5. DTO and JSON Conventions

| Convention | Rule observed | Source |
|---|---|---|
| Field names | `snake_case` JSON keys | all models |
| IDs | `int64`, serialised as JSON numbers | models |
| Timestamps | `time.Time` (RFC 3339 with offset), `created_at`, `updated_at`; nullable `*time.Time` with `omitempty` (`email_verified_at`, `locked_until`, `used_at`) | models |
| Hidden fields | `json:"-"` for `password_hash`, `failed_login_attempts` | `model/user.go` |
| Entity vs response | Entity structs carry `json` and `db` tags; API output goes through a `XxxResponse` struct built by `ToResponse()` | `model/user.go`, `model/role.go` |
| Request DTOs | `XxxRequest` structs with `json` and `validate` tags | `model/*.go` |
| Optional output | `omitempty` on optional/nullable response fields | models |
| Lists | Never `null`: handlers build `make([]T, 0, n)` | handlers |
| Bind source | JSON body only (`c.Bind`); query parameters read with `c.QueryParam` | handlers |

Request DTO naming: `CreateRoleRequest`, `UpdateRoleRequest`, `AssignPermissionRequest`, `ChangeRoleRequest`, `RegisterRequest`, `LoginRequest` (in `internal/model`); `ForgotPasswordRequest`, `ResetPasswordRequest`, `CreatePermissionRequest` live in the handler files (see section 18).

---

## 6. Validation Rules (As Implemented)

Library: go-playground/validator v10 via `internal/validator`. Failure always returns `apperror.Validation(err.Error())` (HTTP 422).

| Field (DTO) | Tags |
|---|---|
| `RegisterRequest.Email` | `required,email` |
| `RegisterRequest.Name` | `required,min=2,max=100` |
| `RegisterRequest.Password` | `required,strongpass` |
| `LoginRequest.Email` / `.Password` | `required,email` / `required` |
| `CreateRoleRequest.Name` / `.Description` | `required,min=2,max=50` / `max=255` |
| `UpdateRoleRequest.Name` / `.Description` | `omitempty,min=2,max=50` / `omitempty,max=255` |
| `AssignPermissionRequest.PermissionID` | `required` |
| `ChangeRoleRequest.RoleID` | `required` |
| `CreatePermissionRequest` | `Name required,min=3,max=100`, `Resource required,min=2,max=50`, `Action required,min=2,max=50` |
| `ForgotPasswordRequest.Email` | `required,email` |
| `ResetPasswordRequest` | `Token required`, `NewPassword required,min=8,max=72` |

Custom rule `strongpass`: `passwordvalidator.Validate(password, 60)` (minimum entropy 60 bits) in `internal/validator/validator.go`. It is used on registration only.

Other validation done in handlers (not by tags): numeric path IDs parsed with `strconv.ParseInt` (400 on failure); `GET /auth/verify-email` requires non-empty `token` query (400); `GET /users` accepts `limit` 1 to 100, otherwise silently falls back to 20.

---

## 7. Authentication, Session and CSRF Conventions

| Topic | Convention | Source |
|---|---|---|
| Auth mechanism | Server-side session cookie, no bearer tokens | ADR 0001 |
| Session key | `user_id` (int64) in the session; set with `sm.Put` after `sm.RenewToken` | `auth_handler.go` |
| Protecting a route | `RequireAuth(sm)` as group middleware | `router.go` |
| Role check | `RequireRole(userRepo, sm, "admin")` as group middleware | `router.go` |
| Session cookie | `HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` only when `APP_ENV=production`; name from `SESSION_COOKIE_NAME` | `session/session.go` |
| Swagger auth | `@Security CookieAuth` on protected handlers | handlers |
| CSRF cookie | `csrf_token`, `HttpOnly`, `SameSite=Strict`, `Secure` over HTTPS, max age 86400 | `middleware/csrf.go` |
| CSRF header | `X-CSRF-Token` required on every method except GET, HEAD, OPTIONS; value must equal the cookie | `middleware/csrf.go` |
| CSRF failure | `apperror.Forbidden("CSRF token invalid or missing")` then 403 | `middleware/csrf.go` |
| Reading the CSRF token in a browser | `GET /api/v1/auth/csrf` returns `{"csrf_token": ...}` | `handler/csrf_handler.go` |
| Password storage | bcrypt, `bcrypt.DefaultCost` | services, seed |
| Generic credential error | Same message for unknown email and wrong password | `auth_service.go`, `auth_handler.go` |
| Unknown-email enumeration | Forgot-password always answers 200 with the same message | `password_reset_handler.go` |

---

## 8. Rate Limit Convention

- Applied per route by passing the middleware as an extra argument: `auth.POST("/login", authHandler.Login, authLimit)`.
- Configuration at the router: `RateLimit(rate.Limit(5.0/60.0), 3)` (5 per minute, burst 3), keyed by `c.RealIP()`, one shared bucket for register and login.
- Exceeded: `apperror.RateLimit("rate limit exceeded, try again later")` then 429 with the standard error body:

```json
{ "error": "RATE_LIMIT_EXCEEDED", "message": "rate limit exceeded, try again later" }
```

- A `Retry-After` header carries the wait in delay-seconds (RFC 9110): the time until the client's bucket holds a token again, at least 1. It is omitted for a limiter that never refills. CORS exposes it (`Access-Control-Expose-Headers`) so the frontend can read it.
- No `X-RateLimit-*` headers are sent.

---

## 9. Pagination Format

Source: `pkg/pagination/pagination.go`.

| Item | Rule |
|---|---|
| Query parameters | `page` (default 1, minimum 1), `limit` (default 20, minimum 1, maximum 100) |
| Invalid values | Non-numeric values fall back to the defaults; out-of-range values are clamped; no error is returned |
| Parser | `pagination.FromContext(c)` returns `Params{Page, Limit}`; `Params.Offset()` = `(page-1)*limit` |
| Response | `pagination.NewResponse(data, params, total)` giving `{"data": [...], "meta": {"page","limit","total","total_pages"}}` |
| Repository signature | `ListWithPagination(ctx, [filters...], page, limit) ([]T, int64, error)`; runs a `COUNT(*)` query then a `LIMIT/OFFSET` query ordered by `id ASC` |
| Filters | Plain query parameters, passed to the repository (`resource` on permissions, `email` on users) |

Used by: `GET /admin/roles`, `GET /admin/permissions`. Not used by `GET /users` (see section 18).

---

## 10. Routing and URL Conventions

| Convention | Rule |
|---|---|
| Version prefix | `/api/v1` |
| Groups | `/auth` (public + authenticated), `/users` (authenticated), `/admin` (authenticated + role `admin`) |
| Resource names | Plural nouns, lowercase, hyphen-free: `roles`, `permissions`, `users` |
| Identifiers | `:id`; sub-resource `:pid` for permission under a role |
| Actions on a resource | Sub-path verbs: `/auth/login`, `/auth/forgot-password`, `/auth/verify-email`, `/admin/users/:id/role` |
| Methods | `GET` read, `POST` create/action, `PUT` update (full-style, used for partial updates too), `DELETE` remove |
| Registration location | Most routes in `internal/router/router.go`; `verify-email`, `forgot-password`, `reset-password`, `/metrics`, `/health`, `/ready`, `/live`, `/swagger/*` are registered directly in `cmd/api/main.go` |
| Un-versioned endpoints | `/health`, `/ready`, `/live`, `/metrics`, `/swagger/*` |
| Handler method names | Verb-first: `List`, `Get`, `Create`, `Update`, `Delete`, `AssignPermission`; for `users`: `ListUsers`, `GetUser` |

---

## 11. Logging Conventions

| Topic | Convention | Source |
|---|---|---|
| Logger | `*slog.Logger` injected into services and seed; `slog` default set in `main.go` | `main.go` |
| Format/level | `LOG_FORMAT` (`json` or `text`), `LOG_LEVEL` (default info) | `logger/logger.go` |
| Message style | Lowercase, short sentence or phrase, no trailing period | all log calls |
| Attribute keys | `snake_case`: `user_id`, `expires_at`, `request_id`, `latency_ms`, `remote_ip`, `user_agent`, `bytes_in`, `bytes_out`, `count`, `email` | code |
| Levels | `Info` for lifecycle and business events, `Warn` for degraded config (admin not seeded), `Error` for server errors | code |
| Request log | One line per request by `SlogLogger`; level from status: <400 info `request completed`, 4xx warn `client error`, 5xx error `server error` | `middleware/logger.go` |
| Request ID | `X-Request-ID` header, attached to the per-request logger as `request_id` | `middleware/logger.go` |
| Handlers | Do **not** log; they return errors, and the underlying cause travels in `AppError.Err` | handlers |
| Services | Log business events with the injected logger (`s.log.Info(...)`) | services |
| Sensitive data | The password-reset token and email are not logged; the **email-verification token is currently logged** (see section 18) | services |

---

## 12. Code Structure and Naming Conventions

### 12.1 Layering
Code is grouped by domain (`auth`, `user`, `accesscontrol`, `report`, `stats`, `audit`). Within one: handler in `<domain>/rest` (bind, validate, map errors, respond) then Service in `<domain>impl` (rules, orchestration) then Store in `<domain>impl` (SQL via Squirrel). Contracts, errors and models live in the parent `<domain>` package. Simple CRUD handlers call stores directly (`accesscontrol`; `CONTRIBUTING.md`, `docs/ARCHITECTURE.md`).

### 12.2 Files and Packages

| Kind | Pattern | Examples |
|---|---|---|
| Contracts | `<domain>/<domain>.go`: `Service`, `Store` interfaces and `Err...` sentinels | `report/report.go`, `accesscontrol/accesscontrol.go` |
| Models | `<domain>/model.go` | `user/model.go` |
| Handlers | `<domain>/rest/<resource>_handler.go` | `accesscontrol/rest/role_handler.go`, `user/rest/admin_user_handler.go` |
| Services | `<domain>impl/<domain>.go` (more files when a domain has several) | `reportimpl/report.go`, `authimpl/verification.go` |
| Stores | `<domain>impl/store.go` or `<name>_store.go`; mocks in `mock.go` | `userimpl/store.go`, `acimpl/role_store.go` |
| Middleware | one concern per file | `rbac.go`, `ratelimit.go` |
| Tests | `<file>_test.go`, integration `<file>_integration_test.go` with build tag | `authimpl/auth_test.go` |
| Packages | Domain name, `<domain>impl`, `rest`; import `rest` packages with an alias | `reportrest`, `acrest` |

### 12.3 Types and Functions

| Element | Convention | Example |
|---|---|---|
| Handler type | `<Resource>Handler` with unexported dependencies | `RoleHandler{roleRepo, permissionRepo}` |
| Constructor | Handlers: `New<Type>(deps...) *<Type>`; services and stores: `NewService` / `New<Name>Store` returning the interface | `NewRoleHandler`, `reportimpl.NewService` |
| Store interface | `Store` (or `<Name>Store`) in the parent package | `user.Store`, `accesscontrol.RoleStore` |
| Implementations | unexported `service` / `store` structs | `userimpl.store` |
| Mocks | `Mock<Name>Store` with overridable `...Func` fields | `userimpl.MockStore` |
| Context | First parameter `ctx context.Context` for all service and store methods; no Echo types below `rest` | all |
| Entity conversion | `func (x *Entity) ToResponse() EntityResponse` | `User.ToResponse` |
| Constants | Error codes as typed `Code` constants; lifetimes as exported constants (`TokenLifetime`, `PasswordResetLifetime`) | `apperror`, repositories |
| Import grouping | stdlib, third-party, then module imports, blank-line separated (goimports) | all files |
| Doc comments | Swagger `// Name godoc` blocks on handlers; short doc comments on exported types and functions | handlers, models |
| Section separators | `// ====...` banner comments in `router.go`, `seed.go`, `apperror.go`, `accesscontrol/model.go` | |

### 12.4 Repository Query Pattern
- SQL built with `r.db.Builder` (Squirrel, `$n` placeholders); column lists kept in constants (`roleColumns`, `passwordResetColumns`).
- Errors wrapped as `fmt.Errorf("build query: %w", err)`, `fmt.Errorf("query roles: %w", err)`, `fmt.Errorf("scan role: %w", err)`.
- Rows loops: `defer rows.Close()`, `rows.Next()/Scan`, return `rows.Err()`.
- Shared helper `deleteByID` (in `acimpl`) for deletes (error when no rows affected).
- `// #nosec G115 - limit validated` is used on `uint64` conversions of limit/offset.

### 12.1 Filter Labels
- Use "All months" for the month filter (descriptive, specific).
- Use "All" for generic filters (role, status, etc.).
- Use "Any" only when the filter is a true wildcard (rare).
- Why: "Any" is ambiguous (any = random?). "All months" is clear.

---

## 13. Database and Migration Conventions

| Topic | Convention | Source |
|---|---|---|
| Tool | goose, SQL files in `internal/database/migrations/`, embedded | `migrate.go` |
| File name | `YYYYMMDDHHMMSS_<snake_case_description>.sql` | migrations |
| Structure | `-- +goose Up` / `-- +goose Down`, each wrapped in `-- +goose StatementBegin/End` | migrations |
| Tables | Plural snake_case (`users`, `roles`, `role_permissions`, `verification_tokens`, `password_resets`) | migrations |
| Primary key | `id BIGSERIAL PRIMARY KEY` (sessions use `token TEXT`) | migrations |
| Timestamps | `created_at`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` | migrations |
| Foreign keys | `<entity>_id BIGINT REFERENCES <table>(id) ON DELETE CASCADE` for token tables | migrations |
| Index names | `idx_<table>_<column>` (`sessions_expiry_idx` is the one exception) | migrations |
| Uniqueness | Column-level `UNIQUE NOT NULL` (`users.email`, `roles.name`, token columns) | migrations |
| Down migrations | Present in every file; drop indexes/columns/tables in reverse order | migrations |
| Seeds | Idempotent Go code (`ON CONFLICT ... DO NOTHING`), run at every startup | `seed.go` |

---

## 14. Swagger / OpenAPI Conventions

Source: handler comment blocks and [docs/swagger.yaml](swagger.yaml) (Swagger 2.0, generated by `swag init`).

```go
// Create godoc
// @Summary      Create a new role
// @Tags         admin/roles
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        body body model.CreateRoleRequest true "Role payload"
// @Success      201 {object} model.RoleResponse
// @Router       /admin/roles [post]
```

| Element | Convention |
|---|---|
| Doc header | `// <MethodName> godoc` |
| Tags | `auth`, `users`, `system`, `admin/roles`, `admin/permissions`, `admin/users` |
| Security | `@Security CookieAuth` (apiKey in cookie `app_session`) on authenticated endpoints |
| Params | `@Param id path int true "Role ID"`; body `@Param body body <type> true "<description>"` |
| Router path | Relative to `@BasePath /api/v1`; `{id}` brace style; `[method]` lowercase |
| Global info | `@title`, `@version`, `@BasePath`, `@securityDefinitions` in `cmd/api/main.go` |
| Generation | `make swagger` (`swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`) |
| Error responses | `@Failure <code> {object} map[string]string` where present (not on every handler) |

---

## 15. Configuration Conventions

- Environment variable names `UPPER_SNAKE_CASE`, grouped by prefix: `APP_`, `DB_`, `SESSION_`, `LOG_`, `CORS_`, `ADMIN_`.
- Loaded once in `config.Load()`; helpers `getEnv`, `getEnvDuration` (accepts integer seconds or a Go duration string), `getEnvSlice` (comma separated, trimmed).
- Required values are checked in `Config.Validate()` and fail startup with `config validation: ...`.
- `.env.example` documents every variable and is grouped with banner comments.
- Stage values: `local`, `dev`, `prod` (`APP_ENV` also accepts `development` and `production`).
- Test environment variables are cleaned with a `cleanupEnv()` helper.

---

## 16. Testing Conventions

| Topic | Convention |
|---|---|
| Framework | Standard `testing`; testify used only in the integration test |
| Test names | `Test<Type>_<Method>_<Scenario>` (`TestAuthService_Login_WrongPassword`), `Test<Function>_<Case>` (`TestLoad_MissingDatabaseURL`) |
| Table tests | `tests := []struct{...}` with `t.Run(tt.name, ...)` in `apperror` and `config` |
| Service tests | In-memory `MockUserRepo` with overridable `...Func` fields, no HTTP layer |
| Handler tests | `httptest.NewRecorder`, `httptest.NewRequestWithContext(context.Background(), ...)` (required by the `noctx` linter) |
| Integration tests | Build tag `//go:build integration`, Postgres via testcontainers, migrations applied in the test helper |
| Environment | `os.Setenv` with `defer cleanupEnv()` |
| Commands | `make test`; integration `go test -tags=integration ./internal/repository/` |
| Coverage target | "Aim for 70%+ coverage on new code" (`CONTRIBUTING.md`) |

---

## 17. Git and Commit Conventions

From `CONTRIBUTING.md` and the repository history.

- **Format:** Conventional Commits: `type(scope): short description`, optional body, optional footer.
- **Types listed:** `feat, fix, docs, style, refactor, test, chore, ci, perf`.
- **Scopes seen in history:** `auth`, `security`, `build`, `swagger`.
- **Branch names:** `feature/<name>` (documented in `CONTRIBUTING.md`); the repository history uses `main` only.
- **Pre-commit checklist (documented):** `make fmt`, `make lint`, `make test`, integration tests when relevant, `make build`.
- **PR checklist (documented):** style, tests, documentation, no lint errors, CI passes, conventional commits.
- **Linting that CI enforces:** `.golangci.yml` rules including `noctx`, `gosec`, `gocyclo` 15, `revive`.

---

## 18. Inconsistencies Found (Listed, Not Fixed)

Observed in the code at commit `f40e94c`. No change is proposed here.

| # | Area | Inconsistency | Where |
|---|---|---|---|
| 1 | Response envelope | Single resources are bare objects, paginated lists are `{data, meta}`, `GET /users` is a bare array, and message-only responses are `{message}`. There is no single envelope | handlers, `pkg/pagination` |
| 2 | Users list | `GET /users` is documented as paginated (`page`, `limit` params, "paginated list") but the handler only reads `limit` and returns a bare array; `UserRepository.ListWithPagination` exists but is unused by the handler | `user_handler.go`, `interfaces.go` |
| 3 | Pagination clamping | Repositories re-implement default 20 / max 100 clamping that `pagination.FromContext` already applies | `role_repo.go`, `permission_repo.go` |
| 4 | Swagger success types | Roles and permissions lists are annotated `{array}` although they return a `{data, meta}` object | `role_handler.go`, `permission_handler.go` |
| 5 | Swagger failures | `@Failure` annotations exist on some handlers and not on others; `GET /auth/me` documents `map[string]int64`; no handler documents the 403/429 responses produced by middleware | handlers |
| 6 | Swagger metadata | `@title` is `Template Go Echo API`, description and contact are the template's placeholders | `cmd/api/main.go` |
| 7 | DTO location | Most DTOs are in `internal/model`; `CreatePermissionRequest`, `ForgotPasswordRequest`, `ResetPasswordRequest` are declared in handler files | handlers |
| 8 | Sentinel comparison | `errors.Is(err, service.ErrX)` in `auth_handler.go`, plain `err == service.ErrX` in the verification and password-reset handlers | handlers |
| 9 | Session key | Constant `middleware.UserIDKey` exists, but `"user_id"` string literals are used in `auth_handler.go` and `rbac.go` | `auth_handler.go`, `rbac.go` |
| 10 | Not found on delete | `deleteByID` returns a plain `fmt.Errorf("<table> not found")`; handlers map any repository error to 500 `failed to delete ...`, so deleting a missing role or permission yields 500, not 404 | `helpers.go`, role/permission handlers |
| 11 | Conflict checks | Create checks for duplicate role/permission names and returns 409; role update has no duplicate check | `role_handler.go` |
| 12 | Validation detail | `details` is supported by `AppError` but never set; validation errors are the raw validator string | `apperror`, handlers |
| 13 | Validation strictness | Registration uses `strongpass`; reset-password accepts any 8 to 72 character password | `model/user.go`, `password_reset_handler.go` |
| 14 | Optional update fields | `UpdateRoleRequest` uses plain strings with `omitempty`, so "omitted" and "empty" are indistinguishable | `model/role.go` |
| 15 | Layering | Auth, verification and reset use a service layer; role, permission and user handlers use repositories directly | handlers |
| 16 | Handler field names | Dependency field is `service`, `authService`, `repo`, `roleRepo`, `userRepo` depending on the handler | handlers |
| 17 | Config usage | `DB_*` values and `SESSION_IDLE_TIMEOUT` are loaded into `Config` but not read by the application; `session.New` reads environment variables directly | `config.go`, `session.go` |
| 18 | Request log status | Application errors are logged and counted as status 200 because only `*echo.HTTPError` is translated | `middleware/logger.go`, `metrics.go` |
| 19 | Sensitive logging | The verification token is logged at info level; the password-reset token is not | `verification_service.go` |
| 20 | Rate limiting | Applied to register and login only; other unauthenticated token endpoints are not limited | `router.go`, `main.go` |
| 21 | Route registration | Some `/api/v1/auth` routes are registered in `main.go`, others in `router.go` | `main.go`, `router.go` |
| 22 | Index naming | `sessions_expiry_idx` differs from the `idx_<table>_<column>` pattern | migrations |
| 23 | Comment language | Some comments and SQL notes are in Tagalog (for example `session.go`, the email-verification migration) while most are English | `session.go`, migrations |
| 24 | Status for delete/revoke vs assign | Revoke returns 204 with no body; assign returns 200 with a message body | `role_handler.go` |
| 25 | Naming leftovers | Compose names, README URLs and Swagger title still say "template" | compose, README, `main.go` |

---

## 19. Areas Without an Established Convention

The code does not (yet) define a pattern for these. They are listed so nobody assumes one exists.

| Area | Status |
|---|---|
| Transactions across repositories | No transaction helper or convention (all repository methods use the pool directly) |
| Audit logging | No audit log code exists |
| Soft deletes | None; deletes are hard deletes |
| Ownership checks (resource owner vs admin) | No ownership logic exists yet |
| Permission-based route protection | `RequirePermission` exists but is used nowhere |
| Request body size limits, request timeouts per route | None configured beyond server-level timeouts |
| Outbound HTTP / email sending | No client or abstraction |
| API versioning beyond `/v1` | None |
| Idempotency keys, ETags, conditional requests | None |
| Date/time input parsing in requests | No request field currently carries a date |
| Internationalisation of messages | None; all messages are English |
| Frontend contract (TypeScript types, error code handling) | None in this repository |

---

## 20. Appendix

### 20.1 Source Files Read
`internal/apperror/apperror.go`, `internal/apperror/response.go`, `internal/handler/*.go`, `internal/model/*.go`, `internal/validator/validator.go`, `internal/middleware/{logger,csrf,ratelimit,auth,rbac,metrics}.go`, `pkg/pagination/pagination.go`, `cmd/api/main.go`, `internal/router/router.go`, `internal/repository/{interfaces,helpers,role_repo,mock_repo}.go`, `internal/service/*.go`, `internal/database/{seed,migrate}.go` and migrations, `docs/swagger.yaml`, `CONTRIBUTING.md`, `.golangci.yml`.

### 20.2 Related Documents
- `docs/00-Repository-Analysis.md`
- `docs/01-PRD.md`
- `docs/02-App-Flow.md`
- `docs/03-Tech-Stack.md`
- `docs/adr/0001` to `0005`
- `06-Implementation-Plan.md`
