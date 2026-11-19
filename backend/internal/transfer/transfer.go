// Package transfer copies a Postgres database into a new SQLite one, so a
// library synced on Postgres moves to the desktop app without a re-sync.
package transfer

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// column is one column to copy: its name, and the Postgres expression that
// reads it in the form the SQLite schema stores (see the comment at the top of
// db/sqlite/migrations/00001_init.sql).
type column struct {
	name string
	pg   string
}

func plain(name string) column { return column{name, name} }

// ts reads a timestamptz as UTC text with milliseconds, e.g. 2026-11-16T17:30:00.123Z.
func ts(name string) column {
	return column{name, `to_char(` + name + ` AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')`}
}

// list reads a text[] as a JSON array.
func list(name string) column { return column{name, `to_json(` + name + `)::text`} }

// tables lists every table in foreign-key order, with every column except
// generated ones (apps.review_pct is recomputed by SQLite).
var tables = []struct {
	name    string
	columns []column
}{
	{"users", []column{plain("steam_id"), plain("persona_name"), plain("avatar_url"), ts("created_at"), ts("last_login_at")}},
	{"sessions", []column{plain("token_hash"), plain("steam_id"), ts("created_at"), ts("expires_at")}},
	{"apps", []column{
		plain("appid"), plain("name"), plain("type"), plain("short_description"), plain("header_image"),
		{"release_date", `to_char(release_date, 'YYYY-MM-DD')`}, plain("release_date_text"),
		plain("coming_soon"), plain("is_free"),
		list("developers"), list("publishers"), list("genres"), list("categories"),
		plain("platform_windows"), plain("platform_mac"), plain("platform_linux"),
		plain("metacritic_score"), plain("price_cents"), plain("price_currency"),
		plain("review_score"), plain("review_score_desc"), plain("reviews_positive"), plain("reviews_total"),
		{"details_raw", `details_raw::text`}, plain("details_status"),
		ts("details_fetched_at"), ts("reviews_fetched_at"), ts("created_at"), ts("updated_at"),
	}},
	{"owned_games", []column{plain("steam_id"), plain("appid"), plain("playtime_forever"), plain("playtime_2weeks"), ts("last_played_at"), ts("synced_at")}},
	{"sync_runs", []column{
		plain("id"), plain("steam_id"), ts("started_at"), ts("finished_at"), plain("status"),
		plain("games_seen"), plain("details_fetched"), plain("error"), plain("details_total"), ts("details_started_at"),
	}},
	{"developer_games", []column{plain("developer"), {"games", `games::text`}, ts("fetched_at")}},
	{"mcp_tokens", []column{plain("steam_id"), plain("token_hash"), ts("created_at")}},
}

// Counts is the number of rows copied per table.
type Counts map[string]int64

// PostgresToSQLite copies every row from pg into dst, an SQLite database
// already migrated to the matching schema and still empty. It runs in one
// SQLite transaction, so a failed copy leaves dst empty, and checks
// afterwards that every table has as many rows as in Postgres. Postgres is
// only read.
func PostgresToSQLite(ctx context.Context, pg *pgxpool.Pool, dst *sql.DB) (Counts, error) {
	for _, t := range tables {
		var n int64
		if err := dst.QueryRowContext(ctx, "SELECT count(*) FROM "+t.name).Scan(&n); err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, fmt.Errorf("the SQLite database already has %d rows in %s; import only into a new file", n, t.name)
		}
	}

	// One read-only snapshot, so a sync running meanwhile can't leave owned
	// games pointing at apps that weren't copied.
	src, err := pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Rollback(ctx) }() // read-only: nothing to keep
	if _, err := src.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		return nil, err
	}

	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit

	counts := Counts{}
	for _, t := range tables {
		n, err := copyTable(ctx, src, tx, t.name, t.columns)
		if err != nil {
			return nil, err
		}
		counts[t.name] = n

		var want, got int64
		if err := src.QueryRow(ctx, "SELECT count(*) FROM "+t.name).Scan(&want); err != nil {
			return nil, err
		}
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM "+t.name).Scan(&got); err != nil {
			return nil, err
		}
		if got != want {
			return nil, fmt.Errorf("%s: Postgres has %d rows but %d were copied", t.name, want, got)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return counts, nil
}

// copyTable copies every row of one table from src to dst and returns how
// many it copied.
func copyTable(ctx context.Context, src pgx.Tx, dst *sql.Tx, table string, columns []column) (int64, error) {
	names := make([]string, len(columns))
	exprs := make([]string, len(columns))
	for i, c := range columns {
		names[i], exprs[i] = c.name, c.pg
	}
	rows, err := src.Query(ctx, "SELECT "+strings.Join(exprs, ", ")+" FROM "+table)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", table, err)
	}
	defer rows.Close()
	//nolint:gosec // G202: table and column names come from the fixed tables list.
	insert, err := dst.PrepareContext(ctx, "INSERT INTO "+table+" ("+strings.Join(names, ", ")+
		") VALUES (?"+strings.Repeat(", ?", len(names)-1)+")")
	if err != nil {
		return 0, err
	}
	defer insert.Close()

	var n int64
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return 0, fmt.Errorf("read %s: %w", table, err)
		}
		if _, err := insert.ExecContext(ctx, values...); err != nil {
			return 0, fmt.Errorf("write %s: %w", table, err)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read %s: %w", table, err)
	}
	return n, nil
}
