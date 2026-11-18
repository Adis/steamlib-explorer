// Command steamlib-explorer is the desktop app: it runs the whole app on
// this computer, puts an icon in the system tray (Open, Quit) and opens the
// explorer in the default browser. It needs no settings: the database goes
// in the user's data folder and the first page asks for a Steam API key.
//
// The terminal version for development and hosting is cmd/server.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"fyne.io/systray"
	"github.com/godbus/dbus/v5"

	"github.com/Adis/steamlib-explorer/backend/internal/app"
	"github.com/Adis/steamlib-explorer/backend/internal/browser"
	"github.com/Adis/steamlib-explorer/backend/internal/config"
	"github.com/Adis/steamlib-explorer/backend/internal/paths"
)

// version is set by release builds (-X main.version=1.2.3).
var version = "dev"

var (
	//go:embed icon/icon.png
	iconPNG []byte
	//go:embed icon/icon.ico
	iconICO []byte
)

func main() {
	if err := run(); err != nil {
		reportStartupError(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Started twice (e.g. from the menu again): show the running one.
	if alreadyRunning(cfg) {
		openBrowser(cfg.PublicURL)
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if !trayAvailable() {
		slog.Warn("no desktop session bus, so no tray icon: stop the app with Ctrl+C")
		return app.Run(ctx, cfg, func() { openBrowser(cfg.PublicURL) })
	}

	var runErr error
	systray.Run(func() {
		if runtime.GOOS == "windows" {
			systray.SetIcon(iconICO)
		} else {
			systray.SetIcon(iconPNG)
		}
		systray.SetTitle("Steamlib Explorer")
		systray.SetTooltip("Steamlib Explorer " + version + " is running")
		open := systray.AddMenuItem("Open Steamlib Explorer", "Show your library in the browser")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Stop Steamlib Explorer")

		appCtx, cancel := context.WithCancel(ctx)
		go func() {
			for {
				select {
				case <-open.ClickedCh:
					openBrowser(cfg.PublicURL)
				case <-quit.ClickedCh:
					cancel()
				case <-appCtx.Done():
					return
				}
			}
		}()
		go func() {
			defer systray.Quit()
			defer cancel()
			runErr = app.Run(appCtx, cfg, func() { openBrowser(cfg.PublicURL) })
		}()
	}, nil)
	return runErr
}

// trayAvailable reports whether a tray icon can be shown. On Linux the tray
// talks over the desktop's D-Bus session bus; without one (a bare terminal
// session) the tray library crashes on exit, so the app runs without it.
func trayAvailable() bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return true
	}
	_, err := dbus.SessionBus() // shared connection, which the tray then reuses
	return err == nil
}

// openBrowser opens url, or logs where to go when it can't.
func openBrowser(url string) {
	if err := browser.Open(url); err != nil {
		slog.Warn("could not open the browser; open the address yourself", "url", url, "error", err)
	}
}

// alreadyRunning reports whether this app already answers on the configured
// port, so a second start opens the browser instead of failing.
func alreadyRunning(cfg config.Config) bool {
	host := cfg.ListenHost
	if host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	c := http.Client{Timeout: time.Second}
	resp, err := c.Get("http://" + net.JoinHostPort(host, strconv.Itoa(cfg.Port)) + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var health struct {
		Database string `json:"database"`
	}
	return json.NewDecoder(resp.Body).Decode(&health) == nil && health.Database != ""
}

// reportStartupError leaves the error where a user without a terminal can
// find it: startup-error.txt in the data folder (and stderr when there is one).
func reportStartupError(err error) {
	msg := fmt.Sprintf("%s  Steamlib Explorer could not start: %v\n", time.Now().Format(time.RFC3339), err)
	fmt.Fprint(os.Stderr, msg)
	if dir, derr := paths.DataDir(); derr == nil {
		_ = os.MkdirAll(dir, 0o700)
		_ = os.WriteFile(filepath.Join(dir, "startup-error.txt"), []byte(msg), 0o600)
	}
}
