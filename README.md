# steamlib-explorer
Personal, self-hosted repository of Steam games allowing you to browse and filter your Steam library looking for a new game to play.

## Development

Requirements: Docker, Go 1.25+ and Node.js 20.19+.

```sh
cp .env.example .env  # then set STEAM_API_KEY
make db-up            # start Postgres 17 on localhost:5432
make web-install      # once: install the frontend's packages

make run              # terminal 1: backend on :8080, applies migrations first
make web              # terminal 2: frontend on http://localhost:5173
```

Open http://localhost:5173, sign in through Steam and press **Sync library**. The page shows the sync's progress and how many games have store details and reviews.

In development the browser only talks to Vite on port 5173, which forwards `/api` and `/auth` to the backend, so `PUBLIC_URL` must be `http://localhost:5173` (as in `.env.example`).

### Logs

Every incoming request (`msg="api request"`) and every call to Steam (`msg="outgoing call"`, with status, duration and the first 300 characters of the response) is logged to the backend console and appended to `backend/logs/api-calls.txt`. The Steam API key is replaced with `REDACTED`. Sync progress lines (`refresh started`, `library saved`, `refresh finished`) carry the `run_id`.

### Settings

| Variable | Required | Default | What it is |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | | Postgres connection string |
| `STEAM_API_KEY` | yes | | [Steam Web API key](https://steamcommunity.com/dev/apikey), used to read libraries and profiles |
| `PUBLIC_URL` | no | `http://localhost:$PORT` | Address the browser uses for the app; Steam redirects back here |
| `PORT` | no | `8080` | Port the backend listens on |
| `LOG_FILE` | no | `logs/api-calls.txt` | File that also receives every log line, relative to `backend/`; `-` logs to the console only |

Each user's Steam profile and game details must be public for their library to load.

`make test` runs the backend tests, `make test-db` also runs the ones that need Postgres, `make generate` rebuilds the sqlc code after you edit `backend/db/queries`, and `make build` puts a binary in `backend/bin/server`. The `migrate-*` targets are still there for rolling back or checking status by hand.

`make migrate-down` rolls back the last migration and `make db-down` stops Postgres (data stays in the `pgdata` volume).

## Database

The schema lives in [`backend/db/migrations`](backend/db/migrations) and is applied with [goose](https://github.com/pressly/goose). The migrations are embedded in the backend binary and run every time it starts.

| Table | One row per | Holds |
| --- | --- | --- |
| `users` | person who signed in with Steam | SteamID, name, avatar |
| `sessions` | login session | hash of the session cookie, expiry |
| `apps` | Steam app | store details (genres, categories, platforms, price, Metacritic) and review counts, shared by all users |
| `owned_games` | user + app | playtime and last played date |
| `sync_runs` | sync | status, progress and error of each library sync (one running per user) |

Genres and categories are `text[]` columns with GIN indexes, so filtering by "has all of these genres" is a single indexed check. `apps.review_pct` is computed by Postgres from the positive and total review counts.

## API

| Method and path | What it does |
| --- | --- |
| `GET /healthz` | 200 when the database answers |
| `GET /auth/steam/login` | Redirects to Steam's sign-in page |
| `GET /auth/steam/callback` | Steam sends the browser here; verifies the login, creates the session cookie, redirects to `PUBLIC_URL` |
| `POST /auth/logout` | Ends the session |
| `GET /api/me` | The signed-in user (`steam_id`, `persona_name`, `avatar_url`), or 401 |
| `POST /api/sync` | Starts a sync of the signed-in user's library (202), or 409 if one is running |
| `GET /api/sync` | The latest sync (status, games seen, store details fetched of total, error) and library counts |

A sync reads the library with one `GetOwnedGames` call, then fetches store details and reviews for every game whose store data is missing or older than 30 days, about 3 seconds per game (two store requests, 1.5 seconds apart). If Steam starts refusing requests, the sync stops with a message and keeps what it fetched; the next sync continues from there.
