// Package app runs the whole backend: database, Steam client, sync and the
// HTTP server with the embedded frontend. The command-line server and the
// desktop app both start it.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/api"
	"github.com/Adis/steamlib-explorer/backend/internal/apikey"
	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/config"
	"github.com/Adis/steamlib-explorer/backend/internal/database"
	"github.com/Adis/steamlib-explorer/backend/internal/logging"
	"github.com/Adis/steamlib-explorer/backend/internal/steam"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
	"github.com/Adis/steamlib-explorer/backend/internal/syncer"
	"github.com/Adis/steamlib-explorer/backend/internal/webui"
)

// Run serves the app until ctx is cancelled, then shuts down cleanly and
// returns nil. ready, if not nil, is called once the port is listening.
func Run(ctx context.Context, cfg config.Config, ready func()) error {
	_, closeLog, err := logging.Setup(cfg.LogFile, slog.LevelInfo)
	if err != nil {
		return err
	}
	defer func() { _ = closeLog() }()
	if cfg.LogFile != "" {
		slog.Info("logging to console and file", "file", cfg.LogFile)
	}

	queries, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer queries.Close()

	if n, err := queries.FailRunningSyncs(ctx); err != nil {
		return err
	} else if n > 0 {
		slog.Warn("marked refreshes interrupted by the last shutdown as failed", "count", n)
	}

	// Every call to Steam goes through the logging transport.
	steamHTTP := &http.Client{Timeout: 30 * time.Second, Transport: &logging.Transport{Service: "steam"}}
	// STEAM_API_KEY wins, and then the keychain is never touched; otherwise
	// the key saved by the setup page, if any.
	keys := &apikey.Store{Keyring: apikey.OSKeyring{}, File: cfg.SettingsFile}
	apiKey := cfg.SteamAPIKey
	if apiKey == "" {
		saved, where, err := keys.Load()
		if err != nil {
			return err
		}
		apiKey = saved
		if apiKey == "" {
			slog.Info("no Steam API key yet: the app will ask for one")
		} else {
			slog.Info("using the saved Steam API key", "from", where)
		}
	}
	steamClient := steam.New(apiKey, steam.WithHTTPClient(steamHTTP))

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
		// Only requests addressed to this app, against DNS rebinding.
		Handler: api.CheckHost(cfg.PublicURL)(api.NewRouter(api.Deps{
			DB: queries, Auth: authService, Refresher: refresher, SyncQ: queries, Games: queries,
			Setup:      &keySetup{client: steamClient, keys: keys},
			Developers: &api.DeveloperCache{Search: steamClient, Saved: queries}, Owned: queries,
			UI: webui.Handler(),
		})),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Listen before calling ready, so a browser opened then finds the page.
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "public_url", cfg.PublicURL)
		errc <- srv.Serve(ln)
	}()
	if ready != nil {
		ready()
	}

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
