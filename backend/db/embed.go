// Package db holds the SQL migrations, embedded into the binary.
package db

import "embed"

// Postgres holds the Postgres migrations, in postgres/migrations.
//
//go:embed postgres/migrations/*.sql
var Postgres embed.FS

// SQLite holds the SQLite migrations, in sqlite/migrations.
//
//go:embed sqlite/migrations/*.sql
var SQLite embed.FS
