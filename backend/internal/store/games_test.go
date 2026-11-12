package store

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/database"
)

// Runs only when TEST_DATABASE_URL points at a disposable Postgres database,
// e.g. `make test-db`.
func TestListGamesAndFacets(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("TRUNCATE users, apps CASCADE")
	exec("INSERT INTO users (steam_id) VALUES (1), (2)")
	exec(`INSERT INTO apps (appid, name, genres, categories, platform_windows, platform_linux,
	                        metacritic_score, release_date, reviews_positive, reviews_total, review_score) VALUES
		(10, 'Alpha',      '{Action,Indie}', '{Single-player}',         true, true,  90, '2015-05-01', 95, 100, 7),
		(20, 'beta 100%',  '{Action}',       '{Single-player,Co-op}',   true, false, 70, '2020-01-01', 60, 100, 5),
		(30, 'Gamma',      '{RPG}',          '{Single-player}',         true, true,  NULL, NULL, NULL, NULL, NULL),
		(40, 'Not mine',   '{Action}',       '{}',                      true, true,  99, '2021-01-01', 1, 1, 9)`)
	exec(`INSERT INTO owned_games (steam_id, appid, playtime_forever, playtime_2weeks, last_played_at) VALUES
		(1, 10, 600, 0,  '2024-01-01'),
		(1, 20, 30,  30, '2025-01-01'),
		(1, 30, 0,   0,  NULL),
		(2, 40, 10,  0,  NULL)`)

	q := New(pool)
	list := func(p ListGamesParams) ([]int64, int64) {
		t.Helper()
		p.SteamID = 1
		if p.Sort == "" {
			p.Sort = "name"
		}
		if p.PageLimit == 0 {
			p.PageLimit = 50
		}
		rows, err := q.ListGames(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		var total int64
		for _, r := range rows {
			ids = append(ids, r.Appid)
			total = r.TotalCount
		}
		return ids, total
	}
	ptr := func(i int32) *int32 { return &i }
	f64 := func(f float64) *float64 { return &f }
	str := func(s string) *string { return &s }
	yes, no := true, false
	date := func(s string) *time.Time { d, _ := time.Parse(time.DateOnly, s); return &d }

	tests := []struct {
		name string
		p    ListGamesParams
		want []int64
	}{
		{"all, by name", ListGamesParams{}, []int64{10, 20, 30}},
		{"name desc", ListGamesParams{Descending: true}, []int64{30, 20, 10}},
		{"name search", ListGamesParams{NamePattern: str("%ET%")}, []int64{20}},
		{"literal percent", ListGamesParams{NamePattern: str(`%100\%%`)}, []int64{20}},
		{"genres must all match", ListGamesParams{Genres: []string{"Action", "Indie"}}, []int64{10}},
		{"category", ListGamesParams{Categories: []string{"Co-op"}}, []int64{20}},
		{"played", ListGamesParams{Played: &yes}, []int64{10, 20}},
		{"unplayed", ListGamesParams{Played: &no}, []int64{30}},
		{"playtime range", ListGamesParams{MinMinutes: ptr(1), MaxMinutes: ptr(60)}, []int64{20}},
		{"min review", ListGamesParams{MinReviewPct: f64(80)}, []int64{10}},
		{"min review score", ListGamesParams{MinReviewScore: ptr(6)}, []int64{10}},
		{"min review score mixed", ListGamesParams{MinReviewScore: ptr(5)}, []int64{10, 20}},
		{"min metacritic", ListGamesParams{MinMetacritic: ptr(75)}, []int64{10}},
		{"released between", ListGamesParams{ReleasedAfter: date("2016-01-01"), ReleasedBefore: date("2020-12-31")}, []int64{20}},
		{"linux", ListGamesParams{LinuxOnly: true}, []int64{10, 30}},
		{"mac", ListGamesParams{MacOnly: true}, nil},
		{"playtime desc", ListGamesParams{Sort: "playtime", Descending: true}, []int64{10, 20, 30}},
		{"recent desc", ListGamesParams{Sort: "recent", Descending: true}, []int64{20, 10, 30}},
		{"last played desc, never played last", ListGamesParams{Sort: "last_played", Descending: true}, []int64{20, 10, 30}},
		{"review asc, unknown last", ListGamesParams{Sort: "review"}, []int64{20, 10, 30}},
		{"metacritic desc", ListGamesParams{Sort: "metacritic", Descending: true}, []int64{10, 20, 30}},
		{"release asc", ListGamesParams{Sort: "release"}, []int64{10, 20, 30}},
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

	got, total := list(ListGamesParams{PageLimit: 1, PageOffset: 1})
	if !slices.Equal(got, []int64{20}) || total != 3 {
		t.Errorf("second page = %v (total %d), want [20] (total 3)", got, total)
	}

	genres, err := q.GenreFacets(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	wantGenres := []GenreFacetsRow{{"Action", 2}, {"Indie", 1}, {"RPG", 1}}
	if !slices.Equal(genres, wantGenres) {
		t.Errorf("genres = %v, want %v", genres, wantGenres)
	}
	cats, err := q.CategoryFacets(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	wantCats := []CategoryFacetsRow{{"Single-player", 3}, {"Co-op", 1}}
	if !slices.Equal(cats, wantCats) {
		t.Errorf("categories = %v, want %v", cats, wantCats)
	}
}
