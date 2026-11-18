package steam

import (
	"context"
	"net/http"
	"testing"
)

func TestDeveloperGames(t *testing.T) {
	serve := serveFile(t, "search_developer.json")
	var gotPath string
	var gotQuery map[string][]string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query()
		serve(w, r)
	})
	games, err := c.DeveloperGames(context.Background(), "Valve & Co")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/search/results/" || gotQuery["developer"][0] != "Valve & Co" || gotQuery["category1"][0] != "998" || gotQuery["infinite"][0] != "1" {
		t.Errorf("request %s %v", gotPath, gotQuery)
	}
	if len(games) != 2 {
		t.Fatalf("games = %+v, want 2 (the repeated row once)", games)
	}
	cs := games[0]
	if cs.AppID != 730 || cs.Name != "Counter-Strike 2" || cs.ReleasedText != "21 Aug, 2012" ||
		cs.CapsuleImage != "https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/730/capsule_231x87.jpg?t=1793054556" {
		t.Errorf("first = %+v", cs)
	}
	if cs.ReviewDesc != "Very Positive" || cs.ReviewPct == nil || *cs.ReviewPct != 85 || cs.ReviewsTotal == nil || *cs.ReviewsTotal != 2651619 {
		t.Errorf("reviews = %q %v %v", cs.ReviewDesc, cs.ReviewPct, cs.ReviewsTotal)
	}
	dod := games[1]
	if dod.AppID != 300 || dod.Name != "Day of Defeat: Source & More" || dod.ReleasedText != "Coming soon" || dod.ReviewDesc != "" || dod.ReviewPct != nil {
		t.Errorf("second = %+v", dod)
	}
}

func TestDeveloperGamesEmpty(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":1,"results_html":"\n<!-- List Items -->\n","total_count":0,"start":0}`))
	})
	games, err := c.DeveloperGames(context.Background(), "Nobody")
	if err != nil || len(games) != 0 {
		t.Errorf("games = %v, err = %v", games, err)
	}
}
