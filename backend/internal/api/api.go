// Package api holds the HTTP routes.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/logging"
)

// Pinger is the part of the database pool the health check needs.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	DB        Pinger
	Auth      *auth.Service
	Refresher Refresher
	SyncQ     SyncQueries
	Games     GameQueries
	Setup     KeySetup
	// UI serves the frontend for every path the routes above do not match.
	// Nil leaves those paths as 404s.
	UI http.Handler
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequestID)
		r.Use(middleware.RealIP)
		r.Use(logging.Requests)

		r.Get("/healthz", health(d.DB))

		r.Get("/auth/steam/login", d.Auth.Login)
		r.Get("/auth/steam/callback", d.Auth.Callback)
		r.Post("/auth/logout", d.Auth.Logout)

		// First-run setup, before anyone can sign in.
		if d.Setup != nil {
			r.Get("/api/setup", setupStatus(d.Setup))
			r.Post("/api/setup", saveAPIKey(d.Setup))
		}

		r.Route("/api", func(r chi.Router) {
			r.Use(d.Auth.RequireUser)
			r.Get("/me", auth.Me)
			r.Post("/sync", startSync(d.Refresher))
			r.Get("/sync", syncStatus(d.SyncQ))
			r.Get("/games", listGames(d.Games))
			r.Get("/games/{appid}", getGame(d.Games))
			r.Get("/facets", listFacets(d.Games))
		})
	})

	// The frontend's files are left out of the request log, which is about
	// the API and Steam.
	if d.UI != nil {
		r.Handle("/*", d.UI)
	}
	return r
}

func health(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": "down"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "up"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
