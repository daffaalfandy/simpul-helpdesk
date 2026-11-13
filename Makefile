SHELL := /bin/bash

# Local settings (copy .env.example to .env).
-include .env
# Uploaded files in development; absolute, so it doesn't depend on `cd api`.
DEV_STORAGE_DIR := $(CURDIR)/data/storage
STORAGE_DIR ?= $(DEV_STORAGE_DIR)
export

API := api
WEB := web
COMPOSE := docker compose -f deploy/compose.yaml
GOTOOL := go tool -modfile=tools.mod

.DEFAULT_GOAL := help
.PHONY: help up down reset-db migrate seed dev dev-api dev-web gen gen-check test test-api test-web lint build

help: ## List targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'

up: ## Start Postgres and Mailpit
	$(COMPOSE) up -d --wait

down: ## Stop dev services (keeps data)
	$(COMPOSE) down

reset-db: ## Delete the dev database volume and uploaded files
	$(COMPOSE) down -v
	rm -rf "$(DEV_STORAGE_DIR)"

migrate: ## Apply database migrations
	cd $(API) && go run ./cmd/simdesk migrate up

seed: ## Load demo data (prints the dev password)
	cd $(API) && go run ./cmd/simdesk seed

dev: ## Run API (with jobs) on :8080 and web on :5173
	$(MAKE) -j2 dev-api dev-web

dev-api:
	cd $(API) && go run ./cmd/simdesk serve --worker

dev-web:
	cd $(WEB) && npm run dev

gen: ## Regenerate sqlc, OpenAPI server and TS client code
	cd $(API) && $(GOTOOL) sqlc generate && $(GOTOOL) oapi-codegen -config oapi-codegen.yaml openapi.yaml
	cd $(WEB) && npm run gen

gen-check: gen ## Fail if generated code is out of date
	@git diff --exit-code -- $(API) $(WEB) && test -z "$$(git status --porcelain -- $(API) $(WEB))" \
		|| (echo "Generated code is out of date: run make gen and commit." && exit 1)

test: test-api test-web ## Run all tests (Docker needed for database tests)

test-api:
	cd $(API) && go test ./...

test-web:
	cd $(WEB) && npm test

lint: ## Static checks
	cd $(API) && test -z "$$(gofmt -l .)" || (gofmt -l . && echo "gofmt needed" && exit 1)
	cd $(API) && go vet ./... && $(GOTOOL) staticcheck ./...
	cd $(WEB) && npm run lint

build: ## Build the web app into the Go binary at bin/simdesk
	cd $(WEB) && npm run build
	touch $(API)/internal/webui/dist/.gitkeep
	cd $(API) && CGO_ENABLED=0 go build -trimpath -o ../bin/simdesk ./cmd/simdesk
