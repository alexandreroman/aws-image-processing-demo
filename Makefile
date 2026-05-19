.DEFAULT_GOAL := dev

# Canonical environment. Required for deploy targets, baseline for dev.
# Optional: missing .env is not an error.
ifneq (,$(wildcard .env))
include .env
export
endif

# Local dev overlay: only loaded for dev/test targets so deploy
# targets see only `.env`. Sequential include → later assignments win.
DEV_TARGETS := dev backend worker frontend app-up app-down app-logs \
               infra-up infra-down infra-logs test check
GOALS := $(or $(MAKECMDGOALS),$(.DEFAULT_GOAL))
ifneq (,$(filter $(DEV_TARGETS),$(GOALS)))
ifneq (,$(wildcard .env.local))
include .env.local
export
endif
endif

##@ Infra

.PHONY: infra-up
infra-up: ## Bring up local infra (Moto, Temporal dev server, bucket/table init)
	docker-compose up -d temporal moto init

.PHONY: infra-down
infra-down: ## Stop infra containers (keeps containers and network around)
	docker-compose stop temporal moto init

.PHONY: infra-logs
infra-logs: ## Follow logs from infra containers
	docker-compose logs -f temporal moto

##@ App

.PHONY: backend
backend: ## Run the backend HTTP server on :8000 with hot reload
	go tool air -c .air.backend.toml

.PHONY: worker
worker: ## Run the Temporal worker with hot reload
	go tool air -c .air.worker.toml

.PHONY: frontend
frontend: ## Run the Nuxt dev server on :3000
	pnpm -C frontend dev

.PHONY: dev
dev: frontend/node_modules infra-up ## Start infra, then run backend + worker + frontend on the host with hot reload
	@$(MAKE) -j backend worker frontend

frontend/node_modules: frontend/package.json
	pnpm -C frontend install

##@ Stack

.PHONY: app-up
app-up: ## Bring up the full stack in Docker (infra + worker + backend + frontend)
	docker-compose up -d --build

.PHONY: app-down
app-down: ## Tear down the full stack (removes containers and network)
	docker-compose down

.PHONY: app-logs
app-logs: ## Follow logs from every stack container
	docker-compose logs -f

##@ Quality

.PHONY: test
test: ## Run Go unit tests
	go test ./...

.PHONY: check
check: ## Run static checks across modules
	go vet ./...
	pnpm -C frontend lint

##@ Build

.PHONY: worker-lambda-zip
worker-lambda-zip: ## Build the worker Lambda deployment artifact (build/worker.zip)
	@mkdir -p build
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
	  go build \
	  -tags lambda.norpc \
	  -ldflags "-s -w -X main.buildID=$(shell git rev-parse --short HEAD)" \
	  -o build/bootstrap ./cmd/worker
	cd build && rm -f worker.zip && zip worker.zip bootstrap

##@ Deploy

.PHONY: deploy
deploy: ## Provision AWS infra and deploy the frontend (uses .env)
	./scripts/deploy.sh

.PHONY: frontend-deploy
frontend-deploy: ## Rebuild and sync the frontend (uses .env)
	./scripts/frontend-deploy.sh

.PHONY: teardown
teardown: ## Destroy all AWS infrastructure (uses .env)
	./scripts/teardown.sh

##@ Helpers

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(firstword $(MAKEFILE_LIST))
