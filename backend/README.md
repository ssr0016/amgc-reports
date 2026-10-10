# Template Go Echo Squirrel

[![CI](https://github.com/ssr0016/template-go-echo-squirrel/actions/workflows/ci.yml/badge.svg)](https://github.com/ssr0016/template-go-echo-squirrel/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![Swagger](https://img.shields.io/badge/API-Swagger-85EA2D?style=flat&logo=swagger)](https://swagger.io/)

Production-ready Go backend starter kit with **Echo**, **Squirrel**, **pgx**, **Postgres**, **complete auth**, and **RBAC**.

## Live Demo

- **API:** https://template-go-echo-squirrel-production.up.railway.app
- **Swagger:** https://template-go-echo-squirrel-production.up.railway.app/swagger/index.html
- **Health:** https://template-go-echo-squirrel-production.up.railway.app/health
- **Metrics:** https://template-go-echo-squirrel-production.up.railway.app/metrics

## Features

### Core
- Echo v4 - Fast, minimalist web framework
- PostgreSQL + pgx - High-performance database driver
- Squirrel - Type-safe SQL query builder
- Goose - Database migrations
- Type-safe config - Centralized config loader
- Structured logging - log/slog (JSON/text)
- Standardized errors - Custom apperror package

### Authentication (Complete Flow)
- Session-based auth - alexedwards/scs + Postgres store
- CSRF protection - Double-submit cookie pattern
- Rate limiting - Per-IP rate limiter
- Password strength - 60-bit entropy minimum
- Email verification - Verify email via token
- Password reset - Forgot password flow
- Account lockout - 5 failed logins → 15 min lock
- Bcrypt hashing - Cost 10

### Authorization (RBAC)
- Roles - Dynamic (admin, editor, user)
- Permissions - Granular (users:read, users:write, etc.)
- Role-permission mapping - Many-to-many
- RequireRole middleware - Role-based access
- RequirePermission middleware - Permission-based access
- Admin endpoints - Full RBAC management

### API Features
- Swagger UI - Interactive API docs
- Pagination - Standard data + meta format
- Filtering - Query param filters
- Health checks - /health, /ready, /live
- Prometheus metrics - /metrics endpoint

### Testing
- Unit tests - Mock repositories
- Integration tests - Testcontainers (real Postgres)
- Coverage - Unit + integration
- CI/CD - GitHub Actions (lint, test, build, security)

### Development
- Docker - Multi-stage production build
- Air - Hot reload for development
- Makefile - Common commands
- Documentation - README, SETUP, ROADMAP, CUSTOMIZATION, ARCHITECTURE

## Quick Start

git clone https://github.com/ssr0016/template-go-echo-squirrel.git
cd template-go-echo-squirrel
cp .env.example .env
nano .env
make db-start
go mod download
make migrate-up
make swagger
make dev

Expected output:

    API Server:  http://localhost:8080
    Swagger UI:  http://localhost:8080/swagger/index.html
    Health:      http://localhost:8080/health
    Metrics:     http://localhost:8080/metrics

## API Endpoints

### Public
| Method | Path | Description |
|---|---|---|
| GET | /health | Health check |
| GET | /ready | Readiness (DB ping) |
| GET | /live | Liveness |
| GET | /metrics | Prometheus metrics |
| POST | /api/v1/auth/register | Register user |
| POST | /api/v1/auth/login | Login user |
| POST | /api/v1/auth/forgot-password | Request reset |
| POST | /api/v1/auth/reset-password | Reset password |
| GET | /api/v1/auth/verify-email | Verify email |

### Authenticated
| Method | Path | Description |
|---|---|---|
| POST | /api/v1/auth/logout | Logout |
| GET | /api/v1/auth/me | Current user |
| GET | /api/v1/users | List users (paginated) |
| GET | /api/v1/users/:id | Get user |

### Admin (role: admin)
| Method | Path | Description |
|---|---|---|
| GET | /api/v1/admin/roles | List roles |
| POST | /api/v1/admin/roles | Create role |
| GET | /api/v1/admin/roles/:id | Get role |
| PUT | /api/v1/admin/roles/:id | Update role |
| DELETE | /api/v1/admin/roles/:id | Delete role |
| POST | /api/v1/admin/roles/:id/permissions | Assign permission |
| DELETE | /api/v1/admin/roles/:id/permissions/:pid | Revoke |
| GET | /api/v1/admin/permissions | List permissions |
| POST | /api/v1/admin/permissions | Create permission |
| GET | /api/v1/admin/users/:id | Get user with role |
| PUT | /api/v1/admin/users/:id/role | Change user role |

## Auth Flow

1. Register - POST /api/v1/auth/register
2. Verify email - GET /api/v1/auth/verify-email?token=XXX
3. Login - POST /api/v1/auth/login (sets app_session cookie)
4. Access protected routes - with session cookie
5. Forgot password - POST /api/v1/auth/forgot-password
6. Reset password - POST /api/v1/auth/reset-password
7. Account lockout - 5 failed logins → 15 min

## RBAC

### Default Roles
| Role | Permissions |
|---|---|
| admin | All (9 permissions) |
| editor | users:read, users:write |
| user | users:read |

### Default Admin
- Email: admin@example.com
- Password: AdminPass123!

## Pagination

List endpoints support pagination:

    GET /api/v1/users?page=1&limit=10

Response format:

    {
      "data": [...],
      "meta": {
        "page": 1,
        "limit": 10,
        "total": 100,
        "total_pages": 10
      }
    }

Limits: default page 1, default limit 20, max limit 100.

## Testing

### Unit Tests

    make test
    go test ./internal/... -cover

### Integration Tests (Real DB)

    go test -tags=integration ./internal/repository/ -v

Uses testcontainers (Docker required).

## Observability

### Health Checks
| Endpoint | Purpose |
|---|---|
| /health | Basic health |
| /ready | Readiness (DB ping) |
| /live | Liveness |

### Metrics (Prometheus)

    curl http://localhost:8080/metrics

Metrics: http_requests_total, http_request_duration_seconds, http_requests_in_flight, db_connections_active, db_connections_idle.

## Makefile Commands

    make help           Show all
    make dev            Hot reload
    make run            Direct run
    make build          Build binary
    make test           Run tests
    make lint           Run linter
    make swagger        Generate Swagger
    make db-start       Start DB
    make db-stop        Stop DB
    make db-reset       Wipe + restart
    make migrate-up     Apply migrations
    make migrate-down   Rollback
    make migrate-status Check status

## Project Structure

    template-go-echo-squirrel/
    ├── cmd/api/main.go
    ├── internal/
    │   ├── apperror/     Error handling
    │   ├── config/       Config loader
    │   ├── database/     DB + migrations + seed
    │   ├── handler/      HTTP handlers
    │   ├── logger/       slog wrapper
    │   ├── middleware/   Auth, CSRF, RBAC, metrics
    │   ├── model/        Domain models
    │   ├── repository/   Data access
    │   ├── router/       Routes
    │   ├── service/      Business logic
    │   ├── session/      Session config
    │   ├── testutil/     Test helpers
    │   └── validator/    Custom validation
    ├── pkg/
    │   ├── metrics/      Prometheus metrics
    │   └── pagination/   Pagination helpers
    ├── docs/             Swagger + guides
    ├── Dockerfile
    ├── docker-compose.yaml
    ├── docker-compose.prod.yaml
    ├── Makefile
    └── README.md

## Security

- Session-based auth (Postgres store)
- CSRF protection (double-submit cookie)
- Rate limiting (5/min on auth)
- Password strength (60-bit entropy)
- HttpOnly + SameSite=Lax cookies
- Bcrypt password hashing
- Session renewal on login
- Email verification
- Password reset (1-hour tokens)
- Account lockout (5 attempts, 15 min)
- RBAC (roles + permissions)
- Security scan (gosec) in CI
- Race detector in tests

## Docker Services

| Service | Port | Purpose |
|---|---|---|
| pg | 5435 | Postgres (main) |
| test-pg | 5436 | Postgres (test) |
| pg-admin | 5051 | PgAdmin UI |

PgAdmin: http://localhost:5051 (admin@example.com / admin)

## Deployment

Deployed on Railway with:
- Auto-deploy from GitHub
- Postgres database
- SSL/TLS
- Custom domain ready

## Documentation

- SETUP.md - Setup guide
- ROADMAP.md - Project roadmap
- CUSTOMIZATION.md - Customization guide
- docs/ARCHITECTURE.md - Architecture rules
- docs/API.md - API examples

## Contributing

See CONTRIBUTING.md.

## License

MIT
