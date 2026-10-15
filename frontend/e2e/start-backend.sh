#!/usr/bin/env bash
# Starts the API for the e2e run on :8081 against a fresh database, so every run
# begins empty (audit rows are permanent, so a reused database would pile up).
# Needs Postgres on localhost:$TEST_DB_PORT (default 5440): `make db-start` in
# backend/ locally, a `services:` container in CI. Migrations and the admin seed
# run when the API starts.
set -euo pipefail
cd "$(dirname "$0")/../../backend"
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi
: "${DB_USER:=postgres}" "${DB_PASSWORD:=postgres}" "${TEST_DB_PORT:=5440}"

DB=pastoral_e2e
PGPASSWORD="$DB_PASSWORD" psql -h localhost -p "$TEST_DB_PORT" -U "$DB_USER" -d postgres -q -v ON_ERROR_STOP=1 \
  -c "DROP DATABASE IF EXISTS $DB WITH (FORCE)" -c "CREATE DATABASE $DB"

export APP_PORT=8081
export APP_URL=http://localhost:8081
export DB_PORT="$TEST_DB_PORT" DB_NAME="$DB"
export DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@localhost:${TEST_DB_PORT}/${DB}?sslmode=disable"
export MIGRATE_DATABASE_URL=
export CORS_ALLOWED_ORIGINS=http://localhost:3001
# Lets each test present its own client address (X-Real-IP), so the 5 per minute
# login limit and the per-IP global limit apply per test, not to the whole run.
export TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128
export ADMIN_EMAIL=e2e-admin@example.com ADMIN_NAME=E2E\ Admin ADMIN_PASSWORD=E2eAdmin!2345
export LOG_LEVEL=warn
exec go run ./cmd/api
