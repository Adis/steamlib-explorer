-- name: EnsureApp :exec
-- Creates a placeholder app from the library listing; store details fill it in later.
INSERT INTO apps (appid, name)
VALUES (?, ?)
ON CONFLICT (appid) DO NOTHING;

-- name: UpsertOwnedGame :exec
INSERT INTO owned_games (steam_id, appid, playtime_forever, playtime_2weeks, last_played_at, synced_at)
VALUES (?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
ON CONFLICT (steam_id, appid) DO UPDATE SET
    playtime_forever = excluded.playtime_forever,
    playtime_2weeks  = excluded.playtime_2weeks,
    last_played_at   = excluded.last_played_at,
    synced_at        = excluded.synced_at;

-- name: AppsNeedingDetails :many
-- The user's apps whose store data is missing or older than the given time, oldest first.
SELECT a.appid
FROM owned_games o
JOIN apps a ON a.appid = o.appid
WHERE o.steam_id = ?
  AND (a.details_fetched_at IS NULL OR a.details_fetched_at < ?)
ORDER BY a.details_fetched_at NULLS FIRST, a.appid;

-- name: AppsNeedingReviews :many
-- The user's store apps whose reviews are missing or older than the given time, oldest first.
SELECT a.appid
FROM owned_games o
JOIN apps a ON a.appid = o.appid
WHERE o.steam_id = ?
  AND a.details_status = 'ok'
  AND (a.reviews_fetched_at IS NULL OR a.reviews_fetched_at < ?)
ORDER BY a.reviews_fetched_at NULLS FIRST, a.appid;

-- name: UpdateAppDetails :exec
UPDATE apps SET
    name               = ?,
    type               = ?,
    short_description  = ?,
    header_image       = ?,
    release_date       = ?,
    release_date_text  = ?,
    coming_soon        = ?,
    is_free            = ?,
    developers         = ?,
    publishers         = ?,
    genres             = ?,
    categories         = ?,
    platform_windows   = ?,
    platform_mac       = ?,
    platform_linux     = ?,
    metacritic_score   = ?,
    price_cents        = ?,
    price_currency     = ?,
    details_raw        = ?,
    details_status     = 'ok',
    details_fetched_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    updated_at         = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE appid = ?;

-- name: MarkAppDetailsStatus :exec
-- Records 'unavailable' or 'error' so the app isn't retried until it goes stale.
UPDATE apps SET details_status = ?,
    details_fetched_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE appid = ?;

-- name: UpdateAppReviews :exec
UPDATE apps SET
    review_score       = ?,
    review_score_desc  = ?,
    reviews_positive   = ?,
    reviews_total      = ?,
    reviews_fetched_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    updated_at         = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE appid = ?;

-- name: StartSyncRun :one
INSERT INTO sync_runs (steam_id) VALUES (?) RETURNING id;

-- name: UpdateSyncProgress :exec
-- The first progress save marks when fetching store details began.
UPDATE sync_runs SET games_seen = ?, details_total = ?, details_fetched = ?,
    details_started_at = COALESCE(details_started_at, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
WHERE id = ?;

-- name: FinishSyncRun :exec
UPDATE sync_runs SET finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), status = ?, error = ?
WHERE id = ?;

-- name: FailRunningSyncs :execrows
-- On startup: runs left 'running' by a crash or restart can never finish.
UPDATE sync_runs SET finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), status = 'error',
    error = 'interrupted by a server restart'
WHERE status = 'running';

-- name: LatestSyncRun :one
SELECT * FROM sync_runs WHERE steam_id = ? ORDER BY started_at DESC, id DESC LIMIT 1;

-- name: LibraryStats :one
SELECT
    count(*)                                                  AS games,
    count(*) FILTER (WHERE a.details_status = 'ok')           AS with_details,
    count(*) FILTER (WHERE a.details_status = 'unavailable')  AS unavailable,
    count(*) FILTER (WHERE a.details_status = 'error')        AS failed,
    count(*) FILTER (WHERE a.reviews_fetched_at IS NOT NULL)  AS with_reviews,
    count(*) FILTER (WHERE o.playtime_forever > 0)            AS played,
    CAST(coalesce(sum(o.playtime_forever), 0) AS INTEGER)     AS playtime_minutes,
    CAST(coalesce(sum(o.playtime_2weeks), 0) AS INTEGER)      AS playtime_2weeks_minutes
FROM owned_games o
JOIN apps a ON a.appid = o.appid
WHERE o.steam_id = ?;
