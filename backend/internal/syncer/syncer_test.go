package syncer

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Adis/steamlib-explorer/backend/internal/database"
	"github.com/Adis/steamlib-explorer/backend/internal/steam"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

const user = uint64(76561197960287930)

type fakeSteam struct {
	mu          sync.Mutex
	games       []steam.OwnedGame
	ownedErr    error
	unavailable map[int64]bool
	rateLimitOn int64 // appid whose details answer ErrRateLimited
	detailCalls []int64
	reviewCalls [][]int64
	block       chan struct{} // when set, AppDetails waits on it
}

func (f *fakeSteam) OwnedGames(context.Context, uint64) ([]steam.OwnedGame, error) {
	return f.games, f.ownedErr
}

func (f *fakeSteam) AppDetails(ctx context.Context, id int64) (*steam.AppDetails, error) {
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f.mu.Lock()
	f.detailCalls = append(f.detailCalls, id)
	f.mu.Unlock()
	if f.unavailable[id] {
		return nil, steam.ErrAppUnavailable
	}
	if id == f.rateLimitOn {
		return nil, steam.ErrRateLimited
	}
	mc := 90
	return &steam.AppDetails{
		AppID: id, Name: "Store name", Type: "game", Genres: []string{"RPG", "Indie"},
		Categories: []string{"Single-player"}, Developers: []string{}, Publishers: []string{},
		Windows: true, MetacriticScore: &mc, Raw: []byte(`{"name":"Store name"}`),
	}, nil
}

func (f *fakeSteam) AppReviews(_ context.Context, ids []int64) (map[int64]steam.ReviewSummary, error) {
	f.mu.Lock()
	f.reviewCalls = append(f.reviewCalls, ids)
	f.mu.Unlock()
	out := map[int64]steam.ReviewSummary{}
	for _, id := range ids {
		out[id] = steam.ReviewSummary{Score: 8, ScoreDesc: "Very Positive", Positive: 90, TotalCount: 100}
	}
	return out, nil
}

func setup(t *testing.T) (*pgxpool.Pool, *store.Queries) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users, apps CASCADE"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO users (steam_id) VALUES ($1)", int64(user)); err != nil {
		t.Fatal(err)
	}
	return pool, store.New(pool)
}

func runToEnd(t *testing.T, s *Syncer) store.SyncRun {
	t.Helper()
	if _, err := s.Start(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	run, err := s.q.(*store.Queries).LatestSyncRun(context.Background(), int64(user))
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestRefresh(t *testing.T) {
	pool, q := setup(t)
	ctx := context.Background()
	played := time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC)
	f := &fakeSteam{
		games: []steam.OwnedGame{
			{AppID: 10, Name: "Library name", PlaytimeForever: 600, LastPlayed: &played},
			{AppID: 20, Name: "Delisted game"},
		},
		unavailable: map[int64]bool{20: true},
	}
	s := New(ctx, f, q)

	run := runToEnd(t, s)
	if run.Status != "ok" || run.GamesSeen != 2 || run.DetailsTotal != 2 || run.DetailsFetched != 2 {
		t.Fatalf("run = %+v", run)
	}

	var name, status string
	var genres []string
	var pct float64
	if err := pool.QueryRow(ctx, "SELECT name, details_status, genres, review_pct::float8 FROM apps WHERE appid = 10").Scan(&name, &status, &genres, &pct); err != nil {
		t.Fatal(err)
	}
	if name != "Store name" || status != "ok" || len(genres) != 2 || pct != 90 {
		t.Errorf("app 10 = %q %q %v %v", name, status, genres, pct)
	}
	if err := pool.QueryRow(ctx, "SELECT details_status FROM apps WHERE appid = 20").Scan(&status); err != nil || status != "unavailable" {
		t.Errorf("app 20 status = %q, %v", status, err)
	}
	var minutes int
	var last time.Time
	if err := pool.QueryRow(ctx, "SELECT playtime_forever, last_played_at FROM owned_games WHERE appid = 10").Scan(&minutes, &last); err != nil {
		t.Fatal(err)
	}
	if minutes != 600 || !last.Equal(played) {
		t.Errorf("owned game = %d %v", minutes, last)
	}

	stats, err := q.LibraryStats(ctx, int64(user))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Games != 2 || stats.WithDetails != 1 || stats.Unavailable != 1 || stats.WithReviews != 1 {
		t.Errorf("stats = %+v", stats)
	}

	// Reviews are fetched in one batch, only for the app with a store page.
	if len(f.reviewCalls) != 1 || len(f.reviewCalls[0]) != 1 || f.reviewCalls[0][0] != 10 {
		t.Errorf("review calls = %v", f.reviewCalls)
	}

	// A second Refresh doesn't refetch fresh store data.
	f.detailCalls, f.reviewCalls = nil, nil
	run = runToEnd(t, s)
	if run.Status != "ok" || run.DetailsTotal != 0 || len(f.detailCalls) != 0 || len(f.reviewCalls) != 0 {
		t.Errorf("second run = %+v, detail calls %v, review calls %v", run, f.detailCalls, f.reviewCalls)
	}
}

// Apps whose details came from an earlier Refresh but whose reviews never
// arrived get them on the next Refresh, without refetching details.
func TestRefreshBackfillsMissingReviews(t *testing.T) {
	pool, q := setup(t)
	ctx := context.Background()
	var games []steam.OwnedGame
	for id := int64(1); id <= steam.MaxReviewBatch+5; id++ {
		games = append(games, steam.OwnedGame{AppID: id, Name: "g"})
	}
	f := &fakeSteam{games: games}
	s := New(ctx, f, q)
	runToEnd(t, s)
	if _, err := pool.Exec(ctx, `UPDATE apps SET reviews_fetched_at = NULL, reviews_total = NULL,
		reviews_positive = NULL, review_score = NULL, review_score_desc = NULL`); err != nil {
		t.Fatal(err)
	}

	f.detailCalls, f.reviewCalls = nil, nil
	run := runToEnd(t, s)
	if run.Status != "ok" || len(f.detailCalls) != 0 {
		t.Fatalf("run = %+v, detail calls %v", run, f.detailCalls)
	}
	if len(f.reviewCalls) != 2 || len(f.reviewCalls[0]) != steam.MaxReviewBatch || len(f.reviewCalls[1]) != 5 {
		t.Errorf("review batches = %d", len(f.reviewCalls))
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM apps WHERE review_pct = 90").Scan(&n); err != nil || n != len(games) {
		t.Errorf("apps with reviews = %d, %v", n, err)
	}
}

func TestRefreshPrivateLibrary(t *testing.T) {
	_, q := setup(t)
	s := New(context.Background(), &fakeSteam{ownedErr: steam.ErrPrivateProfile}, q)
	run := runToEnd(t, s)
	if run.Status != "error" || run.Error == nil || run.FinishedAt == nil {
		t.Fatalf("run = %+v", run)
	}
}

func TestRefreshStopsWhenRateLimited(t *testing.T) {
	_, q := setup(t)
	f := &fakeSteam{
		games:       []steam.OwnedGame{{AppID: 1, Name: "a"}, {AppID: 2, Name: "b"}, {AppID: 3, Name: "c"}},
		rateLimitOn: 2,
	}
	run := runToEnd(t, New(context.Background(), f, q))
	if run.Status != "error" || run.DetailsFetched != 1 || run.DetailsTotal != 3 {
		t.Fatalf("run = %+v", run)
	}
	if len(f.detailCalls) != 2 {
		t.Errorf("detail calls = %v, want it to stop at app 2", f.detailCalls)
	}
}

func TestOneRefreshPerUser(t *testing.T) {
	_, q := setup(t)
	f := &fakeSteam{games: []steam.OwnedGame{{AppID: 1, Name: "a"}}, block: make(chan struct{})}
	s := New(context.Background(), f, q)
	if _, err := s.Start(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(context.Background(), user); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Start err = %v, want ErrAlreadyRunning", err)
	}
	close(f.block)
	s.Wait()
	if _, err := s.Start(context.Background(), user); err != nil {
		t.Errorf("Start after finish: %v", err)
	}
	s.Wait()
}

func TestShutdownInterruptsRefresh(t *testing.T) {
	_, q := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakeSteam{games: []steam.OwnedGame{{AppID: 1, Name: "a"}}, block: make(chan struct{})}
	s := New(ctx, f, q)
	if _, err := s.Start(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	cancel()
	s.Wait()
	run, err := q.LatestSyncRun(context.Background(), int64(user))
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "error" || run.Error == nil {
		t.Errorf("run = %+v, want an interrupted error", run)
	}
}
