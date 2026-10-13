# Setup Guide

Step-by-step guide to get this project running locally.

## Prerequisites

### 1. Go 1.26+
sudo apt install golang-go
go version

Alternatively, install Go 1.26 or newer from https://go.dev/dl/ (the apt package may be older).

`go.mod` requires Go 1.26.0 or newer. Go 1.21 and later download the required
toolchain automatically (`GOTOOLCHAIN=auto`, the default), so an older Go on
your PATH still works, but the first `go build` or `go install` needs network
access. `go version` shows the version Go switched to; `GOTOOLCHAIN=local go
version` shows the version actually installed. On an offline or air-gapped
machine the download fails (`go.mod requires go >= 1.26.0 (running go 1.23.0;
GOTOOLCHAIN=local)`), so install Go 1.26 or newer yourself first. The same
applies to the tools below: `go install` may fetch a newer toolchain for them.

### 2. Docker & Docker Compose
sudo apt install docker.io docker-compose-v2
sudo usermod -aG docker $USER
docker --version

### 3. Make
sudo apt install make

### 4. Dev Tools
go install github.com/air-verse/air@v1.67.4
go install github.com/swaggo/swag/cmd/swag@v1.16.6
go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2

export PATH="$PATH:$(go env GOPATH)/bin"
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc

These versions are defined in the Makefile (`AIR_VERSION`, `SWAG_VERSION`,
`GOOSE_VERSION`, `GOLANGCI_LINT_VERSION`); after cloning, `make tools`
installs the same set. The golangci-lint version matches CI. Keep the two in
sync when you bump a version.

Check an installed tool with `go version -m $(go env GOPATH)/bin/<tool>`. Do
not rely on `swag --version`: it prints a stale string (a v1.16.6 build
reports v1.16.4).

## Setup Steps

### 1. Clone
git clone https://github.com/ssr0016/pastoral-reports.git
cd pastoral-reports/backend

### 2. Configure
cp .env.example .env
openssl rand -hex 32
nano .env

### 3. Start Database
make db-start
docker ps | grep pastoral

### 4. Install Dependencies
go mod download
go mod tidy

### 5. Run Migrations
make migrate-up

If `make migrate-up` or `make migrate-status` fails with a connection error
such as `failed to connect to user=<your-os-user> database=` (note the empty
database name), `DATABASE_URL` is empty because there is no `.env` yet. `.env`
is not committed; copy `.env.example` to `.env` first (step 2).

### 6. Generate Swagger
make swagger

### 7. Start Server
make dev

Expected output:
🚀 API Server:  http://localhost:8080
📖 Swagger UI:  http://localhost:8080/swagger/index.html
❤️  Health:      http://localhost:8080/health

## Verification

### Test 1: Health
curl -s http://localhost:8080/health

### Test 2: Register
curl -c /tmp/c.txt http://localhost:8080/health > /dev/null
CSRF=$(grep csrf_token /tmp/c.txt | awk '{print $NF}')
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $CSRF" \
  -b /tmp/c.txt \
  -d '{"email":"test@example.com","name":"Test","password":"X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"}'

## Getting tokens for local development

Verification and password-reset tokens are no longer written to the logs, and
email delivery is not implemented yet. Read them from the database instead.

### Quick way

```bash
make dev-token USER=test@example.com              # email verification token
make dev-token USER=test@example.com TYPE=reset   # password reset token
```

Then, for example:

```bash
curl "http://localhost:8080/api/v1/auth/verify-email?token=$(make -s dev-token USER=test@example.com)"
```

`USER` must be passed on the command line (the shell's own `$USER` is ignored).
The target runs `psql` inside the `pg` container from `docker-compose.yaml`, so
`make db-start` and `make migrate-up` must have run.

### With psql or pgAdmin

```sql
-- latest email verification token
SELECT t.token, t.expires_at, t.used_at
FROM verification_tokens t JOIN users u ON u.id = t.user_id
WHERE u.email = 'test@example.com'
ORDER BY t.created_at DESC LIMIT 1;

-- latest password reset token
SELECT t.token, t.expires_at, t.used_at
FROM password_resets t JOIN users u ON u.id = t.user_id
WHERE u.email = 'test@example.com'
ORDER BY t.created_at DESC LIMIT 1;
```

pgAdmin (http://localhost:5051, credentials in the table above): register a
server with host `pg`, port `5432`, and the `DB_USER` / `DB_PASSWORD` /
`DB_NAME` values from `.env`, then run the queries in the Query Tool.

> Production email delivery is tracked as E-1 in docs/06-Implementation-Plan.md.

## Deploying to Railway

The repository root holds `backend/` (Go) and `frontend/` (Next.js), so Railway
cannot detect the Go app from the root (Railpack fails with "failed to prepare
the build"). Deploy `backend/` as its own service, built from its `Dockerfile`.

### 1. Service settings (dashboard, one time)

- **Root Directory**: `backend`. The Dockerfile copies `go.mod` and `go.sum`,
  so the build context must be `backend/`.
- **Pre-deploy Command** (Deploy tab): `/app/api migrate`. See
  [Railway Config-as-code deprecation](#railway-config-as-code-deprecation).

There is no start command to set: the image runs `/app/api`.

### 2. Environment variables

| Variable | Value | Notes |
|---|---|---|
| `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` | Required. Reference the Railway Postgres service. Switches to the `app_user` URL in the [rollout](#app_user-rollout-runbook). |
| `MIGRATE_DATABASE_URL` | `migrate_user` URL (private host) | Optional. See [below](#migrate_database_url). Required once `DATABASE_URL` is `app_user`. |
| `APP_ENV` | `production` | Exactly this value: the session cookie is `Secure` only for `production`. |
| `LOG_FORMAT` | `json` | |
| `CORS_ALLOWED_ORIGINS` | frontend origin(s), comma-separated | Defaults to `http://localhost:3000`. |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | initial admin | Set both or neither. Password at least 12 characters. |
| `APP_URL` | the public URL | Only used in startup log lines. |
| `TRUSTED_PROXY_CIDRS` | leave unset on Railway | Comma-separated CIDRs of the proxies whose `X-Real-IP` is believed. Defaults to `100.64.0.0/10` (Railway's edge range). Set it when deploying elsewhere; if it is wrong, every user shares one rate-limit bucket and the app logs a warning after 100 requests. |

`PORT` is injected by Railway and read as a fallback when `APP_PORT` is unset.
Do not set `APP_PORT` on Railway.

### 3. Migrations

With `APP_ENV=production` the server does **not** run migrations on startup.
Instead the service's Pre-deploy Command, `/app/api migrate`, runs between
the build and the deploy, in a separate container with the service's variables
and private network. It applies the embedded goose migrations using
`MIGRATE_DATABASE_URL` (falling back to the internal `DATABASE_URL`), so no
public database URL is needed.

- If it exits non-zero, the deploy does not proceed and the previous version
  keeps running. It is not retried.
- Railway enforces its own pre-deploy timeout; the 300 seconds in `railway.json`
  (`preDeployTimeoutSeconds`) is not read.
- Its log lines carry `phase=migrate`, and each applied migration is logged.
  "no migrations to run" means the database was already up to date.
- `api migrate` runs only migrations; it never starts the server. Any other
  argument is a usage error (exit 2), so a typo cannot start the server.

Run it by hand only to bootstrap or recover, using the same command locally
against a database you can reach:

    DATABASE_URL='<url>' go run ./cmd/api migrate

The admin user is seeded on startup when `ADMIN_EMAIL` and `ADMIN_PASSWORD`
are set.

### 4. After the first deploy

- `curl https://<your-service>.up.railway.app/health`
- Check the logs for the `peer_ip` and `remote_ip` fields. `peer_ip` is the
  proxy and should sit inside `TRUSTED_PROXY_CIDRS`; `remote_ip` is the client.
  If the app logs "No request from a trusted proxy in the first 100 requests",
  the range is wrong and every user shares one rate-limit bucket.
- Confirm the proxy overwrites `X-Real-IP` (a client must not be able to choose
  its address): `curl -s -H 'X-Real-IP: 1.2.3.4' 'https://<your-service>.up.railway.app/health?marker=x'`.
  The logged `remote_ip` must be your own address, not `1.2.3.4`.

### MIGRATE_DATABASE_URL

A separate connection URL used only by `api migrate`. If it is not set, it
falls back to `DATABASE_URL`, so single-URL setups need no change. Point it at
a role with DDL privileges so that `DATABASE_URL` can be a limited role such
as `app_user`. The server itself never connects with `MIGRATE_DATABASE_URL`,
but the variable is still in the service's environment, so a compromise of the
running server can read it.

In production use `migrate_user` (created by migration
`20251012000001_create_migrate_user_role.sql`), not the `postgres` superuser.
`migrate_user` owns the tables, so it can still `DROP` them; it cannot create
roles, change server settings (`ALTER SYSTEM`) or run programs
(`COPY ... TO PROGRAM`). The pre-deploy log line `applying migrations` carries
`role=`, which shows who ran the migrations.

#### migrate_user rollout runbook

The role is `NOLOGIN` until you enable it, so nothing changes on deploy.

1. Deploy a release containing the migration. `MIGRATE_DATABASE_URL` is still
   the owner URL, so the pre-deploy command creates the role and transfers
   ownership of the tables to it.
2. In the Postgres console, check the result:
   `SELECT version();` and
   `SELECT tablename, tableowner FROM pg_tables WHERE schemaname = 'public';`
   Every row should show `migrate_user`. If a table has another owner, fix
   that before continuing.
3. Enable login. In the console type `\password migrate_user`, then
   `ALTER ROLE migrate_user LOGIN;`. Store the password in your password manager.
4. Set `MIGRATE_DATABASE_URL` to
   `postgresql://migrate_user:<password>@postgres.railway.internal:5432/railway`.
5. Redeploy. With nothing pending, the pre-deploy command should log
   `applying migrations role=migrate_user` and `migrations complete`.

**Rollback:** set `MIGRATE_DATABASE_URL` back to the owner URL and redeploy.
No data changes; optionally `ALTER ROLE migrate_user NOLOGIN;`.

**Fresh environments:** the migrations that create roles need a superuser or
the database owner, so bootstrap a new database with the owner URL, then
switch to `migrate_user` as above.

### app_user rollout runbook

Migration `20251012000000_create_app_user_role.sql` creates `app_user` as
`NOLOGIN`, so nothing uses it until you enable it. To switch the running
service to it:

1. In the Railway Postgres console: `ALTER ROLE app_user LOGIN PASSWORD '<random>';`
2. Set `MIGRATE_DATABASE_URL` on the service to the Postgres owner URL (or to
   `migrate_user` once the [migrate_user rollout](#migrate_user-rollout-runbook)
   is done).
3. Set `DATABASE_URL` on the service to the `app_user` URL (private network host).
4. Redeploy.
5. Verify: `/health`, `/ready`, login, and role create/delete.

   5b. In the Deploy Logs, look for `admin user seeded`. It confirms the
   startup seed works as `app_user`.

Do step 2 before step 3, or the pre-deploy migrate runs as `app_user` and
fails on DDL.

**Rollback:** set `DATABASE_URL` back to the Postgres URL and redeploy. It is
one variable and no data changes.

**DELETE limitation:** `app_user` has the `DELETE` privilege, so this is not
protection against data loss. The goal is to block DDL and escalation (`DROP`,
`ALTER`, `CREATE`, `TRUNCATE`, `COPY ... TO PROGRAM`, `CREATE ROLE`).

### Railway Config-as-code deprecation

The pre-deploy command is set in the Railway UI (Deploy → Pre-deploy
Command), not in `railway.json`, because Railway has deprecated
Config-as-code. Existing config files keep working until 2026-12-01, but a
service that has not used one can no longer opt in; here the config path was
never set, so Railway does not read `backend/railway.json`. The file stays in
the repo for reference and for a future move to Infrastructure as Code. Treat
the dashboard as the source of truth; changing `railway.json` has no effect.

## Access URLs

| Service | URL | Credentials |
|---|---|---|
| API | http://localhost:8080 | - |
| Swagger | http://localhost:8080/swagger/index.html | - |
| PgAdmin | http://localhost:5051 | admin@example.com / admin |

## Troubleshooting

### Port in use
sudo fuser -k 8080/tcp

### DB connection failed
docker compose restart pg

### Permission denied on volumes
sudo chown -R $USER:$USER volumes/

### Swagger not loading
make swagger
