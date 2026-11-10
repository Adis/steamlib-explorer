package steam

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AppDetails is the store data we keep for an app.
type AppDetails struct {
	AppID            int64
	Name             string
	Type             string
	ShortDescription string
	HeaderImage      string
	ReleaseDate      *time.Time // nil when Steam's text isn't a date ("Q3 2027", "Coming soon")
	ReleaseDateText  string
	ComingSoon       bool
	IsFree           bool
	Developers       []string
	Publishers       []string
	Genres           []string
	Categories       []string
	Windows          bool
	Mac              bool
	Linux            bool
	MetacriticScore  *int
	PriceCents       *int // final price after discount; nil when free or not for sale
	PriceCurrency    string
	Raw              json.RawMessage // the "data" object as Steam sent it
}

type appDetailsData struct {
	Type             string   `json:"type"`
	Name             string   `json:"name"`
	SteamAppID       int64    `json:"steam_appid"`
	IsFree           bool     `json:"is_free"`
	ShortDescription string   `json:"short_description"`
	HeaderImage      string   `json:"header_image"`
	Developers       []string `json:"developers"`
	Publishers       []string `json:"publishers"`
	PriceOverview    *struct {
		Currency string `json:"currency"`
		Final    int    `json:"final"`
	} `json:"price_overview"`
	Platforms struct {
		Windows bool `json:"windows"`
		Mac     bool `json:"mac"`
		Linux   bool `json:"linux"`
	} `json:"platforms"`
	Metacritic *struct {
		Score int `json:"score"`
	} `json:"metacritic"`
	Categories []struct {
		Description string `json:"description"`
	} `json:"categories"`
	Genres []struct {
		Description string `json:"description"`
	} `json:"genres"`
	ReleaseDate struct {
		ComingSoon bool   `json:"coming_soon"`
		Date       string `json:"date"`
	} `json:"release_date"`
}

// AppDetails fetches store data for one app. It returns ErrAppUnavailable
// when the store has no page for it.
func (c *Client) AppDetails(ctx context.Context, appID int64) (*AppDetails, error) {
	q := url.Values{}
	q.Set("appids", strconv.FormatInt(appID, 10))
	q.Set("cc", c.country)
	q.Set("l", c.language)
	u := c.storeBase + "/api/appdetails?" + q.Encode()

	// The answer is keyed by the requested appid: {"620":{"success":true,"data":{...}}}
	var r map[string]struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := c.getStore(ctx, u, &r); err != nil {
		return nil, err
	}
	entry, ok := r[strconv.FormatInt(appID, 10)]
	if !ok || !entry.Success || len(entry.Data) == 0 {
		return nil, ErrAppUnavailable
	}
	return parseAppDetails(appID, entry.Data)
}

func parseAppDetails(appID int64, raw json.RawMessage) (*AppDetails, error) {
	var d appDetailsData
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("steam: decode appdetails %d: %w", appID, err)
	}
	a := &AppDetails{
		AppID:            appID,
		Name:             d.Name,
		Type:             d.Type,
		ShortDescription: d.ShortDescription,
		HeaderImage:      d.HeaderImage,
		ReleaseDate:      parseReleaseDate(d.ReleaseDate.Date),
		ReleaseDateText:  d.ReleaseDate.Date,
		ComingSoon:       d.ReleaseDate.ComingSoon,
		IsFree:           d.IsFree,
		Developers:       nonNil(d.Developers),
		Publishers:       nonNil(d.Publishers),
		Windows:          d.Platforms.Windows,
		Mac:              d.Platforms.Mac,
		Linux:            d.Platforms.Linux,
		Raw:              raw,
	}
	a.Genres = make([]string, 0, len(d.Genres))
	for _, g := range d.Genres {
		a.Genres = append(a.Genres, g.Description)
	}
	a.Categories = make([]string, 0, len(d.Categories))
	for _, c := range d.Categories {
		a.Categories = append(a.Categories, c.Description)
	}
	if d.Metacritic != nil && d.Metacritic.Score > 0 {
		s := d.Metacritic.Score
		a.MetacriticScore = &s
	}
	if d.PriceOverview != nil {
		p := d.PriceOverview.Final
		a.PriceCents = &p
		a.PriceCurrency = d.PriceOverview.Currency
	}
	return a, nil
}

// Steam writes release dates in the store's locale; with l=english these are
// the forms seen in practice.
var releaseDateLayouts = []string{
	"2 Jan, 2006",
	"Jan 2, 2006",
	"2 Jan 2006",
	"January 2, 2006",
	"2 January, 2006",
	"Jan 2006",
	"January 2006",
	"2006",
}

func parseReleaseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range releaseDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
