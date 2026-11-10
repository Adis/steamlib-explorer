-- +goose Up

-- People who signed in with Steam. Each user sees only their own library.
CREATE TABLE users (
    steam_id       BIGINT      PRIMARY KEY,          -- 64-bit SteamID from Steam sign-in
    persona_name   TEXT,
    avatar_url     TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Login sessions. Only a hash of the cookie value is stored.
CREATE TABLE sessions (
    token_hash  BYTEA       PRIMARY KEY,             -- SHA-256 of the session cookie
    steam_id    BIGINT      NOT NULL REFERENCES users (steam_id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessions_steam_id ON sessions (steam_id);

-- Store data for a Steam app, shared by all users.
CREATE TABLE apps (
    appid               BIGINT      PRIMARY KEY,
    name                TEXT        NOT NULL,
    type                TEXT,                         -- game, dlc, demo, ...
    short_description   TEXT,
    header_image        TEXT,
    release_date        DATE,                         -- parsed; NULL when Steam's text can't be parsed
    release_date_text   TEXT,                         -- as Steam shows it, e.g. "Q3 2027"
    coming_soon         BOOLEAN     NOT NULL DEFAULT FALSE,
    is_free             BOOLEAN     NOT NULL DEFAULT FALSE,
    developers          TEXT[]      NOT NULL DEFAULT '{}',
    publishers          TEXT[]      NOT NULL DEFAULT '{}',
    genres              TEXT[]      NOT NULL DEFAULT '{}',
    categories          TEXT[]      NOT NULL DEFAULT '{}',
    platform_windows    BOOLEAN     NOT NULL DEFAULT FALSE,
    platform_mac        BOOLEAN     NOT NULL DEFAULT FALSE,
    platform_linux      BOOLEAN     NOT NULL DEFAULT FALSE,
    metacritic_score    INT,
    price_cents         INT,
    price_currency      TEXT,
    -- From store.steampowered.com/appreviews
    review_score        INT,                          -- 1..9
    review_score_desc   TEXT,                         -- e.g. "Very Positive"
    reviews_positive    INT,
    reviews_total       INT,
    review_pct          NUMERIC(5,2) GENERATED ALWAYS AS (
        CASE WHEN reviews_total > 0
             THEN round(reviews_positive * 100.0 / reviews_total, 2)
        END
    ) STORED,
    -- Raw appdetails payload, so new fields can be backfilled without refetching
    details_raw         JSONB,
    -- Sync bookkeeping
    details_status      TEXT        NOT NULL DEFAULT 'pending'
                        CHECK (details_status IN ('pending', 'ok', 'unavailable', 'error')),
    details_fetched_at  TIMESTAMPTZ,
    reviews_fetched_at  TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX apps_genres_gin     ON apps USING GIN (genres);
CREATE INDEX apps_categories_gin ON apps USING GIN (categories);
CREATE INDEX apps_name_lower     ON apps (lower(name));

-- A user's library: what they own and how much they played it.
CREATE TABLE owned_games (
    steam_id          BIGINT      NOT NULL REFERENCES users (steam_id) ON DELETE CASCADE,
    appid             BIGINT      NOT NULL REFERENCES apps (appid) ON DELETE CASCADE,
    playtime_forever  INT         NOT NULL DEFAULT 0,   -- minutes
    playtime_2weeks   INT         NOT NULL DEFAULT 0,   -- minutes
    last_played_at    TIMESTAMPTZ,
    synced_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (steam_id, appid)
);

CREATE INDEX owned_games_appid ON owned_games (appid);

-- One row per Refresh, for the UI status and debugging.
CREATE TABLE sync_runs (
    id               BIGSERIAL   PRIMARY KEY,
    steam_id         BIGINT      NOT NULL REFERENCES users (steam_id) ON DELETE CASCADE,
    started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at      TIMESTAMPTZ,
    status           TEXT        NOT NULL DEFAULT 'running'
                     CHECK (status IN ('running', 'ok', 'error')),
    games_seen       INT         NOT NULL DEFAULT 0,
    details_fetched  INT         NOT NULL DEFAULT 0,
    error            TEXT
);

CREATE INDEX sync_runs_steam_id_started ON sync_runs (steam_id, started_at DESC);

-- +goose Down
DROP TABLE sync_runs;
DROP TABLE owned_games;
DROP TABLE apps;
DROP TABLE sessions;
DROP TABLE users;
