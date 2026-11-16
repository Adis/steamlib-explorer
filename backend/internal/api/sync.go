package api

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/steam"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/syncer"
)

// Refresher starts a Refresh for a user.
type Refresher interface {
	Start(ctx context.Context, steamID uint64) (int64, error)
}

// SyncQueries reads Refresh state.
type SyncQueries interface {
	LatestSyncRun(ctx context.Context, steamID int64) (store.SyncRun, error)
	LibraryStats(ctx context.Context, steamID int64) (store.LibraryStatsRow, error)
}

type syncRunJSON struct {
	ID             int64      `json:"id"`
	Status         string     `json:"status"` // running | ok | error
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	GamesSeen      int32      `json:"games_seen"`
	DetailsTotal   int32      `json:"details_total"`
	DetailsFetched int32      `json:"details_fetched"`
	// When fetching store details began; null while the library is still being read.
	DetailsStartedAt *time.Time `json:"details_started_at"`
	// Estimated seconds until the Refresh finishes; null when not running or
	// once only the reviews step is left.
	SecondsLeft *int    `json:"seconds_left"`
	Error       *string `json:"error"`
}

type libraryStatsJSON struct {
	Games       int64 `json:"games"`
	WithDetails int64 `json:"with_details"`
	WithReviews int64 `json:"with_reviews"`
	Unavailable int64 `json:"unavailable"`
	Failed      int64 `json:"failed"`
}

type syncStatusJSON struct {
	Run   *syncRunJSON     `json:"run"` // null before the first Refresh
	Stats libraryStatsJSON `json:"stats"`
}

// startSync handles POST /api/sync.
func startSync(r Refresher) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		u, _ := auth.UserFrom(req.Context())
		id, err := r.Start(req.Context(), u.SteamID)
		if errors.Is(err, syncer.ErrAlreadyRunning) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a refresh is already running"})
			return
		}
		if err != nil {
			slog.Error("start refresh", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not start the refresh"})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]int64{"run_id": id})
	}
}

// syncStatus handles GET /api/sync: the latest Refresh and library counts.
func syncStatus(q SyncQueries) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		u, _ := auth.UserFrom(req.Context())
		var out syncStatusJSON

		run, err := q.LatestSyncRun(req.Context(), int64(u.SteamID))
		switch {
		case errors.Is(err, store.ErrNotFound):
		case err != nil:
			slog.Error("load refresh status", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		default:
			out.Run = &syncRunJSON{
				ID: run.ID, Status: run.Status, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt,
				GamesSeen: run.GamesSeen, DetailsTotal: run.DetailsTotal, DetailsFetched: run.DetailsFetched,
				DetailsStartedAt: run.DetailsStartedAt, SecondsLeft: secondsLeft(run, time.Now()),
				Error: run.Error,
			}
		}

		stats, err := q.LibraryStats(req.Context(), int64(u.SteamID))
		if err != nil {
			slog.Error("load library stats", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		out.Stats = libraryStatsJSON{
			Games: stats.Games, WithDetails: stats.WithDetails, WithReviews: stats.WithReviews,
			Unavailable: stats.Unavailable, Failed: stats.Failed,
		}
		writeJSON(w, http.StatusOK, out)
	}
}

const (
	// Seconds per app assumed before a Refresh has a pace of its own.
	defaultSecondsPerApp = 3.0
	// How many apps' worth of weight the default pace keeps, so the first few
	// fetches don't swing the estimate.
	defaultPaceWeight = 5.0
	// Seconds per request of the reviews step that follows the details.
	secondsPerReviewBatch = 1.5
)

// secondsLeft estimates how long a running Refresh still needs: the store
// details left at the pace observed so far, plus the batched reviews step.
// The pace starts at defaultSecondsPerApp and moves to the observed one as
// apps are fetched. It returns nil when there is nothing to estimate.
func secondsLeft(run store.SyncRun, now time.Time) *int {
	if run.Status != "running" || run.DetailsStartedAt == nil || run.DetailsFetched >= run.DetailsTotal {
		return nil
	}
	elapsed := max(now.Sub(*run.DetailsStartedAt).Seconds(), 0)
	pace := (elapsed + defaultSecondsPerApp*defaultPaceWeight) / (float64(run.DetailsFetched) + defaultPaceWeight)
	details := float64(run.DetailsTotal-run.DetailsFetched) * pace
	reviews := math.Ceil(float64(run.DetailsTotal)/steam.MaxReviewBatch) * secondsPerReviewBatch
	left := int(math.Ceil(details + reviews))
	return &left
}
