SHELL := /bin/sh

ENV_FILE ?= .env.local
COMPOSE ?= docker-compose -f docker-compose.yml --env-file $(ENV_FILE)
GO_CACHE_ENV := GOCACHE=$(CURDIR)/.gocache GOMODCACHE=$(CURDIR)/.gomodcache

.PHONY: build test migrate seed-endpoints run-api run-scheduler run-collector run-retention docker-up docker-down docker-logs

build:
	$(GO_CACHE_ENV) go build ./cmd/api-service
	$(GO_CACHE_ENV) go build ./cmd/scheduler-service
	$(GO_CACHE_ENV) go build ./cmd/collector-service
	$(GO_CACHE_ENV) go build ./cmd/retention-service

test:
	$(GO_CACHE_ENV) go test ./...

migrate:
	$(COMPOSE) up -d postgres
	$(COMPOSE) exec -T postgres sh -c 'for f in /migrations/*.sql; do psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -f "$$f"; done'

seed-endpoints:
	$(COMPOSE) up -d postgres
	$(COMPOSE) exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -f /migrations/000007_seed_exchange_api_endpoints.sql'
	$(COMPOSE) exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -f /migrations/000008_seed_derivative_collection_policies.sql'

run-api:
	$(GO_CACHE_ENV) go run ./cmd/api-service

run-scheduler:
	$(GO_CACHE_ENV) go run ./cmd/scheduler-service

run-collector:
	$(GO_CACHE_ENV) go run ./cmd/collector-service

run-retention:
	$(GO_CACHE_ENV) go run ./cmd/retention-service

docker-up:
	$(COMPOSE) up -d --build postgres redis api-service prometheus

docker-down:
	$(COMPOSE) down

docker-logs:
	$(COMPOSE) logs -f --tail=200
