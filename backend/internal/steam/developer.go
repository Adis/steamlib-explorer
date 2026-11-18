package steam

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// MaxDeveloperGames is the most games DeveloperGames asks the store for.
const MaxDeveloperGames = 50

// StoreGame is one row of a store search: what the store's search page shows.
type StoreGame struct {
	AppID        int64
	Name         string
	CapsuleImage string // 231x87 search capsule
	ReleasedText string // as the store shows it, e.g. "21 Aug, 2012"
	ReviewDesc   string // e.g. "Very Positive"; empty without reviews
	ReviewPct    *int   // positive share, 0..100
	ReviewsTotal *int
}

// DeveloperGames lists the games (not DLC or soundtracks) the store files
// under a developer, in the store's own order (most relevant first). There
// is no API for this, so it reads the store search's result rows, the same
// request the search page makes while scrolling. An unknown developer gives
// an empty list.
func (c *Client) DeveloperGames(ctx context.Context, developer string) ([]StoreGame, error) {
	q := url.Values{}
	q.Set("developer", developer)
	q.Set("category1", "998") // games only
	q.Set("infinite", "1")    // JSON with the rows as HTML
	q.Set("start", "0")
	q.Set("count", strconv.Itoa(MaxDeveloperGames))
	q.Set("cc", c.country)
	q.Set("l", c.language)
	var r struct {
		ResultsHTML string `json:"results_html"`
	}
	if err := c.getStore(ctx, c.storeBase+"/search/results/?"+q.Encode(), &r); err != nil {
		return nil, err
	}
	return parseSearchRows(r.ResultsHTML), nil
}

var (
	rowRe      = regexp.MustCompile(`(?s)<a [^>]*data-ds-appid="(\d+)"[^>]*class="search_result_row.*?</a>`)
	titleRe    = regexp.MustCompile(`(?s)<span class="title">(.*?)</span>`)
	capsuleRe  = regexp.MustCompile(`(?s)<div class="search_capsule"><img src="([^"]+)"`)
	releasedRe = regexp.MustCompile(`(?s)<div class="search_released[^"]*">(.*?)</div>`)
	reviewRe   = regexp.MustCompile(`data-tooltip-html="([^"]*)"`)
	// "Very Positive<br>85% of the 2,651,619 user reviews ..."
	reviewTextRe = regexp.MustCompile(`^(.*?)<br>(\d+)% of the ([\d,.]+) user reviews`)
)

// parseSearchRows reads the games out of a store search's results_html.
// Rows whose appid the store repeats are kept once; rows that are bundles
// or packages (no data-ds-appid) are skipped.
func parseSearchRows(page string) []StoreGame {
	seen := map[int64]bool{}
	out := []StoreGame{}
	for _, m := range rowRe.FindAllStringSubmatch(page, -1) {
		id, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil || seen[id] {
			continue
		}
		row := m[0]
		g := StoreGame{AppID: id}
		if t := titleRe.FindStringSubmatch(row); t != nil {
			g.Name = html.UnescapeString(strings.TrimSpace(t[1]))
		}
		if g.Name == "" {
			continue
		}
		seen[id] = true
		if c := capsuleRe.FindStringSubmatch(row); c != nil {
			g.CapsuleImage = html.UnescapeString(c[1])
		}
		if r := releasedRe.FindStringSubmatch(row); r != nil {
			g.ReleasedText = html.UnescapeString(strings.TrimSpace(r[1]))
		}
		if r := reviewRe.FindStringSubmatch(row); r != nil {
			text := html.UnescapeString(r[1]) // the tooltip's own <br> tags appear after one unescape
			if v := reviewTextRe.FindStringSubmatch(text); v != nil {
				g.ReviewDesc = strings.TrimSpace(v[1])
				if pct, err := strconv.Atoi(v[2]); err == nil {
					g.ReviewPct = &pct
				}
				if n, err := strconv.Atoi(strings.NewReplacer(",", "", ".", "").Replace(v[3])); err == nil {
					g.ReviewsTotal = &n
				}
			}
		}
		out = append(out, g)
	}
	return out
}
