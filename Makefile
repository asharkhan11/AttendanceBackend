## attendance-backend Makefile
## Loads configuration from .env

.PHONY: run build migrate-up migrate-down seed test lint docker-build docker-run docker-stop help

# Load .env if present
ifneq (,$(wildcard .env))
  include .env
  export
endif

BINARY=./bin/attendance-api
MIGRATIONS_DIR=internal/db/migrations
DB_URL?=$(DATABASE_URL)

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

run: ## Run the API server locally
	go run ./cmd/api/

build: ## Build static binaries to ./bin/
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/api/
	go build -o ./bin/attendance-migrate ./cmd/migrate/

migrate-up: ## Apply all pending database migrations
	@echo "Running migrations UP..."
	go run ./cmd/migrate/ up

migrate-down: ## Roll back the last database migration
	@echo "Rolling back last migration..."
	go run ./cmd/migrate/ down 1

seed: ## Seed demo data into the database
	@echo "Seeding database..."
	go run ./cmd/seed/

test: ## Run all tests
	go test -v ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

swagger: ## Regenerate Swagger/OpenAPI documentation
	docker run --rm -v "$(CURDIR):/app" -w /app golang:1.22-alpine go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go

sqlc-generate: ## Regenerate sqlc query code
	sqlc generate

docker-build: ## Build the production Docker image
	docker build -f deployments/Dockerfile -t attendance-backend:latest .

docker-run: ## Run the container stack via docker-compose
	docker compose -f deployments/docker-compose.yml up --build -d

docker-stop: ## Stop the docker-compose stack
	docker compose -f deployments/docker-compose.yml down

clean: ## Remove built binaries
	rm -rf bin/
