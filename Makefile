GOOSE   := go run github.com/pressly/goose/v3/cmd/goose@v3.26.0
MIGRATIONS := backend/db/migrations

-include .env
export

.PHONY: db-up db-down migrate-up migrate-down migrate-status

db-up:          ## Start Postgres in Docker
	docker compose up -d --wait db

db-down:        ## Stop Postgres (data is kept in the pgdata volume)
	docker compose down

migrate-up:     ## Apply all migrations
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" up

migrate-down:   ## Roll back the last migration
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" down

migrate-status: ## Show which migrations are applied
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" status
