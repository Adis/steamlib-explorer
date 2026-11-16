// Command server runs the Steamlib Explorer backend from a terminal: for
// development and for hosting. The desktop app is cmd/steamlib-explorer.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Adis/steamlib-explorer/backend/internal/app"
	"github.com/Adis/steamlib-explorer/backend/internal/browser"
	"github.com/Adis/steamlib-explorer/backend/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	openBrowser := flag.Bool("open", false, "open the app in the default browser once it is running")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return app.Run(ctx, cfg, func() {
		if *openBrowser {
			if err := browser.Open(cfg.PublicURL); err != nil {
				slog.Warn("could not open the browser; open the address yourself", "url", cfg.PublicURL, "error", err)
			}
		}
	})
}
