package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/store/storetest"
)

// Runs only when TEST_DATABASE_URL points at a disposable Postgres database,
// e.g. `make test-db`. Every test starts by emptying it.
func TestStore(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	storetest.Run(t, func(t *testing.T) store.Store {
		s, err := Open(context.Background(), url)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(s.Close)
		if _, err := s.Pool().Exec(context.Background(), "TRUNCATE users, apps, developer_games CASCADE"); err != nil {
			t.Fatal(err)
		}
		return s
	})
}
