package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Adis/steamlib-explorer/backend/internal/auth"
	"github.com/Adis/steamlib-explorer/backend/internal/steam"
	"github.com/Adis/steamlib-explorer/backend/internal/store"
)

// DeveloperSearch lists a developer's games on the Steam store.
type DeveloperSearch interface {
	DeveloperGames(ctx context.Context, developer string) ([]steam.StoreGame, error)
}

// OwnedQueries tells which apps the signed-in user owns.
type OwnedQueries interface {
	OwnedAmong(ctx context.Context, steamID int64, appids []int64) ([]int64, error)
}

// developerCacheTTL is how long a developer's store list is reused. The list
// is only asked for when a game's details page opens, a developer's catalogue
// changes rarely, and the store is slow to ask (requests are spaced 1.5 s
// apart, shared with sync), so each developer is asked at most once a day.
const developerCacheTTL = 24 * time.Hour

// developerFetchTimeout bounds one store request, through the store's
// rate-limit backoffs.
const developerFetchTimeout = 4 * time.Minute

// developerCacheMax caps how many developers' lists are kept in memory.
const developerCacheMax = 500

// SavedDeveloperGames keeps developers' store lists in the database, so the
// cache survives restarts.
type SavedDeveloperGames interface {
	GetDeveloperGames(ctx context.Context, developer string) (store.DeveloperGames, error)
	SaveDeveloperGames(ctx context.Context, arg store.DeveloperGames) error
	DeleteDeveloperGamesBefore(ctx context.Context, before time.Time) error
}

// DeveloperCache remembers each developer's store list, for every user, so
// opening game pages doesn't keep asking the store: in memory, and in the
// database when Saved is set. Pages opened at the same time for one developer
// share one store request.
type DeveloperCache struct {
	Search DeveloperSearch
	Saved  SavedDeveloperGames // optional
	now    func() time.Time

	mu       sync.Mutex
	entries  map[string]developerEntry
	inFlight map[string]*developerFetch
}

type developerEntry struct {
	games   []steam.StoreGame
	fetched time.Time
}

// developerFetch is a store request in progress; done closes when it ends.
type developerFetch struct {
	done  chan struct{}
	games []steam.StoreGame
	err   error
}

func (c *DeveloperCache) Games(ctx context.Context, developer string) ([]steam.StoreGame, error) {
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	key := strings.ToLower(developer)

	c.mu.Lock()
	if e, ok := c.entries[key]; ok && now().Sub(e.fetched) < developerCacheTTL {
		c.mu.Unlock()
		return e.games, nil
	}
	f, ok := c.inFlight[key]
	if !ok {
		f = &developerFetch{done: make(chan struct{})}
		if c.inFlight == nil {
			c.inFlight = map[string]*developerFetch{}
		}
		c.inFlight[key] = f
		go c.fetch(key, developer, f, now)
	}
	c.mu.Unlock()

	select {
	case <-f.done:
		return f.games, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// fetch asks the store for one developer's games and caches the answer. It
// isn't tied to the page that started it: other pages may be waiting on it,
// and an answer that arrives after the page gave up is still worth keeping.
func (c *DeveloperCache) fetch(key, developer string, f *developerFetch, now func() time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), developerFetchTimeout)
	defer cancel()

	fetched := now()
	if games, at, ok := c.loadSaved(ctx, key, fetched); ok {
		f.games, fetched = games, at
	} else {
		f.games, f.err = c.Search.DeveloperGames(ctx, developer)
		if f.err == nil {
			c.save(ctx, key, f.games, fetched)
		}
	}

	c.mu.Lock()
	delete(c.inFlight, key)
	if f.err == nil {
		c.store(key, developerEntry{games: f.games, fetched: fetched}, now())
	}
	c.mu.Unlock()
	close(f.done)
}

// loadSaved returns the list saved in the database, if there is one younger
// than the TTL.
func (c *DeveloperCache) loadSaved(ctx context.Context, key string, now time.Time) ([]steam.StoreGame, time.Time, bool) {
	if c.Saved == nil {
		return nil, time.Time{}, false
	}
	saved, err := c.Saved.GetDeveloperGames(ctx, key)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Warn("read saved developer games", "developer", key, "error", err)
		}
		return nil, time.Time{}, false
	}
	if now.Sub(saved.FetchedAt) >= developerCacheTTL {
		return nil, time.Time{}, false
	}
	var games []steam.StoreGame
	if err := json.Unmarshal(saved.Games, &games); err != nil {
		slog.Warn("saved developer games unreadable", "developer", key, "error", err)
		return nil, time.Time{}, false
	}
	return games, saved.FetchedAt, true
}

// save keeps a fresh list in the database and drops expired ones. Failing
// only costs a store request later, so it is logged, not returned.
func (c *DeveloperCache) save(ctx context.Context, key string, games []steam.StoreGame, fetched time.Time) {
	if c.Saved == nil {
		return
	}
	b, err := json.Marshal(games)
	if err == nil {
		err = c.Saved.SaveDeveloperGames(ctx, store.DeveloperGames{Developer: key, Games: b, FetchedAt: fetched})
	}
	if err == nil {
		err = c.Saved.DeleteDeveloperGamesBefore(ctx, fetched.Add(-developerCacheTTL))
	}
	if err != nil {
		slog.Warn("save developer games", "developer", key, "error", err)
	}
}

// store saves an entry, first making room: expired entries go, then the
// oldest while the cache is full. c.mu must be held.
func (c *DeveloperCache) store(key string, e developerEntry, now time.Time) {
	if c.entries == nil {
		c.entries = map[string]developerEntry{}
	}
	if _, ok := c.entries[key]; !ok && len(c.entries) >= developerCacheMax {
		oldestKey, oldest := "", now
		for k, v := range c.entries {
			if now.Sub(v.fetched) >= developerCacheTTL {
				delete(c.entries, k)
			} else if v.fetched.Before(oldest) {
				oldestKey, oldest = k, v.fetched
			}
		}
		if len(c.entries) >= developerCacheMax {
			delete(c.entries, oldestKey)
		}
	}
	c.entries[key] = e
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
