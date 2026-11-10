// Command server runs the Steamlib Explorer backend.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/api"
	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/config"
	"github.com/Adis/steamlib-explorer/backend/internal/database"
	"github.com/Adis/steamlib-explorer/backend/internal/steam"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}

	steamClient := steam.New(cfg.SteamAPIKey)
	authService := &auth.Service{
		OpenID:    &auth.OpenID{Endpoint: auth.SteamOpenIDEndpoint, HTTP: &http.Client{Timeout: 15 * time.Second}},
		Store:     store.AuthStore{Q: store.New(pool)},
		Profiles:  auth.SteamProfiles{Client: steamClient},
		PublicURL: cfg.PublicURL,
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           api.NewRouter(api.Deps{DB: pool, Auth: authService}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "public_url", cfg.PublicURL)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}
