// Package syncer runs a Refresh: it copies a user's Steam library into the
// database, then fills in store details for each app and reviews in batches.
package syncer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Adis/steamlib-explorer/backend/internal/steam"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

// ErrAlreadyRunning means the user already has a Refresh in progress.
var ErrAlreadyRunning = errors.New("syncer: a refresh is already running")

// Steam is the part of the Steam client a Refresh needs.
type Steam interface {
	OwnedGames(ctx context.Context, steamID uint64) ([]steam.OwnedGame, error)
	AppDetails(ctx context.Context, appID int64) (*steam.AppDetails, error)
	AppReviews(ctx context.Context, appIDs []int64) (map[int64]steam.ReviewSummary, error)
}

// Queries is the part of the generated store a Refresh needs.
type Queries interface {
	StartSyncRun(ctx context.Context, steamID int64) (int64, error)
	UpdateSyncProgress(ctx context.Context, arg store.UpdateSyncProgressParams) error
	FinishSyncRun(ctx context.Context, arg store.FinishSyncRunParams) error
	EnsureApp(ctx context.Context, arg store.EnsureAppParams) error
	UpsertOwnedGame(ctx context.Context, arg store.UpsertOwnedGameParams) error
	AppsNeedingDetails(ctx context.Context, arg store.AppsNeedingDetailsParams) ([]int64, error)
	AppsNeedingReviews(ctx context.Context, arg store.AppsNeedingReviewsParams) ([]int64, error)
	UpdateAppDetails(ctx context.Context, arg store.UpdateAppDetailsParams) error
	MarkAppDetailsStatus(ctx context.Context, arg store.MarkAppDetailsStatusParams) error
	UpdateAppReviews(ctx context.Context, arg store.UpdateAppReviewsParams) error
}

// Syncer starts Refreshes in the background, one at a time per user.
type Syncer struct {
	steam Steam
	q     Queries
	// StaleAfter is how old store data may get before a Refresh fetches it again.
	StaleAfter time.Duration

	ctx     context.Context // cancelled on shutdown
	wg      sync.WaitGroup
	mu      sync.Mutex
	running map[uint64]bool
}

// New returns a Syncer whose background work stops when ctx is cancelled.
func New(ctx context.Context, s Steam, q Queries) *Syncer {
	return &Syncer{steam: s, q: q, StaleAfter: 30 * 24 * time.Hour, ctx: ctx, running: map[uint64]bool{}}
}

// Start records a new Refresh and runs it in the background. It returns the
// run's id, or ErrAlreadyRunning.
func (s *Syncer) Start(ctx context.Context, steamID uint64) (int64, error) {
	s.mu.Lock()
	if s.running[steamID] {
		s.mu.Unlock()
		return 0, ErrAlreadyRunning
	}
	s.running[steamID] = true
	s.mu.Unlock()

	runID, err := s.q.StartSyncRun(ctx, int64(steamID))
	if err != nil {
		s.done(steamID)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // another process holds the running slot
			return 0, ErrAlreadyRunning
		}
		return 0, err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.done(steamID)
		s.run(s.ctx, runID, steamID)
	}()
	return runID, nil
}

// Wait blocks until every background Refresh has stopped.
func (s *Syncer) Wait() { s.wg.Wait() }

func (s *Syncer) done(steamID uint64) {
	s.mu.Lock()
	delete(s.running, steamID)
	s.mu.Unlock()
}

type progress struct{ games, total, fetched int32 }

func (s *Syncer) run(ctx context.Context, runID int64, steamID uint64) {
	log := slog.With("run_id", runID, "steam_id", steamID)
	log.Info("refresh started")
	started := time.Now()

	err := s.refresh(ctx, log, runID, steamID)

	// The finish write must happen even when ctx was cancelled by shutdown.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	status, msg := "ok", (*string)(nil)
	if err != nil {
		status = "error"
		m := userMessage(err)
		msg = &m
		log.Error("refresh failed", "error", err, "duration", time.Since(started).Round(time.Millisecond))
	} else {
		log.Info("refresh finished", "duration", time.Since(started).Round(time.Millisecond))
	}
	if err := s.q.FinishSyncRun(finishCtx, store.FinishSyncRunParams{ID: runID, Status: status, Error: msg}); err != nil {
		log.Error("record refresh result", "error", err)
	}
}

func (s *Syncer) refresh(ctx context.Context, log *slog.Logger, runID int64, steamID uint64) error {
	games, err := s.steam.OwnedGames(ctx, steamID)
	if err != nil {
		return fmt.Errorf("fetch library: %w", err)
	}
	for _, g := range games {
		if err := s.q.EnsureApp(ctx, store.EnsureAppParams{Appid: g.AppID, Name: g.Name}); err != nil {
			return fmt.Errorf("save app %d: %w", g.AppID, err)
		}
		if err := s.q.UpsertOwnedGame(ctx, store.UpsertOwnedGameParams{
			SteamID:         int64(steamID),
			Appid:           g.AppID,
			PlaytimeForever: int32(g.PlaytimeForever),
			Playtime2weeks:  int32(g.Playtime2Weeks),
			LastPlayedAt:    g.LastPlayed,
		}); err != nil {
			return fmt.Errorf("save owned game %d: %w", g.AppID, err)
		}
	}

	appIDs, err := s.q.AppsNeedingDetails(ctx, store.AppsNeedingDetailsParams{
		SteamID:     int64(steamID),
		StaleBefore: time.Now().Add(-s.StaleAfter),
	})
	if err != nil {
		return fmt.Errorf("list apps to fetch: %w", err)
	}
	p := progress{games: int32(len(games)), total: int32(len(appIDs))}
	s.saveProgress(ctx, log, runID, p)
	log.Info("library saved", "games", len(games), "apps_to_fetch", len(appIDs))

	for _, appID := range appIDs {
		if err := s.fetchApp(ctx, log, appID); err != nil {
			return err // rate limited or shutting down; what's fetched so far is kept
		}
		p.fetched++
		s.saveProgress(ctx, log, runID, p)
	}
	return s.fetchReviews(ctx, log, steamID)
}

// fetchReviews stores review summaries for the user's store apps whose
// reviews are missing or stale, many apps per request. This also fills in
// reviews for apps whose details were fetched by an earlier Refresh.
func (s *Syncer) fetchReviews(ctx context.Context, log *slog.Logger, steamID uint64) error {
	appIDs, err := s.q.AppsNeedingReviews(ctx, store.AppsNeedingReviewsParams{
		SteamID:     int64(steamID),
		StaleBefore: time.Now().Add(-s.StaleAfter),
	})
	if err != nil {
		return fmt.Errorf("list apps needing reviews: %w", err)
	}
	log.Info("fetching reviews", "apps", len(appIDs))
	for batch := range slices.Chunk(appIDs, steam.MaxReviewBatch) {
		got, err := s.steam.AppReviews(ctx, batch)
		switch {
		case stops(err):
			return err
		case err != nil:
			// Left for the next Refresh, like a failed details fetch.
			log.Warn("reviews failed", "first_appid", batch[0], "apps", len(batch), "error", err)
			continue
		}
		for _, appID := range batch {
			r, ok := got[appID]
			if !ok {
				continue // Steam didn't answer for it; retried next Refresh
			}
			if err := s.q.UpdateAppReviews(ctx, store.UpdateAppReviewsParams{
				Appid:           appID,
				ReviewScore:     ptr(int32(r.Score)),
				ReviewScoreDesc: nonEmpty(r.ScoreDesc),
				ReviewsPositive: ptr(int32(r.Positive)),
				ReviewsTotal:    ptr(int32(r.TotalCount)),
			}); err != nil {
				return fmt.Errorf("save reviews for %d: %w", appID, err)
			}
		}
	}
	return nil
}

// fetchApp stores store details for one app. It only returns an error
// that should stop the whole Refresh.
func (s *Syncer) fetchApp(ctx context.Context, log *slog.Logger, appID int64) error {
	log = log.With("appid", appID)

	d, err := s.steam.AppDetails(ctx, appID)
	switch {
	case errors.Is(err, steam.ErrAppUnavailable):
		log.Info("app has no store page")
		return s.markApp(ctx, appID, "unavailable")
	case stops(err):
		return err
	case err != nil:
		log.Warn("store details failed", "error", err)
		return s.markApp(ctx, appID, "error")
	}
	if err := s.q.UpdateAppDetails(ctx, detailsParams(d)); err != nil {
		return fmt.Errorf("save details for %d: %w", appID, err)
	}
	return nil
}

func (s *Syncer) markApp(ctx context.Context, appID int64, status string) error {
	return s.q.MarkAppDetailsStatus(ctx, store.MarkAppDetailsStatusParams{Appid: appID, DetailsStatus: status})
}

func (s *Syncer) saveProgress(ctx context.Context, log *slog.Logger, runID int64, p progress) {
	if err := s.q.UpdateSyncProgress(ctx, store.UpdateSyncProgressParams{
		ID: runID, GamesSeen: p.games, DetailsTotal: p.total, DetailsFetched: p.fetched,
	}); err != nil {
		log.Warn("save refresh progress", "error", err)
	}
}

// stops reports errors that end the Refresh rather than skipping one app.
func stops(err error) bool {
	return errors.Is(err, steam.ErrRateLimited) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// userMessage turns an error into the text shown in the app.
func userMessage(err error) string {
	switch {
	case errors.Is(err, steam.ErrPrivateProfile):
		return "Steam didn't share your library. It does that only when your profile's Game details are Public, or when this app runs with your own Steam API key. Change one of them, then sync again."
	case errors.Is(err, steam.ErrRateLimited):
		return "Steam is limiting requests. What was fetched so far is saved; refresh again in a few minutes to continue."
	case errors.Is(err, context.Canceled):
		return "The refresh was interrupted because the server stopped. Refresh again to continue."
	default:
		return "The refresh failed: " + err.Error()
	}
}

func detailsParams(d *steam.AppDetails) store.UpdateAppDetailsParams {
	p := store.UpdateAppDetailsParams{
		Appid:            d.AppID,
		Name:             d.Name,
		Type:             nonEmpty(d.Type),
		ShortDescription: nonEmpty(d.ShortDescription),
		HeaderImage:      nonEmpty(d.HeaderImage),
		ReleaseDate:      d.ReleaseDate,
		ReleaseDateText:  nonEmpty(d.ReleaseDateText),
		ComingSoon:       d.ComingSoon,
		IsFree:           d.IsFree,
		Developers:       d.Developers,
		Publishers:       d.Publishers,
		Genres:           d.Genres,
		Categories:       d.Categories,
		PlatformWindows:  d.Windows,
		PlatformMac:      d.Mac,
		PlatformLinux:    d.Linux,
		PriceCurrency:    nonEmpty(d.PriceCurrency),
		DetailsRaw:       d.Raw,
	}
	if d.MetacriticScore != nil {
		p.MetacriticScore = ptr(int32(*d.MetacriticScore))
	}
	if d.PriceCents != nil {
		p.PriceCents = ptr(int32(*d.PriceCents))
	}
	return p
}

func ptr[T any](v T) *T { return &v }

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
