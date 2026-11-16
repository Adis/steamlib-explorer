// Command pg2sqlite copies a Steamlib Explorer Postgres database into a new
// SQLite file, so a synced library moves over without a re-sync. Postgres is
// only read.
//
//	go run ./cmd/pg2sqlite -to steamlib.db                 # reads DATABASE_URL
//	go run ./cmd/pg2sqlite -from postgres://… -to steamlib.db
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Adis/steamlib-explorer/backend/internal/store/postgres"
	"github.com/Adis/steamlib-explorer/backend/internal/store/sqlite"
	"github.com/Adis/steamlib-explorer/backend/internal/transfer"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pg2sqlite:", err)
		os.Exit(1)
	}
}

func run() error {
	from := flag.String("from", os.Getenv("DATABASE_URL"), "Postgres URL to copy from (default: DATABASE_URL)")
	to := flag.String("to", "", "SQLite file to create, e.g. steamlib.db (must not exist yet)")
	flag.Parse()

	if !strings.HasPrefix(*from, "postgres://") && !strings.HasPrefix(*from, "postgresql://") {
		return fmt.Errorf("-from must be a postgres:// URL, got %q", *from)
	}
	path := strings.TrimPrefix(*to, "sqlite:")
	if path == "" {
		return errors.New("give the SQLite file to create with -to, e.g. -to steamlib.db")
	}
	// Refuse to touch an existing file, so a second run can't mix two copies.
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; move it away or pick another name", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Only warnings: the migration lines would read as if Postgres changed.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	ctx := context.Background()
	src, err := postgres.Open(ctx, *from) // also brings Postgres up to the latest schema, additively
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := sqlite.Open(ctx, path)
	if err != nil {
		return err
	}

	counts, err := transfer.PostgresToSQLite(ctx, src.Pool(), dst.DB())
	dst.Close()
	if err != nil {
		os.Remove(path) // nothing was committed; don't leave an empty file behind
		os.Remove(path + "-wal")
		os.Remove(path + "-shm")
		return err
	}
	fmt.Printf("Copied %d users, %d games, %d library entries, %d syncs and %d sign-in sessions into %s.\n",
		counts["users"], counts["apps"], counts["owned_games"], counts["sync_runs"], counts["sessions"], path)
	fmt.Printf("To use it, set DATABASE_URL=sqlite:%s in .env. Postgres is unchanged.\n", path)
	return nil
}
