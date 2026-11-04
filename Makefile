.DEFAULT_GOAL := help

ifneq ($(CI),)
DEV_CPU_LIMIT_ARGS :=
endif
DEV_RUN_EXTRA_ARGS += -v "$(CURDIR)/.cache/go:/tmp/go-cache" -v "$(CURDIR)/.cache/mod:/tmp/go-mod-cache"
DEV_RUN_DIND_EXTRA_ARGS += -v "$(CURDIR)/.cache/go:/tmp/go-cache" -v "$(CURDIR)/.cache/mod:/tmp/go-mod-cache"
include Makefile.servicepack

format: dev-image ## Format source without traversing dependency caches
	@$(DEV_RUN) go tool golangci-lint fmt ./cmd/... ./internal/... ./pkg/...
	@$(DEV_RUN) sh -c 'find scripts -name "*.sh" -type f -exec shfmt -w {} +'

build: dev-image ## Build the helper binary with the project's current toolchain
	@$(DEV_RUN) sh -ceu 'mkdir -p build; CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags="-s -w -X main.appName=autonomous-stack" -o build/autonomous-stack ./cmd'

# Custom targets below this line
# Note: Override warnings are expected and can be ignored

# Example: override a framework command by uncommenting and editing this.
#
# Left COMMENTED on purpose. As a live target it shadowed the framework's real
# `build`, so `make build` printed a line and produced no binary — which broke
# the README's own Quick Start (`make own` → `make build` → ./build/<name> run)
# for everyone who followed it.
#
# build: ## Custom build command
# 	@echo "Running custom build..."

# Add your custom targets below this line

.PHONY: cache-dirs bootstrap-registration
dev-image: cache-dirs
cache-dirs:
	@mkdir -p .cache/go .cache/mod

bootstrap-registration: dev-image ## Rebuild service registration from current implementations
	@$(DEV_RUN) bash scripts/generate-services.sh

inspect-registration: dev-image
	@$(DEV_RUN) go tool gofindimpl -interface internal/pkg/service-manager/service_manager.go:Service -dir internal/pkg/services

.PHONY: init setup up restart stop down status logs pull image check-compose
init: ## Create .env with generated internal tokens, without replacing an existing file
	@bash scripts/init.sh
image: init ## Build the handoff and setup image locally
	@docker compose build handoff prepare
setup: image ## Validate .env and regenerate private runtime configuration
	@docker compose --profile tools run --rm --no-deps prepare
up: setup ## Prepare configuration and start the stack
	@docker compose up -d
restart: setup ## Rebuild, regenerate configuration and recreate services
	@docker compose up -d --force-recreate
stop: ## Stop the stack without removing data
	@docker compose stop
down: ## Remove this stack's containers and network, preserving all data
	@docker compose down
status: ## Show stack health
	@docker compose ps
logs: ## Follow stack logs
	@docker compose logs --tail=100 -f
pull: ## Refresh first-party service images
	@docker compose pull claude browser mailbox
check-compose: ## Validate the deployment model without printing secrets
	@docker compose config --quiet

.PHONY: test-runtime
test-runtime: image ## Test helper, browser and Mailbox with isolated offline-account fixtures
	@mkdir -p .testing
	@bash scripts/test-runtime.sh

.PHONY: audit-compose
audit-compose: dev-image ## Check rendered deployment policy and documented bootstrap exceptions
	@umask 077; file=$$(mktemp .cache/compose.XXXXXX); trap 'rm -f "$$file"' EXIT; \
		docker compose config --format json >"$$file"; \
		$(DEV_RUN) bash scripts/audit-compose.sh "/work/$$file"
