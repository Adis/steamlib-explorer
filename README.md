# steamlib-explorer
Personal, self-hosted repository of Steam games allowing you to browse and filter your Steam library looking for a new game to play.

## Development

Requirements: Docker and Go 1.24+ (Go is used to run the migration tool).

```sh
cp .env.example .env
make db-up          # start Postgres 17 on localhost:5432
make migrate-up     # create the tables
make migrate-status # check what is applied
```

`make migrate-down` rolls back the last migration and `make db-down` stops Postgres (data stays in the `pgdata` volume).

## Database

The schema lives in [`backend/db/migrations`](backend/db/migrations) and is applied with [goose](https://github.com/pressly/goose).

| Table | One row per | Holds |
| --- | --- | --- |
| `users` | person who signed in with Steam | SteamID, name, avatar |
| `sessions` | login session | hash of the session cookie, expiry |
| `apps` | Steam app | store details (genres, categories, platforms, price, Metacritic) and review counts, shared by all users |
| `owned_games` | user + app | playtime and last played date |
| `sync_runs` | Refresh | status and counts of each library refresh |

Genres and categories are `text[]` columns with GIN indexes, so filtering by "has all of these genres" is a single indexed check. `apps.review_pct` is computed by Postgres from the positive and total review counts.
