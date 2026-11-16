package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Adis/steamlib-explorer/backend/db"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

// Store implements store.Store on Postgres, with the sqlc-generated queries
// in this package.
type Store struct {
	pool *pgxpool.Pool
	q    *Queries
}

var _ store.Store = (*Store)(nil)

// Open connects to Postgres at url and applies every pending migration.
func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool, q: New(pool)}, nil
}

// Pool is the connection pool, for tests that check rows directly.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	return Migrate(ctx, sqlDB)
}

// Migrate applies every pending Postgres migration embedded in the binary.
func Migrate(ctx context.Context, sqlDB *sql.DB) error {
	migrations, err := fs.Sub(db.Postgres, "postgres/migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	for _, r := range results {
		slog.Info("applied migration", "file", r.Source.Path, "duration", r.Duration)
	}
	return nil
}

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *Store) Close()                         { s.pool.Close() }

// mapErr turns Postgres errors the callers care about into store errors.
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return fmt.Errorf("%w: %s", store.ErrConflict, pgErr.Message)
	}
	return err
}

func (s *Store) UpsertUser(ctx context.Context, arg store.UpsertUserParams) (store.User, error) {
	u, err := s.q.UpsertUser(ctx, UpsertUserParams(arg))
	return store.User(u), mapErr(err)
}

func (s *Store) CreateSession(ctx context.Context, arg store.CreateSessionParams) error {
	return mapErr(s.q.CreateSession(ctx, CreateSessionParams(arg)))
}

func (s *Store) GetSessionUser(ctx context.Context, tokenHash []byte) (store.User, error) {
	u, err := s.q.GetSessionUser(ctx, tokenHash)
	return store.User(u), mapErr(err)
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	return mapErr(s.q.DeleteSession(ctx, tokenHash))
}

func (s *Store) StartSyncRun(ctx context.Context, steamID int64) (int64, error) {
	id, err := s.q.StartSyncRun(ctx, steamID)
	return id, mapErr(err)
}

func (s *Store) UpdateSyncProgress(ctx context.Context, arg store.UpdateSyncProgressParams) error {
	return mapErr(s.q.UpdateSyncProgress(ctx, UpdateSyncProgressParams(arg)))
}

func (s *Store) FinishSyncRun(ctx context.Context, arg store.FinishSyncRunParams) error {
	return mapErr(s.q.FinishSyncRun(ctx, FinishSyncRunParams(arg)))
}

func (s *Store) FailRunningSyncs(ctx context.Context) (int64, error) {
	n, err := s.q.FailRunningSyncs(ctx)
	return n, mapErr(err)
}

func (s *Store) LatestSyncRun(ctx context.Context, steamID int64) (store.SyncRun, error) {
	r, err := s.q.LatestSyncRun(ctx, steamID)
	return store.SyncRun(r), mapErr(err)
}

func (s *Store) LibraryStats(ctx context.Context, steamID int64) (store.LibraryStatsRow, error) {
	r, err := s.q.LibraryStats(ctx, steamID)
	return store.LibraryStatsRow(r), mapErr(err)
}

func (s *Store) EnsureApp(ctx context.Context, arg store.EnsureAppParams) error {
	return mapErr(s.q.EnsureApp(ctx, EnsureAppParams(arg)))
}

func (s *Store) UpsertOwnedGame(ctx context.Context, arg store.UpsertOwnedGameParams) error {
	return mapErr(s.q.UpsertOwnedGame(ctx, UpsertOwnedGameParams(arg)))
}

func (s *Store) AppsNeedingDetails(ctx context.Context, arg store.AppsNeedingDetailsParams) ([]int64, error) {
	ids, err := s.q.AppsNeedingDetails(ctx, AppsNeedingDetailsParams(arg))
	return ids, mapErr(err)
}

func (s *Store) AppsNeedingReviews(ctx context.Context, arg store.AppsNeedingReviewsParams) ([]int64, error) {
	ids, err := s.q.AppsNeedingReviews(ctx, AppsNeedingReviewsParams(arg))
	return ids, mapErr(err)
}

func (s *Store) UpdateAppDetails(ctx context.Context, arg store.UpdateAppDetailsParams) error {
	return mapErr(s.q.UpdateAppDetails(ctx, UpdateAppDetailsParams(arg)))
}

func (s *Store) MarkAppDetailsStatus(ctx context.Context, arg store.MarkAppDetailsStatusParams) error {
	return mapErr(s.q.MarkAppDetailsStatus(ctx, MarkAppDetailsStatusParams(arg)))
}

func (s *Store) UpdateAppReviews(ctx context.Context, arg store.UpdateAppReviewsParams) error {
	return mapErr(s.q.UpdateAppReviews(ctx, UpdateAppReviewsParams(arg)))
}

func (s *Store) ListGames(ctx context.Context, arg store.ListGamesParams) ([]store.ListGamesRow, error) {
	p := ListGamesParams{
		SteamID:        arg.SteamID,
		Genres:         arg.Genres,
		Categories:     arg.Categories,
		Played:         arg.Played,
		MinMinutes:     arg.MinMinutes,
		MaxMinutes:     arg.MaxMinutes,
		MinReviewPct:   arg.MinReviewPct,
		MinReviewScore: arg.MinReviewScore,
		MinReviews:     arg.MinReviews,
		MinMetacritic:  arg.MinMetacritic,
		ReleasedAfter:  arg.ReleasedAfter,
		ReleasedBefore: arg.ReleasedBefore,
		WindowsOnly:    arg.WindowsOnly,
		MacOnly:        arg.MacOnly,
		LinuxOnly:      arg.LinuxOnly,
		Sort:           arg.Sort,
		Descending:     arg.Descending,
		PageOffset:     arg.PageOffset,
		PageLimit:      arg.PageLimit,
	}
	if arg.NameContains != nil {
		pattern := "%" + escapeLike(*arg.NameContains) + "%"
		p.NamePattern = &pattern
	}
	rows, err := s.q.ListGames(ctx, p)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]store.ListGamesRow, len(rows))
	for i, r := range rows {
		out[i] = store.ListGamesRow(r)
	}
	return out, nil
}

// escapeLike makes text match literally inside ILIKE.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (s *Store) GenreFacets(ctx context.Context, steamID int64) ([]store.GenreFacetsRow, error) {
	rows, err := s.q.GenreFacets(ctx, steamID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]store.GenreFacetsRow, len(rows))
	for i, r := range rows {
		out[i] = store.GenreFacetsRow(r)
	}
	return out, nil
}

func (s *Store) CategoryFacets(ctx context.Context, steamID int64) ([]store.CategoryFacetsRow, error) {
	rows, err := s.q.CategoryFacets(ctx, steamID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]store.CategoryFacetsRow, len(rows))
	for i, r := range rows {
		out[i] = store.CategoryFacetsRow(r)
	}
	return out, nil
}
