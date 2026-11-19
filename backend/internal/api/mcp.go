package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

// The MCP server (/mcp) lets an AI assistant such as Claude read the library
// of the user whose token it was given. It only reads: nothing it offers
// changes the library, starts a Refresh or reaches Steam, except "More from"
// lists, which come from the Steam store like on a game's page.

// MCPQueries is what the MCP tools read.
type MCPQueries interface {
	GameQueries
	SyncQueries
	OwnedQueries
}

// mcpVersion is shown to MCP clients as the server version.
const mcpVersion = "1.0.0"

const (
	mcpDefaultLimit = 25
	mcpMaxLimit     = 100
	// newReleaseMonths matches the "New" badge in the app.
	newReleaseMonths = 6
)

// reviewRatings are Steam's review ratings by name, as review_score 1-9.
var reviewRatings = map[string]int{
	"Overwhelmingly Positive": 9,
	"Very Positive":           8,
	"Positive":                7,
	"Mostly Positive":         6,
	"Mixed":                   5,
	"Mostly Negative":         4,
	"Negative":                3,
	"Very Negative":           2,
	"Overwhelmingly Negative": 1,
}

// newMCPHandler serves the MCP tools over Streamable HTTP. verify turns a
// bearer token into the Steam ID of its owner.
func newMCPHandler(q MCPQueries, developers *DeveloperCache, verify auth.TokenVerifier, now func() time.Time) http.Handler {
	t := &mcpTools{q: q, developers: developers, now: now}
	server := mcp.NewServer(&mcp.Implementation{Name: "steamlib-explorer", Title: "Steamlib Explorer", Version: mcpVersion}, &mcp.ServerOptions{
		Instructions: "Reads the user's Steam library as synced into Steamlib Explorer: owned games with playtime, " +
			"Steam store details (genres, categories, developers, release date, platforms, Metacritic) and Steam " +
			"review ratings. Data is as fresh as the user's last Refresh in the app (see library_stats). " +
			"Playtime is in hours. Use search_games to find games, get_game for one game, library_stats for an " +
			"overview and more_from_developer for a developer's other games on the Steam store.",
	})

	closed, open := false, true
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}
	mcp.AddTool(server, &mcp.Tool{
		Name:  "search_games",
		Title: "Search games",
		Description: "Finds games in the user's Steam library with the same filters as the app's explorer. " +
			"All filters are optional and combine with AND. Returns the total number of matches and one page of games, " +
			"best rated first unless sort says otherwise. Genres and categories must match Steam's names exactly; " +
			"library_stats lists the most common genres.",
		Annotations: readOnly,
	}, t.searchGames)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_game",
		Title:       "Get game",
		Description: "Returns everything known about one game in the user's library, by Steam app id.",
		Annotations: readOnly,
	}, t.getGame)
	mcp.AddTool(server, &mcp.Tool{
		Name:  "library_stats",
		Title: "Library stats",
		Description: "An overview of the user's library: how many games, how many were played, total playtime, " +
			"the most common genres, the most played and recently played games, and when the library was last synced.",
		Annotations: readOnly,
	}, t.libraryStats)
	devAnnotations := *readOnly
	devAnnotations.OpenWorldHint = &open
	mcp.AddTool(server, &mcp.Tool{
		Name:  "more_from_developer",
		Title: "More from developer",
		Description: "Lists a developer's games on the Steam store, each marked owned or not, like \"More from\" " +
			"on a game's page in the app. Give the developer's name, or the app id of a game in the library to use " +
			"its first developer. The store is asked at most once a day per developer and can take a few seconds.",
		Annotations: &devAnnotations,
	}, t.moreFromDeveloper)

	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		// Every request stands alone: nothing to keep between them.
		Stateless:    true,
		JSONResponse: true,
		Logger:       slog.Default(),
	})
	return auth.RequireBearerToken(verify, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(h)
}

type mcpTools struct {
	q          MCPQueries
	developers *DeveloperCache
	now        func() time.Time
}

// steamID is the user the request's token belongs to.
func steamID(req *mcp.CallToolRequest) (int64, error) {
	if req.Extra == nil || req.Extra.TokenInfo == nil {
		return 0, errors.New("not signed in")
	}
	return strconv.ParseInt(req.Extra.TokenInfo.UserID, 10, 64)
}

// mcpGame is a game as the tools return it: compact, with playtime in hours.
type mcpGame struct {
	AppID          int64    `json:"appid"`
	Name           string   `json:"name"`
	Genres         []string `json:"genres"`
	Developers     []string `json:"developers"`
	ReleaseDate    string   `json:"release_date,omitempty" jsonschema:"YYYY-MM-DD, or as Steam shows it when not a date"`
	NewRelease     bool     `json:"new_release,omitempty" jsonschema:"released in the last 6 months"`
	ComingSoon     bool     `json:"coming_soon,omitempty"`
	PlaytimeHours  float64  `json:"playtime_hours"`
	Recent2wHours  float64  `json:"playtime_last_2_weeks_hours,omitempty"`
	LastPlayed     string   `json:"last_played,omitempty" jsonschema:"YYYY-MM-DD"`
	ReviewRating   string   `json:"review_rating,omitempty" jsonschema:"Steam's rating, e.g. Very Positive"`
	ReviewPercent  *float64 `json:"review_percent,omitempty" jsonschema:"share of positive Steam reviews"`
	Reviews        *int32   `json:"reviews,omitempty" jsonschema:"number of Steam reviews"`
	Metacritic     *int32   `json:"metacritic,omitempty"`
	Platforms      []string `json:"platforms"`
	StoreDataState string   `json:"store_data,omitempty" jsonschema:"set when store details are missing: pending (not fetched yet), unavailable (no store page) or error"`
}

// mcpGameDetails is one game with everything known about it.
type mcpGameDetails struct {
	mcpGame
	Description   string   `json:"description,omitempty"`
	Categories    []string `json:"categories"`
	Publishers    []string `json:"publishers"`
	Price         string   `json:"price,omitempty" jsonschema:"current store price, e.g. 19.99 EUR, or Free"`
	StoreURL      string   `json:"store_url"`
	MetacriticURL string   `json:"metacritic_url,omitempty"`
}

func hours(minutes int32) float64 { return math.Round(float64(minutes)/6) / 10 }

func (t *mcpTools) toGame(g store.ListGamesRow) mcpGame {
	out := mcpGame{
		AppID: g.Appid, Name: g.Name, Genres: g.Genres, Developers: g.Developers, ComingSoon: g.ComingSoon,
		PlaytimeHours: hours(g.PlaytimeForever), Recent2wHours: hours(g.Playtime2weeks),
		ReviewPercent: g.ReviewPct, Reviews: g.ReviewsTotal, Metacritic: g.MetacriticScore,
		Platforms: []string{},
	}
	if out.Genres == nil {
		out.Genres = []string{}
	}
	if out.Developers == nil {
		out.Developers = []string{}
	}
	if g.ReleaseDate != nil {
		out.ReleaseDate = g.ReleaseDate.Format(time.DateOnly)
		out.NewRelease = !g.ComingSoon && isNewRelease(*g.ReleaseDate, t.now())
	} else if g.ReleaseDateText != nil {
		out.ReleaseDate = *g.ReleaseDateText
	}
	if g.LastPlayedAt != nil {
		out.LastPlayed = g.LastPlayedAt.Format(time.DateOnly)
	}
	if g.ReviewScoreDesc != nil {
		out.ReviewRating = *g.ReviewScoreDesc
	}
	if g.ReviewPct != nil {
		p := math.Round(*g.ReviewPct*10) / 10
		out.ReviewPercent = &p
	}
	if g.PlatformWindows {
		out.Platforms = append(out.Platforms, "windows")
	}
	if g.PlatformMac {
		out.Platforms = append(out.Platforms, "mac")
	}
	if g.PlatformLinux {
		out.Platforms = append(out.Platforms, "linux")
	}
	if g.DetailsStatus != "ok" {
		out.StoreDataState = g.DetailsStatus
	}
	return out
}

// isNewRelease matches the app's "New" badge: released in the last six
// months, by calendar date.
func isNewRelease(released, now time.Time) bool {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	day := time.Date(released.Year(), released.Month(), released.Day(), 0, 0, 0, 0, time.UTC)
	return !day.Before(today.AddDate(0, -newReleaseMonths, 0)) && !day.After(today)
}

type searchGamesIn struct {
	Name           string   `json:"name,omitempty" jsonschema:"text the game's name contains, ignoring case"`
	Genres         []string `json:"genres,omitempty" jsonschema:"Steam genres the game has, all of them (e.g. RPG, Strategy)"`
	Categories     []string `json:"categories,omitempty" jsonschema:"Steam categories the game has, all of them (e.g. Single-player, Co-op, Steam Deck Verified)"`
	Played         *bool    `json:"played,omitempty" jsonschema:"true: only games played at all; false: only never played"`
	MinHours       *float64 `json:"min_hours,omitempty" jsonschema:"at least this much playtime, in hours"`
	MaxHours       *float64 `json:"max_hours,omitempty" jsonschema:"at most this much playtime, in hours"`
	ReviewRatings  []string `json:"review_ratings,omitempty" jsonschema:"Steam review ratings to keep, any of: Overwhelmingly Positive, Very Positive, Positive, Mostly Positive, Mixed, Mostly Negative, Negative, Very Negative, Overwhelmingly Negative"`
	MinReviewPct   *float64 `json:"min_review_percent,omitempty" jsonschema:"at least this share of positive Steam reviews, 0-100"`
	MinReviews     *int     `json:"min_reviews,omitempty" jsonschema:"at least this many Steam reviews (Steam gives no rating below 10)"`
	MinMetacritic  *int     `json:"min_metacritic,omitempty" jsonschema:"Metacritic score of at least this, 0-100"`
	ReleasedAfter  string   `json:"released_after,omitempty" jsonschema:"released on or after this date, YYYY-MM-DD"`
	ReleasedBefore string   `json:"released_before,omitempty" jsonschema:"released on or before this date, YYYY-MM-DD"`
	NewReleases    bool     `json:"new_releases,omitempty" jsonschema:"only games released in the last 6 months"`
	Platforms      []string `json:"platforms,omitempty" jsonschema:"platforms the game runs on, all of them: windows, mac, linux"`
	Sort           string   `json:"sort,omitempty" jsonschema:"review (default: Steam rating, then least played), name, playtime, recent (last 2 weeks), last_played, metacritic or release"`
	Order          string   `json:"order,omitempty" jsonschema:"asc or desc; default desc, except asc for name"`
	Limit          int      `json:"limit,omitempty" jsonschema:"games per page, 1-100, default 25"`
	Offset         int      `json:"offset,omitempty" jsonschema:"games to skip, for the next pages"`
}

type searchGamesOut struct {
	Total  int64     `json:"total" jsonschema:"games matching, across all pages"`
	Offset int       `json:"offset"`
	Games  []mcpGame `json:"games"`
}

// searchGames reuses the explorer's query parameters, so it filters and
// checks values exactly like GET /api/games.
func (t *mcpTools) searchGames(ctx context.Context, req *mcp.CallToolRequest, in searchGamesIn) (*mcp.CallToolResult, searchGamesOut, error) {
	id, err := steamID(req)
	if err != nil {
		return nil, searchGamesOut{}, err
	}
	v := url.Values{}
	set := func(k, s string) {
		if s != "" {
			v.Set(k, s)
		}
	}
	num := func(f *float64) string {
		if f == nil {
			return ""
		}
		return strconv.FormatFloat(*f, 'f', -1, 64)
	}
	integer := func(n *int) string {
		if n == nil {
			return ""
		}
		return strconv.Itoa(*n)
	}
	set("q", in.Name)
	v["genre"] = in.Genres
	v["category"] = in.Categories
	if in.Played != nil {
		v.Set("played", strconv.FormatBool(*in.Played))
	}
	set("min_hours", num(in.MinHours))
	set("max_hours", num(in.MaxHours))
	for _, r := range in.ReviewRatings {
		score, ok := reviewRatings[strings.TrimSpace(r)]
		if !ok {
			return nil, searchGamesOut{}, fmt.Errorf("unknown review rating %q: use one of Overwhelmingly Positive, Very Positive, Positive, Mostly Positive, Mixed, Mostly Negative, Negative, Very Negative, Overwhelmingly Negative", r)
		}
		v.Add("review_score", strconv.Itoa(score))
	}
	set("min_review", num(in.MinReviewPct))
	set("min_reviews", integer(in.MinReviews))
	set("min_metacritic", integer(in.MinMetacritic))
	set("released_after", in.ReleasedAfter)
	set("released_before", in.ReleasedBefore)
	v["platform"] = in.Platforms
	set("sort", in.Sort)
	set("order", in.Order)
	limit := in.Limit
	if limit == 0 {
		limit = mcpDefaultLimit
	}
	if limit < 1 || limit > mcpMaxLimit {
		return nil, searchGamesOut{}, fmt.Errorf("limit must be from 1 to %d", mcpMaxLimit)
	}
	v.Set("limit", strconv.Itoa(limit))
	set("offset", strconv.Itoa(in.Offset))

	p, err := parseGameFilters(v)
	if err != nil {
		return nil, searchGamesOut{}, err
	}
	if in.NewReleases {
		today := t.now().UTC().Truncate(24 * time.Hour)
		cutoff := today.AddDate(0, -newReleaseMonths, 0)
		if p.ReleasedAfter == nil || p.ReleasedAfter.Before(cutoff) {
			p.ReleasedAfter = &cutoff
		}
		if p.ReleasedBefore == nil || p.ReleasedBefore.After(today) {
			p.ReleasedBefore = &today
		}
	}
	p.SteamID = id

	rows, err := t.q.ListGames(ctx, p)
	if err != nil {
		slog.Error("mcp: search games", "error", err)
		return nil, searchGamesOut{}, errors.New("reading the library failed")
	}
	out := searchGamesOut{Offset: int(p.PageOffset), Games: make([]mcpGame, 0, len(rows))}
	for _, g := range rows {
		out.Total = g.TotalCount
		out.Games = append(out.Games, t.toGame(g))
	}
	if len(rows) == 0 && p.PageOffset > 0 {
		// Past the last page: count the matches anyway.
		p.PageOffset, p.PageLimit = 0, 1
		if first, err := t.q.ListGames(ctx, p); err == nil && len(first) > 0 {
			out.Total = first[0].TotalCount
		}
	}
	return nil, out, nil
}

type getGameIn struct {
	AppID int64 `json:"appid" jsonschema:"the game's Steam app id"`
}

func (t *mcpTools) getGame(ctx context.Context, req *mcp.CallToolRequest, in getGameIn) (*mcp.CallToolResult, mcpGameDetails, error) {
	id, err := steamID(req)
	if err != nil {
		return nil, mcpGameDetails{}, err
	}
	g, err := t.ownedGame(ctx, id, in.AppID)
	if err != nil {
		return nil, mcpGameDetails{}, err
	}
	out := mcpGameDetails{
		mcpGame:    t.toGame(g),
		Categories: g.Categories,
		Publishers: g.Publishers,
		StoreURL:   "https://store.steampowered.com/app/" + strconv.FormatInt(g.Appid, 10),
	}
	if out.Categories == nil {
		out.Categories = []string{}
	}
	if out.Publishers == nil {
		out.Publishers = []string{}
	}
	if g.ShortDescription != nil {
		out.Description = *g.ShortDescription
	}
	out.MetacriticURL = g.MetacriticURL
	switch {
	case g.IsFree:
		out.Price = "Free"
	case g.PriceCents != nil && g.PriceCurrency != nil:
		out.Price = fmt.Sprintf("%.2f %s", float64(*g.PriceCents)/100, *g.PriceCurrency)
	}
	return nil, out, nil
}

// ownedGame reads one game of the user's, with an error the assistant can
// act on when it isn't theirs.
func (t *mcpTools) ownedGame(ctx context.Context, steamID, appid int64) (store.ListGamesRow, error) {
	if appid <= 0 {
		return store.ListGamesRow{}, errors.New("appid must be a positive whole number")
	}
	rows, err := t.q.ListGames(ctx, store.ListGamesParams{SteamID: steamID, AppID: &appid, Sort: "name", PageLimit: 1})
	if err != nil {
		slog.Error("mcp: get game", "appid", appid, "error", err)
		return store.ListGamesRow{}, errors.New("reading the library failed")
	}
	if len(rows) == 0 {
		return store.ListGamesRow{}, fmt.Errorf("app %d is not in the user's library; search_games finds games by name", appid)
	}
	return rows[0], nil
}

type libraryStatsIn struct{}

type genreCount struct {
	Genre string `json:"genre"`
	Games int64  `json:"games"`
}

type libraryStatsOut struct {
	Games               int64        `json:"games"`
	Played              int64        `json:"played" jsonschema:"games with any playtime"`
	NeverPlayed         int64        `json:"never_played"`
	PlaytimeHours       float64      `json:"playtime_hours" jsonschema:"total, all games"`
	Playtime2WeeksHours float64      `json:"playtime_last_2_weeks_hours"`
	TopGenres           []genreCount `json:"top_genres" jsonschema:"the 15 most common genres"`
	MostPlayed          []mcpGame    `json:"most_played" jsonschema:"the 10 games with the most playtime"`
	RecentlyPlayed      []mcpGame    `json:"recently_played" jsonschema:"games played in the last 2 weeks, most played first"`
	WithoutStoreDetails int64        `json:"without_store_details" jsonschema:"games whose store details aren't fetched yet or have no store page; filters on genre, review etc. skip them"`
	LastSync            string       `json:"last_sync,omitempty" jsonschema:"when the library was last read from Steam (RFC 3339)"`
	LastSyncStatus      string       `json:"last_sync_status,omitempty" jsonschema:"running, ok or error"`
}

func (t *mcpTools) libraryStats(ctx context.Context, req *mcp.CallToolRequest, _ libraryStatsIn) (*mcp.CallToolResult, libraryStatsOut, error) {
	id, err := steamID(req)
	if err != nil {
		return nil, libraryStatsOut{}, err
	}
	fail := func(what string, err error) (*mcp.CallToolResult, libraryStatsOut, error) {
		slog.Error("mcp: library stats", "reading", what, "error", err)
		return nil, libraryStatsOut{}, errors.New("reading the library failed")
	}
	stats, err := t.q.LibraryStats(ctx, id)
	if err != nil {
		return fail("stats", err)
	}
	out := libraryStatsOut{
		Games: stats.Games, Played: stats.Played, NeverPlayed: stats.Games - stats.Played,
		PlaytimeHours:       math.Round(float64(stats.PlaytimeMinutes)/6) / 10,
		Playtime2WeeksHours: math.Round(float64(stats.Playtime2weeksMinutes)/6) / 10,
		WithoutStoreDetails: stats.Games - stats.WithDetails,
		TopGenres:           []genreCount{},
		MostPlayed:          []mcpGame{},
		RecentlyPlayed:      []mcpGame{},
	}

	genres, err := t.q.GenreFacets(ctx, id)
	if err != nil {
		return fail("genres", err)
	}
	for i, g := range genres {
		if i == 15 {
			break
		}
		out.TopGenres = append(out.TopGenres, genreCount{Genre: g.Value, Games: g.Games})
	}

	played := true
	most, err := t.q.ListGames(ctx, store.ListGamesParams{SteamID: id, Played: &played, Sort: "playtime", Descending: true, PageLimit: 10})
	if err != nil {
		return fail("most played", err)
	}
	for _, g := range most {
		out.MostPlayed = append(out.MostPlayed, t.toGame(g))
	}
	recent, err := t.q.ListGames(ctx, store.ListGamesParams{SteamID: id, Sort: "recent", Descending: true, PageLimit: 10})
	if err != nil {
		return fail("recently played", err)
	}
	for _, g := range recent {
		if g.Playtime2weeks > 0 {
			out.RecentlyPlayed = append(out.RecentlyPlayed, t.toGame(g))
		}
	}

	run, err := t.q.LatestSyncRun(ctx, id)
	switch {
	case errors.Is(err, store.ErrNotFound):
	case err != nil:
		return fail("last sync", err)
	default:
		at := run.StartedAt
		if run.FinishedAt != nil {
			at = *run.FinishedAt
		}
		out.LastSync = at.UTC().Format(time.RFC3339)
		out.LastSyncStatus = run.Status
	}
	return nil, out, nil
}

type moreFromDeveloperIn struct {
	Developer string `json:"developer,omitempty" jsonschema:"the developer's name, as Steam shows it"`
	AppID     int64  `json:"appid,omitempty" jsonschema:"or: a game in the library, to use its first developer"`
}

type developerGame struct {
	AppID         int64  `json:"appid"`
	Name          string `json:"name"`
	Owned         bool   `json:"owned" jsonschema:"in the user's library"`
	Released      string `json:"released,omitempty" jsonschema:"as the store shows it"`
	ReviewRating  string `json:"review_rating,omitempty"`
	ReviewPercent *int   `json:"review_percent,omitempty"`
	Reviews       *int   `json:"reviews,omitempty"`
	StoreURL      string `json:"store_url"`
}

type moreFromDeveloperOut struct {
	Developer string          `json:"developer"`
	Games     []developerGame `json:"games"`
}

func (t *mcpTools) moreFromDeveloper(ctx context.Context, req *mcp.CallToolRequest, in moreFromDeveloperIn) (*mcp.CallToolResult, moreFromDeveloperOut, error) {
	id, err := steamID(req)
	if err != nil {
		return nil, moreFromDeveloperOut{}, err
	}
	if t.developers == nil {
		return nil, moreFromDeveloperOut{}, errors.New("developer lists aren't available on this server")
	}
	developer := strings.TrimSpace(in.Developer)
	if developer == "" {
		if in.AppID == 0 {
			return nil, moreFromDeveloperOut{}, errors.New("give developer or appid")
		}
		g, err := t.ownedGame(ctx, id, in.AppID)
		if err != nil {
			return nil, moreFromDeveloperOut{}, err
		}
		if len(g.Developers) == 0 {
			return nil, moreFromDeveloperOut{}, fmt.Errorf("the Steam store lists no developer for %s", g.Name)
		}
		developer = g.Developers[0]
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	games, err := t.developers.Games(fetchCtx, developer)
	if err != nil {
		slog.Warn("mcp: developer games", "developer", developer, "error", err)
		return nil, moreFromDeveloperOut{}, errors.New("the Steam store didn't answer, try again later")
	}
	ids := make([]int64, len(games))
	for i, g := range games {
		ids[i] = g.AppID
	}
	owned := map[int64]bool{}
	if len(ids) > 0 {
		got, err := t.q.OwnedAmong(ctx, id, ids)
		if err != nil {
			slog.Error("mcp: owned among", "error", err)
			return nil, moreFromDeveloperOut{}, errors.New("reading the library failed")
		}
		for _, a := range got {
			owned[a] = true
		}
	}
	out := moreFromDeveloperOut{Developer: developer, Games: make([]developerGame, 0, len(games))}
	for _, g := range games {
		out.Games = append(out.Games, developerGame{
			AppID: g.AppID, Name: g.Name, Owned: owned[g.AppID], Released: g.ReleasedText,
			ReviewRating: g.ReviewDesc, ReviewPercent: g.ReviewPct, Reviews: g.ReviewsTotal,
			StoreURL: "https://store.steampowered.com/app/" + strconv.FormatInt(g.AppID, 10),
		})
	}
	return nil, out, nil
}
