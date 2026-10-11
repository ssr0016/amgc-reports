# Setup Guide

Step-by-step guide to get this project running locally.

## Prerequisites

### 1. Go 1.26+
sudo apt install golang-go
go version

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
go install github.com/air-verse/air@latest
go install github.com/swaggo/swag/cmd/swag@latest
go install github.com/pressly/goose/v3/cmd/goose@latest

export PATH="$PATH:$(go env GOPATH)/bin"
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc

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
docker ps | grep template-go

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
