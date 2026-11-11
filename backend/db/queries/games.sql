-- name: ListGames :many
-- One page of the user's games with their store data. Every filter is
-- optional: a NULL argument turns it off.
SELECT
    a.appid, a.name, a.header_image, a.short_description,
    a.release_date, a.release_date_text, a.coming_soon,
    a.genres, a.categories, a.developers, a.publishers,
    a.platform_windows, a.platform_mac, a.platform_linux,
    a.metacritic_score, a.price_cents, a.price_currency, a.is_free,
    a.review_score_desc, a.review_pct, a.reviews_total, a.details_status,
    o.playtime_forever, o.playtime_2weeks, o.last_played_at,
    count(*) OVER () AS total_count
FROM owned_games o
JOIN apps a ON a.appid = o.appid
WHERE o.steam_id = sqlc.arg(steam_id)
  AND (sqlc.narg(name_pattern)::text IS NULL OR a.name ILIKE sqlc.narg(name_pattern)::text)
  AND (sqlc.narg(genres)::text[] IS NULL OR a.genres @> sqlc.narg(genres)::text[])
  AND (sqlc.narg(categories)::text[] IS NULL OR a.categories @> sqlc.narg(categories)::text[])
  AND (sqlc.narg(played)::boolean IS NULL OR (o.playtime_forever > 0) = sqlc.narg(played)::boolean)
  AND (sqlc.narg(min_minutes)::int IS NULL OR o.playtime_forever >= sqlc.narg(min_minutes)::int)
  AND (sqlc.narg(max_minutes)::int IS NULL OR o.playtime_forever <= sqlc.narg(max_minutes)::int)
  AND (sqlc.narg(min_review_pct)::numeric IS NULL OR a.review_pct >= sqlc.narg(min_review_pct)::numeric)
  AND (sqlc.narg(min_metacritic)::int IS NULL OR a.metacritic_score >= sqlc.narg(min_metacritic)::int)
  AND (sqlc.narg(released_after)::date IS NULL OR a.release_date >= sqlc.narg(released_after)::date)
  AND (sqlc.narg(released_before)::date IS NULL OR a.release_date <= sqlc.narg(released_before)::date)
  AND (NOT sqlc.arg(windows_only)::boolean OR a.platform_windows)
  AND (NOT sqlc.arg(mac_only)::boolean OR a.platform_mac)
  AND (NOT sqlc.arg(linux_only)::boolean OR a.platform_linux)
ORDER BY
    -- Only the CASE matching sort/descending yields a value; the others are NULL for every row.
    CASE WHEN sqlc.arg(sort)::text = 'name'        AND NOT sqlc.arg(descending)::boolean THEN lower(a.name) END ASC,
    CASE WHEN sqlc.arg(sort)::text = 'name'        AND sqlc.arg(descending)::boolean     THEN lower(a.name) END DESC,
    CASE WHEN sqlc.arg(sort)::text = 'playtime'    AND NOT sqlc.arg(descending)::boolean THEN o.playtime_forever END ASC,
    CASE WHEN sqlc.arg(sort)::text = 'playtime'    AND sqlc.arg(descending)::boolean     THEN o.playtime_forever END DESC,
    CASE WHEN sqlc.arg(sort)::text = 'recent'      AND NOT sqlc.arg(descending)::boolean THEN o.playtime_2weeks END ASC,
    CASE WHEN sqlc.arg(sort)::text = 'recent'      AND sqlc.arg(descending)::boolean     THEN o.playtime_2weeks END DESC,
    CASE WHEN sqlc.arg(sort)::text = 'last_played' AND NOT sqlc.arg(descending)::boolean THEN o.last_played_at END ASC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'last_played' AND sqlc.arg(descending)::boolean     THEN o.last_played_at END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'review'      AND NOT sqlc.arg(descending)::boolean THEN a.review_pct END ASC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'review'      AND sqlc.arg(descending)::boolean     THEN a.review_pct END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'metacritic'  AND NOT sqlc.arg(descending)::boolean THEN a.metacritic_score END ASC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'metacritic'  AND sqlc.arg(descending)::boolean     THEN a.metacritic_score END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'release'     AND NOT sqlc.arg(descending)::boolean THEN a.release_date END ASC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'release'     AND sqlc.arg(descending)::boolean     THEN a.release_date END DESC NULLS LAST,
    lower(a.name), a.appid
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: GenreFacets :many
-- Every genre in the user's library with how many of their games have it.
SELECT g::text AS value, count(*)::bigint AS games
FROM owned_games o
JOIN apps a ON a.appid = o.appid
CROSS JOIN LATERAL unnest(a.genres) AS g
WHERE o.steam_id = $1
GROUP BY g
ORDER BY games DESC, g;

-- name: CategoryFacets :many
-- Every store category (Single-player, Co-op, ...) in the user's library with counts.
SELECT c::text AS value, count(*)::bigint AS games
FROM owned_games o
JOIN apps a ON a.appid = o.appid
CROSS JOIN LATERAL unnest(a.categories) AS c
WHERE o.steam_id = $1
GROUP BY c
ORDER BY games DESC, c;
