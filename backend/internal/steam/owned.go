package steam

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// OwnedGame is one entry of a user's library.
type OwnedGame struct {
	AppID           int64
	Name            string
	PlaytimeForever int        // minutes
	Playtime2Weeks  int        // minutes
	LastPlayed      *time.Time // nil when never played
}

type ownedGamesResponse struct {
	Response struct {
		GameCount *int `json:"game_count"`
		Games     []struct {
			AppID           int64  `json:"appid"`
			Name            string `json:"name"`
			PlaytimeForever int    `json:"playtime_forever"`
			Playtime2Weeks  int    `json:"playtime_2weeks"`
			RTimeLastPlayed int64  `json:"rtime_last_played"`
		} `json:"games"`
	} `json:"response"`
}

// OwnedGames returns every game the user owns, including played free games.
// It returns ErrPrivateProfile when Steam hides the library.
func (c *Client) OwnedGames(ctx context.Context, steamID uint64) ([]OwnedGame, error) {
	q := url.Values{}
	q.Set("key", c.apiKey)
	q.Set("steamid", strconv.FormatUint(steamID, 10))
	q.Set("include_appinfo", "1")
	q.Set("include_played_free_games", "1")
	q.Set("format", "json")
	u := c.apiBase + "/IPlayerService/GetOwnedGames/v1/?" + q.Encode()

	var r ownedGamesResponse
	if _, err := c.getJSON(ctx, u, &r); err != nil {
		return nil, err
	}
	// A private library comes back as {"response":{}}: no game_count at all.
	if r.Response.GameCount == nil || *r.Response.GameCount == 0 {
		return nil, ErrPrivateProfile
	}

	games := make([]OwnedGame, 0, len(r.Response.Games))
	for _, g := range r.Response.Games {
		og := OwnedGame{
			AppID:           g.AppID,
			Name:            g.Name,
			PlaytimeForever: g.PlaytimeForever,
			Playtime2Weeks:  g.Playtime2Weeks,
		}
		if g.RTimeLastPlayed > 0 {
			t := time.Unix(g.RTimeLastPlayed, 0).UTC()
			og.LastPlayed = &t
		}
		games = append(games, og)
	}
	return games, nil
}
