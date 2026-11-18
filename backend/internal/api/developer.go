package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/steam"
)

// DeveloperSearch lists a developer's games on the Steam store.
type DeveloperSearch interface {
	DeveloperGames(ctx context.Context, developer string) ([]steam.StoreGame, error)
}

// OwnedQueries tells which apps the signed-in user owns.
type OwnedQueries interface {
	OwnedAmong(ctx context.Context, steamID int64, appids []int64) ([]int64, error)
}

// developerCacheTTL is how long a developer's store list is reused. A
// developer's catalogue changes rarely, and the store is slow to ask
// (requests are spaced 1.5 s apart, shared with sync).
const developerCacheTTL = 6 * time.Hour

// DeveloperCache remembers each developer's store list in memory, for every
// user, so opening game pages doesn't keep asking the store.
type DeveloperCache struct {
	Search DeveloperSearch
	now    func() time.Time

	mu      sync.Mutex
	entries map[string]developerEntry
}

type developerEntry struct {
	games   []steam.StoreGame
	fetched time.Time
}

func (c *DeveloperCache) Games(ctx context.Context, developer string) ([]steam.StoreGame, error) {
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	key := strings.ToLower(developer)
	c.mu.Lock()
	e, ok := c.entries[key]
	c.mu.Unlock()
	if ok && now().Sub(e.fetched) < developerCacheTTL {
		return e.games, nil
	}
	games, err := c.Search.DeveloperGames(ctx, developer)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[string]developerEntry{}
	}
	c.entries[key] = developerEntry{games: games, fetched: now()}
	c.mu.Unlock()
	return games, nil
}

type developerGameJSON struct {
	AppID           int64   `json:"appid"`
	Name            string  `json:"name"`
	StoreURL        string  `json:"store_url"`
	CapsuleImage    *string `json:"capsule_image"`
	ReleasedText    *string `json:"released_text"`
	ReviewScoreDesc *string `json:"review_score_desc"`
	ReviewPct       *int    `json:"review_pct"`
	ReviewsTotal    *int    `json:"reviews_total"`
	Owned           bool    `json:"owned"` // in the signed-in user's library
}

// listDeveloperGames handles GET /api/developer-games?developer=<name>: the
// developer's games on the Steam store, each marked owned or not.
func listDeveloperGames(cache *DeveloperCache, q OwnedQueries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.UserFrom(r.Context())
		developer := strings.TrimSpace(r.URL.Query().Get("developer"))
		if developer == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "developer is required"})
			return
		}
		// Don't keep the page waiting through the store's long backoffs.
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		games, err := cache.Games(ctx, developer)
		if err != nil {
			slog.Warn("developer games", "developer", developer, "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the Steam store didn't answer, try again later"})
			return
		}

		ids := make([]int64, len(games))
		for i, g := range games {
			ids[i] = g.AppID
		}
		owned := map[int64]bool{}
		if len(ids) > 0 {
			got, err := q.OwnedAmong(r.Context(), int64(u.SteamID), ids)
			if err != nil {
				slog.Error("owned among", "error", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
				return
			}
			for _, id := range got {
				owned[id] = true
			}
		}

		out := struct {
			Developer string              `json:"developer"`
			Games     []developerGameJSON `json:"games"`
		}{developer, make([]developerGameJSON, 0, len(games))}
		for _, g := range games {
			out.Games = append(out.Games, developerGameJSON{
				AppID:           g.AppID,
				Name:            g.Name,
				StoreURL:        "https://store.steampowered.com/app/" + strconv.FormatInt(g.AppID, 10),
				CapsuleImage:    nonEmpty(g.CapsuleImage),
				ReleasedText:    nonEmpty(g.ReleasedText),
				ReviewScoreDesc: nonEmpty(g.ReviewDesc),
				ReviewPct:       g.ReviewPct,
				ReviewsTotal:    g.ReviewsTotal,
				Owned:           owned[g.AppID],
			})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
