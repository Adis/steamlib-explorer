package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

// MCPTokens keeps each user's MCP access token. Only a hash is stored, so a
// token is shown once, when made.
type MCPTokens interface {
	SaveMCPToken(ctx context.Context, steamID int64, tokenHash []byte) error
	MCPTokenCreatedAt(ctx context.Context, steamID int64) (time.Time, error)
	GetMCPTokenUser(ctx context.Context, tokenHash []byte) (store.User, error)
	DeleteMCPToken(ctx context.Context, steamID int64) error
}

// mcpTokenPrefix marks the app's tokens, so one pasted in the wrong place is
// recognisable (and secret scanners can find them).
const mcpTokenPrefix = "slx_"

func newMCPToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return mcpTokenPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

func hashMCPToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// verifyMCPToken checks the bearer token sent to /mcp. The token stands for
// its user: the tools read that user's library.
func verifyMCPToken(tokens MCPTokens) mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		if !strings.HasPrefix(token, mcpTokenPrefix) {
			return nil, mcpauth.ErrInvalidToken
		}
		u, err := tokens.GetMCPTokenUser(ctx, hashMCPToken(token))
		if errors.Is(err, store.ErrNotFound) {
			return nil, mcpauth.ErrInvalidToken
		}
		if err != nil {
			return nil, fmt.Errorf("check MCP token: %w", err)
		}
		return &mcpauth.TokenInfo{UserID: strconv.FormatInt(u.SteamID, 10)}, nil
	}
}

type mcpTokenJSON struct {
	Exists    bool       `json:"exists"`
	CreatedAt *time.Time `json:"created_at"`
	// Token is the new token, only in the answer to POST: it can't be read
	// again later.
	Token string `json:"token,omitempty"`
}

// mcpTokenStatus handles GET /api/mcp-token: whether the user has a token.
func mcpTokenStatus(tokens MCPTokens) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.UserFrom(r.Context())
		created, err := tokens.MCPTokenCreatedAt(r.Context(), int64(u.SteamID))
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusOK, mcpTokenJSON{})
			return
		}
		if err != nil {
			slog.Error("read MCP token", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		writeJSON(w, http.StatusOK, mcpTokenJSON{Exists: true, CreatedAt: &created})
	}
}

// createMCPToken handles POST /api/mcp-token: makes a new token, replacing
// the user's old one, and returns it this once.
func createMCPToken(tokens MCPTokens) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// JSON only, like the setup page: another website can't send it
		// without the browser asking first.
		if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "send JSON"})
			return
		}
		u, _ := auth.UserFrom(r.Context())
		token, err := newMCPToken()
		if err == nil {
			err = tokens.SaveMCPToken(r.Context(), int64(u.SteamID), hashMCPToken(token))
		}
		var created time.Time
		if err == nil {
			created, err = tokens.MCPTokenCreatedAt(r.Context(), int64(u.SteamID))
		}
		if err != nil {
			slog.Error("create MCP token", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		slog.Info("MCP token created", "steam_id", u.SteamID)
		writeJSON(w, http.StatusOK, mcpTokenJSON{Exists: true, CreatedAt: &created, Token: token})
	}
}

// deleteMCPToken handles DELETE /api/mcp-token: assistants using the token
// lose access at once.
func deleteMCPToken(tokens MCPTokens) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.UserFrom(r.Context())
		if err := tokens.DeleteMCPToken(r.Context(), int64(u.SteamID)); err != nil {
			slog.Error("delete MCP token", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		slog.Info("MCP token revoked", "steam_id", u.SteamID)
		w.WriteHeader(http.StatusNoContent)
	}
}
