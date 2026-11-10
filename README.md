# steamlib-explorer
Personal, self-hosted repository of Steam games allowing you to browse and filter your Steam library looking for a new game to play.

## Development

Requirements: Docker and Go 1.25+.

```sh
cp .env.example .env  # then set STEAM_API_KEY
make db-up          # start Postgres 17 on localhost:5432
make run            # start the backend on :8080, applying migrations first
curl localhost:8080/healthz
```

Open http://localhost:8080/auth/steam/login to sign in with Steam, then http://localhost:8080/api/me shows who you are.

### Settings

| Variable | Required | Default | What it is |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | | Postgres connection string |
| `STEAM_API_KEY` | yes | | [Steam Web API key](https://steamcommunity.com/dev/apikey), used to read libraries and profiles |
| `PUBLIC_URL` | no | `http://localhost:$PORT` | Address the browser uses for the app; Steam redirects back here |
| `PORT` | no | `8080` | Port the backend listens on |

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
| `sync_runs` | Refresh | status and counts of each library refresh |

Genres and categories are `text[]` columns with GIN indexes, so filtering by "has all of these genres" is a single indexed check. `apps.review_pct` is computed by Postgres from the positive and total review counts.

## API

| Method and path | What it does |
| --- | --- |
| `GET /healthz` | 200 when the database answers |
| `GET /auth/steam/login` | Redirects to Steam's sign-in page |
| `GET /auth/steam/callback` | Steam sends the browser here; verifies the login, creates the session cookie, redirects to `PUBLIC_URL` |
| `POST /auth/logout` | Ends the session |
| `GET /api/me` | The signed-in user (`steam_id`, `persona_name`, `avatar_url`), or 401 |
