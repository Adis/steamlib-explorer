// Package database opens the store the configuration asks for.
package database

import (
	"context"

	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/store/postgres"
)

// Open connects to the database at url and applies pending migrations.
func Open(ctx context.Context, url string) (store.Store, error) {
	return postgres.Open(ctx, url)
}
