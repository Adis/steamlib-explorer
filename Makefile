GOOSE   := go run github.com/pressly/goose/v3/cmd/goose@v3.26.0
SQLC    := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0
MIGRATIONS := backend/db/postgres/migrations

-include .env
export

.PHONY: dev check-tools db-up db-down run web web-install web-build build serve pg-to-sqlite test test-db generate migrate-up migrate-down migrate-status

# Put $(FAIL) at the start of a recipe line to get a shell function that
# prints what went wrong and how to fix it, then stops make:
#   @$(FAIL) some-command || fail "What went wrong" "How to fix it"
FAIL = fail() { printf '\n\033[1;31m✗ %s\033[0m\n  %s\n\n' "$$1" "$$2" >&2; exit 1; };

# Docker is needed only when DATABASE_URL points at Postgres; without it the
# backend uses a SQLite file.
USES_POSTGRES := $(filter postgres://% postgresql://%,$(DATABASE_URL))

dev:            ## Start everything, e.g. after a git pull: Postgres (if used), new npm packages, backend and frontend (Ctrl+C stops both)
	@$(MAKE) --no-print-directory check-tools
	@$(if $(USES_POSTGRES),$(MAKE) --no-print-directory db-up,echo "→ Using SQLite, so Docker is not needed")
	@$(MAKE) --no-print-directory frontend/node_modules/.package-lock.json
	@echo "→ Starting the backend and frontend (Ctrl+C stops both)…"
	@$(MAKE) --no-print-directory -j2 run web

check-tools:    ## Check that go and npm (and docker, for Postgres) are installed
	@$(FAIL) for tool in $(if $(USES_POSTGRES),docker) go npm; do \
	  command -v $$tool >/dev/null 2>&1 || fail "$$tool is not installed or not on your PATH." "Install it, open a new terminal and run make dev again."; \
	done

# npm writes this file on every install, so packages are installed again only
# when package.json or package-lock.json is newer.
frontend/node_modules/.package-lock.json: frontend/package.json frontend/package-lock.json
	@echo "→ Frontend packages changed, running npm install…"
	@$(FAIL) cd frontend && npm install || fail "npm install failed in frontend/ (the npm errors are above)." \
	  "Retry with: cd frontend && npm install. A network error usually just needs a retry; otherwise delete frontend/node_modules and try again."
	@touch $@

db-up:          ## Start Postgres in Docker
	@echo "→ Starting Postgres in Docker…"
	@$(FAIL) out=$$(docker info 2>&1) || case "$$out" in \
	  *"permission denied"*) fail "Docker refused the connection: your user may not use Docker (permission denied)." \
	    "Add yourself to the docker group: sudo usermod -aG docker \$$USER, then log out and back in." ;; \
	  *) fail "Docker is not running: make could not reach the Docker daemon." \
	    "Start it with: sudo systemctl start docker (or open Docker Desktop), then run make dev again." ;; \
	esac
	@$(FAIL) docker compose up -d --wait db || fail "The Postgres container did not start or did not become healthy (Docker's message is above)." \
	  "See why with: docker compose logs db. If port 5432 is already taken, stop the other Postgres first."

db-down:        ## Stop Postgres (data is kept in the pgdata volume)
	docker compose down

run:            ## Run the backend (applies migrations on startup)
	cd backend && go run ./cmd/server

web-install:    ## Install the frontend's npm packages
	cd frontend && npm install

web:            ## Run the frontend on http://localhost:5173 (proxies /api and /auth to the backend)
	cd frontend && npm run dev

web-build: frontend/node_modules/.package-lock.json ## Build the frontend into backend/internal/webui/dist, where the backend embeds it
	cd frontend && npm run build

build: web-build ## Build one binary, backend/bin/server, that serves the API and the frontend on http://localhost:8080
	cd backend && go build -o bin/server ./cmd/server

serve: build ## Build the one binary and run it: the whole app on http://localhost:8080, no Vite
	@$(if $(USES_POSTGRES),$(MAKE) --no-print-directory db-up,true)
	cd backend && PUBLIC_URL=http://localhost:$(or $(PORT),8080) ./bin/server

pg-to-sqlite: db-up ## Copy the Postgres in DATABASE_URL into a new SQLite file: the app's default one, or SQLITE=path
	cd backend && go run ./cmd/pg2sqlite $(if $(SQLITE),-to $(SQLITE))

test:           ## Run the backend tests
	cd backend && go test ./...

test-db:        ## Run the backend tests, including the ones that need Postgres
	psql "$(DATABASE_URL)" -qc "DROP DATABASE IF EXISTS steamlib_test" -c "CREATE DATABASE steamlib_test"
	cd backend && TEST_DATABASE_URL="$(subst /steamlib?,/steamlib_test?,$(DATABASE_URL))" go test -p 1 ./...

generate:       ## Regenerate Go code from backend/db/postgres/queries with sqlc
	cd backend && $(SQLC) generate

migrate-up:     ## Apply all migrations
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" up

migrate-down:   ## Roll back the last migration
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" down

migrate-status: ## Show which migrations are applied
	$(GOOSE) -dir $(MIGRATIONS) postgres "$(DATABASE_URL)" status
