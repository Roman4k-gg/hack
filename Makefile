SHELL := /bin/sh

COMPOSE       := docker compose
COMPOSE_DEV   := docker compose --profile dev
COMPOSE_DEMO  := docker compose --profile demo
COMPOSE_OBS   := docker compose --profile dev --profile observability

.DEFAULT_GOAL := up

.PHONY: help up up-dev up-demo up-observability down seed demo-reset logs ps \
        test test-core test-ai test-web coverage e2e lint fmt \
        dev-core dev-ai dev-web dev-mcp

help:
	@grep -E '^[a-zA-Z_-]+:' $(MAKEFILE_LIST) | cut -d: -f1 | sort -u | awk '{printf "  %-20s\n", $$0}'

up:
	$(COMPOSE) up --build -d --wait
	@echo "core:    http://localhost:8081/healthz"
	@echo "ready:   http://localhost:8081/readyz"

up-dev:
	$(COMPOSE_DEV) up --build -d --wait
	@echo "web:     http://localhost:3000"

up-demo:
	$(COMPOSE_DEMO) up --build -d --wait
	@echo "web:     http://localhost:3000"
	@echo "mocks:   http://localhost:8090/healthz"

up-observability:
	$(COMPOSE_OBS) up --build -d

down:
	$(COMPOSE_DEV) --profile demo --profile observability down --remove-orphans

seed:
	$(COMPOSE) --profile demo run --rm seed

demo-reset:
	$(COMPOSE_DEV) --profile demo down -v
	$(MAKE) up
	$(MAKE) seed

logs:
	$(COMPOSE_DEV) --profile demo logs -f --tail=100

ps:
	$(COMPOSE_DEV) --profile demo ps

test: test-core
	@if [ -d ai ]; then $(MAKE) test-ai; fi
	@if [ -d web ]; then $(MAKE) test-web; fi

test-core:
	cd core && go test ./...

test-ai:
	cd ai && uv run pytest

test-web:
	cd web && pnpm vitest run

coverage:
	cd core && go test ./... -race -coverprofile=coverage.out -covermode=atomic
	cd core && go tool cover -html=coverage.out -o coverage.html
	@mkdir -p docs/coverage
	@cp core/coverage.out docs/coverage/core-coverage.out
	@echo "coverage: core/coverage.html"

e2e:
	cd web && pnpm playwright test

lint:
	cd core && go vet ./...
	cd core && golangci-lint run ./...
	@if [ -d ai ]; then cd ai && uv run ruff check . && uv run ruff format --check .; fi
	@if [ -d web ]; then cd web && pnpm lint; fi

fmt:
	cd core && gofmt -l -w .
	@if [ -d ai ]; then cd ai && uv run ruff format .; fi
	@if [ -d web ]; then cd web && pnpm format; fi

dev-core:
	cd core && go run ./cmd/api

dev-ai:
	cd ai && uv run uvicorn app.main:app --reload --port 8000

dev-web:
	cd web && pnpm dev

dev-mcp:
	cd ai && uv run python -m app.mcp
