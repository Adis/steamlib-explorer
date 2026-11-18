-- name: GetDeveloperGames :one
SELECT games, fetched_at FROM developer_games WHERE developer = ?;

-- name: SaveDeveloperGames :exec
INSERT INTO developer_games (developer, games, fetched_at)
VALUES (?, ?, ?)
ON CONFLICT (developer) DO UPDATE SET games = excluded.games, fetched_at = excluded.fetched_at;

-- name: DeleteDeveloperGamesBefore :exec
-- Drops lists fetched before the given time; they're too old to use.
DELETE FROM developer_games WHERE fetched_at < ?;
