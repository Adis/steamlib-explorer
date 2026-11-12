GOOSE   := go run github.com/pressly/goose/v3/cmd/goose@v3.26.0
SQLC    := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0
MIGRATIONS := backend/db/migrations

-include .env
export

.PHONY: dev db-up db-down run web web-install build test test-db generate migrate-up migrate-down migrate-status

dev:            ## Start everything, e.g. after a git pull: Postgres, new npm packages, backend and frontend (Ctrl+C stops both)
	$(MAKE) db-up frontend/node_modules/.package-lock.json
	$(MAKE) -j2 run web

# npm writes this file on every install, so packages are installed again only
# when package.json or package-lock.json is newer.
frontend/node_modules/.package-lock.json: frontend/package.json frontend/package-lock.json
	cd frontend && npm install
	touch $@

db-up:          ## Start Postgres in Docker
	docker compose up -d --wait db

db-down:        ## Stop Postgres (data is kept in the pgdata volume)
	docker compose down

run:            ## Run the backend (applies migrations on startup)
	cd backend && go run ./cmd/server

web-install:    ## Install the frontend's npm packages
	cd frontend && npm install

web:            ## Run the frontend on http://localhost:5173 (proxies /api and /auth to the backend)
	cd frontend && npm run dev

build:          ## Build the backend binary into backend/bin/server
	cd backend && go build -o bin/server ./cmd/server

test:           ## Run the backend tests
	cd backend && go test ./...

test-db:        ## Run the backend tests, including the ones that need Postgres
	psql "$(DATABASE_URL)" -qc "DROP DATABASE IF EXISTS steamlib_test" -c "CREATE DATABASE steamlib_test"
	cd backend && TEST_DATABASE_URL="$(subst /steamlib?,/steamlib_test?,$(DATABASE_URL))" go test -p 1 ./...

generate:       ## Regenerate Go code from backend/db/queries with sqlc
	cd backend && $(SQLC) generate

migrate-up:     ## Apply all migrations
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" up

migrate-down:   ## Roll back the last migration
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" down

migrate-status: ## Show which migrations are applied
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" status
