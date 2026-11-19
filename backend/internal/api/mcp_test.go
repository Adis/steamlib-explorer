package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Adis/steamlib-explorer/backend/internal/steam"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/store/sqlite"
)

const mcpUser = int64(42)

// mcpStore is an SQLite store with a small library for user 42 and a
// game owned only by user 7.
func mcpStore(t *testing.T) *sqlite.Store {
	t.Helper()
	ctx := context.Background()
	s, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "mcp.db"))
	must(t, err)
	t.Cleanup(s.Close)
	for _, id := range []int64{mcpUser, 7} {
		_, err := s.UpsertUser(ctx, store.UpsertUserParams{SteamID: id})
		must(t, err)
	}
	day := func(s string) *time.Time {
		d, _ := time.Parse(time.DateOnly, s)
		return &d
	}
	played := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	for _, g := range []struct {
		id              int64
		name            string
		genres          []string
		dev             string
		released        *time.Time
		minutes, recent int32
		score, total    int32
		owner           int64
	}{
		{1, "Hollow Depths", []string{"Action", "Indie"}, "Team Cave", day("2017-02-24"), 3000, 120, 9, 1000, mcpUser},
		{2, "Star Trader", []string{"Strategy", "Simulation"}, "Orbit Works", day("2026-09-01"), 0, 0, 8, 200, mcpUser},
		{3, "Cave Diver", []string{"Action"}, "Team Cave", day("2020-05-05"), 90, 0, 5, 50, mcpUser},
		{4, "Not Mine", []string{"Action"}, "Elsewhere", day("2019-01-01"), 10, 0, 9, 500, 7},
	} {
		must(t, s.EnsureApp(ctx, store.EnsureAppParams{Appid: g.id, Name: g.name}))
		must(t, s.UpdateAppDetails(ctx, store.UpdateAppDetailsParams{
			Appid: g.id, Name: g.name, Genres: g.genres, Categories: []string{"Single-player"},
			Developers: []string{g.dev}, Publishers: []string{"Pub"}, PlatformWindows: true, PlatformLinux: g.id == 1,
			ReleaseDate: g.released, ShortDescription: new2("About " + g.name), PriceCents: new2[int32](1999), PriceCurrency: new2("EUR"),
			DetailsRaw: []byte(`{}`),
		}))
		must(t, s.UpdateAppReviews(ctx, store.UpdateAppReviewsParams{
			Appid: g.id, ReviewScore: &g.score, ReviewScoreDesc: new2(map[int32]string{9: "Overwhelmingly Positive", 8: "Very Positive", 5: "Mixed"}[g.score]),
			ReviewsPositive: new2(g.total * 9 / 10), ReviewsTotal: &g.total,
		}))
		var last *time.Time
		if g.minutes > 0 {
			last = &played
		}
		must(t, s.UpsertOwnedGame(ctx, store.UpsertOwnedGameParams{SteamID: g.owner, Appid: g.id, PlaytimeForever: g.minutes, Playtime2weeks: g.recent, LastPlayedAt: last}))
	}
	run, err := s.StartSyncRun(ctx, mcpUser)
	must(t, err)
	must(t, s.FinishSyncRun(ctx, store.FinishSyncRunParams{ID: run, Status: "ok"}))
	return s
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func new2[T any](v T) *T { return &v }

// bearer adds a token to every request.
type bearer struct {
	token string
	next  http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.next.RoundTrip(r)
}

// mcpServer serves /mcp from s and returns a token for user 42.
func mcpServer(t *testing.T, s *sqlite.Store, search DeveloperSearch) (*httptest.Server, string) {
	t.Helper()
	token, err := newMCPToken()
	must(t, err)
	must(t, s.SaveMCPToken(context.Background(), mcpUser, hashMCPToken(token)))
	now := func() time.Time { return time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC) }
	var devs *DeveloperCache
	if search != nil {
		devs = &DeveloperCache{Search: search}
	}
	srv := httptest.NewServer(newMCPHandler(s, devs, verifyMCPToken(s), now))
	t.Cleanup(srv.Close)
	return srv, token
}

func connect(t *testing.T, url, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   url,
		HTTPClient: &http.Client{Transport: bearer{token, http.DefaultTransport}},
		MaxRetries: -1, DisableStandaloneSSE: true,
	}, nil)
	must(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// call runs a tool and decodes its structured result into out. It fails the
// test on a tool error unless wantErr, and then returns the error text.
func call(t *testing.T, cs *mcp.ClientSession, tool string, args any, out any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	must(t, err)
	if res.IsError {
		return res.Content[0].(*mcp.TextContent).Text
	}
	b, err := json.Marshal(res.StructuredContent)
	must(t, err)
	must(t, json.Unmarshal(b, out))
	return ""
}

func TestMCPNeedsToken(t *testing.T) {
	s := mcpStore(t)
	srv, _ := mcpServer(t, s, nil)
	for _, token := range []string{"", "slx_wrong", "not-a-token"} {
		req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		must(t, err)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("token %q: status %d, want 401", token, res.StatusCode)
		}
	}
	// A revoked token stops working at once.
	srv2, token := mcpServer(t, s, nil)
	must(t, s.DeleteMCPToken(context.Background(), mcpUser))
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	if _, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: srv2.URL, HTTPClient: &http.Client{Transport: bearer{token, http.DefaultTransport}}, MaxRetries: -1,
	}, nil); err == nil {
		t.Error("connected with a revoked token")
	}
}

func TestMCPTools(t *testing.T) {
	srv, token := mcpServer(t, mcpStore(t), nil)
	cs := connect(t, srv.URL, token)

	tools, err := cs.ListTools(context.Background(), nil)
	must(t, err)
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", tool.Name)
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"get_game", "library_stats", "more_from_developer", "search_games"}) {
		t.Errorf("tools = %v", names)
	}
}

func TestMCPSearchGames(t *testing.T) {
	srv, token := mcpServer(t, mcpStore(t), nil)
	cs := connect(t, srv.URL, token)

	ids := func(out searchGamesOut) []int64 {
		var got []int64
		for _, g := range out.Games {
			got = append(got, g.AppID)
		}
		return got
	}
	for _, tt := range []struct {
		name  string
		args  map[string]any
		want  []int64
		total int64
	}{
		{"everything, best rated first", map[string]any{}, []int64{1, 2, 3}, 3},
		{"by genre", map[string]any{"genres": []string{"Action"}}, []int64{1, 3}, 2},
		{"never played", map[string]any{"played": false}, []int64{2}, 1},
		{"by rating name", map[string]any{"review_ratings": []string{"Mixed", "Very Positive"}}, []int64{2, 3}, 2},
		{"new releases", map[string]any{"new_releases": true}, []int64{2}, 1},
		{"hours and linux", map[string]any{"min_hours": 10, "platforms": []string{"linux"}}, []int64{1}, 1},
		{"name, sorted", map[string]any{"name": "cave", "sort": "name"}, []int64{3}, 1},
		{"paged", map[string]any{"sort": "name", "limit": 1, "offset": 1}, []int64{1}, 3},
		{"past the end", map[string]any{"limit": 2, "offset": 10}, nil, 3},
	} {
		var out searchGamesOut
		if msg := call(t, cs, "search_games", tt.args, &out); msg != "" {
			t.Errorf("%s: error %s", tt.name, msg)
			continue
		}
		if !slices.Equal(ids(out), tt.want) || out.Total != tt.total {
			t.Errorf("%s: games %v (total %d), want %v (total %d)", tt.name, ids(out), out.Total, tt.want, tt.total)
		}
	}

	var out searchGamesOut
	call(t, cs, "search_games", map[string]any{"name": "Hollow"}, &out)
	g := out.Games[0]
	if g.PlaytimeHours != 50 || g.Recent2wHours != 2 || g.ReviewRating != "Overwhelmingly Positive" || g.ReleaseDate != "2017-02-24" ||
		!slices.Equal(g.Platforms, []string{"windows", "linux"}) || g.LastPlayed != "2026-11-10" || g.NewRelease {
		t.Errorf("game = %+v", g)
	}

	for args, want := range map[string]map[string]any{
		"unknown review rating": {"review_ratings": []string{"Great"}},
		"sort must be":          {"sort": "price"},
		"limit must be":         {"limit": 500},
		"platform must be":      {"platforms": []string{"amiga"}},
	} {
		if msg := call(t, cs, "search_games", want, &out); !strings.Contains(msg, args) {
			t.Errorf("%v: error %q, want it to mention %q", want, msg, args)
		}
	}
}

func TestMCPGetGame(t *testing.T) {
	srv, token := mcpServer(t, mcpStore(t), nil)
	cs := connect(t, srv.URL, token)

	var g mcpGameDetails
	if msg := call(t, cs, "get_game", map[string]any{"appid": 2}, &g); msg != "" {
		t.Fatal(msg)
	}
	if g.Name != "Star Trader" || g.Description != "About Star Trader" || g.Price != "19.99 EUR" || !g.NewRelease ||
		g.StoreURL != "https://store.steampowered.com/app/2" || !slices.Equal(g.Publishers, []string{"Pub"}) {
		t.Errorf("game = %+v", g)
	}
	// Another user's game is out of reach.
	if msg := call(t, cs, "get_game", map[string]any{"appid": 4}, &g); !strings.Contains(msg, "not in the user's library") {
		t.Errorf("other user's game: %q", msg)
	}
}

func TestMCPLibraryStats(t *testing.T) {
	srv, token := mcpServer(t, mcpStore(t), nil)
	cs := connect(t, srv.URL, token)

	var out libraryStatsOut
	if msg := call(t, cs, "library_stats", map[string]any{}, &out); msg != "" {
		t.Fatal(msg)
	}
	if out.Games != 3 || out.Played != 2 || out.NeverPlayed != 1 || out.PlaytimeHours != 51.5 || out.Playtime2WeeksHours != 2 {
		t.Errorf("totals = %+v", out)
	}
	if len(out.TopGenres) == 0 || out.TopGenres[0] != (genreCount{"Action", 2}) {
		t.Errorf("top genres = %+v", out.TopGenres)
	}
	if len(out.MostPlayed) != 2 || out.MostPlayed[0].AppID != 1 || len(out.RecentlyPlayed) != 1 {
		t.Errorf("most played %+v, recent %+v", out.MostPlayed, out.RecentlyPlayed)
	}
	if out.LastSyncStatus != "ok" || out.LastSync == "" {
		t.Errorf("last sync = %q %q", out.LastSync, out.LastSyncStatus)
	}
}

func TestMCPMoreFromDeveloper(t *testing.T) {
	search := &fakeSearch{games: []steam.StoreGame{{AppID: 1, Name: "Hollow Depths"}, {AppID: 99, Name: "Hollow Depths 2", ReviewDesc: "Very Positive"}}}
	srv, token := mcpServer(t, mcpStore(t), search)
	cs := connect(t, srv.URL, token)

	var out moreFromDeveloperOut
	if msg := call(t, cs, "more_from_developer", map[string]any{"appid": 3}, &out); msg != "" {
		t.Fatal(msg)
	}
	if out.Developer != "Team Cave" || len(out.Games) != 2 || !out.Games[0].Owned || out.Games[1].Owned || out.Games[1].ReviewRating != "Very Positive" {
		t.Errorf("more from = %+v", out)
	}
	if msg := call(t, cs, "more_from_developer", map[string]any{}, &out); !strings.Contains(msg, "give developer or appid") {
		t.Errorf("no args: %q", msg)
	}
}

func TestMCPTokenEndpoints(t *testing.T) {
	s := mcpStore(t)
	serve := func(h http.HandlerFunc, method, contentType string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/mcp-token", strings.NewReader("{}"))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		rec := httptest.NewRecorder()
		h(rec, asUser(req))
		return rec
	}
	var got mcpTokenJSON
	rec := serve(mcpTokenStatus(s), http.MethodGet, "")
	must(t, json.NewDecoder(rec.Body).Decode(&got))
	if got.Exists || got.Token != "" {
		t.Errorf("before: %+v", got)
	}

	if rec := serve(createMCPToken(s), http.MethodPost, "text/plain"); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("form post: status %d", rec.Code)
	}
	rec = serve(createMCPToken(s), http.MethodPost, "application/json")
	must(t, json.NewDecoder(rec.Body).Decode(&got))
	if !got.Exists || !strings.HasPrefix(got.Token, "slx_") || got.CreatedAt == nil {
		t.Fatalf("created: %+v", got)
	}
	info, err := verifyMCPToken(s)(context.Background(), got.Token, nil)
	if err != nil || info.UserID != "42" {
		t.Errorf("verify new token = %+v, %v", info, err)
	}

	rec = serve(mcpTokenStatus(s), http.MethodGet, "")
	var status mcpTokenJSON
	must(t, json.NewDecoder(rec.Body).Decode(&status))
	if !status.Exists || status.Token != "" {
		t.Errorf("status shows %+v; the token must not be readable again", status)
	}

	if rec := serve(deleteMCPToken(s), http.MethodDelete, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete: status %d", rec.Code)
	}
	if _, err := verifyMCPToken(s)(context.Background(), got.Token, nil); err == nil {
		t.Error("deleted token still verifies")
	}
}

func TestIsNewRelease(t *testing.T) {
	now := time.Date(2026, 11, 19, 23, 0, 0, 0, time.UTC)
	for date, want := range map[string]bool{
		"2026-11-19": true, "2026-05-19": true, "2026-05-18": false, "2026-11-20": false,
	} {
		d, _ := time.Parse(time.DateOnly, date)
		if got := isNewRelease(d, now); got != want {
			t.Errorf("%s: %v, want %v", date, got, want)
		}
	}
}
