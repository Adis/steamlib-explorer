// Package store is the app's database layer: the Store interface the rest of
// the backend uses, and the types it takes and returns. The engine-specific
// implementations live in subpackages (postgres).
package store

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound is returned when a query for one row finds none.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned when a write breaks a unique constraint, e.g. a
	// second running Refresh for the same user.
	ErrConflict = errors.New("conflicts with an existing row")
)

// Store is everything the backend reads from and writes to the database.
type Store interface {
	Ping(ctx context.Context) error
	Close()

	// Sign-in
	UpsertUser(ctx context.Context, arg UpsertUserParams) (User, error)
	CreateSession(ctx context.Context, arg CreateSessionParams) error
	// GetSessionUser returns ErrNotFound for an unknown or expired session.
	GetSessionUser(ctx context.Context, tokenHash []byte) (User, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error

	// Library sync
	// StartSyncRun returns ErrConflict when the user already has a running Refresh.
	StartSyncRun(ctx context.Context, steamID int64) (int64, error)
	UpdateSyncProgress(ctx context.Context, arg UpdateSyncProgressParams) error
	FinishSyncRun(ctx context.Context, arg FinishSyncRunParams) error
	// FailRunningSyncs marks Refreshes left running by a restart as failed.
	FailRunningSyncs(ctx context.Context) (int64, error)
	// LatestSyncRun returns ErrNotFound when the user never synced.
	LatestSyncRun(ctx context.Context, steamID int64) (SyncRun, error)
	LibraryStats(ctx context.Context, steamID int64) (LibraryStatsRow, error)
	EnsureApp(ctx context.Context, arg EnsureAppParams) error
	UpsertOwnedGame(ctx context.Context, arg UpsertOwnedGameParams) error
	// RemoveOwnedGamesExcept deletes the user's owned games whose appid is not
	// in keep (refunded, removed or no longer listed), and returns how many.
	// The apps' store data stays.
	RemoveOwnedGamesExcept(ctx context.Context, steamID int64, keep []int64) (int64, error)
	AppsNeedingDetails(ctx context.Context, arg AppsNeedingDetailsParams) ([]int64, error)
	AppsNeedingReviews(ctx context.Context, arg AppsNeedingReviewsParams) ([]int64, error)
	UpdateAppDetails(ctx context.Context, arg UpdateAppDetailsParams) error
	MarkAppDetailsStatus(ctx context.Context, arg MarkAppDetailsStatusParams) error
	UpdateAppReviews(ctx context.Context, arg UpdateAppReviewsParams) error

	// Explorer
	ListGames(ctx context.Context, arg ListGamesParams) ([]ListGamesRow, error)
	GenreFacets(ctx context.Context, steamID int64) ([]GenreFacetsRow, error)
	CategoryFacets(ctx context.Context, steamID int64) ([]CategoryFacetsRow, error)
	// OwnedAmong returns which of appids the user owns, in ascending order.
	OwnedAmong(ctx context.Context, steamID int64, appids []int64) ([]int64, error)
}

type User struct {
	SteamID     int64
	PersonaName *string
	AvatarUrl   *string
	CreatedAt   time.Time
	LastLoginAt time.Time
}

type UpsertUserParams struct {
	SteamID     int64
	PersonaName *string
	AvatarUrl   *string
}

type CreateSessionParams struct {
	TokenHash []byte
	SteamID   int64
	ExpiresAt time.Time
}

type SyncRun struct {
	ID               int64
	SteamID          int64
	StartedAt        time.Time
	FinishedAt       *time.Time
	Status           string
	GamesSeen        int32
	DetailsFetched   int32
	Error            *string
	DetailsTotal     int32
	DetailsStartedAt *time.Time
}

type UpdateSyncProgressParams struct {
	ID             int64
	GamesSeen      int32
	DetailsTotal   int32
	DetailsFetched int32
}

type FinishSyncRunParams struct {
	ID     int64
	Status string
	Error  *string
}

type LibraryStatsRow struct {
	Games       int64
	WithDetails int64
	Unavailable int64
	Failed      int64
	WithReviews int64
}

type EnsureAppParams struct {
	Appid int64
	Name  string
}

type UpsertOwnedGameParams struct {
	SteamID         int64
	Appid           int64
	PlaytimeForever int32
	Playtime2weeks  int32
	LastPlayedAt    *time.Time
}

type AppsNeedingDetailsParams struct {
	SteamID     int64
	StaleBefore time.Time
}

type AppsNeedingReviewsParams struct {
	SteamID     int64
	StaleBefore time.Time
}

type UpdateAppDetailsParams struct {
	Name             string
	Type             *string
	ShortDescription *string
	HeaderImage      *string
	ReleaseDate      *time.Time
	ReleaseDateText  *string
	ComingSoon       bool
	IsFree           bool
	Developers       []string
	Publishers       []string
	Genres           []string
	Categories       []string
	PlatformWindows  bool
	PlatformMac      bool
	PlatformLinux    bool
	MetacriticScore  *int32
	PriceCents       *int32
	PriceCurrency    *string
	DetailsRaw       []byte
	Appid            int64
}

type MarkAppDetailsStatusParams struct {
	Appid         int64
	DetailsStatus string
}

type UpdateAppReviewsParams struct {
	Appid           int64
	ReviewScore     *int32
	ReviewScoreDesc *string
	ReviewsPositive *int32
	ReviewsTotal    *int32
}

// ListGamesParams filters, sorts and pages a user's games. Nil and false
// fields turn their filter off.
type ListGamesParams struct {
	SteamID int64
	// AppID keeps only this one game.
	AppID *int64
	// NameContains matches names containing this text, ignoring case.
	NameContains   *string
	Genres         []string // has all of these
	Categories     []string // has all of these
	Played         *bool
	MinMinutes     *int32
	MaxMinutes     *int32
	MinReviewPct   *float64
	MinReviewScore *int32
	ReviewScores   []int32 // has any of these Steam ratings (1..9)
	// MinReviews keeps games with at least this many reviews, and games
	// whose review count isn't known yet.
	MinReviews    *int32
	MinMetacritic *int32
	// DetailsStatus keeps games whose store details are in this state:
	// pending, ok, unavailable (no store page) or error (fetch failed).
	DetailsStatus *string
	// HasReviews keeps games whose reviews were (true) or weren't (false)
	// fetched yet.
	HasReviews     *bool
	ReleasedAfter  *time.Time
	ReleasedBefore *time.Time
	WindowsOnly    bool
	MacOnly        bool
	LinuxOnly      bool
	// Sort is name, playtime, recent, last_played, review, metacritic or release.
	Sort       string
	Descending bool
	PageOffset int32
	PageLimit  int32
}

type ListGamesRow struct {
	Appid            int64
	Name             string
	HeaderImage      *string
	ShortDescription *string
	ReleaseDate      *time.Time
	ReleaseDateText  *string
	ComingSoon       bool
	Genres           []string
	Categories       []string
	Developers       []string
	Publishers       []string
	PlatformWindows  bool
	PlatformMac      bool
	PlatformLinux    bool
	MetacriticScore  *int32
	MetacriticURL    string // Metacritic's review page as Steam's store details link it, or ""
	PriceCents       *int32
	PriceCurrency    *string
	IsFree           bool
	ReviewScoreDesc  *string
	ReviewPct        *float64
	ReviewsTotal     *int32
	DetailsStatus    string
	PlaytimeForever  int32
	Playtime2weeks   int32
	LastPlayedAt     *time.Time
	TotalCount       int64 // matching games across all pages
}

type GenreFacetsRow struct {
	Value string
	Games int64
}

type CategoryFacetsRow struct {
	Value string
	Games int64
}
