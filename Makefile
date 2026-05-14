.DEFAULT_GOAL := dev

# Load .env into Make's environment so host-mode targets (make dev,
# make backend, make worker) see the same vars as the Docker stack.
# Optional: missing .env is not an error.
ifneq (,$(wildcard .env))
include .env
export
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

##@ Deploy

.PHONY: deploy
deploy: ## Provision AWS infra (tofu init + apply) and deploy the frontend
	tofu -chdir=infra init
	tofu -chdir=infra apply
	$(MAKE) frontend-deploy

.PHONY: frontend-deploy
frontend-deploy: ## Build the frontend, sync to S3, invalidate CloudFront
	pnpm -C frontend install
	NUXT_PUBLIC_SAMPLES_BUCKET=$$(tofu -chdir=infra output -raw images_bucket) \
		pnpm -C frontend generate
	aws s3 sync frontend/.output/public/ \
		s3://$$(tofu -chdir=infra output -raw frontend_bucket)/ --delete
	aws cloudfront create-invalidation \
		--distribution-id $$(tofu -chdir=infra output -raw cloudfront_distribution_id) \
		--paths '/*'

.PHONY: teardown
teardown: ## Destroy all AWS infrastructure
	tofu -chdir=infra destroy

##@ Helpers

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
