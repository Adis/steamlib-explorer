package transfer

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/store/postgres"
	"github.com/Adis/steamlib-explorer/backend/internal/store/sqlite"
)

func ptr[T any](v T) *T { return &v }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// openPostgres returns an empty Postgres store; it needs TEST_DATABASE_URL
// (e.g. `make test-db`).
func openPostgres(t *testing.T) *postgres.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pg, err := postgres.Open(context.Background(), url)
	must(t, err)
	t.Cleanup(pg.Close)
	_, err = pg.Pool().Exec(context.Background(), "TRUNCATE users, apps CASCADE")
	must(t, err)
	return pg
}

func openSQLite(t *testing.T) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "copy.db"))
	must(t, err)
	t.Cleanup(s.Close)
	return s
}

// The copy reads the same through the Store interface as the original.
func TestPostgresToSQLite(t *testing.T) {
	ctx := context.Background()
	pg := openPostgres(t)

	const user = int64(76561197960287930)
	_, err := pg.UpsertUser(ctx, store.UpsertUserParams{SteamID: user, PersonaName: ptr("Gamer"), AvatarUrl: ptr("https://a/b.jpg")})
	must(t, err)
	_, err = pg.UpsertUser(ctx, store.UpsertUserParams{SteamID: 2})
	must(t, err)
	must(t, pg.CreateSession(ctx, store.CreateSessionParams{TokenHash: []byte{0, 1, 255}, SteamID: user, ExpiresAt: time.Now().Add(time.Hour)}))

	played := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	must(t, pg.EnsureApp(ctx, store.EnsureAppParams{Appid: 10, Name: "Library name"}))
	must(t, pg.UpdateAppDetails(ctx, store.UpdateAppDetailsParams{
		Appid: 10, Name: "Élan, \"quoted\"", Type: ptr("game"), ShortDescription: ptr("Line one\nline two"),
		HeaderImage: ptr("https://img/10.jpg"), ReleaseDate: ptr(time.Date(2015, 5, 1, 0, 0, 0, 0, time.UTC)),
		ReleaseDateText: ptr("1 May, 2015"), IsFree: true,
		Developers: []string{"Dev, Inc."}, Publishers: []string{}, Genres: []string{"Action", "Indie"},
		Categories: []string{"Single-player"}, PlatformWindows: true, PlatformLinux: true,
		MetacriticScore: ptr[int32](88), PriceCents: ptr[int32](1999), PriceCurrency: ptr("EUR"),
		DetailsRaw: []byte(`{"name": "Élan"}`),
	}))
	must(t, pg.UpdateAppReviews(ctx, store.UpdateAppReviewsParams{
		Appid: 10, ReviewScore: ptr[int32](8), ReviewScoreDesc: ptr("Very Positive"), ReviewsPositive: ptr[int32](2), ReviewsTotal: ptr[int32](3),
	}))
	must(t, pg.EnsureApp(ctx, store.EnsureAppParams{Appid: 20, Name: "No store page"}))
	must(t, pg.MarkAppDetailsStatus(ctx, store.MarkAppDetailsStatusParams{Appid: 20, DetailsStatus: "unavailable"}))
	must(t, pg.EnsureApp(ctx, store.EnsureAppParams{Appid: 30, Name: "Not fetched yet"}))
	must(t, pg.UpsertOwnedGame(ctx, store.UpsertOwnedGameParams{SteamID: user, Appid: 10, PlaytimeForever: 600, Playtime2weeks: 30, LastPlayedAt: &played}))
	must(t, pg.UpsertOwnedGame(ctx, store.UpsertOwnedGameParams{SteamID: user, Appid: 20}))
	must(t, pg.UpsertOwnedGame(ctx, store.UpsertOwnedGameParams{SteamID: user, Appid: 30}))
	must(t, pg.UpsertOwnedGame(ctx, store.UpsertOwnedGameParams{SteamID: 2, Appid: 10}))
	run, err := pg.StartSyncRun(ctx, user)
	must(t, err)
	must(t, pg.UpdateSyncProgress(ctx, store.UpdateSyncProgressParams{ID: run, GamesSeen: 3, DetailsTotal: 2, DetailsFetched: 2}))
	must(t, pg.FinishSyncRun(ctx, store.FinishSyncRunParams{ID: run, Status: "ok"}))

	lite := openSQLite(t)
	counts, err := PostgresToSQLite(ctx, pg.Pool(), lite.DB())
	must(t, err)
	want := Counts{"users": 2, "sessions": 1, "apps": 3, "owned_games": 4, "sync_runs": 1}
	if !reflect.DeepEqual(counts, want) {
		t.Errorf("counts = %v, want %v", counts, want)
	}

	// Postgres keeps microseconds, the SQLite copy milliseconds.
	ms := func(t *time.Time) *time.Time {
		if t == nil {
			return nil
		}
		v := t.Truncate(time.Millisecond).UTC()
		return &v
	}
	for _, steamID := range []int64{user, 2} {
		p := store.ListGamesParams{SteamID: steamID, Sort: "name", PageLimit: 50}
		a, err := pg.ListGames(ctx, p)
		must(t, err)
		b, err := lite.ListGames(ctx, p)
		must(t, err)
		for i := range a {
			a[i].LastPlayedAt = ms(a[i].LastPlayedAt)
			b[i].LastPlayedAt = ms(b[i].LastPlayedAt)
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("user %d games differ:\npostgres %+v\nsqlite   %+v", steamID, a, b)
		}
	}
	for name, get := range map[string]func(store.Store) (any, error){
		"stats":      func(s store.Store) (any, error) { return s.LibraryStats(ctx, user) },
		"genres":     func(s store.Store) (any, error) { return s.GenreFacets(ctx, user) },
		"categories": func(s store.Store) (any, error) { return s.CategoryFacets(ctx, user) },
		"user": func(s store.Store) (any, error) {
			u, err := s.GetSessionUser(ctx, []byte{0, 1, 255})
			u.CreatedAt, u.LastLoginAt = *ms(&u.CreatedAt), *ms(&u.LastLoginAt)
			return u, err
		},
		"sync run": func(s store.Store) (any, error) {
			r, err := s.LatestSyncRun(ctx, user)
			r.StartedAt, r.FinishedAt, r.DetailsStartedAt = *ms(&r.StartedAt), ms(r.FinishedAt), ms(r.DetailsStartedAt)
			return r, err
		},
	} {
		a, err := get(pg)
		must(t, err)
		b, err := get(lite)
		must(t, err)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s differs:\npostgres %+v\nsqlite   %+v", name, a, b)
		}
	}

	// The copy keeps working: the next sync run gets a new id, and fresh apps
	// aren't refetched.
	next, err := lite.StartSyncRun(ctx, user)
	must(t, err)
	if next <= run {
		t.Errorf("next sync run id = %d, want more than %d", next, run)
	}
	ids, err := lite.AppsNeedingDetails(ctx, store.AppsNeedingDetailsParams{SteamID: user, StaleBefore: time.Now().Add(-time.Hour)})
	must(t, err)
	if !slices.Equal(ids, []int64{30}) {
		t.Errorf("apps needing details after the copy = %v, want [30]", ids)
	}

	// A second import into the same file is refused and changes nothing.
	if _, err := PostgresToSQLite(ctx, pg.Pool(), lite.DB()); err == nil {
		t.Error("second import into a non-empty file should fail")
	}
}

// Every Postgres column is either copied or generated, so a future migration
// that adds a column fails here until the copy learns about it.
func TestCopiesEveryColumn(t *testing.T) {
	pg := openPostgres(t)
	rows, err := pg.Pool().Query(context.Background(), `
		SELECT table_name, column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name <> 'goose_db_version' AND is_generated = 'NEVER'`)
	must(t, err)
	defer rows.Close()
	listed := map[string]bool{}
	for _, tb := range tables {
		for _, c := range tb.columns {
			listed[tb.name+"."+c.name] = true
		}
	}
	for rows.Next() {
		var table, col string
		must(t, rows.Scan(&table, &col))
		if !listed[table+"."+col] {
			t.Errorf("Postgres column %s.%s is not copied to SQLite; add it to tables in transfer.go", table, col)
		}
	}
	must(t, rows.Err())
}
