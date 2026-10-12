---
description: Create a PostgreSQL migration for AmgReports using goose
---
Create a new database migration for AmgReports:

1. Use goose format with `-- +goose Up` and `-- +goose Down` directives
2. Use pgx/v5-compatible SQL with proper parameter binding ($1, $2, etc.)
3. Follow naming convention: `YYYYMMDDHHMMSS_description.sql` in `internal/database/migrations/`
4. Add appropriate indexes for foreign keys and frequently queried columns
5. Update affected repository files in `internal/repository/`
6. Apply migration: `make migrate-up`
7. Rollback if needed: `make migrate-down`
8. Verify status: `make migrate-status`
9. Run integration tests: `make test-integration`
