-- +goose Up
-- The Steam store's list of a developer's games ("More from <developer>" on
-- a game's page), kept a day so the store is asked at most once a day per
-- developer, also across restarts. Only a cache: rows can go at any time.
CREATE TABLE developer_games (
    developer   TEXT        PRIMARY KEY,  -- lowercased developer name
    games       JSONB       NOT NULL,     -- the store's rows, as the app read them
    fetched_at  TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE developer_games;
