# Beacon — Infrastructure Monitoring Platform
# Root Makefile. Backend targets shell into ./backend.

.DEFAULT_GOAL := help
SHELL := /bin/bash

BACKEND_DIR := backend
# Auto-load deploy/.env (host-port overrides etc.) when present so `make up`
# picks up local customizations without extra flags.
COMPOSE := docker compose -f deploy/docker-compose.yml $(if $(wildcard deploy/.env),--env-file deploy/.env,)

# Work around a macOS (recent Darwin) + Go internal-linker issue that produces
# test binaries missing LC_UUID ("signal: abort trap"). Forcing external linking
# uses the system linker, which emits LC_UUID. No effect on Linux/CI.
GOTESTFLAGS :=
ifeq ($(shell uname -s),Darwin)
GOTESTFLAGS += -ldflags=-linkmode=external
endif

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_.-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

## ---------------------------------------------------------------------------
## Backend
## ---------------------------------------------------------------------------
.PHONY: tidy
tidy: ## go mod tidy
	cd $(BACKEND_DIR) && go mod tidy

.PHONY: build
build: ## Build api + worker binaries into backend/bin
	cd $(BACKEND_DIR) && go build -o bin/api ./cmd/api && go build -o bin/worker ./cmd/worker

.PHONY: run-api
run-api: ## Run the API server locally
	cd $(BACKEND_DIR) && go run ./cmd/api

.PHONY: run-worker
run-worker: ## Run the background worker locally
	cd $(BACKEND_DIR) && go run ./cmd/worker

.PHONY: test
test: ## Run all Go tests
	cd $(BACKEND_DIR) && go test $(GOTESTFLAGS) ./... -count=1

.PHONY: cover
cover: ## Run tests with coverage report
	cd $(BACKEND_DIR) && go test $(GOTESTFLAGS) ./... -covermode=atomic -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -1

.PHONY: vet
vet: ## go vet
	cd $(BACKEND_DIR) && go vet ./...

.PHONY: fmt
fmt: ## gofmt all packages
	cd $(BACKEND_DIR) && gofmt -w -s .

.PHONY: check
check: fmt vet test ## fmt + vet + test

## ---------------------------------------------------------------------------
## Database
## ---------------------------------------------------------------------------
.PHONY: migrate-up
migrate-up: ## Apply all pending migrations (uses backend/.env)
	cd $(BACKEND_DIR) && go run ./cmd/api migrate up

.PHONY: migrate-status
migrate-status: ## Show migration status
	cd $(BACKEND_DIR) && go run ./cmd/api migrate status

## ---------------------------------------------------------------------------
## Docker / Compose (full stack: pg, redis, prometheus, blackbox, alertmanager, api, worker, frontend)
## ---------------------------------------------------------------------------
.PHONY: up
up: ## Start the full stack
	$(COMPOSE) up -d --build
	@# A rebuilt backend gets a new container IP; recreate the gateway so nginx
	@# re-resolves it (avoids a stale-upstream 502 after code changes).
	$(COMPOSE) up -d --force-recreate --no-deps nginx

.PHONY: down
down: ## Stop the full stack
	$(COMPOSE) down

.PHONY: logs
logs: ## Tail stack logs
	$(COMPOSE) logs -f --tail=100

.PHONY: ps
ps: ## Show stack status
	$(COMPOSE) ps

## ---------------------------------------------------------------------------
## Frontend
## ---------------------------------------------------------------------------
.PHONY: fe-dev
fe-dev: ## Run the frontend dev server
	cd frontend && npm run dev

.PHONY: fe-build
fe-build: ## Build the frontend
	cd frontend && npm run build
