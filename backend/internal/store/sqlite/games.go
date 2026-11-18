package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

// The explorer's queries are built here rather than by sqlc, whose SQLite
// parser can't handle the optional filters, json_each or the sort switch.
// They return the same rows as db/postgres/queries/games.sql.

// sortColumns maps each sort to its ORDER BY expression. Missing values sort
// last in both directions, like the Postgres query's NULLS LAST.
var sortColumns = map[string]string{
	"name":        "casefold(a.name)",
	"playtime":    "o.playtime_forever",
	"recent":      "o.playtime_2weeks",
	"last_played": "o.last_played_at",
	"review":      "a.review_pct",
	"metacritic":  "a.metacritic_score",
	"release":     "a.release_date",
}

func (s *Store) ListGames(ctx context.Context, arg store.ListGamesParams) ([]store.ListGamesRow, error) {
	where := []string{"o.steam_id = ?"}
	args := []any{arg.SteamID}
	add := func(cond string, a ...any) {
		where = append(where, cond)
		args = append(args, a...)
	}

	if arg.NameContains != nil {
		add("instr(casefold(a.name), casefold(?)) > 0", *arg.NameContains)
	}
	// Has every listed value: none of the wanted values is missing.
	hasAll := func(column string, want []string) {
		if want != nil {
			add(`NOT EXISTS (SELECT 1 FROM json_each(?) w
			     WHERE w.value NOT IN (SELECT h.value FROM json_each(`+column+`) h))`, jsonList(want))
		}
	}
	if arg.AppID != nil {
		add("a.appid = ?", *arg.AppID)
	}
	hasAll("a.genres", arg.Genres)
	hasAll("a.categories", arg.Categories)
	if arg.Played != nil {
		add("(o.playtime_forever > 0) = ?", *arg.Played)
	}
	if arg.MinMinutes != nil {
		add("o.playtime_forever >= ?", *arg.MinMinutes)
	}
	if arg.MaxMinutes != nil {
		add("o.playtime_forever <= ?", *arg.MaxMinutes)
	}
	if arg.MinReviewPct != nil {
		add("a.review_pct >= ?", *arg.MinReviewPct)
	}
	if arg.MinReviewScore != nil { // Steam's 1..9 rating; 0 (no reviews) never matches.
		add("a.review_score >= ?", *arg.MinReviewScore)
	}
	if arg.ReviewScores != nil { // any of these ratings
		b, _ := json.Marshal(arg.ReviewScores) // an []int32 always marshals
		add("a.review_score IN (SELECT value FROM json_each(?))", string(b))
	}
	if arg.MinReviews != nil { // games whose review count isn't known yet are kept
		add("(a.reviews_total IS NULL OR a.reviews_total >= ?)", *arg.MinReviews)
	}
	if arg.MinMetacritic != nil {
		add("a.metacritic_score >= ?", *arg.MinMetacritic)
	}
	if arg.DetailsStatus != nil {
		add("a.details_status = ?", *arg.DetailsStatus)
	}
	if arg.HasReviews != nil {
		add("(a.reviews_fetched_at IS NOT NULL) = ?", *arg.HasReviews)
	}
	if arg.ReleasedAfter != nil {
		add("a.release_date >= ?", nullDate(arg.ReleasedAfter))
	}
	if arg.ReleasedBefore != nil {
		add("a.release_date <= ?", nullDate(arg.ReleasedBefore))
	}
	if arg.WindowsOnly {
		add("a.platform_windows")
	}
	if arg.MacOnly {
		add("a.platform_mac")
	}
	if arg.LinuxOnly {
		add("a.platform_linux")
	}

	var order []string
	if col, ok := sortColumns[arg.Sort]; ok {
		dir := " ASC"
		if arg.Descending {
			dir = " DESC"
		}
		if arg.Sort == "review" {
			// Steam's rating (Overwhelmingly Positive, Very Positive, ...)
			// comes first, so a "Positive" game at 100% from a handful of
			// reviews stays below the Overwhelmingly Positive ones. Within a
			// rating the least played come first, then percent positive.
			order = append(order, "a.review_score"+dir+" NULLS LAST", "o.playtime_forever ASC")
		}
		order = append(order, col+dir+" NULLS LAST")
	}
	order = append(order, "casefold(a.name)", "a.appid")

	query := `SELECT
	    a.appid, a.name, a.header_image, a.short_description,
	    a.release_date, a.release_date_text, a.coming_soon,
	    a.genres, a.categories, a.developers, a.publishers,
	    a.platform_windows, a.platform_mac, a.platform_linux,
	    a.metacritic_score, json_extract(a.details_raw, '$.metacritic.url'), a.price_cents, a.price_currency, a.is_free,
	    a.review_score_desc, a.review_pct, a.reviews_total, a.details_status,
	    o.playtime_forever, o.playtime_2weeks, o.last_played_at,
	    count(*) OVER () AS total_count
	FROM owned_games o
	JOIN apps a ON a.appid = o.appid
	WHERE ` + strings.Join(where, "\n  AND ") + `
	ORDER BY ` + strings.Join(order, ", ") + `
	LIMIT ? OFFSET ?`
	args = append(args, arg.PageLimit, arg.PageOffset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []store.ListGamesRow{}
	for rows.Next() {
		var (
			r                                               store.ListGamesRow
			header, desc, releaseText, currency, reviewDesc sql.NullString
			metacriticURL                                   sql.NullString
			released, lastPlayed                            sql.NullString
			genres, categories, developers, publishers      string
			metacritic, price, reviewsTotal                 sql.NullInt64
			reviewPct                                       sql.NullFloat64
		)
		if err := rows.Scan(
			&r.Appid, &r.Name, &header, &desc,
			&released, &releaseText, &r.ComingSoon,
			&genres, &categories, &developers, &publishers,
			&r.PlatformWindows, &r.PlatformMac, &r.PlatformLinux,
			&metacritic, &metacriticURL, &price, &currency, &r.IsFree,
			&reviewDesc, &reviewPct, &reviewsTotal, &r.DetailsStatus,
			&r.PlaytimeForever, &r.Playtime2weeks, &lastPlayed,
			&r.TotalCount,
		); err != nil {
			return nil, err
		}
		r.HeaderImage, r.ShortDescription = stringPtr(header), stringPtr(desc)
		r.ReleaseDate, r.ReleaseDateText = datePtr(released), stringPtr(releaseText)
		r.Genres, r.Categories = parseList(genres), parseList(categories)
		r.Developers, r.Publishers = parseList(developers), parseList(publishers)
		r.MetacriticScore, r.PriceCents, r.PriceCurrency = intPtr(metacritic), intPtr(price), stringPtr(currency)
		r.ReviewScoreDesc, r.ReviewPct, r.ReviewsTotal = stringPtr(reviewDesc), floatPtr(reviewPct), intPtr(reviewsTotal)
		r.MetacriticURL = metacriticURL.String
		r.LastPlayedAt = timePtr(lastPlayed)
		out = append(out, r)
	}
	return out, rows.Err()
}

// facets counts the user's games per value of a list column, most common first.
func (s *Store) facets(ctx context.Context, column string, steamID int64) ([]store.GenreFacetsRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT v.value, count(*) AS games
		FROM owned_games o
		JOIN apps a ON a.appid = o.appid, json_each(`+column+`) v
		WHERE o.steam_id = ?
		GROUP BY v.value
		ORDER BY games DESC, v.value`, steamID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []store.GenreFacetsRow{}
	for rows.Next() {
		var f store.GenreFacetsRow
		if err := rows.Scan(&f.Value, &f.Games); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) OwnedAmong(ctx context.Context, steamID int64, appids []int64) ([]int64, error) {
	b, _ := json.Marshal(appids) // an []int64 always marshals
	rows, err := s.db.QueryContext(ctx, `SELECT appid FROM owned_games
	    WHERE steam_id = ? AND appid IN (SELECT value FROM json_each(?))
	    ORDER BY appid`, steamID, string(b))
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) GenreFacets(ctx context.Context, steamID int64) ([]store.GenreFacetsRow, error) {
	return s.facets(ctx, "a.genres", steamID)
}

func (s *Store) CategoryFacets(ctx context.Context, steamID int64) ([]store.CategoryFacetsRow, error) {
	rows, err := s.facets(ctx, "a.categories", steamID)
	out := make([]store.CategoryFacetsRow, len(rows))
	for i, r := range rows {
		out[i] = store.CategoryFacetsRow(r)
	}
	return out, err
}
