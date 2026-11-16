// Package database opens the store DATABASE_URL asks for.
package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/store/postgres"
	"github.com/Adis/steamlib-explorer/backend/internal/store/sqlite"
)

// Open connects to the database at url and applies pending migrations.
//
//	postgres://user:pass@host:5432/db?sslmode=disable  Postgres
//	sqlite:steamlib.db                                 SQLite file, relative to the working folder
//	sqlite:/home/me/.local/share/steamlib/steamlib.db  SQLite file, absolute path
func Open(ctx context.Context, url string) (store.Store, error) {
	engine, path, err := Parse(url)
	if err != nil {
		return nil, err
	}
	if engine == "sqlite" {
		return sqlite.Open(ctx, path)
	}
	return postgres.Open(ctx, url)
}

// Parse returns the engine ("postgres" or "sqlite") a DATABASE_URL names and,
// for SQLite, the file path.
func Parse(url string) (engine, path string, err error) {
	switch {
	case strings.HasPrefix(url, "postgres://"), strings.HasPrefix(url, "postgresql://"):
		return "postgres", "", nil
	case strings.HasPrefix(url, "sqlite:"):
		path = strings.TrimPrefix(url, "sqlite:")
		// Also accept sqlite:///abs/path and sqlite://rel/path.
		if rest, ok := strings.CutPrefix(path, "//"); ok {
			path = rest
		}
		if path == "" || strings.ContainsAny(path, "?#") {
			return "", "", fmt.Errorf("DATABASE_URL %q: give a file after sqlite:, e.g. sqlite:steamlib.db", url)
		}
		return "sqlite", path, nil
	}
	return "", "", fmt.Errorf("DATABASE_URL must start with postgres:// or sqlite:, got %q", url)
}
