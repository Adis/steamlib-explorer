package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	tests := []struct {
		name string
		db   fakeDB
		want int
	}{
		{"database up", fakeDB{}, http.StatusOK},
		{"database down", fakeDB{errors.New("down")}, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			NewRouter(Deps{DB: tt.db, Auth: &auth.Service{}}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestUIServesOtherPaths(t *testing.T) {
	ui := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	router := NewRouter(Deps{DB: fakeDB{}, Auth: &auth.Service{}, UI: ui})
	for path, want := range map[string]int{
		"/":                http.StatusTeapot,
		"/assets/index.js": http.StatusTeapot,
		"/healthz":         http.StatusOK,
		"/api/me":          http.StatusUnauthorized, // API routes still win over the UI
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, want)
		}
	}
}
