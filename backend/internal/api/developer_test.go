package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/steam"
)

type fakeSearch struct {
	calls int
	games []steam.StoreGame
	err   error
}

func (f *fakeSearch) DeveloperGames(_ context.Context, developer string) ([]steam.StoreGame, error) {
	f.calls++
	return f.games, f.err
}

type fakeOwned struct{ ids []int64 }

func (f fakeOwned) OwnedAmong(_ context.Context, _ int64, appids []int64) ([]int64, error) {
	return f.ids, nil
}

func TestListDeveloperGames(t *testing.T) {
	pct := 85
	search := &fakeSearch{games: []steam.StoreGame{
		{AppID: 620, Name: "Portal 2", CapsuleImage: "https://x/620.jpg", ReleasedText: "18 Apr, 2011", ReviewDesc: "Overwhelmingly Positive", ReviewPct: &pct},
		{AppID: 400, Name: "Portal"},
	}}
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	cache := &DeveloperCache{Search: search, now: func() time.Time { return now }}
	get := func(query string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		listDeveloperGames(cache, fakeOwned{ids: []int64{400}})(rec, asUser(httptest.NewRequest(http.MethodGet, "/api/developer-games?"+query, nil)))
		return rec
	}

	rec := get("developer=Valve")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		Developer string              `json:"developer"`
		Games     []developerGameJSON `json:"games"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Developer != "Valve" || len(out.Games) != 2 {
		t.Fatalf("out = %+v", out)
	}
	p2, p1 := out.Games[0], out.Games[1]
	if p2.Owned || !p1.Owned || p2.StoreURL != "https://store.steampowered.com/app/620" || *p2.ReviewPct != 85 || *p2.ReleasedText != "18 Apr, 2011" {
		t.Errorf("games = %+v", out.Games)
	}
	if p1.CapsuleImage != nil || p1.ReviewScoreDesc != nil {
		t.Errorf("empty fields should be null: %+v", p1)
	}

	// Cached for every spelling of the name, until the TTL passes.
	get("developer=valve")
	if search.calls != 1 {
		t.Errorf("store asked %d times, want 1 (cached)", search.calls)
	}
	now = now.Add(developerCacheTTL + time.Minute)
	get("developer=Valve")
	if search.calls != 2 {
		t.Errorf("store asked %d times after the TTL, want 2", search.calls)
	}

	if rec := get("developer=%20"); rec.Code != http.StatusBadRequest {
		t.Errorf("blank developer: status %d, want 400", rec.Code)
	}
	search.err = errors.New("boom")
	now = now.Add(developerCacheTTL + time.Minute)
	if rec := get("developer=Valve"); rec.Code != http.StatusBadGateway {
		t.Errorf("store down: status %d, want 502", rec.Code)
	}
}
