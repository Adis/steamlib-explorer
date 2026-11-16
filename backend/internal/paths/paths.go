// Package paths says where the app keeps its files when nothing else is
// configured: one folder per user, in the place each OS expects.
package paths

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const appDir = "steamlib-explorer"

// DataDir is the folder for the app's database (and later its settings):
//
//	Linux    $XDG_DATA_HOME/steamlib-explorer, else ~/.local/share/steamlib-explorer
//	Windows  %AppData%\steamlib-explorer
//	macOS    ~/Library/Application Support/steamlib-explorer
//
// STEAMLIB_DATA_DIR overrides it. The folder is not created here.
func DataDir() (string, error) {
	if dir := os.Getenv("STEAMLIB_DATA_DIR"); dir != "" {
		return dir, nil
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" {
		if xdg := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(xdg) {
			return filepath.Join(xdg, appDir), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("can't find your home folder; set DATABASE_URL or STEAMLIB_DATA_DIR")
		}
		return filepath.Join(home, ".local", "share", appDir), nil
	}
	base, err := os.UserConfigDir() // %AppData% on Windows, Application Support on macOS
	if err != nil {
		return "", errors.New("can't find your settings folder; set DATABASE_URL or STEAMLIB_DATA_DIR")
	}
	return filepath.Join(base, appDir), nil
}

// SettingsFile is settings.json in the data folder.
func SettingsFile() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// DefaultDatabase is the SQLite file used when DATABASE_URL is not set.
func DefaultDatabase() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "steamlib.db"), nil
}
