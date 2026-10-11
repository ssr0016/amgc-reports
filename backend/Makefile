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
help: ## Show this help
	@echo "$(CYAN)Available commands:$(RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(RESET) %s\n", $$1, $$2}'

.PHONY: dev
dev: ## Run with hot reload (air)
	@air -c .air.toml

.PHONY: run
run: ## Run the API without hot reload
	@go run $(CMD_PATH)/main.go

.PHONY: build
build: ## Build the binary into ./bin
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "$(GREEN)✓ Built $(BIN_DIR)/$(BINARY_NAME)$(RESET)"

.PHONY: clean
clean: ## Remove build artifacts and caches
	@rm -rf $(BIN_DIR) tmp
	@go clean -cache -testcache

.PHONY: install
install: ## Download and tidy Go modules
	@go mod download
	@go mod tidy

.PHONY: tools
tools: ## Install dev tools (air, swag, goose, golangci-lint)
	@go install github.com/air-verse/air@latest
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install github.com/pressly/goose/v3/cmd/goose@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: db-start
db-start: ## Start the dev, test and pgAdmin containers
	@docker compose up -d pg test-pg pg-admin
	@echo "$(GREEN)✓ DB containers started$(RESET)"

.PHONY: db-stop
db-stop: ## Stop all containers
	@docker compose down

.PHONY: db-logs
db-logs: ## Follow the dev database logs
	@docker compose logs -f pg

.PHONY: db-reset
db-reset: ## Destroy volumes and recreate the containers
	@docker compose down -v
	@docker compose up -d pg test-pg pg-admin

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	@goose -dir internal/database/migrations postgres "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	@goose -dir internal/database/migrations postgres "$(DATABASE_URL)" down

.PHONY: migrate-status
migrate-status: ## Show migration status
	@goose -dir internal/database/migrations postgres "$(DATABASE_URL)" status

.PHONY: test
test: ## Run tests with race detector and coverage
	@go test ./cmd/... ./internal/... ./pkg/... $(shell test -d api && printf './api/...') -race -cover

.PHONY: lint
lint: ## Run golangci-lint
	@golangci-lint run ./cmd/... ./internal/... ./pkg/... $(shell test -d api && printf './api/...')

.PHONY: fmt
fmt: ## Format code with gofmt
	@gofmt -w -s .

.PHONY: vet
vet: ## Run go vet
	@go vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy
	@go mod tidy

.PHONY: swagger
swagger: ## Generate Swagger docs
	@swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	@echo "$(GREEN)✓ Swagger docs generated$(RESET)"

.PHONY: dev-token
dev-token: export DEV_EMAIL := $(USER)
dev-token: export DEV_TYPE := $(or $(TYPE),verify)
dev-token: ## Print a dev token: make dev-token USER=<email> [TYPE=verify|reset]
	@if [ "$(origin USER)" != "command line" ] || [ -z "$$DEV_EMAIL" ]; then \
		echo "usage: make dev-token USER=<email> [TYPE=verify|reset]" >&2; exit 2; fi
	@case "$$DEV_TYPE" in \
		verify) table=verification_tokens ;; \
		reset)  table=password_resets ;; \
		*) echo "TYPE must be verify or reset" >&2; exit 2 ;; \
	esac; \
	token=$$(printf '%s\n' "SELECT t.token FROM $$table t JOIN users u ON u.id = t.user_id WHERE u.email = :'email' ORDER BY t.created_at DESC LIMIT 1;" | \
		docker compose exec -T -e DEV_EMAIL pg sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -At -v ON_ERROR_STOP=1 -v email="$$DEV_EMAIL"') \
		|| { echo "query failed (are migrations applied? try: make migrate-up)" >&2; exit 1; }; \
	if [ -z "$$token" ]; then printf 'no %s token found for %s\n' "$$DEV_TYPE" "$$DEV_EMAIL" >&2; exit 1; fi; \
	echo "$$token"
