package store

import (
	"context"
	"errors"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
)

// AuthStore implements auth.Store on top of a Store.
type AuthStore struct{ Q Store }

var _ auth.Store = AuthStore{}

func (s AuthStore) UpsertUser(ctx context.Context, u auth.User) error {
	_, err := s.Q.UpsertUser(ctx, UpsertUserParams{
		SteamID:     int64(u.SteamID),
		PersonaName: optional(u.PersonaName),
		AvatarUrl:   optional(u.AvatarURL),
	})
	return err
}

func (s AuthStore) CreateSession(ctx context.Context, tokenHash []byte, steamID uint64, expires time.Time) error {
	return s.Q.CreateSession(ctx, CreateSessionParams{TokenHash: tokenHash, SteamID: int64(steamID), ExpiresAt: expires})
}

func (s AuthStore) SessionUser(ctx context.Context, tokenHash []byte) (auth.User, error) {
	u, err := s.Q.GetSessionUser(ctx, tokenHash)
	if errors.Is(err, ErrNotFound) {
		return auth.User{}, auth.ErrNoSession
	}
	if err != nil {
		return auth.User{}, err
	}
	return auth.User{SteamID: uint64(u.SteamID), PersonaName: deref(u.PersonaName), AvatarURL: deref(u.AvatarUrl)}, nil
}

func (s AuthStore) DeleteSession(ctx context.Context, tokenHash []byte) error {
	return s.Q.DeleteSession(ctx, tokenHash)
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
