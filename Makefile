.DEFAULT_GOAL := help

BACKEND  := apps/backend
FRONTEND := apps/frontend

# Load .env so the Go commands see DB_* and JWT_* without a wrapper script.
ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: help
help: ## Show the available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: setup
setup: ## Copy .env.example to .env and install frontend dependencies
	@[ -f .env ] || cp .env.example .env
	cd $(FRONTEND) && npm install

.PHONY: run
run: ## Run the API on the host
	cd $(BACKEND) && go run ./cmd/api

.PHONY: run-frontend
run-frontend: ## Run the Next.js dev server
	cd $(FRONTEND) && npm run dev

.PHONY: build
build: ## Build the backend binaries and the frontend bundle
	cd $(BACKEND) && go build -trimpath -o bin/api ./cmd/api
	cd $(BACKEND) && go build -trimpath -o bin/migrate ./cmd/migrate
	cd $(BACKEND) && go build -trimpath -o bin/seed ./cmd/seed
	cd $(FRONTEND) && npm run build

.PHONY: migrate
migrate: ## Create the database if needed and apply all migrations
	cd $(BACKEND) && go run ./cmd/migrate -command up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	cd $(BACKEND) && go run ./cmd/migrate -command down

.PHONY: migrate-version
migrate-version: ## Print the current schema version
	cd $(BACKEND) && go run ./cmd/migrate -command version

.PHONY: seed
seed: ## Load development data, refused when APP_ENV is production
	cd $(BACKEND) && go run ./cmd/seed

.PHONY: test
test: ## Run the backend test suite
	cd $(BACKEND) && go test ./... -count=1

.PHONY: test-cover
test-cover: ## Run tests and report total coverage
	cd $(BACKEND) && go test ./... -count=1 -coverprofile=coverage.out
	cd $(BACKEND) && go tool cover -func=coverage.out | tail -1

.PHONY: swagger
swagger: ## Regenerate the OpenAPI specification from the handler annotations
	cd $(BACKEND) && go run github.com/swaggo/swag/cmd/swag@latest init \
		--generalInfo cmd/api/main.go \
		--dir ./ \
		--parseDependency \
		--parseInternal \
		--output docs

.PHONY: lint
lint: ## Check formatting and run go vet plus the TypeScript linter
	cd $(BACKEND) && test -z "$$(gofmt -l ./cmd ./internal)" || (echo "gofmt needed:"; gofmt -l ./cmd ./internal; exit 1)
	cd $(BACKEND) && go vet ./...
	cd $(FRONTEND) && npm run lint

.PHONY: sync-db-docs
sync-db-docs: ## Copy the backend migrations into database/migrations for review
	cp $(BACKEND)/migrations/*.sql database/migrations/

.PHONY: docker-up
docker-up: ## Start postgres, run migrations and seed, then start backend and frontend
	docker compose up -d --build

.PHONY: docker-down
docker-down: ## Stop the stack and keep the database volume
	docker compose down

.PHONY: docker-reset
docker-reset: ## Stop the stack and delete the database volume
	docker compose down -v

.PHONY: docker-logs
docker-logs: ## Follow the backend and frontend logs
	docker compose logs -f backend frontend
