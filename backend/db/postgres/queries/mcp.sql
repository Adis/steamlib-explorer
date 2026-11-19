-- name: SaveMCPToken :exec
-- Makes the user's MCP token, replacing the one they had.
INSERT INTO mcp_tokens (steam_id, token_hash)
VALUES ($1, $2)
ON CONFLICT (steam_id) DO UPDATE SET token_hash = excluded.token_hash, created_at = now();

-- name: GetMCPToken :one
SELECT created_at FROM mcp_tokens WHERE steam_id = $1;

-- name: GetMCPTokenUser :one
-- The user an MCP token belongs to.
SELECT u.*
FROM mcp_tokens t
JOIN users u ON u.steam_id = t.steam_id
WHERE t.token_hash = $1;

-- name: DeleteMCPToken :exec
DELETE FROM mcp_tokens WHERE steam_id = $1;
