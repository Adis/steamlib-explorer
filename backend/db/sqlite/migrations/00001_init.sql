-- +goose Up
-- The SQLite schema matches Postgres migrations 00001 to 00003 in one step,
-- with SQLite types:
--   timestamps  TEXT, UTC, always 'YYYY-MM-DDTHH:MM:SS.SSSZ' so text order is time order
--   dates       TEXT 'YYYY-MM-DD'
--   lists       TEXT holding a JSON array of strings
--   booleans    INTEGER 0 or 1
-- Future changes get new numbered files here and in the Postgres folder.

-- People who signed in with Steam. Each user sees only their own library.
CREATE TABLE users (
    steam_id       INTEGER PRIMARY KEY,                    -- 64-bit SteamID from Steam sign-in
    persona_name   TEXT,
    avatar_url     TEXT,
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    last_login_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- Login sessions. Only a hash of the cookie value is stored.
CREATE TABLE sessions (
    token_hash  BLOB    PRIMARY KEY,                       -- SHA-256 of the session cookie
    steam_id    INTEGER NOT NULL REFERENCES users (steam_id) ON DELETE CASCADE,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    expires_at  TEXT    NOT NULL
);

CREATE INDEX sessions_steam_id ON sessions (steam_id);

-- Store data for a Steam app, shared by all users.
CREATE TABLE apps (
    appid               INTEGER PRIMARY KEY,
    name                TEXT    NOT NULL,
    type                TEXT,                              -- game, dlc, demo, ...
    short_description   TEXT,
    header_image        TEXT,
    release_date        TEXT,                              -- parsed; NULL when Steam's text can't be parsed
    release_date_text   TEXT,                              -- as Steam shows it, e.g. "Q3 2027"
    coming_soon         BOOLEAN NOT NULL DEFAULT 0,
    is_free             BOOLEAN NOT NULL DEFAULT 0,
    developers          TEXT    NOT NULL DEFAULT '[]',
    publishers          TEXT    NOT NULL DEFAULT '[]',
    genres              TEXT    NOT NULL DEFAULT '[]',
    categories          TEXT    NOT NULL DEFAULT '[]',
    platform_windows    BOOLEAN NOT NULL DEFAULT 0,
    platform_mac        BOOLEAN NOT NULL DEFAULT 0,
    platform_linux      BOOLEAN NOT NULL DEFAULT 0,
    metacritic_score    INTEGER,
    price_cents         INTEGER,
    price_currency      TEXT,
    review_score        INTEGER,                           -- 1..9
    review_score_desc   TEXT,                              -- e.g. "Very Positive"
    reviews_positive    INTEGER,
    reviews_total       INTEGER,
    review_pct          REAL GENERATED ALWAYS AS (
        CASE WHEN reviews_total > 0
             THEN round(reviews_positive * 100.0 / reviews_total, 2)
        END
    ) STORED,
    -- Raw appdetails payload (JSON), so new fields can be backfilled without refetching
    details_raw         TEXT,
    -- Sync bookkeeping
    details_status      TEXT    NOT NULL DEFAULT 'pending'
                        CHECK (details_status IN ('pending', 'ok', 'unavailable', 'error')),
    details_fetched_at  TEXT,
    reviews_fetched_at  TEXT,
    created_at          TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at          TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- A user's library: what they own and how much they played it.
CREATE TABLE owned_games (
    steam_id          INTEGER NOT NULL REFERENCES users (steam_id) ON DELETE CASCADE,
    appid             INTEGER NOT NULL REFERENCES apps (appid) ON DELETE CASCADE,
    playtime_forever  INTEGER NOT NULL DEFAULT 0,          -- minutes
    playtime_2weeks   INTEGER NOT NULL DEFAULT 0,          -- minutes
    last_played_at    TEXT,
    synced_at         TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (steam_id, appid)
);

CREATE INDEX owned_games_appid ON owned_games (appid);

-- One row per Refresh, for the UI status and debugging.
CREATE TABLE sync_runs (
    id                  INTEGER PRIMARY KEY,
    steam_id            INTEGER NOT NULL REFERENCES users (steam_id) ON DELETE CASCADE,
    started_at          TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    finished_at         TEXT,
    status              TEXT    NOT NULL DEFAULT 'running'
                        CHECK (status IN ('running', 'ok', 'error')),
    games_seen          INTEGER NOT NULL DEFAULT 0,
    details_fetched     INTEGER NOT NULL DEFAULT 0,
    error               TEXT,
    -- How many apps the Refresh plans to fetch store details for.
    details_total       INTEGER NOT NULL DEFAULT 0,
    -- When it started fetching store details, for the time-left estimate.
    details_started_at  TEXT
);

CREATE INDEX sync_runs_steam_id_started ON sync_runs (steam_id, started_at DESC);

-- At most one running Refresh per user.
CREATE UNIQUE INDEX sync_runs_one_running ON sync_runs (steam_id) WHERE status = 'running';

-- +goose Down
DROP TABLE sync_runs;
DROP TABLE owned_games;
DROP TABLE apps;
DROP TABLE sessions;
DROP TABLE users;
