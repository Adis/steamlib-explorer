-- name: EnsureApp :exec
-- Creates a placeholder app from the library listing; store details fill it in later.
INSERT INTO apps (appid, name)
VALUES ($1, $2)
ON CONFLICT (appid) DO NOTHING;

-- name: UpsertOwnedGame :exec
INSERT INTO owned_games (steam_id, appid, playtime_forever, playtime_2weeks, last_played_at, synced_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (steam_id, appid) DO UPDATE SET
    playtime_forever = EXCLUDED.playtime_forever,
    playtime_2weeks  = EXCLUDED.playtime_2weeks,
    last_played_at   = EXCLUDED.last_played_at,
    synced_at        = now();

-- name: AppsNeedingDetails :many
-- The user's apps whose store data is missing or older than stale_before, oldest first.
SELECT a.appid
FROM owned_games o
JOIN apps a ON a.appid = o.appid
WHERE o.steam_id = $1
  AND (a.details_fetched_at IS NULL OR a.details_fetched_at < sqlc.arg(stale_before)::timestamptz)
ORDER BY a.details_fetched_at NULLS FIRST, a.appid;

-- name: UpdateAppDetails :exec
UPDATE apps SET
    name               = sqlc.arg(name),
    type               = sqlc.arg(type),
    short_description  = sqlc.arg(short_description),
    header_image       = sqlc.arg(header_image),
    release_date       = sqlc.arg(release_date),
    release_date_text  = sqlc.arg(release_date_text),
    coming_soon        = sqlc.arg(coming_soon),
    is_free            = sqlc.arg(is_free),
    developers         = sqlc.arg(developers),
    publishers         = sqlc.arg(publishers),
    genres             = sqlc.arg(genres),
    categories         = sqlc.arg(categories),
    platform_windows   = sqlc.arg(platform_windows),
    platform_mac       = sqlc.arg(platform_mac),
    platform_linux     = sqlc.arg(platform_linux),
    metacritic_score   = sqlc.arg(metacritic_score),
    price_cents        = sqlc.arg(price_cents),
    price_currency     = sqlc.arg(price_currency),
    details_raw        = sqlc.arg(details_raw),
    details_status     = 'ok',
    details_fetched_at = now(),
    updated_at         = now()
WHERE appid = sqlc.arg(appid);

-- name: MarkAppDetailsStatus :exec
-- Records 'unavailable' or 'error' so the app isn't retried until it goes stale.
UPDATE apps SET details_status = $2, details_fetched_at = now(), updated_at = now()
WHERE appid = $1;

-- name: UpdateAppReviews :exec
UPDATE apps SET
    review_score       = $2,
    review_score_desc  = $3,
    reviews_positive   = $4,
    reviews_total      = $5,
    reviews_fetched_at = now(),
    updated_at         = now()
WHERE appid = $1;

-- name: StartSyncRun :one
INSERT INTO sync_runs (steam_id) VALUES ($1) RETURNING id;

-- name: UpdateSyncProgress :exec
UPDATE sync_runs SET games_seen = $2, details_total = $3, details_fetched = $4
WHERE id = $1;

-- name: FinishSyncRun :exec
UPDATE sync_runs SET finished_at = now(), status = $2, error = $3
WHERE id = $1;

-- name: FailRunningSyncs :execrows
-- On startup: runs left 'running' by a crash or restart can never finish.
UPDATE sync_runs SET finished_at = now(), status = 'error', error = 'interrupted by a server restart'
WHERE status = 'running';

-- name: LatestSyncRun :one
SELECT * FROM sync_runs WHERE steam_id = $1 ORDER BY started_at DESC LIMIT 1;

-- name: LibraryStats :one
SELECT
    count(*)::bigint                                                  AS games,
    count(*) FILTER (WHERE a.details_status = 'ok')::bigint           AS with_details,
    count(*) FILTER (WHERE a.details_status = 'unavailable')::bigint  AS unavailable,
    count(*) FILTER (WHERE a.details_status = 'error')::bigint        AS failed,
    count(*) FILTER (WHERE a.reviews_fetched_at IS NOT NULL)::bigint  AS with_reviews
FROM owned_games o
JOIN apps a ON a.appid = o.appid
WHERE o.steam_id = $1;
