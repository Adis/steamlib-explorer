// Package db holds the SQL migrations, embedded into the binary.
package db

import "embed"

// Postgres holds the Postgres migrations, in postgres/migrations.
//
//go:embed postgres/migrations/*.sql
var Postgres embed.FS
