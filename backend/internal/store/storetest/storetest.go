// Package storetest checks that a store.Store implementation behaves the way
// the backend expects. Every engine runs the same tests.
package storetest

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

// Run runs every test. open must return an empty, migrated store.
func Run(t *testing.T, open func(t *testing.T) store.Store) {
	t.Run("Auth", func(t *testing.T) { testAuth(t, open(t)) })
	t.Run("ListGamesAndFacets", func(t *testing.T) { testListGames(t, open(t)) })
	t.Run("SyncRuns", func(t *testing.T) { testSyncRuns(t, open(t)) })
	t.Run("AppsNeedingRefresh", func(t *testing.T) { testAppsNeeding(t, open(t)) })
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func ptr[T any](v T) *T { return &v }

func date(s string) *time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return &d
}

func testAuth(t *testing.T, s store.Store) {
	ctx := context.Background()
	a := store.AuthStore{Q: s}

	const id = uint64(76561197960287930)
	must(t, a.UpsertUser(ctx, auth.User{SteamID: id, PersonaName: "Old name"}))
	must(t, a.UpsertUser(ctx, auth.User{SteamID: id, PersonaName: "New name", AvatarURL: "https://a/b.jpg"}))

	live, expired := []byte("live-token-hash"), []byte("expired-token-hash")
	must(t, a.CreateSession(ctx, live, id, time.Now().Add(time.Hour)))
	must(t, a.CreateSession(ctx, expired, id, time.Now().Add(-time.Minute)))

	u, err := a.SessionUser(ctx, live)
	must(t, err)
	if u.SteamID != id || u.PersonaName != "New name" || u.AvatarURL != "https://a/b.jpg" {
		t.Errorf("SessionUser = %+v", u)
	}
	if _, err := a.SessionUser(ctx, expired); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("expired session: err = %v, want ErrNoSession", err)
	}
	if _, err := a.SessionUser(ctx, []byte("unknown")); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("unknown session: err = %v, want ErrNoSession", err)
	}

	must(t, a.DeleteSession(ctx, live))
	if _, err := a.SessionUser(ctx, live); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("deleted session: err = %v, want ErrNoSession", err)
	}
}

type app struct {
	id              int64
	name            string
	genres, cats    []string
	windows, linux  bool
	metacritic      *int32
	released        *time.Time
	positive, total int32 // reviews; total 0 means none fetched
	reviewScore     int32
	owner           int64
	minutes, recent int32
	lastPlayed      *time.Time
}

// seed adds apps with store details and reviews, owned by their owner.
func seed(t *testing.T, s store.Store, apps []app) {
	t.Helper()
	ctx := context.Background()
	for _, id := range []int64{1, 2} {
		_, err := s.UpsertUser(ctx, store.UpsertUserParams{SteamID: id})
		must(t, err)
	}
	for _, a := range apps {
		must(t, s.EnsureApp(ctx, store.EnsureAppParams{Appid: a.id, Name: "Library name"}))
		must(t, s.UpdateAppDetails(ctx, store.UpdateAppDetailsParams{
			Appid: a.id, Name: a.name, Genres: a.genres, Categories: a.cats,
			Developers: []string{"Dev"}, Publishers: []string{},
			PlatformWindows: a.windows, PlatformLinux: a.linux,
			MetacriticScore: a.metacritic, ReleaseDate: a.released,
			DetailsRaw: []byte(`{"name":"` + a.name + `"}`),
		}))
		if a.total > 0 {
			must(t, s.UpdateAppReviews(ctx, store.UpdateAppReviewsParams{
				Appid: a.id, ReviewScore: ptr(a.reviewScore), ReviewScoreDesc: ptr("Mixed"),
				ReviewsPositive: ptr(a.positive), ReviewsTotal: ptr(a.total),
			}))
		}
		must(t, s.UpsertOwnedGame(ctx, store.UpsertOwnedGameParams{
			SteamID: a.owner, Appid: a.id, PlaytimeForever: a.minutes, Playtime2weeks: a.recent, LastPlayedAt: a.lastPlayed,
		}))
	}
}

func testListGames(t *testing.T, s store.Store) {
	ctx := context.Background()
	seed(t, s, []app{
		{id: 10, name: "Alpha", genres: []string{"Action", "Indie"}, cats: []string{"Single-player"}, windows: true, linux: true,
			metacritic: ptr[int32](90), released: date("2015-05-01"), positive: 95, total: 100, reviewScore: 7,
			owner: 1, minutes: 600, lastPlayed: date("2024-01-01")},
		{id: 20, name: "beta 100%", genres: []string{"Action"}, cats: []string{"Single-player", "Co-op"}, windows: true,
			metacritic: ptr[int32](70), released: date("2020-01-01"), positive: 60, total: 100, reviewScore: 5,
			owner: 1, minutes: 30, recent: 30, lastPlayed: date("2025-01-01")},
		{id: 30, name: "Gamma", genres: []string{"RPG"}, cats: []string{"Single-player"}, windows: true, linux: true, owner: 1},
		{id: 40, name: "Not mine", genres: []string{"Action"}, cats: []string{}, windows: true, linux: true,
			metacritic: ptr[int32](99), released: date("2021-01-01"), positive: 1, total: 1, reviewScore: 9, owner: 2, minutes: 10},
	})

	list := func(p store.ListGamesParams) ([]int64, int64) {
		t.Helper()
		p.SteamID = 1
		if p.Sort == "" {
			p.Sort = "name"
		}
		if p.PageLimit == 0 {
			p.PageLimit = 50
		}
		rows, err := s.ListGames(ctx, p)
		must(t, err)
		var ids []int64
		var total int64
		for _, r := range rows {
			ids = append(ids, r.Appid)
			total = r.TotalCount
		}
		return ids, total
	}
	yes, no := true, false

	tests := []struct {
		name string
		p    store.ListGamesParams
		want []int64
	}{
		{"all, by name", store.ListGamesParams{}, []int64{10, 20, 30}},
		{"name desc", store.ListGamesParams{Descending: true}, []int64{30, 20, 10}},
		{"name search ignores case", store.ListGamesParams{NameContains: ptr("ET")}, []int64{20}},
		{"percent is literal", store.ListGamesParams{NameContains: ptr("100%")}, []int64{20}},
		{"underscore is literal", store.ListGamesParams{NameContains: ptr("a_")}, nil},
		{"genres must all match", store.ListGamesParams{Genres: []string{"Action", "Indie"}}, []int64{10}},
		{"category", store.ListGamesParams{Categories: []string{"Co-op"}}, []int64{20}},
		{"played", store.ListGamesParams{Played: &yes}, []int64{10, 20}},
		{"unplayed", store.ListGamesParams{Played: &no}, []int64{30}},
		{"playtime range", store.ListGamesParams{MinMinutes: ptr[int32](1), MaxMinutes: ptr[int32](60)}, []int64{20}},
		{"min review", store.ListGamesParams{MinReviewPct: ptr(80.0)}, []int64{10}},
		{"min review score", store.ListGamesParams{MinReviewScore: ptr[int32](6)}, []int64{10}},
		{"min review score mixed", store.ListGamesParams{MinReviewScore: ptr[int32](5)}, []int64{10, 20}},
		{"min metacritic", store.ListGamesParams{MinMetacritic: ptr[int32](75)}, []int64{10}},
		{"released between", store.ListGamesParams{ReleasedAfter: date("2016-01-01"), ReleasedBefore: date("2020-12-31")}, []int64{20}},
		{"released on the boundary days", store.ListGamesParams{ReleasedAfter: date("2020-01-01"), ReleasedBefore: date("2020-01-01")}, []int64{20}},
		{"linux", store.ListGamesParams{LinuxOnly: true}, []int64{10, 30}},
		{"mac", store.ListGamesParams{MacOnly: true}, nil},
		{"playtime desc", store.ListGamesParams{Sort: "playtime", Descending: true}, []int64{10, 20, 30}},
		{"recent desc", store.ListGamesParams{Sort: "recent", Descending: true}, []int64{20, 10, 30}},
		{"last played desc, never played last", store.ListGamesParams{Sort: "last_played", Descending: true}, []int64{20, 10, 30}},
		{"review asc, unknown last", store.ListGamesParams{Sort: "review"}, []int64{20, 10, 30}},
		{"review desc, unknown last", store.ListGamesParams{Sort: "review", Descending: true}, []int64{10, 20, 30}},
		{"metacritic desc", store.ListGamesParams{Sort: "metacritic", Descending: true}, []int64{10, 20, 30}},
		{"release asc", store.ListGamesParams{Sort: "release"}, []int64{10, 20, 30}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, total := list(tt.p)
			if !slices.Equal(got, tt.want) {
				t.Errorf("appids = %v, want %v", got, tt.want)
			}
			if total != int64(len(tt.want)) {
				t.Errorf("total = %d, want %d", total, len(tt.want))
			}
		})
	}

	got, total := list(store.ListGamesParams{PageLimit: 1, PageOffset: 1})
	if !slices.Equal(got, []int64{20}) || total != 3 {
		t.Errorf("second page = %v (total %d), want [20] (total 3)", got, total)
	}

	rows, err := s.ListGames(ctx, store.ListGamesParams{SteamID: 1, Sort: "name", PageLimit: 1})
	must(t, err)
	r := rows[0]
	if r.Name != "Alpha" || !slices.Equal(r.Genres, []string{"Action", "Indie"}) || !slices.Equal(r.Developers, []string{"Dev"}) ||
		r.Publishers == nil || len(r.Publishers) != 0 || r.ReviewPct == nil || *r.ReviewPct != 95 ||
		r.ReleaseDate == nil || !r.ReleaseDate.Equal(*date("2015-05-01")) ||
		r.LastPlayedAt == nil || !r.LastPlayedAt.Equal(*date("2024-01-01")) || r.PlaytimeForever != 600 || r.DetailsStatus != "ok" {
		t.Errorf("first row = %+v", r)
	}

	genres, err := s.GenreFacets(ctx, 1)
	must(t, err)
	wantGenres := []store.GenreFacetsRow{{Value: "Action", Games: 2}, {Value: "Indie", Games: 1}, {Value: "RPG", Games: 1}}
	if !slices.Equal(genres, wantGenres) {
		t.Errorf("genres = %v, want %v", genres, wantGenres)
	}
	cats, err := s.CategoryFacets(ctx, 1)
	must(t, err)
	wantCats := []store.CategoryFacetsRow{{Value: "Single-player", Games: 3}, {Value: "Co-op", Games: 1}}
	if !slices.Equal(cats, wantCats) {
		t.Errorf("categories = %v, want %v", cats, wantCats)
	}
}

func testSyncRuns(t *testing.T, s store.Store) {
	ctx := context.Background()
	_, err := s.UpsertUser(ctx, store.UpsertUserParams{SteamID: 1})
	must(t, err)

	if _, err := s.LatestSyncRun(ctx, 1); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("LatestSyncRun before any: err = %v, want ErrNotFound", err)
	}
	first, err := s.StartSyncRun(ctx, 1)
	must(t, err)
	if _, err := s.StartSyncRun(ctx, 1); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("second running sync: err = %v, want ErrConflict", err)
	}

	must(t, s.UpdateSyncProgress(ctx, store.UpdateSyncProgressParams{ID: first, GamesSeen: 5, DetailsTotal: 4, DetailsFetched: 1}))
	run, err := s.LatestSyncRun(ctx, 1)
	must(t, err)
	if run.ID != first || run.Status != "running" || run.GamesSeen != 5 || run.DetailsTotal != 4 || run.DetailsFetched != 1 ||
		run.DetailsStartedAt == nil || run.FinishedAt != nil || time.Since(run.StartedAt) > time.Minute {
		t.Errorf("running = %+v", run)
	}
	started := *run.DetailsStartedAt
	time.Sleep(5 * time.Millisecond)
	must(t, s.UpdateSyncProgress(ctx, store.UpdateSyncProgressParams{ID: first, GamesSeen: 5, DetailsTotal: 4, DetailsFetched: 2}))
	run, err = s.LatestSyncRun(ctx, 1)
	must(t, err)
	if !run.DetailsStartedAt.Equal(started) {
		t.Errorf("details_started_at moved from %v to %v", started, run.DetailsStartedAt)
	}

	must(t, s.FinishSyncRun(ctx, store.FinishSyncRunParams{ID: first, Status: "error", Error: ptr("Steam said no")}))
	run, err = s.LatestSyncRun(ctx, 1)
	must(t, err)
	if run.Status != "error" || run.Error == nil || *run.Error != "Steam said no" || run.FinishedAt == nil {
		t.Errorf("finished = %+v", run)
	}

	time.Sleep(5 * time.Millisecond) // latest is by start time
	second, err := s.StartSyncRun(ctx, 1)
	must(t, err)
	n, err := s.FailRunningSyncs(ctx)
	must(t, err)
	if n != 1 {
		t.Errorf("FailRunningSyncs = %d, want 1", n)
	}
	run, err = s.LatestSyncRun(ctx, 1)
	must(t, err)
	if run.ID != second || run.Status != "error" || run.Error == nil {
		t.Errorf("interrupted = %+v", run)
	}
}

func testAppsNeeding(t *testing.T, s store.Store) {
	ctx := context.Background()
	_, err := s.UpsertUser(ctx, store.UpsertUserParams{SteamID: 1})
	must(t, err)
	for _, id := range []int64{1, 2, 3} {
		must(t, s.EnsureApp(ctx, store.EnsureAppParams{Appid: id, Name: "Game"}))
		must(t, s.UpsertOwnedGame(ctx, store.UpsertOwnedGameParams{SteamID: 1, Appid: id}))
	}
	// EnsureApp keeps an existing app's details.
	must(t, s.UpdateAppDetails(ctx, store.UpdateAppDetailsParams{Appid: 1, Name: "Store name", Genres: []string{}, Categories: []string{}, Developers: []string{}, Publishers: []string{}}))
	must(t, s.EnsureApp(ctx, store.EnsureAppParams{Appid: 1, Name: "Library name"}))
	must(t, s.MarkAppDetailsStatus(ctx, store.MarkAppDetailsStatusParams{Appid: 2, DetailsStatus: "unavailable"}))

	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)

	ids, err := s.AppsNeedingDetails(ctx, store.AppsNeedingDetailsParams{SteamID: 1, StaleBefore: past})
	must(t, err)
	if !slices.Equal(ids, []int64{3}) {
		t.Errorf("needing details (fresh ones skipped) = %v, want [3]", ids)
	}
	ids, err = s.AppsNeedingDetails(ctx, store.AppsNeedingDetailsParams{SteamID: 1, StaleBefore: future})
	must(t, err)
	if len(ids) != 3 || ids[0] != 3 {
		t.Errorf("needing details (all stale) = %v, want 3 first, then 1 and 2", ids)
	}

	ids, err = s.AppsNeedingReviews(ctx, store.AppsNeedingReviewsParams{SteamID: 1, StaleBefore: past})
	must(t, err)
	if !slices.Equal(ids, []int64{1}) {
		t.Errorf("needing reviews = %v, want [1] (the only app with store details)", ids)
	}
	must(t, s.UpdateAppReviews(ctx, store.UpdateAppReviewsParams{Appid: 1, ReviewsPositive: ptr[int32](1), ReviewsTotal: ptr[int32](2)}))
	ids, err = s.AppsNeedingReviews(ctx, store.AppsNeedingReviewsParams{SteamID: 1, StaleBefore: past})
	must(t, err)
	if len(ids) != 0 {
		t.Errorf("needing reviews after fetch = %v, want none", ids)
	}

	stats, err := s.LibraryStats(ctx, 1)
	must(t, err)
	if stats != (store.LibraryStatsRow{Games: 3, WithDetails: 1, Unavailable: 1, Failed: 0, WithReviews: 1}) {
		t.Errorf("stats = %+v", stats)
	}
	rows, err := s.ListGames(ctx, store.ListGamesParams{SteamID: 1, Sort: "name", PageLimit: 10})
	must(t, err)
	if len(rows) != 3 || rows[0].Name != "Game" || rows[2].Name != "Store name" || rows[2].ReviewPct == nil || *rows[2].ReviewPct != 50 {
		t.Errorf("rows = %+v", rows)
	}
}
