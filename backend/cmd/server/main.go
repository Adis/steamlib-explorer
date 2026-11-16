// Command server runs the Steamlib Explorer backend.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/api"
	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/config"
	"github.com/Adis/steamlib-explorer/backend/internal/database"
	"github.com/Adis/steamlib-explorer/backend/internal/logging"
	"github.com/Adis/steamlib-explorer/backend/internal/steam"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/syncer"
	"github.com/Adis/steamlib-explorer/backend/internal/webui"
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

	_, closeLog, err := logging.Setup(cfg.LogFile, slog.LevelInfo)
	if err != nil {
		return err
	}
	defer closeLog()
	if cfg.LogFile != "" {
		slog.Info("logging to console and file", "file", cfg.LogFile)
	}

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}
	queries := store.New(pool)
	if n, err := queries.FailRunningSyncs(ctx); err != nil {
		return err
	} else if n > 0 {
		slog.Warn("marked refreshes interrupted by the last shutdown as failed", "count", n)
	}

	// Every call to Steam goes through the logging transport.
	steamHTTP := &http.Client{Timeout: 30 * time.Second, Transport: &logging.Transport{Service: "steam"}}
	steamClient := steam.New(cfg.SteamAPIKey, steam.WithHTTPClient(steamHTTP))

	authService := &auth.Service{
		OpenID:    &auth.OpenID{Endpoint: auth.SteamOpenIDEndpoint, HTTP: steamHTTP},
		Store:     store.AuthStore{Q: queries},
		Profiles:  auth.SteamProfiles{Client: steamClient},
		PublicURL: cfg.PublicURL,
	}

	// Refreshes outlive the request that starts them, so they get their own
	// context, cancelled on shutdown.
	syncCtx, cancelSyncs := context.WithCancel(context.Background())
	defer cancelSyncs()
	refresher := syncer.New(syncCtx, steamClient, queries)

	srv := &http.Server{
		Addr: net.JoinHostPort(cfg.ListenHost, strconv.Itoa(cfg.Port)),
		Handler: api.NewRouter(api.Deps{
			DB: pool, Auth: authService, Refresher: refresher, SyncQ: queries, Games: queries,
			UI: webui.Handler(),
		}),
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
	cancelSyncs()
	refresher.Wait()
	return nil
}
