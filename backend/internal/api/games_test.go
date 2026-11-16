package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

type fakeGameQ struct {
	got  store.ListGamesParams
	rows []store.ListGamesRow
}

func (f *fakeGameQ) ListGames(_ context.Context, p store.ListGamesParams) ([]store.ListGamesRow, error) {
	f.got = p
	return f.rows, nil
}

func (f *fakeGameQ) GenreFacets(context.Context, int64) ([]store.GenreFacetsRow, error) {
	return []store.GenreFacetsRow{{Value: "Action", Games: 2}}, nil
}

func (f *fakeGameQ) CategoryFacets(context.Context, int64) ([]store.CategoryFacetsRow, error) {
	return nil, nil
}

func TestParseGameFiltersDefaults(t *testing.T) {
	p, err := parseGameFilters(url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Sort != "review" || !p.Descending || p.PageLimit != 50 || p.PageOffset != 0 {
		t.Errorf("defaults = %+v", p)
	}
	if p.NameContains != nil || p.Genres != nil || p.Played != nil || p.MinReviews != nil || p.MinMinutes != nil || p.WindowsOnly {
		t.Errorf("filters should be off by default: %+v", p)
	}
}

func TestParseGameFilters(t *testing.T) {
	v, err := url.ParseQuery("q=50%25_off&genre=Action&genre=&genre=RPG&category=Co-op&played=true" +
		"&min_hours=1.5&max_hours=10&min_review=80&min_review_score=6&min_reviews=10&min_metacritic=75" +
		"&released_after=2015-01-01&released_before=2020-12-31&platform=linux&platform=mac" +
		"&sort=playtime&limit=20&offset=40")
	if err != nil {
		t.Fatal(err)
	}
	p, err := parseGameFilters(v)
	if err != nil {
		t.Fatal(err)
	}
	if *p.NameContains != `50%_off` {
		t.Errorf("name = %q", *p.NameContains)
	}
	if len(p.Genres) != 2 || p.Genres[1] != "RPG" || len(p.Categories) != 1 {
		t.Errorf("genres %v categories %v", p.Genres, p.Categories)
	}
	if !*p.Played || *p.MinMinutes != 90 || *p.MaxMinutes != 600 {
		t.Errorf("played/minutes = %v %d %d", *p.Played, *p.MinMinutes, *p.MaxMinutes)
	}
	if *p.MinReviewPct != 80 || *p.MinReviewScore != 6 || *p.MinMetacritic != 75 {
		t.Errorf("review/score/metacritic = %v %d %d", *p.MinReviewPct, *p.MinReviewScore, *p.MinMetacritic)
	}
	if *p.MinReviews != 10 {
		t.Errorf("min reviews = %d", *p.MinReviews)
	}
	if p.ReleasedAfter.Format(time.DateOnly) != "2015-01-01" || p.ReleasedBefore.Format(time.DateOnly) != "2020-12-31" {
		t.Errorf("dates = %v %v", p.ReleasedAfter, p.ReleasedBefore)
	}
	if p.WindowsOnly || !p.MacOnly || !p.LinuxOnly {
		t.Errorf("platforms = %v %v %v", p.WindowsOnly, p.MacOnly, p.LinuxOnly)
	}
	if p.Sort != "playtime" || !p.Descending {
		t.Errorf("number sorts default to biggest first: sort %q desc %v", p.Sort, p.Descending)
	}
	if p.PageLimit != 20 || p.PageOffset != 40 {
		t.Errorf("paging = %d/%d", p.PageLimit, p.PageOffset)
	}
}

func TestParseGameFiltersOrder(t *testing.T) {
	for query, want := range map[string]bool{
		"sort=name&order=desc":  true,
		"sort=review&order=asc": false,
		"sort=review":           true,
	} {
		v, _ := url.ParseQuery(query)
		p, err := parseGameFilters(v)
		if err != nil || p.Descending != want {
			t.Errorf("%s: descending = %v (%v), want %v", query, p.Descending, err, want)
		}
	}
}

func TestListGamesRejectsBadParams(t *testing.T) {
	for _, query := range []string{
		"played=maybe", "min_hours=-1", "max_hours=x", "min_review=101", "min_review_score=0", "min_review_score=10", "min_metacritic=7.5",
		"min_reviews=-1", "min_reviews=few",
		"released_after=2020", "platform=amiga", "sort=price", "order=up",
		"limit=0", "limit=201", "offset=-1",
	} {
		f := &fakeGameQ{}
		rec := httptest.NewRecorder()
		listGames(f)(rec, asUser(httptest.NewRequest(http.MethodGet, "/api/games?"+query, nil)))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", query, rec.Code)
		}
	}
}

func TestListGames(t *testing.T) {
	release := time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC)
	pct := 95.0
	f := &fakeGameQ{rows: []store.ListGamesRow{{
		Appid: 10, Name: "Alpha", ReleaseDate: &release, PlatformLinux: true,
		ReviewPct: &pct, DetailsStatus: "ok", PlaytimeForever: 600, TotalCount: 7,
		Genres: []string{"Action"}, Categories: []string{}, Developers: []string{}, Publishers: []string{},
	}}}
	rec := httptest.NewRecorder()
	listGames(f)(rec, asUser(httptest.NewRequest(http.MethodGet, "/api/games?limit=1&offset=3", nil)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if f.got.SteamID != 42 {
		t.Errorf("queried steam_id %d, want the signed-in user", f.got.SteamID)
	}
	var got gamesPageJSON
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 7 || got.Limit != 1 || got.Offset != 3 || len(got.Games) != 1 {
		t.Fatalf("page = %+v", got)
	}
	g := got.Games[0]
	if g.StoreURL != "https://store.steampowered.com/app/10" || *g.ReleaseDate != "2015-05-01" ||
		!g.Platforms.Linux || *g.ReviewPct != 95 || g.PlaytimeMinutes != 600 {
		t.Errorf("game = %+v", g)
	}
}

func TestListGamesEmpty(t *testing.T) {
	rec := httptest.NewRecorder()
	listGames(&fakeGameQ{})(rec, asUser(httptest.NewRequest(http.MethodGet, "/api/games", nil)))
	if body := rec.Body.String(); body != `{"games":[],"total":0,"limit":50,"offset":0}`+"\n" {
		t.Errorf("body = %s", body)
	}
}

func TestListFacets(t *testing.T) {
	rec := httptest.NewRecorder()
	listFacets(&fakeGameQ{})(rec, asUser(httptest.NewRequest(http.MethodGet, "/api/facets", nil)))
	want := `{"genres":[{"value":"Action","games":2}],"categories":[]}` + "\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Errorf("status %d body %s", rec.Code, rec.Body)
	}
}
