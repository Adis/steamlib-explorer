package auth

import (
	"context"

	"github.com/Adis/steamlib-explorer/backend/internal/steam"
)

// SteamProfiles fetches names and avatars with the Steam Web API.
type SteamProfiles struct{ Client *steam.Client }

func (p SteamProfiles) Profile(ctx context.Context, steamID uint64) (User, error) {
	s, err := p.Client.PlayerSummary(ctx, steamID)
	if err != nil {
		return User{}, err
	}
	return User{SteamID: steamID, PersonaName: s.PersonaName, AvatarURL: s.AvatarURL}, nil
}
