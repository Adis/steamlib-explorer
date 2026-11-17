// Package apikey keeps the Steam Web API key that the setup page saves. It
// goes in the OS keychain (Windows Credential Manager, or the Secret Service
// on Linux: GNOME Keyring or KWallet), where other users and backups of the
// data folder can't read it. Without a keychain, as on a headless server, it
// falls back to settings.json in the data folder, readable only by the user.
// STEAM_API_KEY is handled by the caller and wins over both.
package apikey

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/zalando/go-keyring"

	"github.com/Adis/steamlib-explorer/backend/internal/settings"
)

// The keychain entry: shown as "steamlib-explorer" in Credential Manager or
// Passwords and Keys.
const (
	Service = "steamlib-explorer"
	User    = "steam-api-key"
)

// Where a key was found or saved.
const (
	InKeychain     = "keychain"
	InSettingsFile = "settings file"
)

// Keyring is the part of the OS keychain the store needs.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
}

// ErrNotFound is what a Keyring returns for a missing entry. Any other error
// means there is no usable keychain.
var ErrNotFound = keyring.ErrNotFound

// OSKeyring is the real keychain.
type OSKeyring struct{}

func (OSKeyring) Get(service, user string) (string, error) { return keyring.Get(service, user) }
func (OSKeyring) Set(service, user, secret string) error   { return keyring.Set(service, user, secret) }

// Store loads and saves the key.
type Store struct {
	Keyring Keyring
	File    string // settings.json
	mu      sync.Mutex
}

// Load returns the saved key ("" if none) and where it was. A key still in
// settings.json, from before the keychain was used, moves into the keychain
// when there is one, and is then removed from the file.
func (s *Store) Load() (key, where string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fromKeychain, kerr := s.Keyring.Get(Service, User)
	keychainOK := kerr == nil || errors.Is(kerr, ErrNotFound)
	if !keychainOK {
		slog.Info("no OS keychain, using settings.json for the Steam API key", "reason", kerr)
	}

	saved, err := settings.Load(s.File)
	if err != nil {
		return "", "", err
	}

	if kerr == nil && fromKeychain != "" {
		// A move that stopped after writing the keychain left the key in the
		// file too: finish it.
		if saved.SteamAPIKey == fromKeychain {
			s.clearFile(saved)
		}
		return fromKeychain, InKeychain, nil
	}
	if saved.SteamAPIKey == "" {
		return "", "", nil
	}
	if keychainOK {
		if err := s.Keyring.Set(Service, User, saved.SteamAPIKey); err != nil {
			slog.Warn("couldn't move the Steam API key into the OS keychain, keeping it in settings.json", "error", err)
			return saved.SteamAPIKey, InSettingsFile, nil
		}
		slog.Info("moved the Steam API key from settings.json into the OS keychain")
		s.clearFile(saved)
		return saved.SteamAPIKey, InKeychain, nil
	}
	return saved.SteamAPIKey, InSettingsFile, nil
}

// Save stores key in the keychain, or in settings.json when there is none,
// and returns where it went.
func (s *Store) Save(key string) (where string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	saved, err := settings.Load(s.File)
	if err != nil {
		return "", err
	}
	kerr := s.Keyring.Set(Service, User, key)
	if kerr == nil {
		if saved.SteamAPIKey != "" {
			s.clearFile(saved)
		}
		return InKeychain, nil
	}
	slog.Warn("no OS keychain, saving the Steam API key in settings.json", "reason", kerr)
	saved.SteamAPIKey = key
	if err := settings.Save(s.File, saved); err != nil {
		return "", err
	}
	return InSettingsFile, nil
}

// clearFile removes the key from settings.json once the keychain holds it.
// Failing only leaves a copy behind, so it is logged, not returned.
func (s *Store) clearFile(saved settings.Settings) {
	saved.SteamAPIKey = ""
	if err := settings.Save(s.File, saved); err != nil {
		slog.Warn("the Steam API key is in the OS keychain, but removing it from settings.json failed", "file", s.File, "error", err)
	}
}
