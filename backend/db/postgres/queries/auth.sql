-- name: UpsertUser :one
-- Called on every sign-in: creates the user or refreshes their profile.
INSERT INTO users (steam_id, persona_name, avatar_url, last_login_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (steam_id) DO UPDATE SET
    persona_name  = EXCLUDED.persona_name,
    avatar_url    = EXCLUDED.avatar_url,
    last_login_at = now()
RETURNING *;

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, steam_id, expires_at)
VALUES ($1, $2, $3);

-- name: GetSessionUser :one
-- The user behind a session cookie, if the session hasn't expired.
SELECT u.*
FROM sessions s
JOIN users u ON u.steam_id = s.steam_id
WHERE s.token_hash = $1 AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;
