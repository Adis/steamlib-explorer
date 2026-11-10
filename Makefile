GOOSE   := go run github.com/pressly/goose/v3/cmd/goose@v3.26.0
MIGRATIONS := backend/db/migrations

-include .env
export

.PHONY: db-up db-down run build test migrate-up migrate-down migrate-status

db-up:          ## Start Postgres in Docker
	docker compose up -d --wait db

db-down:        ## Stop Postgres (data is kept in the pgdata volume)
	docker compose down

run:            ## Run the backend (applies migrations on startup)
	cd backend && go run ./cmd/server

build:          ## Build the backend binary into backend/bin/server
	cd backend && go build -o bin/server ./cmd/server

test:           ## Run the backend tests
	cd backend && go test ./...

migrate-up:     ## Apply all migrations
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" up

migrate-down:   ## Roll back the last migration
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" down

migrate-status: ## Show which migrations are applied
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" status
