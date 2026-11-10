package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

const (
	// SessionCookie holds the session token.
	SessionCookie = "steamlib_session"
	// stateCookie ties the Steam callback to the browser that started login.
	stateCookie = "steamlib_login_state"
	// SessionTTL is how long a sign-in lasts.
	SessionTTL = 30 * 24 * time.Hour
)

// User is a signed-in Steam user.
type User struct {
	SteamID     uint64 `json:"steam_id,string"`
	PersonaName string `json:"persona_name"`
	AvatarURL   string `json:"avatar_url"`
}

// Store is the persistence the auth package needs.
type Store interface {
	UpsertUser(ctx context.Context, u User) error
	CreateSession(ctx context.Context, tokenHash []byte, steamID uint64, expires time.Time) error
	// SessionUser returns ErrNoSession when the token is unknown or expired.
	SessionUser(ctx context.Context, tokenHash []byte) (User, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken is what the database stores, so a leaked table can't be used to
// impersonate anyone.
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
