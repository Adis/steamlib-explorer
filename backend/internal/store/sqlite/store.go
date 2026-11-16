package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	sqlitedrv "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/Adis/steamlib-explorer/backend/db"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

// Store implements store.Store on an SQLite file, with the sqlc-generated
// queries in this package plus the explorer queries in games.go.
type Store struct {
	db *sql.DB
	q  *Queries
}

var _ store.Store = (*Store)(nil)

func init() {
	// SQLite's lower() only knows ASCII; the name search uses this instead so
	// "é" finds "É" like Postgres's ILIKE does.
	sqlitedrv.MustRegisterDeterministicScalarFunction("casefold", 1,
		func(_ *sqlitedrv.FunctionContext, args []driver.Value) (driver.Value, error) {
			s, ok := args[0].(string)
			if !ok {
				return args[0], nil
			}
			return strings.ToLower(s), nil
		})
}

// Open opens (or creates) the SQLite database file at path and applies every
// pending migration.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("sqlite: no database file given")
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create database folder: %w", err)
		}
	}
	// WAL lets the explorer read while a Refresh writes; busy_timeout makes a
	// writer wait for another instead of failing.
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	if err := Migrate(ctx, sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return &Store{db: sqlDB, q: New(sqlDB)}, nil
}

// DB is the database handle, for tests that check rows directly.
func (s *Store) DB() *sql.DB { return s.db }

// Migrate applies every pending SQLite migration embedded in the binary.
func Migrate(ctx context.Context, sqlDB *sql.DB) error {
	migrations, err := fs.Sub(db.SQLite, "sqlite/migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, migrations)
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

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *Store) Close()                         { s.db.Close() }

// mapErr turns SQLite errors the callers care about into store errors.
func mapErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrNotFound
	}
	var e *sqlitedrv.Error
	if errors.As(err, &e) && e.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return fmt.Errorf("%w: %s", store.ErrConflict, e.Error())
	}
	return err
}

// Conversions between the store types and SQLite's column types (see the
// comment at the top of db/sqlite/migrations/00001_init.sql).

const timeLayout = "2006-01-02T15:04:05.000Z"

func fmtTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		slog.Warn("sqlite: unreadable timestamp", "value", s)
	}
	return t
}

func nullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: fmtTime(*t), Valid: true}
}

func timePtr(s sql.NullString) *time.Time {
	if !s.Valid {
		return nil
	}
	t := parseTime(s.String)
	return &t
}

func nullDate(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.Format(time.DateOnly), Valid: true}
}

func datePtr(s sql.NullString) *time.Time {
	if !s.Valid {
		return nil
	}
	t, err := time.Parse(time.DateOnly, s.String)
	if err != nil {
		return nil
	}
	return &t
}

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func stringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func nullInt(i *int32) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

func intPtr(i sql.NullInt64) *int32 {
	if !i.Valid {
		return nil
	}
	v := int32(i.Int64)
	return &v
}

func floatPtr(f sql.NullFloat64) *float64 {
	if !f.Valid {
		return nil
	}
	return &f.Float64
}

func jsonList(list []string) string {
	if list == nil {
		return "[]"
	}
	b, _ := json.Marshal(list) // a []string always marshals
	return string(b)
}

func parseList(s string) []string {
	out := []string{}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		slog.Warn("sqlite: unreadable list", "value", s)
		return []string{}
	}
	return out
}

func toUser(u User) store.User {
	return store.User{
		SteamID:     u.SteamID,
		PersonaName: stringPtr(u.PersonaName),
		AvatarUrl:   stringPtr(u.AvatarUrl),
		CreatedAt:   parseTime(u.CreatedAt),
		LastLoginAt: parseTime(u.LastLoginAt),
	}
}

func (s *Store) UpsertUser(ctx context.Context, arg store.UpsertUserParams) (store.User, error) {
	u, err := s.q.UpsertUser(ctx, UpsertUserParams{
		SteamID: arg.SteamID, PersonaName: nullString(arg.PersonaName), AvatarUrl: nullString(arg.AvatarUrl),
	})
	if err != nil {
		return store.User{}, mapErr(err)
	}
	return toUser(u), nil
}

func (s *Store) CreateSession(ctx context.Context, arg store.CreateSessionParams) error {
	return mapErr(s.q.CreateSession(ctx, CreateSessionParams{
		TokenHash: arg.TokenHash, SteamID: arg.SteamID, ExpiresAt: fmtTime(arg.ExpiresAt),
	}))
}

func (s *Store) GetSessionUser(ctx context.Context, tokenHash []byte) (store.User, error) {
	u, err := s.q.GetSessionUser(ctx, tokenHash)
	if err != nil {
		return store.User{}, mapErr(err)
	}
	return toUser(u), nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	return mapErr(s.q.DeleteSession(ctx, tokenHash))
}

func (s *Store) StartSyncRun(ctx context.Context, steamID int64) (int64, error) {
	id, err := s.q.StartSyncRun(ctx, steamID)
	return id, mapErr(err)
}

func (s *Store) UpdateSyncProgress(ctx context.Context, arg store.UpdateSyncProgressParams) error {
	return mapErr(s.q.UpdateSyncProgress(ctx, UpdateSyncProgressParams{
		ID: arg.ID, GamesSeen: int64(arg.GamesSeen), DetailsTotal: int64(arg.DetailsTotal), DetailsFetched: int64(arg.DetailsFetched),
	}))
}

func (s *Store) FinishSyncRun(ctx context.Context, arg store.FinishSyncRunParams) error {
	return mapErr(s.q.FinishSyncRun(ctx, FinishSyncRunParams{ID: arg.ID, Status: arg.Status, Error: nullString(arg.Error)}))
}

func (s *Store) FailRunningSyncs(ctx context.Context) (int64, error) {
	n, err := s.q.FailRunningSyncs(ctx)
	return n, mapErr(err)
}

func (s *Store) LatestSyncRun(ctx context.Context, steamID int64) (store.SyncRun, error) {
	r, err := s.q.LatestSyncRun(ctx, steamID)
	if err != nil {
		return store.SyncRun{}, mapErr(err)
	}
	return store.SyncRun{
		ID:               r.ID,
		SteamID:          r.SteamID,
		StartedAt:        parseTime(r.StartedAt),
		FinishedAt:       timePtr(r.FinishedAt),
		Status:           r.Status,
		GamesSeen:        int32(r.GamesSeen),
		DetailsFetched:   int32(r.DetailsFetched),
		Error:            stringPtr(r.Error),
		DetailsTotal:     int32(r.DetailsTotal),
		DetailsStartedAt: timePtr(r.DetailsStartedAt),
	}, nil
}

func (s *Store) LibraryStats(ctx context.Context, steamID int64) (store.LibraryStatsRow, error) {
	r, err := s.q.LibraryStats(ctx, steamID)
	return store.LibraryStatsRow(r), mapErr(err)
}

func (s *Store) EnsureApp(ctx context.Context, arg store.EnsureAppParams) error {
	return mapErr(s.q.EnsureApp(ctx, EnsureAppParams(arg)))
}

func (s *Store) UpsertOwnedGame(ctx context.Context, arg store.UpsertOwnedGameParams) error {
	return mapErr(s.q.UpsertOwnedGame(ctx, UpsertOwnedGameParams{
		SteamID:         arg.SteamID,
		Appid:           arg.Appid,
		PlaytimeForever: int64(arg.PlaytimeForever),
		Playtime2weeks:  int64(arg.Playtime2weeks),
		LastPlayedAt:    nullTime(arg.LastPlayedAt),
	}))
}

func (s *Store) AppsNeedingDetails(ctx context.Context, arg store.AppsNeedingDetailsParams) ([]int64, error) {
	ids, err := s.q.AppsNeedingDetails(ctx, AppsNeedingDetailsParams{
		SteamID: arg.SteamID, DetailsFetchedAt: nullTime(&arg.StaleBefore),
	})
	return ids, mapErr(err)
}

func (s *Store) AppsNeedingReviews(ctx context.Context, arg store.AppsNeedingReviewsParams) ([]int64, error) {
	ids, err := s.q.AppsNeedingReviews(ctx, AppsNeedingReviewsParams{
		SteamID: arg.SteamID, ReviewsFetchedAt: nullTime(&arg.StaleBefore),
	})
	return ids, mapErr(err)
}

func (s *Store) UpdateAppDetails(ctx context.Context, arg store.UpdateAppDetailsParams) error {
	var raw sql.NullString
	if arg.DetailsRaw != nil {
		raw = sql.NullString{String: string(arg.DetailsRaw), Valid: true}
	}
	return mapErr(s.q.UpdateAppDetails(ctx, UpdateAppDetailsParams{
		Name:             arg.Name,
		Type:             nullString(arg.Type),
		ShortDescription: nullString(arg.ShortDescription),
		HeaderImage:      nullString(arg.HeaderImage),
		ReleaseDate:      nullDate(arg.ReleaseDate),
		ReleaseDateText:  nullString(arg.ReleaseDateText),
		ComingSoon:       arg.ComingSoon,
		IsFree:           arg.IsFree,
		Developers:       jsonList(arg.Developers),
		Publishers:       jsonList(arg.Publishers),
		Genres:           jsonList(arg.Genres),
		Categories:       jsonList(arg.Categories),
		PlatformWindows:  arg.PlatformWindows,
		PlatformMac:      arg.PlatformMac,
		PlatformLinux:    arg.PlatformLinux,
		MetacriticScore:  nullInt(arg.MetacriticScore),
		PriceCents:       nullInt(arg.PriceCents),
		PriceCurrency:    nullString(arg.PriceCurrency),
		DetailsRaw:       raw,
		Appid:            arg.Appid,
	}))
}

func (s *Store) MarkAppDetailsStatus(ctx context.Context, arg store.MarkAppDetailsStatusParams) error {
	return mapErr(s.q.MarkAppDetailsStatus(ctx, MarkAppDetailsStatusParams{Appid: arg.Appid, DetailsStatus: arg.DetailsStatus}))
}

func (s *Store) UpdateAppReviews(ctx context.Context, arg store.UpdateAppReviewsParams) error {
	return mapErr(s.q.UpdateAppReviews(ctx, UpdateAppReviewsParams{
		Appid:           arg.Appid,
		ReviewScore:     nullInt(arg.ReviewScore),
		ReviewScoreDesc: nullString(arg.ReviewScoreDesc),
		ReviewsPositive: nullInt(arg.ReviewsPositive),
		ReviewsTotal:    nullInt(arg.ReviewsTotal),
	}))
}
