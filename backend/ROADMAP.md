# ROADMAP

**Production-ready Go backend starter kit** — reusable sa future projects.

---

## STATUS: COMPLETE (100%)

| Phase | Feature | Status |
|---|---|---|
| 1-7 | Foundation | DONE |
| 8 | RBAC | DONE |
| 9 | Pagination | DONE |
| 10 | Observability | DONE |
| 11 | Docs Polish | DONE |
| 12.1 | Email Verification | DONE |
| 12.2 | Password Reset | DONE |
| 12.3 | Account Lockout | DONE |
| 13 | Integration Tests | DONE |
| 14 | Final Polish | DONE |

**Total commits:** 40+
**Live:** https://template-go-echo-squirrel-production.up.railway.app

---

## Phase 1-7: Foundation

- Echo v4 + Squirrel + pgx + Postgres
- Session-based auth (scs + pgxstore)
- CSRF protection
- Rate limiting
- Password strength validation
- Structured logging (slog)
- Custom error handling (apperror)
- Type-safe config
- Unit tests (20 tests)
- CI/CD (4 jobs)
- Docker (multi-stage)
- Documentation (README, SETUP, CUSTOMIZATION)

## Phase 8: RBAC

- Roles table (admin, editor, user)
- Permissions table (9 permissions)
- Role-permissions junction
- User role assignment
- RequireRole middleware
- RequirePermission middleware
- Admin endpoints (12 routes)
- Seed data (3 roles, 9 perms, 1 admin)

## Phase 9: Pagination

- pkg/pagination (generic Response[T])
- ListWithPagination on all repos
- Standard format: data + meta
- Query params: page, limit
- Max limit: 100

## Phase 10: Observability

- Prometheus metrics
- /metrics endpoint
- /health, /ready, /live
- HTTP middleware (auto-capture)
- DB connection gauges

## Phase 11: Docs Polish

- README (286 lines)
- docs/API.md (341 lines)
- docs/ARCHITECTURE.md (208 lines)
- CUSTOMIZATION.md

## Phase 12: Complete Auth

### 12.1 Email Verification
- email_verified column
- verification_tokens table
- VerificationService
- GET /auth/verify-email

### 12.2 Password Reset
- password_resets table
- PasswordResetService
- POST /auth/forgot-password
- POST /auth/reset-password

### 12.3 Account Lockout
- failed_login_attempts column
- locked_until column
- 5 attempts → 15 min lock
- Auto-reset on success

## Phase 13: Integration Tests

- Testcontainers (real Postgres)
- internal/testutil/db.go
- user_repo_integration_test.go (10 tests)
- Build tag: integration

## Phase 14: Final Polish

- Updated README with badges
- Updated API.md with new endpoints
- Added CONTRIBUTING.md
- Updated ROADMAP.md

---

## Future Enhancements (Optional)

These are NOT included in the starter kit:

| Feature | Effort |
|---|---|
| OAuth2 (Google, GitHub) | 1-2 hrs |
| 2FA (TOTP) | 1-2 hrs |
| CLI scaffolding (make scaffold) | 1-2 hrs |
| Frontend (HTML/JS) | 2-4 hrs |
| WebSocket support | 2 hrs |
| GraphQL endpoint | 2 hrs |
| Email service (SMTP) | 30 min |
| File uploads | 30 min |
| Redis caching | 1 hr |
| Background jobs (asynq) | 2 hrs |
| Distributed tracing (OTel) | 1 hr |
| Custom domain deployment | 30 min |

---

## Architecture

See docs/ARCHITECTURE.md for:

- Service layer rules (hybrid)
- Testing strategy
- Decision principles

---

## Contributing

See CONTRIBUTING.md.

---

## License

MIT
