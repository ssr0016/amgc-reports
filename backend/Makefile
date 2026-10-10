.DEFAULT_GOAL := help
CYAN  := \033[36m
GREEN := \033[32m
RESET := \033[0m

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

BINARY_NAME := api
CMD_PATH    := ./cmd/api
BIN_DIR     := ./bin

.PHONY: help
help:
	@echo "$(CYAN)Available commands:$(RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(RESET) %s\n", $$1, $$2}'

.PHONY: dev
dev:
	@air -c .air.toml

.PHONY: run
run:
	@go run $(CMD_PATH)/main.go

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "$(GREEN)✓ Built $(BIN_DIR)/$(BINARY_NAME)$(RESET)"

.PHONY: clean
clean:
	@rm -rf $(BIN_DIR) tmp
	@go clean -cache -testcache

.PHONY: install
install:
	@go mod download
	@go mod tidy

.PHONY: tools
tools:
	@go install github.com/air-verse/air@latest
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install github.com/pressly/goose/v3/cmd/goose@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: db-start
db-start:
	@docker compose up -d pg test-pg pg-admin
	@echo "$(GREEN)✓ DB containers started$(RESET)"

.PHONY: db-stop
db-stop:
	@docker compose down

.PHONY: db-logs
db-logs:
	@docker compose logs -f pg

.PHONY: db-reset
db-reset:
	@docker compose down -v
	@docker compose up -d pg test-pg pg-admin

.PHONY: migrate-up
migrate-up:
	@goose -dir internal/database/migrations postgres "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down:
	@goose -dir internal/database/migrations postgres "$(DATABASE_URL)" down

.PHONY: migrate-status
migrate-status:
	@goose -dir internal/database/migrations postgres "$(DATABASE_URL)" status

.PHONY: test
test:
	@go test ./cmd/... ./internal/... ./pkg/... $(shell test -d api && printf './api/...') -race -cover

.PHONY: lint
lint:
	@golangci-lint run ./cmd/... ./internal/... ./pkg/... $(shell test -d api && printf './api/...')

.PHONY: fmt
fmt:
	@gofmt -w -s .

.PHONY: vet
vet:
	@go vet ./...

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: swagger
swagger:
	@swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	@echo "$(GREEN)✓ Swagger docs generated$(RESET)"
