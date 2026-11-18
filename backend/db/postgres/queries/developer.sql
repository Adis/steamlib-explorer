-- name: GetDeveloperGames :one
SELECT games, fetched_at FROM developer_games WHERE developer = $1;

-- name: SaveDeveloperGames :exec
INSERT INTO developer_games (developer, games, fetched_at)
VALUES ($1, $2, $3)
ON CONFLICT (developer) DO UPDATE SET games = EXCLUDED.games, fetched_at = EXCLUDED.fetched_at;

-- name: DeleteDeveloperGamesBefore :exec
-- Drops lists fetched before the given time; they're too old to use.
DELETE FROM developer_games WHERE fetched_at < $1;
