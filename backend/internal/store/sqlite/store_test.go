package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/store/storetest"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

// Unlike Postgres, SQLite needs no server, so these always run.
func TestStore(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store { return open(t) })
}

// Reopening a file keeps its rows and applies no migration twice, and
// deleting a user removes their library (foreign keys are on).
func TestReopenAndCascade(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sub", "dir", "steamlib.db") // folders are created
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertUser(ctx, store.UpsertUserParams{SteamID: 7}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureApp(ctx, store.EnsureAppParams{Appid: 1, Name: "Game"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertOwnedGame(ctx, store.UpsertOwnedGameParams{SteamID: 7, Appid: 1}); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	stats, err := s.LibraryStats(ctx, 7)
	if err != nil || stats.Games != 1 {
		t.Fatalf("after reopen: stats = %+v, err = %v", stats, err)
	}
	if _, err := s.DB().ExecContext(ctx, "DELETE FROM users WHERE steam_id = 7"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.DB().QueryRowContext(ctx, "SELECT count(*) FROM owned_games").Scan(&n); err != nil || n != 0 {
		t.Errorf("owned_games after deleting the user = %d (err %v), want 0", n, err)
	}
}
