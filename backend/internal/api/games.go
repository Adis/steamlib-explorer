package api

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

// GameQueries reads the signed-in user's games.
type GameQueries interface {
	ListGames(ctx context.Context, arg store.ListGamesParams) ([]store.ListGamesRow, error)
	GenreFacets(ctx context.Context, steamID int64) ([]store.GenreFacetsRow, error)
	CategoryFacets(ctx context.Context, steamID int64) ([]store.CategoryFacetsRow, error)
}

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

var sortKeys = map[string]bool{
	"name": true, "playtime": true, "recent": true, "last_played": true,
	"review": true, "metacritic": true, "release": true,
}

type gameJSON struct {
	AppID            int64      `json:"appid"`
	Name             string     `json:"name"`
	StoreURL         string     `json:"store_url"`
	HeaderImage      *string    `json:"header_image"`
	ShortDescription *string    `json:"short_description"`
	ReleaseDate      *string    `json:"release_date"`      // YYYY-MM-DD, null when Steam's text isn't a date
	ReleaseDateText  *string    `json:"release_date_text"` // as Steam shows it
	ComingSoon       bool       `json:"coming_soon"`
	Genres           []string   `json:"genres"`
	Categories       []string   `json:"categories"`
	Developers       []string   `json:"developers"`
	Publishers       []string   `json:"publishers"`
	Platforms        platforms  `json:"platforms"`
	MetacriticScore  *int32     `json:"metacritic_score"`
	PriceCents       *int32     `json:"price_cents"`
	PriceCurrency    *string    `json:"price_currency"`
	IsFree           bool       `json:"is_free"`
	ReviewScoreDesc  *string    `json:"review_score_desc"`
	ReviewPct        *float64   `json:"review_pct"`
	ReviewsTotal     *int32     `json:"reviews_total"`
	DetailsStatus    string     `json:"details_status"` // pending | ok | unavailable | error
	PlaytimeMinutes  int32      `json:"playtime_minutes"`
	Playtime2Weeks   int32      `json:"playtime_2weeks_minutes"`
	LastPlayedAt     *time.Time `json:"last_played_at"`
}

type platforms struct {
	Windows bool `json:"windows"`
	Mac     bool `json:"mac"`
	Linux   bool `json:"linux"`
}

type gamesPageJSON struct {
	Games  []gameJSON `json:"games"`
	Total  int64      `json:"total"` // matches across all pages
	Limit  int32      `json:"limit"`
	Offset int32      `json:"offset"`
}

// listGames handles GET /api/games.
func listGames(q GameQueries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.UserFrom(r.Context())
		params, err := parseGameFilters(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		params.SteamID = int64(u.SteamID)

		rows, err := q.ListGames(r.Context(), params)
		if err != nil {
			slog.Error("list games", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		out := gamesPageJSON{Games: make([]gameJSON, 0, len(rows)), Limit: params.PageLimit, Offset: params.PageOffset}
		for _, g := range rows {
			out.Total = g.TotalCount
			out.Games = append(out.Games, toGameJSON(g))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// parseGameFilters turns query parameters into query arguments. All are
// optional; see the README for the list.
func parseGameFilters(v url.Values) (store.ListGamesParams, error) {
	// Best rated first by default; the query breaks ties by least playtime.
	p := store.ListGamesParams{Sort: "review", PageLimit: defaultPageSize}
	var err error

	if s := strings.TrimSpace(v.Get("q")); s != "" {
		p.NameContains = &s
	}
	p.Genres = nonEmptyValues(v["genre"])
	p.Categories = nonEmptyValues(v["category"])

	if s := v.Get("played"); s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return p, fmt.Errorf("played must be true or false")
		}
		p.Played = &b
	}
	if p.MinMinutes, err = hoursParam(v, "min_hours"); err != nil {
		return p, err
	}
	if p.MaxMinutes, err = hoursParam(v, "max_hours"); err != nil {
		return p, err
	}
	if s := v.Get("min_review"); s != "" {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f < 0 || f > 100 {
			return p, fmt.Errorf("min_review must be a percentage from 0 to 100")
		}
		p.MinReviewPct = &f
	}
	if p.MinReviewScore, err = intParam(v, "min_review_score", 1, 9); err != nil {
		return p, err
	}
	if p.MinMetacritic, err = intParam(v, "min_metacritic", 0, 100); err != nil {
		return p, err
	}
	if p.ReleasedAfter, err = dateParam(v, "released_after"); err != nil {
		return p, err
	}
	if p.ReleasedBefore, err = dateParam(v, "released_before"); err != nil {
		return p, err
	}
	for _, platform := range nonEmptyValues(v["platform"]) {
		switch platform {
		case "windows":
			p.WindowsOnly = true
		case "mac":
			p.MacOnly = true
		case "linux":
			p.LinuxOnly = true
		default:
			return p, fmt.Errorf("platform must be windows, mac or linux")
		}
	}

	if s := v.Get("sort"); s != "" {
		if !sortKeys[s] {
			return p, fmt.Errorf("sort must be one of name, playtime, recent, last_played, review, metacritic, release")
		}
		p.Sort = s
	}
	switch v.Get("order") {
	case "", "asc":
		// Numbers read best biggest first unless asked otherwise; names A to Z.
		p.Descending = v.Get("order") == "" && p.Sort != "name"
	case "desc":
		p.Descending = true
	default:
		return p, fmt.Errorf("order must be asc or desc")
	}

	if limit, err := intParam(v, "limit", 1, maxPageSize); err != nil {
		return p, err
	} else if limit != nil {
		p.PageLimit = *limit
	}
	if offset, err := intParam(v, "offset", 0, math.MaxInt32); err != nil {
		return p, err
	} else if offset != nil {
		p.PageOffset = *offset
	}
	return p, nil
}

func intParam(v url.Values, name string, lo, hi int) (*int32, error) {
	s := v.Get(name)
	if s == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < lo || n > hi {
		return nil, fmt.Errorf("%s must be a whole number from %d to %d", name, lo, hi)
	}
	i := int32(n)
	return &i, nil
}

// hoursParam reads hours (decimals allowed) and returns minutes, which is
// what Steam reports playtime in.
func hoursParam(v url.Values, name string) (*int32, error) {
	s := v.Get(name)
	if s == "" {
		return nil, nil
	}
	h, err := strconv.ParseFloat(s, 64)
	if err != nil || h < 0 || h > 1_000_000 {
		return nil, fmt.Errorf("%s must be a number of hours, 0 or more", name)
	}
	m := int32(math.Round(h * 60))
	return &m, nil
}

func dateParam(v url.Values, name string) (*time.Time, error) {
	s := v.Get(name)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, fmt.Errorf("%s must be a date like 2020-01-31", name)
	}
	return &t, nil
}

// nonEmptyValues drops blanks and returns nil (filter off) when none remain.
func nonEmptyValues(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func toGameJSON(g store.ListGamesRow) gameJSON {
	j := gameJSON{
		AppID:            g.Appid,
		Name:             g.Name,
		StoreURL:         "https://store.steampowered.com/app/" + strconv.FormatInt(g.Appid, 10),
		HeaderImage:      g.HeaderImage,
		ShortDescription: g.ShortDescription,
		ReleaseDateText:  g.ReleaseDateText,
		ComingSoon:       g.ComingSoon,
		Genres:           g.Genres,
		Categories:       g.Categories,
		Developers:       g.Developers,
		Publishers:       g.Publishers,
		Platforms:        platforms{Windows: g.PlatformWindows, Mac: g.PlatformMac, Linux: g.PlatformLinux},
		MetacriticScore:  g.MetacriticScore,
		PriceCents:       g.PriceCents,
		PriceCurrency:    g.PriceCurrency,
		IsFree:           g.IsFree,
		ReviewScoreDesc:  g.ReviewScoreDesc,
		ReviewPct:        g.ReviewPct,
		ReviewsTotal:     g.ReviewsTotal,
		DetailsStatus:    g.DetailsStatus,
		PlaytimeMinutes:  g.PlaytimeForever,
		Playtime2Weeks:   g.Playtime2weeks,
		LastPlayedAt:     g.LastPlayedAt,
	}
	if g.ReleaseDate != nil {
		d := g.ReleaseDate.Format(time.DateOnly)
		j.ReleaseDate = &d
	}
	return j
}

type facetJSON struct {
	Value string `json:"value"`
	Games int64  `json:"games"`
}

// listFacets handles GET /api/facets: the genres and categories present in
// the user's library, most common first.
func listFacets(q GameQueries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.UserFrom(r.Context())
		id := int64(u.SteamID)

		genres, err := q.GenreFacets(r.Context(), id)
		if err != nil {
			slog.Error("genre facets", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		categories, err := q.CategoryFacets(r.Context(), id)
		if err != nil {
			slog.Error("category facets", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		out := struct {
			Genres     []facetJSON `json:"genres"`
			Categories []facetJSON `json:"categories"`
		}{make([]facetJSON, 0, len(genres)), make([]facetJSON, 0, len(categories))}
		for _, g := range genres {
			out.Genres = append(out.Genres, facetJSON{Value: g.Value, Games: g.Games})
		}
		for _, c := range categories {
			out.Categories = append(out.Categories, facetJSON{Value: c.Value, Games: c.Games})
		}
		writeJSON(w, http.StatusOK, out)
	}
}
