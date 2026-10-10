# ==========================================
# Stage 1: Builder
# ==========================================
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/bin/api ./cmd/api

# ==========================================
# Stage 2: Runtime
# ==========================================
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata wget

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/internal/database/migrations /app/migrations

RUN chown -R app:app /app

USER app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 -O /dev/null http://localhost:8080/health || exit 1

CMD ["/app/api"]
