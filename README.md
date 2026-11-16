# steamlib-explorer
Personal, self-hosted repository of Steam games allowing you to browse and filter your Steam library looking for a new game to play.

## Development

### Requirements

[![Docker](https://img.shields.io/badge/Docker-with_Compose-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/get-started/get-docker/)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![Node.js](https://img.shields.io/badge/Node.js-20.19%2B-5FA04E?logo=nodedotjs&logoColor=white)](https://nodejs.org/en/download)
[![GNU Make](https://img.shields.io/badge/GNU_Make-any-6D00CC?logo=gnu&logoColor=white)](https://www.gnu.org/software/make/)

- [Docker](https://docs.docker.com/get-started/get-docker/) with Compose, to run Postgres 17 (the engine must be running before `make dev`)
- [Go](https://go.dev/dl/) 1.25 or newer, for the backend
- [Node.js](https://nodejs.org/en/download) 20.19 or newer (with npm), for the frontend
- [GNU Make](https://www.gnu.org/software/make/), to run the commands below
- A [Steam Web API key](https://steamcommunity.com/dev/apikey)

### First run

Copy the example settings once, then set `STEAM_API_KEY` in `.env` (or leave it empty and paste the key into the setup page the app shows on first start):

```sh
cp .env.example .env
```

Start everything (Ctrl+C stops the backend and frontend):

```sh
make dev
```

`make dev` starts Postgres 17 on localhost:5432, installs the frontend's packages when `package.json` or `package-lock.json` changed, then runs the backend on localhost:8080 (it applies new migrations first) and the frontend on http://localhost:5173 in one terminal. After a `git pull`, `make dev` is all you need. To run the pieces in separate terminals instead, use `make db-up`, `make web-install`, `make run` and `make web`.

Open http://localhost:5173, sign in through Steam and press **Sync library**. Games appear in the table as soon as the library is read; store details and reviews fill in as the sync goes (reload or wait for it to finish). Filter with the panel on the left and click a column heading to sort, or click it again to reverse. Filters are kept in the address bar, so a filtered view can be bookmarked. **Details** next to the library count shows how many games have store details and reviews.

In development the browser only talks to Vite on port 5173, which forwards `/api` and `/auth` to the backend, so `PUBLIC_URL` must be `http://localhost:5173` (as in `.env.example`).

### One binary

`make build` builds the frontend into `backend/internal/webui/dist` and then the backend, which embeds it, so `backend/bin/server` alone serves the whole app on http://localhost:8080 (the API and the pages, no Vite or Node needed). `make serve` builds it and runs it against the Docker Postgres with `PUBLIC_URL=http://localhost:8080`, whatever `.env` says. This is the first step towards a desktop app; it still needs Postgres for now. A binary built without the frontend (plain `go build`) still runs and shows a page saying how to build it.

### Remote access

By default the app works only on the computer that runs it: Postgres, the backend and the frontend all listen on localhost, so other devices on your network can't reach them, whatever your firewall says. To open the app to your phone or another computer on your network:

1. In `.env`, set `LISTEN_HOST=0.0.0.0` (the backend and the Vite frontend both use it).
2. Set `PUBLIC_URL` to the address the other device will type, for example `http://192.168.1.20:5173` or `http://my-pc.lan:5173`. Steam sends you back there after sign-in, so you must open the app at exactly that address (also on this computer).
3. Allow port 5173 in your firewall, for example `sudo ufw allow from 192.168.1.0/24 to any port 5173 proto tcp`.
4. Run `make dev` again.

Only port 5173 needs to be reachable: Vite forwards to the backend, which talks to Postgres on localhost. Keep Postgres on `127.0.0.1` in `docker-compose.yml`. Docker writes its own firewall rules ahead of ufw's, so a published container port is open to the network even when ufw says deny. The development servers have no HTTPS, so only open them on a network you trust, never to the internet.

### SQLite

The backend can also keep everything in one SQLite file instead of Postgres, so it runs without Docker. This is what the desktop app will use. When `DATABASE_URL` is not set, the backend uses `steamlib.db` in your data folder, created on first start:

| OS | Data folder |
| --- | --- |
| Linux | `$XDG_DATA_HOME/steamlib-explorer`, usually `~/.local/share/steamlib-explorer` |
| Windows | `%AppData%\steamlib-explorer` |
| macOS | `~/Library/Application Support/steamlib-explorer` |

`STEAMLIB_DATA_DIR` picks another folder, and `DATABASE_URL=sqlite:some/file.db` another file. The startup log says which database is in use. With SQLite, `make dev` and `make serve` skip Docker. A new SQLite file starts empty: sign in and sync, or copy your Postgres data over first (below).

To move a library you already synced on Postgres, run this once with `DATABASE_URL` still pointing at Postgres:

```sh
make pg-to-sqlite
```

It starts the Docker Postgres if needed and copies every user, game, library entry, past sync and sign-in session into a new `steamlib.db` in your data folder (`make pg-to-sqlite SQLITE=other.db` writes another file instead, relative to `backend/`). Then remove or comment out `DATABASE_URL` in `.env` and start the app: you stay signed in and nothing needs a re-sync. Postgres is only read, so you can switch back by restoring the old `DATABASE_URL`. The copy refuses to write into a file that already exists, and if anything goes wrong it removes the half-made file. Times are kept to the millisecond (Postgres keeps microseconds).

SQLite runs in WAL mode, so the explorer can read while a sync writes. Back up the `.db` file together with its `-wal` and `-shm` files, or stop the app first.

### First start without any settings

Started with no `.env` at all, the backend uses the SQLite file in your data folder and shows a one-time setup page asking for your Steam Web API key. It checks the key with Steam and saves it in `settings.json` next to the database (readable only by you). The page only works while no key is set, and only accepts JSON, so another website can't set a key for you; to change the key later, edit or delete `settings.json`, or set `STEAM_API_KEY`. `backend/bin/server -open` also opens the app in your default browser once it is listening.

### Desktop app

`make desktop` builds `backend/bin/steamlib-explorer`, the desktop version: one file that needs no Docker, Node or settings. Starting it runs the app on http://localhost:8080, opens it in your default browser and puts an icon in the system tray with **Open Steamlib Explorer** and **Quit**. It uses the SQLite file and `settings.json` in your data folder (see [SQLite](#sqlite)) and writes its log to `logs/steamlib-explorer.log` there; if it can't start, the reason is in `startup-error.txt` in the same folder. Starting it a second time just opens the browser on the one already running.

`make desktop-windows` cross-builds `steamlib-explorer.exe` for Windows from Linux; it runs without a console window. On Linux the tray icon needs a desktop with a system tray (KDE, Xfce, Cinnamon, or GNOME with the AppIndicator extension, which Ubuntu ships); without a desktop session it runs without the icon and stops with Ctrl+C.

### Installers and releases

The **Release** workflow (`.github/workflows/release.yml`) builds the desktop app with [GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`) and makes:

- Windows: `steamlib-explorer_<version>_windows_setup.exe`, an installer made with Inno Setup (`packaging/windows`). It installs for the current user only, so it needs no administrator rights, adds a Start menu entry (a desktop icon is optional) and an uninstaller, which keeps your library and settings. A `.zip` with just the `.exe` is there too.
- Linux: `.deb` (Debian, Ubuntu, Mint), `.rpm` (Fedora, openSUSE) and `.pkg.tar.zst` (Arch, `sudo pacman -U <file>`) packages for x86-64 and ARM64, which add **Steamlib Explorer** to the applications menu, plus a `.tar.gz` with just the program.

To release, tag a commit and push the tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The workflow then makes a **draft** release with all the downloads; check it on GitHub and press **Publish release**. Pull requests that change the packaging, and a manual run from the Actions tab, build everything the same way as a test and keep the files as workflow artifacts instead of releasing them. To try it locally, `goreleaser release --snapshot --clean` writes the Linux and Windows files to `dist/` (the Windows installer needs Windows). The builds aren't code signed, so Windows SmartScreen warns on first start: **More info**, then **Run anyway**.

### Logs

Every incoming request (`msg="api request"`) and every call to Steam (`msg="outgoing call"`, with status, duration and the first 300 characters of the response) is logged to the backend console and appended to `backend/logs/api-calls.txt`. The Steam API key is replaced with `REDACTED`. Sync progress lines (`refresh started`, `library saved`, `refresh finished`) carry the `run_id`.

### Settings

| Variable | Required | Default | What it is |
| --- | --- | --- | --- |
| `DATABASE_URL` | no | `steamlib.db` in your data folder | `postgres://…` for Postgres, or `sqlite:` and a file path for SQLite, e.g. `sqlite:steamlib.db` (relative to `backend/`) or `sqlite:/home/me/steamlib.db`; see [SQLite](#sqlite) |
| `STEAMLIB_DATA_DIR` | no | see [SQLite](#sqlite) | Folder for the default SQLite file |
| `STEAM_API_KEY` | no | the key saved by the setup page | [Steam Web API key](https://steamcommunity.com/dev/apikey), used to read libraries and profiles. Without one, the app shows a setup page that checks the key with Steam and saves it in `settings.json` in the data folder; this variable wins over the saved key |
| `PUBLIC_URL` | no | `http://localhost:$PORT` | Address the browser uses for the app; Steam redirects back here |
| `PORT` | no | `8080` | Port the backend listens on |
| `LISTEN_HOST` | no | `127.0.0.1` (backend), `localhost` (frontend) | Interface the backend and the Vite frontend listen on; `0.0.0.0` allows other devices (see [Remote access](#remote-access)) |
| `LOG_FILE` | no | `logs/api-calls.txt` | File that also receives every log line, relative to `backend/`; `-` logs to the console only |

Steam shares a user's owned games and playtime only when their profile's Game details are public, or when the request uses that user's own API key. So the owner of `STEAM_API_KEY` can keep their profile private (even Friends only or Private); everyone else who signs in needs public game details. The in-app **Help** page explains both options.

`make test` runs the backend tests (the database ones on SQLite), `make test-db` also runs them on Postgres, `make generate` rebuilds the sqlc code after you edit `backend/db/postgres/queries` or `backend/db/sqlite/queries`, and `make build` puts the one-binary app in `backend/bin/server` (see [One binary](#one-binary)). The `migrate-*` targets (Postgres only) are still there for rolling back or checking status by hand.

`make migrate-down` rolls back the last migration and `make db-down` stops Postgres (data stays in the `pgdata` volume).

## Database

The schema lives in [`backend/db/postgres/migrations`](backend/db/postgres/migrations) for Postgres and [`backend/db/sqlite/migrations`](backend/db/sqlite/migrations) for SQLite, and is applied with [goose](https://github.com/pressly/goose). Both describe the same tables; every schema change adds a new migration to each folder. The migrations are embedded in the backend binary and run every time it starts. The rest of the backend talks to the database only through the `Store` interface in [`backend/internal/store`](backend/internal/store); the Postgres implementation (sqlc queries plus a thin adapter) is in `internal/store/postgres`, the SQLite one in `internal/store/sqlite` (sqlc for the simple queries, the explorer's filter query built in Go), and `internal/store/storetest` holds tests every implementation must pass.

| Table | One row per | Holds |
| --- | --- | --- |
| `users` | person who signed in with Steam | SteamID, name, avatar |
| `sessions` | login session | hash of the session cookie, expiry |
| `apps` | Steam app | store details (genres, categories, platforms, price, Metacritic) and review counts, shared by all users |
| `owned_games` | user + app | playtime and last played date |
| `sync_runs` | sync | status, progress and error of each library sync (one running per user) |

In Postgres, genres and categories are `text[]` columns with GIN indexes, so filtering by "has all of these genres" is a single indexed check; SQLite keeps them as JSON arrays and checks them with `json_each`. `apps.review_pct` is a generated column computed by the database from the positive and total review counts.

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
| `GET /api/games` | One page of the signed-in user's games with store data, plus `total` matches (filters below) |
| `GET /api/facets` | Genres and store categories in the user's library with game counts, most common first |

`GET /api/games` takes these optional query parameters. Bad values get a 400 with an `error` message.

| Parameter | Meaning |
| --- | --- |
| `q` | Name contains this text (case-insensitive) |
| `genre`, `category` | Has this genre / store category; repeat to require all of them |
| `played` | `true` for played games, `false` for never played |
| `min_hours`, `max_hours` | Total playtime range in hours (decimals allowed) |
| `min_review` | Minimum positive review percentage, 0 to 100 |
| `min_review_score` | Minimum Steam review rating, 1 (Overwhelmingly Negative) to 9 (Overwhelmingly Positive) |
| `min_reviews` | Minimum number of Steam reviews; games whose reviews were never fetched are kept. The explorer sends 10 unless "Fewer than 10 reviews" is ticked |
| `min_metacritic` | Minimum Metacritic score, 0 to 100 |
| `released_after`, `released_before` | Release date range, `YYYY-MM-DD` (games without a parseable date are left out) |
| `platform` | `windows`, `mac` or `linux`; repeat to require several |
| `sort` | `name`, `playtime`, `recent` (last two weeks), `last_played`, `review` (default; equal ratings show the least played first), `metacritic`, `release` |
| `order` | `asc` or `desc`; defaults to A to Z for `name` and biggest/newest first for the others. Unknown values sort last |
| `limit`, `offset` | Page size (1 to 200, default 50) and start |

A sync reads the library with one `GetOwnedGames` call, then fetches store details and reviews for every game whose store data is missing or older than 30 days, about 3 seconds per game (two store requests, 1.5 seconds apart). If Steam starts refusing requests, the sync stops with a message and keeps what it fetched; the next sync continues from there.
