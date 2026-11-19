-- +goose Up
-- Access tokens for the MCP server (/mcp), which lets an AI assistant such as
-- Claude read a user's library. One per user, made on the Settings page; only
-- a hash is stored, so the token is shown once.
CREATE TABLE mcp_tokens (
    steam_id    BIGINT      PRIMARY KEY REFERENCES users (steam_id) ON DELETE CASCADE,
    token_hash  BYTEA       NOT NULL UNIQUE,  -- SHA-256 of the token
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE mcp_tokens;
