package syncer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/steam"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/store/postgres"
	"github.com/Adis/steamlib-explorer/backend/internal/store/sqlite"
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

// eachStore runs test against SQLite, and against Postgres too when
// TEST_DATABASE_URL points at a disposable database (e.g. `make test-db`).
// exec runs raw SQL that both engines accept.
func eachStore(t *testing.T, test func(t *testing.T, q store.Store, exec func(sql string))) {
	ctx := context.Background()
	prepare := func(t *testing.T, q store.Store) {
		if _, err := q.UpsertUser(ctx, store.UpsertUserParams{SteamID: int64(user)}); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("sqlite", func(t *testing.T) {
		st, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(st.Close)
		prepare(t, st)
		test(t, st, func(sql string) {
			t.Helper()
			if _, err := st.DB().ExecContext(ctx, sql); err != nil {
				t.Fatal(err)
			}
		})
	})
	t.Run("postgres", func(t *testing.T) {
		url := os.Getenv("TEST_DATABASE_URL")
		if url == "" {
			t.Skip("TEST_DATABASE_URL not set")
		}
		st, err := postgres.Open(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(st.Close)
		if _, err := st.Pool().Exec(ctx, "TRUNCATE users, apps CASCADE"); err != nil {
			t.Fatal(err)
		}
		prepare(t, st)
		test(t, st, func(sql string) {
			t.Helper()
			if _, err := st.Pool().Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
		})
	})
}

// games lists the user's games by appid.
func games(t *testing.T, q store.Store, p store.ListGamesParams) map[int64]store.ListGamesRow {
	t.Helper()
	p.SteamID, p.Sort, p.PageLimit = int64(user), "name", 200
	rows, err := q.ListGames(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	out := map[int64]store.ListGamesRow{}
	for _, r := range rows {
		out[r.Appid] = r
	}
	return out
}

func runToEnd(t *testing.T, s *Syncer) store.SyncRun {
	t.Helper()
	if _, err := s.Start(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	run, err := s.q.(store.Store).LatestSyncRun(context.Background(), int64(user))
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestRefresh(t *testing.T) {
	eachStore(t, func(t *testing.T, q store.Store, _ func(string)) {
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

		all := games(t, q, store.ListGamesParams{})
		g := all[10]
		if g.Name != "Store name" || g.DetailsStatus != "ok" || len(g.Genres) != 2 || g.ReviewPct == nil || *g.ReviewPct != 90 {
			t.Errorf("app 10 = %+v", g)
		}
		if all[20].DetailsStatus != "unavailable" {
			t.Errorf("app 20 status = %q", all[20].DetailsStatus)
		}
		if g.PlaytimeForever != 600 || g.LastPlayedAt == nil || !g.LastPlayedAt.Equal(played) {
			t.Errorf("owned game = %d %v", g.PlaytimeForever, g.LastPlayedAt)
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
	})
}

// Apps whose details came from an earlier Refresh but whose reviews never
// arrived get them on the next Refresh, without refetching details.
func TestRefreshBackfillsMissingReviews(t *testing.T) {
	eachStore(t, func(t *testing.T, q store.Store, exec func(string)) {
		ctx := context.Background()
		var owned []steam.OwnedGame
		for id := int64(1); id <= steam.MaxReviewBatch+5; id++ {
			owned = append(owned, steam.OwnedGame{AppID: id, Name: "g"})
		}
		f := &fakeSteam{games: owned}
		s := New(ctx, f, q)
		runToEnd(t, s)
		exec(`UPDATE apps SET reviews_fetched_at = NULL, reviews_total = NULL,
			reviews_positive = NULL, review_score = NULL, review_score_desc = NULL`)

		f.detailCalls, f.reviewCalls = nil, nil
		run := runToEnd(t, s)
		if run.Status != "ok" || len(f.detailCalls) != 0 {
			t.Fatalf("run = %+v, detail calls %v", run, f.detailCalls)
		}
		if len(f.reviewCalls) != 2 || len(f.reviewCalls[0]) != steam.MaxReviewBatch || len(f.reviewCalls[1]) != 5 {
			t.Errorf("review batches = %d", len(f.reviewCalls))
		}
		if n := len(games(t, q, store.ListGamesParams{MinReviewPct: ptr(90.0)})); n != len(owned) {
			t.Errorf("apps with reviews = %d, want %d", n, len(owned))
		}
	})
}

func TestRefreshPrivateLibrary(t *testing.T) {
	eachStore(t, func(t *testing.T, q store.Store, _ func(string)) {
		s := New(context.Background(), &fakeSteam{ownedErr: steam.ErrPrivateProfile}, q)
		run := runToEnd(t, s)
		if run.Status != "error" || run.Error == nil || run.FinishedAt == nil {
			t.Fatalf("run = %+v", run)
		}
	})
}

func TestRefreshStopsWhenRateLimited(t *testing.T) {
	eachStore(t, func(t *testing.T, q store.Store, _ func(string)) {
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
	})
}

func TestOneRefreshPerUser(t *testing.T) {
	eachStore(t, func(t *testing.T, q store.Store, _ func(string)) {
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
	})
}

func TestShutdownInterruptsRefresh(t *testing.T) {
	eachStore(t, func(t *testing.T, q store.Store, _ func(string)) {
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
	})
}
