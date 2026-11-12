// Package steam talks to the Steam Web API and the (unofficial) store endpoints.
//
// Endpoints used:
//   - IPlayerService/GetOwnedGames: a user's library, needs an API key
//   - store.steampowered.com/api/appdetails: store data for one app, no key
//   - IStoreBrowseService/GetItems: review summaries for up to 50 apps, no key
//
// The store endpoint throttles at roughly 200 requests per 5 minutes, so
// every store request, and GetItems, waits on one shared limiter, and
// 429/403 answers are retried with exponential backoff.
package steam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

var (
	// ErrPrivateProfile means Steam returned no games: the profile or its
	// game details are not public (or the account owns nothing).
	ErrPrivateProfile = errors.New("steam: library is private or empty")
	// ErrAppUnavailable means the store has no page for the app (delisted,
	// region locked, or not a store app).
	ErrAppUnavailable = errors.New("steam: app has no store page")
	// ErrRateLimited means the store kept refusing requests after all retries.
	ErrRateLimited = errors.New("steam: store rate limit, retries exhausted")
)

const (
	defaultAPIBase   = "https://api.steampowered.com"
	defaultStoreBase = "https://store.steampowered.com"
)

// Client is safe for concurrent use. Share one Client so all store requests
// go through the same limiter.
type Client struct {
	apiKey    string
	http      *http.Client
	apiBase   string
	storeBase string
	country   string // cc= for prices
	language  string // l= for genre and category names

	storeLimiter *rate.Limiter
	backoff      []time.Duration // waits after a 429/403, one per retry
}

// Option changes a Client setting.
type Option func(*Client)

// WithHTTPClient replaces the default HTTP client (30 s timeout).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithBaseURLs points the client at other hosts, e.g. a test server.
func WithBaseURLs(api, store string) Option {
	return func(c *Client) { c.apiBase, c.storeBase = api, store }
}

// WithStoreRate sets the pause between store requests (default 1.5 s).
func WithStoreRate(every time.Duration) Option {
	return func(c *Client) { c.storeLimiter = rate.NewLimiter(rate.Every(every), 1) }
}

// WithBackoff sets the waits before each retry of a throttled store request
// (default 30 s, 60 s, 120 s).
func WithBackoff(waits ...time.Duration) Option { return func(c *Client) { c.backoff = waits } }

// WithLocale sets the store country code and language (default "us", "english").
func WithLocale(country, language string) Option {
	return func(c *Client) { c.country, c.language = country, language }
}

func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:       apiKey,
		http:         &http.Client{Timeout: 30 * time.Second},
		apiBase:      defaultAPIBase,
		storeBase:    defaultStoreBase,
		country:      "us",
		language:     "english",
		storeLimiter: rate.NewLimiter(rate.Every(1500*time.Millisecond), 1),
		backoff:      []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// StatusError is a non-200 answer that wasn't retried.
type StatusError struct {
	URL    string
	Status int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("steam: %s returned HTTP %d", e.URL, e.Status)
}

// getJSON does one GET and decodes the body into v.
func (c *Client) getJSON(ctx context.Context, u string, v any) (status int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, &StatusError{URL: redact(u), Status: resp.StatusCode}
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return resp.StatusCode, fmt.Errorf("steam: decode %s: %w", redact(u), err)
	}
	return resp.StatusCode, nil
}

// getStore waits for the shared limiter, then GETs a store URL, retrying
// with backoff while the store answers 429 or 403.
func (c *Client) getStore(ctx context.Context, u string, v any) error {
	for attempt := 0; ; attempt++ {
		if err := c.storeLimiter.Wait(ctx); err != nil {
			return err
		}
		status, err := c.getJSON(ctx, u, v)
		if status != http.StatusTooManyRequests && status != http.StatusForbidden {
			return err
		}
		if attempt >= len(c.backoff) {
			return ErrRateLimited
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.backoff[attempt]):
		}
	}
}

// redact drops the API key from URLs that end up in errors and logs.
func redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Has("key") {
		q.Set("key", "REDACTED")
		u.RawQuery = q.Encode()
	}
	return u.String()
}
