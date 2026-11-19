-- +goose Up
-- Matches Postgres migration 00005: access tokens for the MCP server (/mcp),
-- one per user. Only a hash is stored, so the token is shown once.
CREATE TABLE mcp_tokens (
    steam_id    INTEGER PRIMARY KEY REFERENCES users (steam_id) ON DELETE CASCADE,
    token_hash  BLOB    NOT NULL UNIQUE,                   -- SHA-256 of the token
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- +goose Down
DROP TABLE mcp_tokens;
