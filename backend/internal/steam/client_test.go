package steam

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient serves fixed responses by path and returns a client with no
// pacing or backoff so tests run instantly.
func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("test-key",
		WithBaseURLs(srv.URL, srv.URL),
		WithStoreRate(time.Nanosecond),
		WithBackoff(time.Millisecond, time.Millisecond),
	)
}

func serveFile(t *testing.T, name string) http.HandlerFunc {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

func TestOwnedGames(t *testing.T) {
	var gotQuery string
	serve := serveFile(t, "owned_games.json")
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		serve(w, r)
	})
	games, err := c.OwnedGames(context.Background(), 76561197960287930)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"steamid=76561197960287930", "include_appinfo=1", "include_played_free_games=1", "key=test-key"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q is missing %q", gotQuery, want)
		}
	}
	if len(games) != 2 {
		t.Fatalf("got %d games, want 2", len(games))
	}
	p := games[0]
	if p.AppID != 620 || p.Name != "Portal 2" || p.PlaytimeForever != 1234 || p.Playtime2Weeks != 60 {
		t.Errorf("unexpected game: %+v", p)
	}
	if p.LastPlayed == nil || p.LastPlayed.Unix() != 1730000000 {
		t.Errorf("LastPlayed = %v", p.LastPlayed)
	}
	if games[1].LastPlayed != nil {
		t.Errorf("never-played game has LastPlayed %v", games[1].LastPlayed)
	}
}

func TestOwnedGamesPrivate(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"response":{}}`))
	})
	_, err := c.OwnedGames(context.Background(), 1)
	if !errors.Is(err, ErrPrivateProfile) {
		t.Fatalf("err = %v, want ErrPrivateProfile", err)
	}
}

func TestOwnedGamesErrorHidesKey(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := c.OwnedGames(context.Background(), 1)
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusUnauthorized {
		t.Fatalf("err = %v, want StatusError 401", err)
	}
	if strings.Contains(err.Error(), "test-key") {
		t.Errorf("error leaks the API key: %v", err)
	}
}

func TestAppDetails(t *testing.T) {
	c := newTestClient(t, serveFile(t, "appdetails_620.json"))
	a, err := c.AppDetails(context.Background(), 620)
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Portal 2" || a.Type != "game" || a.IsFree {
		t.Errorf("basic fields: %+v", a)
	}
	if strings.Join(a.Genres, ",") != "Action,Adventure" {
		t.Errorf("Genres = %v", a.Genres)
	}
	if strings.Join(a.Categories, ",") != "Single-player,Co-op,PvP" {
		t.Errorf("Categories = %v", a.Categories)
	}
	if !a.Windows || a.Mac || !a.Linux {
		t.Errorf("platforms = %v %v %v", a.Windows, a.Mac, a.Linux)
	}
	if a.MetacriticScore == nil || *a.MetacriticScore != 95 {
		t.Errorf("MetacriticScore = %v", a.MetacriticScore)
	}
	if a.PriceCents == nil || *a.PriceCents != 199 || a.PriceCurrency != "USD" {
		t.Errorf("price = %v %s", a.PriceCents, a.PriceCurrency)
	}
	if a.ReleaseDate == nil || a.ReleaseDate.Format("2006-01-02") != "2011-04-18" {
		t.Errorf("ReleaseDate = %v", a.ReleaseDate)
	}
	if len(a.Raw) == 0 {
		t.Error("Raw is empty")
	}
}

func TestAppDetailsFreeAndUpcoming(t *testing.T) {
	c := newTestClient(t, serveFile(t, "appdetails_free_upcoming.json"))
	a, err := c.AppDetails(context.Background(), 999)
	if err != nil {
		t.Fatal(err)
	}
	if !a.IsFree || a.PriceCents != nil || a.MetacriticScore != nil {
		t.Errorf("free game fields: %+v", a)
	}
	if !a.ComingSoon || a.ReleaseDate != nil || a.ReleaseDateText != "Q3 2027" {
		t.Errorf("release: %v %v %q", a.ComingSoon, a.ReleaseDate, a.ReleaseDateText)
	}
	if a.Genres == nil || a.Developers == nil {
		t.Error("missing lists should be empty, not nil")
	}
}

func TestAppDetailsUnavailable(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"12345":{"success":false}}`))
	})
	_, err := c.AppDetails(context.Background(), 12345)
	if !errors.Is(err, ErrAppUnavailable) {
		t.Fatalf("err = %v, want ErrAppUnavailable", err)
	}
}

func TestAppReviews(t *testing.T) {
	var gotPath, gotQuery string
	serve := serveFile(t, "appreviews_620.json")
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		serve(w, r)
	})
	s, err := c.AppReviews(context.Background(), 620)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/appreviews/620" || !strings.Contains(gotQuery, "num_per_page=0") {
		t.Errorf("request = %s?%s", gotPath, gotQuery)
	}
	if s.Score != 9 || s.ScoreDesc != "Overwhelmingly Positive" || s.Positive != 398000 || s.TotalCount != 403000 {
		t.Errorf("summary = %+v", s)
	}
}

func TestStoreRetriesOnThrottle(t *testing.T) {
	var calls atomic.Int32
	body, err := os.ReadFile("testdata/appreviews_620.json")
	if err != nil {
		t.Fatal(err)
	}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write(body)
	})
	if _, err := c.AppReviews(context.Background(), 620); err != nil {
		t.Fatalf("err = %v, want success after 2 retries", err)
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3", calls.Load())
	}
}

func TestStoreGivesUpAfterRetries(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
	})
	_, err := c.AppDetails(context.Background(), 620)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	if calls.Load() != 3 { // first try + 2 retries (two backoff steps in the test client)
		t.Errorf("calls = %d, want 3", calls.Load())
	}
}

func TestStorePacing(t *testing.T) {
	srv := httptest.NewServer(serveFile(t, "appreviews_620.json"))
	defer srv.Close()
	c := New("", WithBaseURLs(srv.URL, srv.URL), WithStoreRate(50*time.Millisecond))

	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.AppReviews(context.Background(), 620); err != nil {
			t.Fatal(err)
		}
	}
	// The first request is immediate, the next two wait 50 ms each.
	if elapsed := time.Since(start); elapsed < 90*time.Millisecond {
		t.Errorf("3 requests took %v, want at least ~100ms", elapsed)
	}
}

func TestParseReleaseDate(t *testing.T) {
	tests := map[string]string{
		"18 Apr, 2011":  "2011-04-18",
		"Apr 18, 2011":  "2011-04-18",
		"18 Apr 2011":   "2011-04-18",
		"April 2011":    "2011-04-01",
		"2011":          "2011-01-01",
		"Q3 2027":       "",
		"Coming soon":   "",
		"":              "",
		"  9 Nov, 2026": "2026-11-09",
	}
	for in, want := range tests {
		got := parseReleaseDate(in)
		switch {
		case want == "" && got != nil:
			t.Errorf("%q: got %v, want nil", in, got)
		case want != "" && (got == nil || got.Format("2006-01-02") != want):
			t.Errorf("%q: got %v, want %s", in, got, want)
		}
	}
}
