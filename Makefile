.DEFAULT_GOAL := dev

# Canonical environment. Required for deploy targets, baseline for dev.
# Optional: missing .env is not an error.
ifneq (,$(wildcard .env))
include .env
export
endif

# Local dev overlay. Loaded only by the host-mode dev and quality targets.
# Two invariants this list encodes:
#   - deploy targets see `.env` only, never the overlay;
#   - the compose stack is self-contained — it inlines every constant and
#     interpolates just ANTHROPIC_API_KEY — so the app-*/infra-down/-logs
#     targets must not pull `.env.local` into Make's environment either.
# Sequential include → later assignments win.
DEV_TARGETS := dev backend worker frontend infra-up test check
GOALS := $(or $(MAKECMDGOALS),$(.DEFAULT_GOAL))
ifneq (,$(filter $(DEV_TARGETS),$(GOALS)))
ifneq (,$(wildcard .env.local))
include .env.local
export
endif
endif

# Keep corepack's first-use download of the pinned pnpm non-interactive. The
# prompt default depends on the entry point, not the version: the `corepack`
# CLI seeds this to 0, the bare `pnpm` shim from `corepack enable` seeds it
# to 1. The recipes below use the CLI, so the pin is belt-and-braces. `?=`
# lets a deliberate =1 (download auditing) win; the `export` is explicit
# because the bare `export` above runs only when .env exists.
COREPACK_ENABLE_DOWNLOAD_PROMPT ?= 0
export COREPACK_ENABLE_DOWNLOAD_PROMPT

# Casper / cmux worktree port isolation. compose.override.yaml (auto-merged
# by docker compose, gitignored) may remap the published host ports so
# parallel workspaces don't collide. When present it is the source of truth:
# read the actual published ports from it so the endpoint banner and the
# host-side `make dev` flow never diverge from what docker binds. Otherwise
# fall back to the conventional defaults.
ifneq (,$(wildcard compose.override.yaml))
# $(call override_port,<container port>,<default host port>) — read the host
# port published for a container port. A hand-edited override may be missing
# a mapping, so warn and fall back to the default instead of silently
# exporting a truncated value such as `TEMPORAL_ADDRESS=localhost:`.
# It stays recursive so `$(call ...)` sees its arguments, and `unexport` keeps
# it out of the recipe environment: the bare `export` above would otherwise
# expand it with empty arguments and fire the warning on every recipe.
override_port = $(or \
  $(shell sed -nE 's/.*"([0-9]+):$(1)".*/\1/p' compose.override.yaml | head -n1),\
  $(warning compose.override.yaml publishes no host port for :$(1) — falling back to $(2))$(2))
unexport override_port

FRONTEND_PORT      := $(call override_port,3000,3000)
TEMPORAL_GRPC_PORT := $(call override_port,7233,7233)
TEMPORAL_UI_PORT   := $(call override_port,8233,8233)
BACKEND_PORT       := $(call override_port,8000,8000)
MOTO_PORT          := $(call override_port,5000,4566)
# Point the host-side dev flow at the remapped Temporal gRPC and Moto ports,
# overriding the fixed values from .env.local.
TEMPORAL_ADDRESS   := localhost:$(TEMPORAL_GRPC_PORT)
AWS_ENDPOINT_URL   := http://localhost:$(MOTO_PORT)
export TEMPORAL_ADDRESS AWS_ENDPOINT_URL
else
FRONTEND_PORT      := 3000
TEMPORAL_GRPC_PORT := 7233
TEMPORAL_UI_PORT   := 8233
BACKEND_PORT       := 8000
MOTO_PORT          := 4566
endif

# Banner listing where the components will be reachable. Both callers below
# hand over to a foreground command right after printing it, so it announces
# what is about to come up rather than claiming the stack already runs.
define show_urls
	@echo ""
	@echo "Starting the stack — once it is up, open:"
	@echo "  Web UI             http://localhost:$(FRONTEND_PORT)"
	@echo "  Backend API        http://localhost:$(FRONTEND_PORT)/api"
	@echo "  Temporal dashboard http://localhost:$(TEMPORAL_UI_PORT)"
endef

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
backend: ## Run the backend HTTP server with hot reload
	PORT=$(BACKEND_PORT) go tool air -c .air.backend.toml

.PHONY: worker
worker: ## Run the Temporal worker with hot reload
	go tool air -c .air.worker.toml

.PHONY: frontend
frontend: ## Run the Nuxt dev server with hot reload
# The pnpm launcher resolves the `packageManager` pin from its own working directory, and
# `-C` is applied only afterwards. Every host-side call therefore enters frontend/ first and
# goes through corepack, so it runs the pinned pnpm rather than whatever is on the PATH.
	cd frontend && \
	PORT=$(FRONTEND_PORT) \
	NUXT_DEV_API_TARGET=http://localhost:$(BACKEND_PORT)/api \
	NUXT_DEV_IMAGES_TARGET=http://localhost:$(MOTO_PORT)/aws-image-processing-demo-images-local \
	corepack pnpm dev

.PHONY: dev
dev: frontend/node_modules infra-up ## Start infra, then run backend + worker + frontend on the host with hot reload
	$(show_urls)
	@$(MAKE) -j backend worker frontend

frontend/node_modules: frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml
	cd frontend && corepack pnpm install

##@ Stack

.PHONY: app-up
app-up: ## Bring up the full stack in Docker (infra + worker + backend + frontend)
	$(show_urls)
	docker-compose up

.PHONY: app-down
app-down: ## Tear down the full stack (removes containers and network)
	docker-compose down

.PHONY: app-logs
app-logs: ## Follow logs from every stack container
	docker-compose logs -f

##@ Quality

.PHONY: test
test: ## Run Go unit tests (race detector on)
	go test -race ./...

.PHONY: check
check: test ## Run unit tests and static checks across modules
	go vet ./...
	cd frontend && corepack pnpm lint

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

##@ Worktree

.PHONY: worktree-ports
worktree-ports: ## Remap host ports off CASPER_PORT so parallel workspaces don't collide
	@if [ -n "$$CASPER_PORT" ] && [ -f compose.yaml ]; then \
		base="$$CASPER_PORT"; \
		{ \
		  echo "# Generated by \`make worktree-ports\` (base port $$base)."; \
		  echo "# Remaps host-published ports off a per-workspace base port so parallel"; \
		  echo "# worktrees don't collide. Internal service-to-service ports are"; \
		  echo "# unchanged. Gitignored and regenerated per workspace — do not edit by"; \
		  echo "# hand."; \
		  echo "services:"; \
		  echo "  frontend:"; \
		  echo "    ports: !override"; \
		  echo "      - \"$$base:3000\""; \
		  echo "  temporal:"; \
		  echo "    ports: !override"; \
		  echo "      - \"$$((base + 1)):7233\""; \
		  echo "      - \"$$((base + 2)):8233\""; \
		  echo "  backend:"; \
		  echo "    ports: !override"; \
		  echo "      - \"$$((base + 3)):8000\""; \
		  echo "  moto:"; \
		  echo "    ports: !override"; \
		  echo "      - \"$$((base + 4)):5000\""; \
		} > compose.override.yaml; \
		echo "[worktree-ports] wrote compose.override.yaml (webui=$$base, temporal grpc=$$((base + 1)), temporal ui=$$((base + 2)), backend=$$((base + 3)), moto=$$((base + 4)))"; \
	else \
		echo "[worktree-ports] CASPER_PORT unset or compose.yaml missing; skipping"; \
	fi

##@ Helpers

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(firstword $(MAKEFILE_LIST))
