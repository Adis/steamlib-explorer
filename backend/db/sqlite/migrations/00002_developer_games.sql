-- +goose Up
-- Matches Postgres migration 00004: the Steam store's list of a developer's
-- games, kept a day so the store is asked at most once a day per developer,
-- also across restarts. Only a cache: rows can go at any time.
CREATE TABLE developer_games (
    developer   TEXT PRIMARY KEY,  -- lowercased developer name
    games       TEXT NOT NULL,     -- JSON: the store's rows, as the app read them
    fetched_at  TEXT NOT NULL      -- UTC 'YYYY-MM-DDTHH:MM:SS.SSSZ'
);

-- +goose Down
DROP TABLE developer_games;
