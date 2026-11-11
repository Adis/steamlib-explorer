package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/database"
)

// Runs only when TEST_DATABASE_URL points at a disposable Postgres database,
// e.g. `make test-db`.
func TestAuthStore(t *testing.T) {
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
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	s := AuthStore{Q: New(pool)}

	const id = uint64(76561197960287930)
	if err := s.UpsertUser(ctx, auth.User{SteamID: id, PersonaName: "Old name"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertUser(ctx, auth.User{SteamID: id, PersonaName: "New name", AvatarURL: "https://a/b.jpg"}); err != nil {
		t.Fatal(err)
	}

	live, expired := []byte("live-token-hash"), []byte("expired-token-hash")
	if err := s.CreateSession(ctx, live, id, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(ctx, expired, id, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}

	u, err := s.SessionUser(ctx, live)
	if err != nil {
		t.Fatal(err)
	}
	if u.SteamID != id || u.PersonaName != "New name" || u.AvatarURL != "https://a/b.jpg" {
		t.Errorf("SessionUser = %+v", u)
	}
	if _, err := s.SessionUser(ctx, expired); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("expired session: err = %v, want ErrNoSession", err)
	}
	if _, err := s.SessionUser(ctx, []byte("unknown")); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("unknown session: err = %v, want ErrNoSession", err)
	}

	if err := s.DeleteSession(ctx, live); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionUser(ctx, live); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("deleted session: err = %v, want ErrNoSession", err)
	}
}
