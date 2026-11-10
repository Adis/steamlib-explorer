package steam

import (
	"context"
	"errors"
	"net/url"
	"strconv"
)

// ErrPlayerNotFound means Steam has no profile for the SteamID.
var ErrPlayerNotFound = errors.New("steam: player not found")

// PlayerSummary is the public profile shown next to a signed-in user.
type PlayerSummary struct {
	SteamID     uint64
	PersonaName string
	AvatarURL   string // 184x184 avatar
}

// PlayerSummary fetches a user's display name and avatar.
func (c *Client) PlayerSummary(ctx context.Context, steamID uint64) (*PlayerSummary, error) {
	q := url.Values{}
	q.Set("key", c.apiKey)
	q.Set("steamids", strconv.FormatUint(steamID, 10))
	u := c.apiBase + "/ISteamUser/GetPlayerSummaries/v2/?" + q.Encode()

	var r struct {
		Response struct {
			Players []struct {
				SteamID     string `json:"steamid"`
				PersonaName string `json:"personaname"`
				AvatarFull  string `json:"avatarfull"`
			} `json:"players"`
		} `json:"response"`
	}
	if _, err := c.getJSON(ctx, u, &r); err != nil {
		return nil, err
	}
	if len(r.Response.Players) == 0 {
		return nil, ErrPlayerNotFound
	}
	p := r.Response.Players[0]
	return &PlayerSummary{SteamID: steamID, PersonaName: p.PersonaName, AvatarURL: p.AvatarFull}, nil
}
