# Current Feature: Architecture alignment with company codebase + JWT auth

> **Status: SPEC FOR REVIEW. No code written.** Nothing here is approved until the owner signs off the open decisions in section 9.
> Source blueprint: the company identity-service pattern (httprouter + sqlx), translated to Echo in its section 5.
> Scope read for this spec: `backend/internal/{handler,service,repository,router,middleware,session,database,model}`, `backend/cmd/api/main.go`, `frontend/src/lib/{api,auth,session}.ts(x)`, `docs/DECISIONS.md` (session entries).
> Not re-read: `cmd/api/echo.go` (where `CSRFProtection` and CORS are mounted), the other handlers/services in detail, the e2e suite.

---

## 1. Where we really are (corrections to the blueprint's assumptions)

The blueprint assumes things that are not true here. Several of the "adopt" items are already done, and one (auth) reverses a deliberate decision.

| Blueprint assumes | AmgReports today | Consequence |
|---|---|---|
| sqlx + xorm, `db.DB` interface, read/write pools | **pgx v5 native** (`pgxpool`) + **Masterminds/squirrel** with `$n` placeholders (`database.DB{Pool, Builder}`). One pool, one database. | Do not move to sqlx. It is a rewrite of every repo for no gain (see section 6). |
| Tx carried in ctx (`InTransaction`) | Tx passed **as a parameter**: `repository.Querier` (satisfied by pool and `pgx.Tx`), opened by `TxRunner.WithTx(ctx, fn(Querier))`. | This is the one pattern worth adopting. It changes signatures across all repos, mocks and services. |
| Custom Go migrator | **goose** SQL files, run by `/app/api migrate` in prod. | Already matches the blueprint's own advice. No change. |
| Echo middleware order via `e.Use` | Echo. `/api/v1` group has the global rate limiter; per-group `RequireAuth`, `RequireRole`, `RequirePermission`. | Already close. See section 5.4. |
| Global auth, anonymous passes through | **Mandatory auth per group**, public routes sit in `/auth`. | Keep. This is the safer default the blueprint itself recommends. |
| JWT | **scs sessions in Postgres** (`sessions` table, `revoked_at`, `user_id`), HttpOnly cookie `app_session`, double-submit **CSRF** cookie + `X-CSRF-Token` header. | JWT replaces a working, revocable mechanism. See section 4 for what that costs and how to keep it. |
| Flat per-domain packages `pkg/<domain>/...` | **Layer-first** packages: `internal/{handler,service,repository,model,middleware,router}`. | Section 2. Biggest churn, least functional value. |

Session behaviours that exist today and must survive the JWT move (all decided, tested):
- D-A10: a non-active user cannot sign in and loses sessions on status change. `RequireRole`, `RequirePermission` and `/auth/me` already re-read the user (`GetWithRole`) on **every request** and answer 401 for non-active.
- Change password: other sessions end, the current one stays, token renewed (`UpdatePasswordKeepSession`).
- Admin reset password (D-A12): all of the user's sessions end, lockout cleared.
- Failed-login lockout: 5 tries, 15 minutes.
- D-F1: session cookie `Domain=.amgreports.com` so Server Components and `proxy.ts` can read it (planned; client-side fetching until it ships).

---

## 2. Folder structure mapping

### 2.1 Target layout (Echo translation of blueprint section 5)

```
backend/
  cmd/api/
    main.go            keep dispatch(); move run() wiring out
    server.go          newHTTPServer (exists)
    wire.go            NEW: builds stores -> services -> handlers, returns rest.Dependencies
  internal/
    <domain>/
      <domain>.go      Service + Store interfaces (the contract)
      model.go         entities, Commands, Queries, Results, error vars, Validate()
      <domain>impl/    service.go (rules) + store.go (SQL) + helpers
      rest/            Echo handlers, one <resource>_handler.go each + routes
    platform/ or keep flat:
      apperror/  config/  database/  logger/  validator/        (unchanged)
      middleware/        authn, rbac, ratelimit, csrf, logger, metrics, bodylimit, clientip
      request/           NEW: User, UserFrom(ctx), WithUser(ctx) (the "request.UserFrom" idea)
    router/              Setup(e, Dependencies): route table only
  pkg/{pagination,metrics}   unchanged
```

Package name `rest`, not `http`: avoids shadowing `net/http` in every handler file. This is a deliberate deviation from blueprint section 5.

### 2.2 File-by-file map

| Blueprint | Ours today | Ours after |
|---|---|---|
| `cmd/<svc>/main.go` (6 lines) | `cmd/api/main.go` (292 lines: swagger header, dispatch, system routes, `run()`) | Keep `dispatch` and system routes (they are tested). Move the construction part of `run()` to `wire.go`. |
| `cmd/<svc>/app/server.go` manual DI, `rest.Dependencies` | `run()` builds repos, services, handlers inline; `router.Setup` takes **14 positional args** | `wire.go` returns one `Dependencies` struct; `router.Setup(e, deps)`. |
| `pkg/<domain>/<thing>/<thing>.go` interfaces | `repository/interfaces.go` (all repo interfaces in one file); services are concrete structs; handler declares its own `ReportServiceInterface` | Interfaces move to the domain package. Services get an interface too (today only reports has one, defined in the handler). |
| `<thing>/model.go` | `model/{user,role,report,stats,...}.go` | `internal/<domain>/model.go`. |
| `<x>impl/service.go` | `service/*_service.go` | `<domain>impl/service.go`. |
| `<x>impl/store.go` | `repository/*_repo.go` | `<domain>impl/store.go`. |
| `protocol/rest/*_handler.go` | `handler/*_handler.go` | `<domain>/rest/*_handler.go`. |
| `config/config_*.go` one per concern | `config/config.go` (60 symbols), `stages.go` | Split into `config_app.go`, `config_db.go`, `config_auth.go` (new JWT block), `config_cors.go`, `config_reports.go`, `config_observability.go`. Pure move. |
| `storage/postgres/migrations/` Go | `backend/migrations/*.sql` (goose) | **Keep.** |
| `api/{errors,response,request}` | `apperror/`, plus ad-hoc `c.JSON` | `apperror` stays. `request/` is new. No `response` package: `c.JSON` is fine in Echo. |
| `middleware/*` | `middleware/*` | Same place. Auth files change (section 4). |

### 2.3 Domain split

| Domain package | Takes from |
|---|---|
| `auth` | `AuthService`, `VerificationService`, `PasswordResetService`, token issuing (new), login/me/change-password models, verification and reset repos |
| `user` | `UserService`, `UserRepo`, `model/user*.go`, admin user handler |
| `access` | `RoleRepo`, `PermissionRepo`, role and permission handlers and models |
| `report` | `ReportService`, `ReportRepo`, `model/report*.go` |
| `stats` | `StatsService`, `StatsRepo` |
| `audit` | `AuditRepo`, `model.AuditLog`, action constants (`AuditChangePassword`, ...) |

### 2.4 Cross-domain rule (the real design question)

`auth` needs user rows (hash, lockout, status); `report` needs the user's role and permissions; every service writes audit rows. The blueprint's answer: parent package interface, concrete store handed over from wiring.

Recommendation: each domain's parent package declares a **small exported Store interface only for the methods other domains call** (e.g. `user.Store`: `GetWithRole`, `GetByEmail`, `RecordFailedLogin`, `ResetLoginAttempts`). Everything else on a store is unexported lowerCamel, as in the blueprint. Cycles are avoided by the rule: **domain packages never import another domain's `impl` or `rest`**, and `audit` imports no other domain.

Cost warning: the current mocks (`mock_repo.go` 39 symbols, `mock_report_repo.go`, `mock_audit_repo.go`) live in `repository` and are used from `service` tests. Unexported store methods cannot be mocked from another package, so service tests become in-package tests with a fake store, or tests use the exported cross-domain interface. About 18 service test files and 27 handler test files are touched.

---

## 3. Rewriting handlers and services without touching business logic

Rule for every step: **wire format and behaviour stay identical.** Existing tests are the safety net; a step is done only when `make test`, `make test-integration`, `make lint`, `npm run gen:api` (no diff) and `npm run e2e` pass.

### 3.1 Handlers (blueprint section 2, "Handler")

Today: `Bind` -> `Validate` -> read `user_id` from `h.sm` -> `c.RealIP()` -> service -> map errors. SQL-free already.

Changes:
1. **Remove `*scs.SessionManager` from handlers.** `AuthHandler`, `AdminUserHandler`, `ReportHandler` hold `sm` only to read `"user_id"`. Replace with `request.UserFrom(ctx)` filled by the auth middleware. This is the single most useful step: it makes the JWT swap invisible to handlers.
2. **Command objects carry metadata.** Handler builds `Create<X>Command` / `Search<X>Query` from the body or query string, then adds actor id, IP and user agent (blueprint: "enrich with request metadata"). Services stop taking loose `(userID, ip, req)` argument lists.
3. `cmd.Validate()` stays in the handler. We already validate via `c.Validate` (validator package); keep that, do not hand-write `Validate()` methods. Deviation from blueprint, justified by existing `validator` package and tests.
4. Error mapping stays in the handler (`reportError`, etc.). No change in status codes.
5. Each handler declares routes via `New<Resource>Handler(...)` plus `Register(g *echo.Group)`? **Not recommended**; keep one central `router.Setup` route table. A single readable table of middleware per route is worth more than the blueprint's per-handler registration. Listed in open decisions.

Naming (blueprint): `Create<X>Command`, `Search<X>Query`, `Search<X>QueryResult`, `Action<Thing><Verb>`.
- Audit constants already follow `Audit<Verb>`; rename to `Action<Thing><Verb>` only if the owner wants it. It touches DB-facing constants but **not** the stored string values, so no migration. Low value; default is **do not rename**.
- Request/response struct renames (`CreateReportRequest` -> `CreateReportCommand`) change swagger schema names and therefore `frontend/src/lib/api-types.ts`. CI fails on drift. Either keep the swagger name via annotation, or accept the regenerated types and fix the frontend imports in the same PR. Default: **rename internally, keep names on the wire** where annotation allows.

### 3.2 Services (blueprint section 2, "Service")

Today: services take repos plus `repository.TxRunner`, open `s.tx.WithTx(ctx, func(q Querier){...})`, pass `q` into each repo call, and write the audit row in the same tx. Logic is good; only plumbing changes.

Changes:
1. Replace `WithTx(ctx, fn(q Querier))` with `InTransaction(ctx, fn(ctx) error)`; repos stop receiving `q` (section 6.1). Audit-in-same-tx guarantee is unchanged: both calls use the transaction ctx.
2. Services get a parent-package interface (`report.Service`, `auth.Service`, ...); `New...Service` returns the interface.
3. Audit actor comes from `request.UserFrom(ctx)` instead of an argument (blueprint: "pulls the caller from request.UserFrom(ctx) for audit fields"). IP and user agent come in on the command.
4. **Do not change**: lockout rules, status-check order (D-A10), `ErrReportNotFound` covering not-yours (D-1), last-admin locking (`LockActiveAdmins`), the 72-byte bcrypt rule, period rule for reports.

### 3.3 Stores

Rename `repository` -> `<domain>impl/store.go`, methods unexported (blueprint), compile-time `var _` interface checks stay. `ErrNotFound` / `ErrUniqueViolation` sentinel errors in `repository/errors.go` move to a shared small package (`internal/dberr`) so every store maps pgx errors the same way.

---

## 4. JWT migration (exact steps)

### 4.1 Read this first: what JWT costs here

Sessions give us **instant revocation** (`sessions.revoked_at`), already wired into 4 flows (status change, change password, admin reset, delete). A stateless access JWT cannot be revoked before it expires. Blueprint section 5 says "Global e.Use(authMiddleware) that sets the session on the context", which is compatible with either mechanism. The question is whether the owner wants JWT specifically (matching the company stack) or just the middleware shape. The spec below assumes JWT is wanted and keeps revocation via three pieces:

1. Short access token (default **10 min**).
2. **Refresh token stored in Postgres** (hashed), rotating, revocable. This replaces the `sessions` table's role.
3. Existing per-request `GetWithRole` check (already in `RequireRole`/`RequirePermission`/`Me`) keeps status and role live. Optional: a `users.tokens_valid_after` timestamp compared with the token's `iat`, so password change / admin reset invalidate access tokens immediately at zero extra query cost (user row is loaded anyway). Recommended; see D3.

### 4.2 Token design

| Item | Choice | Why |
|---|---|---|
| Library | `github.com/golang-jwt/jwt/v5` | Maintained, small. New dependency: confirm with owner (D4). |
| Access algorithm | HS256, secret `JWT_SECRET` (>= 32 bytes) | One service signs and verifies. Upgrade to EdDSA only if a second service must verify. Parser **pins** the algorithm (reject `none` and alg switching). |
| Access claims | `sub` = user id, `sid` = refresh family id, `iss`, `aud`, `iat`, `exp`, `jti` | **No role or permissions in the token.** RBAC stays live from the DB, as today (a role change shows on the next call). |
| Access TTL | 10 min (`JWT_ACCESS_TTL`) | Bounds the stolen-token window. |
| Refresh token | 32 random bytes (crypto/rand), only the **SHA-256 hash** stored | A DB leak does not yield usable tokens. |
| Refresh TTL | 24 h absolute (matches today's `SESSION_LIFETIME` 86400) with 30 min inactivity matching today's idle timeout, configurable | Preserves current session feel. |
| Rotation | Every refresh issues a new refresh token and marks the old one `replaced_by`. Reuse of a rotated token **revokes the whole family** | Standard theft detection. |

### 4.3 Delivery: cookies vs Authorization header (decision D1)

- **Option A (recommended): both tokens in HttpOnly cookies.** Access cookie `access_token`, Path `/`; refresh cookie `refresh_token`, Path `/api/v1/auth`. `Secure` in production, `SameSite=Lax` as today, `Domain=.amgreports.com` once D-F1 ships. Keeps D-F1 (Server Components / `proxy.ts` can read the cookie), keeps `credentials: "include"` and the **CSRF double-submit** unchanged, JS never sees a token (XSS cannot exfiltrate it).
- **Option B: access token in memory, `Authorization: Bearer`.** Removes CSRF for API calls, matches the company's bearer style. But: an access token in JS memory is lost on reload (needs a refresh call before every first paint), SSR cannot read it, which breaks the D-F1 plan, and it widens XSS impact.

The rest of this section is written for A. B changes only 4.5 steps 8-10 and deletes CSRF.

### 4.4 Database

New goose migration (naming `YYYYMMDDHHMMSS_description.sql`):
- Table `refresh_tokens`: `id`, `user_id` (FK users, ON DELETE CASCADE), `family_id`, `token_hash` (unique), `expires_at`, `last_used_at`, `revoked_at`, `replaced_by`, `ip`, `user_agent`, `created_at`. Indexes on `(user_id) WHERE revoked_at IS NULL` and `family_id`.
- Optional: `users.tokens_valid_after timestamptz NOT NULL DEFAULT 'epoch'` (D3).
- **No new audit action** is planned (login/refresh are not audited today). If the owner wants `REFRESH_REUSE_DETECTED` audited, that is an `audit_logs` CHECK-constraint migration shipped in the same deploy as the code (guardrail).
- `sessions` table is **not dropped** in this change. A follow-up migration drops it after the cutover is verified in production (keeps rollback possible).

### 4.5 Steps, in order

Each step leaves the app working and tests green.

**Phase 0: decouple (no behaviour change, still scs).**
1. Add `internal/request`: `User{ID}`, `WithUser(ctx, u)`, `UserFrom(ctx)`.
2. New middleware `Authenticate` that, **using scs for now**, reads `user_id` and calls `WithUser` on the request context. `RequireAuth` becomes "user present or 401".
3. Replace every `sm.GetInt64(ctx, "user_id")` in handlers, `RequireRole`, `RequirePermission` and `RateLimitByUser` with `request.UserFrom`. Drop `sm` from `AuthHandler`, `AdminUserHandler`, `ReportHandler`, `RateLimitByUser`. Login/Logout/ChangePassword keep `sm` until phase 2.
   Verify: full suite green, `git diff` shows no status-code change.

**Phase 1: token plumbing (additive, unused).**
4. Config: new `config_auth.go` with `JWT_SECRET`, `JWT_ISSUER`, `JWT_AUDIENCE`, `JWT_ACCESS_TTL`, `REFRESH_TTL`, `REFRESH_IDLE_TTL`, cookie name/domain. `Validate()`: secret required and >= 32 bytes **in every stage except test**; half-set values rejected (same fail-closed style as the metrics credentials).
5. `internal/auth/token`: Issuer (sign), Parser (pin HS256, check `iss`, `aud`, `exp` with small leeway, require `sub`/`sid`).
6. Migration from 4.4. `RefreshStore` with: create, find-by-hash, rotate (one transaction, row lock on the old token), revoke family, revoke all for user, revoke all except family (change password).
   Tests: unit for token (expired, wrong alg, wrong aud, tampered), integration for rotation, reuse detection, concurrent refresh with the same token (exactly one wins).

**Phase 2: switch (the cutover).**
7. `AuthService.Login`: same credential and lockout logic, then **creates a refresh family and returns a token pair** (user, access, refresh). `Logout` revokes the family. New `Refresh(ctx, refreshToken, ip, ua)`. `ChangePassword` revokes all families **except the current** (replaces `UpdatePasswordKeepSession`'s `UPDATE sessions`); admin reset, status change and delete revoke all of the user's families (replaces `RevokeSessions`). Same transaction as the audit row, as today.
8. Handlers set/clear the two cookies (a small `cookies.go` in `auth/rest`). Remove `RenewToken`/`Put`/`Destroy`. New route `POST /auth/refresh`, in the public `/auth` group, **CSRF-protected**, own rate limit bucket, reads only the refresh cookie.
9. `Authenticate` now reads the access cookie (and, as a convenience for tooling, `Authorization: Bearer` when no cookie is present), parses it, optionally checks `iat >= tokens_valid_after`, then `WithUser`. Invalid or expired token -> 401 `apperror.Unauthorized`. **Missing token on a protected group -> 401** (no anonymous pass-through, blueprint's advice).
10. Remove `sm.LoadAndSave` from `run()`, `session.New`, the scs store, and the `CookieAuth` swagger definition (`app_session`) in favour of the new cookie name. Run `make swagger` then `npm run gen:api`.
11. Rewrite or delete tests that assume sessions: `session/store_integration_test.go`, `repository/session_revocation_integration_test.go`, `router/session_revocation_integration_test.go`, `handler/user_status_session_integration_test.go`. The revocation scenarios must be **kept and rewritten against refresh families**, not deleted: status change, change password, admin reset, delete.

**Phase 3: frontend (ships with phase 2, same deploy).**
12. `frontend/src/lib/api.ts`: on a 401 from any route except `/auth/login` and `/auth/refresh`, call `/auth/refresh` **once, single-flight** (concurrent 401s share one promise), then retry the original request once. If refresh fails, fire `onUnauthorized` (existing hook). Preserve the existing stale-CSRF retry.
13. `auth.ts`: `getCurrentUser` already treats 401 as signed out; it now benefits from the transparent refresh in step 12 (a reload after the access token expired still signs the user in). `resetCsrf()` calls stay (harmless; the CSRF cookie is independent of auth).
14. `session.tsx`: no structural change. No proactive timers (YAGNI; 401-driven refresh is enough).
15. Update `api.test.ts`, `auth.test.ts`, `session.test.tsx`; add tests: refresh on 401 then retry succeeds, refresh failure signs out, concurrent 401s cause one refresh, login 401 does not trigger refresh. Playwright: sign in, wait past access TTL (short TTL via env in e2e), still signed in; logout; password change signs out other contexts.

**Phase 4: cleanup (separate later PR).** Drop `sessions` table, `alexedwards/scs` and `pgxstore` from `go.mod`, scs references in docs.

### 4.6 Rollout and rollback

- Railway "Wait for CI" is off and the pre-deploy command runs `/app/api migrate`, so a push to `main` deploys at once. **Set `JWT_SECRET` (and the other new vars) in Railway before pushing.** Config validation will make the new build refuse to start without it, which is the intended fail-closed behaviour, but it would crash the deploy.
- **Every signed-in user is signed out at cutover** (old cookie is meaningless). 24 churches: schedule outside reporting deadlines, tell the admins.
- Rollback: redeploy previous image. Because migrations are additive and `sessions` is kept until phase 4, the old build works unchanged. Users sign in again.
- Never push without the owner asking (guardrail); each phase is its own reviewable commit set.

---

## 5. Middleware

### 5.1 Chain today vs blueprint

| Blueprint (identity) | Ours | Action |
|---|---|---|
| Tracing -> Logger -> Recovery -> Maintenance -> Headers -> ContextHandler | `e.Use` set in `cmd/api/echo.go` (logger, recover, metrics, body limit, CSRF, CORS, trusted proxy); `/api/v1` group adds global rate limit | Verify order against `echo.go` before changing. No tracing or maintenance flag exists; **do not add** (not asked, no infra). |
| CORS wraps router | Echo CORS with `AllowCredentials: true` and configured origins | Keep. |
| Per-route authorization lists | `perm("reports:read")`, `RequireRole(...)` per group/route | Keep. Already the blueprint pattern. |
| Package-level `reqOnlyUser` vars | built inline in `router.Setup` | Optional cosmetic: name `requireUser`, `requireAdmin`, `perm(...)` once at the top of `Setup`. |

### 5.2 Account-type separation

Blueprint separates `/api/bo`, `/api/aff`, `/api/mem` with `ForUser`. We have one user type with roles (`admin`, `user`), so **keep `/api/v1`** with `RequireRole`. Do not rename paths: it breaks the frontend, swagger, and `docs/API.md` for no benefit.

### 5.3 Middleware to change for JWT

`auth.go` (`RequireAuth`), `rbac.go` (`RequireRole`, `RequirePermission`), `ratelimit.go` (`RateLimitByUser`): all read the scs session today; all switch to `request.UserFrom` in phase 0.

### 5.4 Unverified

Whether mounting `Authenticate` globally with an allowlist of public paths is better than per-group `RequireAuth`. Recommendation: **stay per-group** (explicit, already tested by `router_test.go` and the *_routes_test files). Revisit only if a route is ever added without a guard.

---

## 6. Postgres query patterns

### 6.1 Adopt: transaction carried in ctx (blueprint section 3)

Today a repo method that must join a transaction takes `q Querier`; one that does not takes none; services pick which (e.g. `UserRepo.Create(ctx, q, ...)` vs `UserRepo.GetByID(ctx, id)`). Mixing is easy to get wrong and mocks must match.

Change:
- `database.DB.InTransaction(ctx, fn)` begins a `pgx.Tx`, stores it in ctx under a private key, commits when the outermost call returns nil, rolls back on error **or panic** (as `WithTx` does now).
- **Nested calls join** the outer transaction and never commit (blueprint). Rollback of the outer one undoes the inner work. Note: an error returned by a nested call that the outer swallows still leaves the tx in a usable state in our case because no inner savepoint exists; document that inner failures must propagate. Savepoints are not added.
- A single helper `db.Querier(ctx)` returns the ctx tx if present, else the pool. Every store query goes through it. Stores lose their `q Querier` parameter, `TxRunner` is replaced by the `InTransaction` method on a small interface (so services stay testable with a fake).
- The current `Querier` interface and `pgxpool`/`pgx.Tx` compile-time assertions move into `database`.

Blast radius: `UserRepository` (6 methods take `q`), `ReportRepository` (5), `AuditRepository` (1), all three mocks, `tx_test.go`, `tx_integration_test.go`, every service that calls `WithTx`. Mechanical, compiler-guided, but wide. This is phase P1 in section 8.

### 6.2 Keep (with reasons)

| Blueprint | Verdict |
|---|---|
| sqlx `Get`/`Select`/`NamedExec` | **Skip.** We use pgx native with explicit `Scan`. Moving to sqlx adds a dependency and rewrites ~15 repo files; pgx scanning is already typed. If struct scanning is wanted, `pgx.CollectRows` + `RowToStructByName` is already in pgx v5, no new dependency. Offer, do not do. |
| `?` placeholders + `Rebind` | **Skip.** Squirrel with `PlaceholderFormat(Dollar)` already does this. |
| Where `[]string` + args `[]any`, joined by AND | **Already equivalent**: squirrel `Where`/`sq.Eq`/`sq.ILike` build the same thing safely and everything is parameterised. Keep. Add the existing `escapeLike` to `UserRepo.ListWithPagination`, which currently builds `ILIKE` from `"%" + emailFilter + "%"` without escaping `%` and `_` (a correctness bug, found while reading; not security, since it is parameterised). |
| Count via `SELECT COUNT(id) FROM (<full query>)` | **Do not adopt.** The blueprint itself says it is slow. We run a separate `COUNT(*)` with the same filters, which is better. Keep. |
| Read/write pool split | **Skip.** One database, no replica. Revisit when a read replica exists. Single `Pool` stays. |
| Pool settings from env | **Adopt (small).** `pgxpool.ParseConfig` then set `MaxConns`, `MinConns`, `MaxConnLifetime`, `MaxConnIdleTime` from `DB_MAX_CONNS`, `DB_MIN_CONNS`, `DB_CONN_MAX_LIFETIME`, `DB_CONN_MAX_IDLE_TIME` in `NewWithURL`. Today `pgxpool.New(ctx, dsn)` uses defaults. Defaults preserved when unset. |
| `QueryContext` without closing rows | We always `defer rows.Close()` and check `rows.Err()` (seen in `loadMetrics`). Keep as the standard; add a lint check if one exists. |
| Raw SQL strings, quoted identifiers | Already used where needed (`sessions`, audit); squirrel for dynamic. Keep both. |
| Bulk insert of slices | Already done (`insertMetrics` multi-row insert). |
| Migrations as Go | **Skip**, goose stays. |
| `RETURNING id` helper | Squirrel `Suffix("RETURNING ...")` already used. |

### 6.3 Store organisation rules (carry over)

- Stores contain SQL only; no business rules (e.g. `RecordFailedLogin` threshold logic lives in SQL today: acceptable, document it).
- One `columns` const per entity and a `scan<Entity>` helper (already done for reports, replicate for users and roles where missing).
- Audit rows are inserted by the service inside the same `InTransaction` (guardrail, unchanged).

---

## 7. Testing and verification gates

Per phase, before moving on:
- `cd backend && make test && make test-integration && make lint && make vet`
- `make swagger && cd ../frontend && npm run gen:api` -> `git diff --exit-code frontend/src/lib/api-types.ts` when the API surface is meant to be unchanged
- `npm run lint && npm run typecheck && npm test`
- `npm run e2e` after phase 2/3 (stop the dev server first, as a separate step, per the project rules)
- New code needs tests: token parser, refresh rotation and reuse, concurrent refresh, ctx transaction (commit, rollback, nested join, panic), and the rewritten revocation scenarios.

Do **not** claim a phase done on unit tests alone: ctx-transaction and refresh rotation are only meaningful against real Postgres (testcontainers).

---

## 8. Recommended order and honest effort

| # | Phase | Value | Risk | Size |
|---|---|---|---|---|
| P0 | `request.UserFrom`, drop `sm` from handlers/middleware | High (unlocks JWT, simplifies tests) | Low | Medium |
| P1 | ctx-carried transactions, pool settings, `escapeLike` fix | High | Medium (wide signature change) | Large |
| P2 | JWT + refresh, frontend refresh | The stated goal | **High** (auth, logs everyone out) | Large |
| P3 | Command/Query naming, interfaces on services, `wire.go` + `Dependencies` | Medium | Low | Medium |
| P4 | Folder restructure to `<domain>/{model,<x>impl,rest}` | **Low functional value** | Medium (import cycles, ~150 files, every test moves) | Very large |
| P5 | Config split per concern, drop `sessions` table + scs | Low | Low | Small |

Recommendation: do P0 -> P1 -> P2 first; they carry nearly all the value. Decide on P4 after P3, with real evidence of whether the layer-first layout hurts. The blueprint author themself says they read only one service ("standard means the identity service's pattern") and that the middleware ordering was read, not run; treat their layout as a reference, not a requirement. P4 is pure moves and can be done one domain at a time (start with `stats`, the smallest) so it never blocks a release.

Each phase: own branch, own commits, shown as a full diff before committing, no push until asked.

---

## 9. Open decisions (need the owner before any code)

| ID | Question | Recommendation |
|---|---|---|
| D1 | Cookies (A) or bearer header (B)? | **A.** Keeps CSRF, D-F1 and SSR. |
| D2 | Is real JWT required, or only the middleware shape with sessions kept? | If the goal is "match company style", JWT. If the goal is simply less code, keep scs; it already has revocation. Your call; the spec assumes JWT. |
| D3 | Add `users.tokens_valid_after` so password change/reset kills access tokens instantly? | Yes (cheap). Otherwise accept a 10-minute window. |
| D4 | New dependency `golang-jwt/jwt/v5`? | Yes. |
| D5 | Access TTL, refresh absolute TTL, idle TTL | 10 min / 24 h / 30 min (equal to today's session behaviour). |
| D6 | Rename request/response structs and accept regenerated `api-types.ts`, or keep wire names? | Keep wire names; rename internally. |
| D7 | Central route table (current) or per-handler `Register`? | Central table. |
| D8 | Do the folder restructure (P4) at all? | Defer; decide after P3. |
| D9 | Package name `rest` vs blueprint's `http`/`protocol/rest`? | `rest`. |
| D10 | Audit refresh-token reuse (needs CHECK-constraint migration, same deploy)? | No, log it with `slog` only. |
| D11 | Cutover window (everyone is signed out) | Pick a quiet day; announce to admins. |

When approved, record the decisions in `docs/DECISIONS.md` (source of truth) as new D-entries, and update `docs/ARCHITECTURE.md` (auth, migration history) and `docs/API.md` (`/auth/refresh`, cookie names) in the same PRs as the code.

---

## 10. Risks

1. **Auth regression** is the dominant risk. Mitigation: P0 first (no behaviour change), keep the revocation tests, real-Postgres tests for rotation.
2. **Revocation semantics weaken** with any stateless access token. Mitigation: 10 min TTL, live `GetWithRole`, optional `tokens_valid_after`.
3. **Refresh race**: two tabs refresh at once with the same cookie. Mitigation: row lock in the rotate transaction, short grace window or single-flight on the client; test it.
4. **CSRF on `/auth/refresh`**: it is cookie-authenticated, so it must stay behind CSRF protection.
5. **Deploy ordering**: env vars before push; migration is additive.
6. **Wide mechanical diffs** (P1, P4) hide real bugs in review. Keep them behaviour-neutral and separate from logic changes.
7. **Blueprint provenance**: written from reading one service; the "unauthenticated passes through" and un-closed rows behaviours are explicitly not copied.
