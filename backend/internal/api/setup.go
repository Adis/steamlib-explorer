package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strings"

	"github.com/Adis/steamlib-explorer/backend/internal/steam"
)

// KeySetup is what the first-run setup needs: whether a Steam Web API key is
// set, a way to check one with Steam, and a way to save and start using it.
type KeySetup interface {
	HasAPIKey() bool
	CheckAPIKey(ctx context.Context, key string) error
	SaveAPIKey(key string) error
}

// Steam Web API keys are 32 hexadecimal characters.
var apiKeyPattern = regexp.MustCompile(`^[0-9A-Fa-f]{32}$`)

// setupStatus tells the frontend whether to show the setup page. It needs no
// sign-in: it runs before anyone can sign in.
func setupStatus(s KeySetup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"needs_api_key": !s.HasAPIKey()})
	}
}

// saveAPIKey stores the key typed on the setup page. It needs no sign-in, so
// it only works while no key is set: once there is one, it can only be
// changed in the OS keychain (or settings.json) or with STEAM_API_KEY.
func saveAPIKey(s KeySetup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// JSON only: another website can't send a cross-origin JSON request
		// without the browser asking first, and this server never says yes.
		if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "send JSON"})
			return
		}
		if s.HasAPIKey() {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a Steam API key is already set"})
			return
		}
		var body struct {
			Key string `json:"steam_api_key"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "send {\"steam_api_key\": \"…\"}"})
			return
		}
		key := strings.TrimSpace(body.Key)
		if !apiKeyPattern.MatchString(key) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "That doesn't look like a Steam Web API key: it should be 32 letters and digits (0-9, A-F)."})
			return
		}
		if err := s.CheckAPIKey(r.Context(), key); errors.Is(err, steam.ErrBadAPIKey) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Steam didn't accept this key. Check that you copied all of it."})
			return
		} else if err != nil {
			slog.Warn("checking a Steam API key failed", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Couldn't reach Steam to check the key. Check your internet connection and try again."})
			return
		}
		if err := s.SaveAPIKey(key); err != nil {
			slog.Error("saving the Steam API key failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "The key works, but saving it failed: " + err.Error()})
			return
		}
		slog.Info("Steam API key saved from the setup page")
		w.WriteHeader(http.StatusNoContent)
	}
}
