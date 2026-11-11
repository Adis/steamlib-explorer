package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/syncer"
)

type fakeRefresher struct {
	err    error
	gotFor uint64
}

func (f *fakeRefresher) Start(_ context.Context, id uint64) (int64, error) {
	f.gotFor = id
	return 7, f.err
}

type fakeSyncQ struct{ run *store.SyncRun }

func (f fakeSyncQ) LatestSyncRun(context.Context, int64) (store.SyncRun, error) {
	if f.run == nil {
		return store.SyncRun{}, pgx.ErrNoRows
	}
	return *f.run, nil
}

func (f fakeSyncQ) LibraryStats(context.Context, int64) (store.LibraryStatsRow, error) {
	return store.LibraryStatsRow{Games: 3, WithDetails: 2, Unavailable: 1}, nil
}

func asUser(r *http.Request) *http.Request {
	return r.WithContext(auth.WithUser(r.Context(), auth.User{SteamID: 42}))
}

func TestStartSync(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"started", nil, http.StatusAccepted},
		{"already running", syncer.ErrAlreadyRunning, http.StatusConflict},
		{"database error", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeRefresher{err: tt.err}
			rec := httptest.NewRecorder()
			startSync(f)(rec, asUser(httptest.NewRequest(http.MethodPost, "/api/sync", nil)))
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
			if f.gotFor != 42 {
				t.Errorf("refresh started for %d, want the signed-in user", f.gotFor)
			}
		})
	}
}

func TestSyncStatus(t *testing.T) {
	msg := "nope"
	run := &store.SyncRun{ID: 7, Status: "error", StartedAt: time.Now(), GamesSeen: 3, DetailsTotal: 3, DetailsFetched: 1, Error: &msg}
	for _, q := range []fakeSyncQ{{}, {run: run}} {
		rec := httptest.NewRecorder()
		syncStatus(q)(rec, asUser(httptest.NewRequest(http.MethodGet, "/api/sync", nil)))
		var got syncStatusJSON
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("status %d, %v", rec.Code, err)
		}
		if got.Stats.Games != 3 || got.Stats.Unavailable != 1 {
			t.Errorf("stats = %+v", got.Stats)
		}
		if (q.run == nil) != (got.Run == nil) {
			t.Errorf("run = %+v, want present: %v", got.Run, q.run != nil)
		}
		if got.Run != nil && (got.Run.DetailsFetched != 1 || *got.Run.Error != "nope") {
			t.Errorf("run = %+v", got.Run)
		}
	}
}
