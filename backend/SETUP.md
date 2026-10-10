# Setup Guide

Step-by-step guide to get this project running locally.

## Prerequisites

### 1. Go 1.26+
sudo apt install golang-go
go version

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
git clone https://github.com/ssr0016/template-go-echo-squirrel.git
cd template-go-echo-squirrel

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
