package steam

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
)

// MaxReviewBatch is the most apps AppReviews asks for in one request.
const MaxReviewBatch = 50

// ReviewSummary is the user rating shown on a store page.
type ReviewSummary struct {
	Score      int    // 1..9; 0 when the app has no reviews
	ScoreDesc  string // e.g. "Very Positive"
	Positive   int    // derived from TotalCount and Steam's whole percentage
	TotalCount int
}

// AppReviews fetches review summaries for up to MaxReviewBatch apps in one
// request to IStoreBrowseService/GetItems. Apps Steam doesn't know are left
// out of the result; apps without reviews get a zero TotalCount.
//
// It replaces store.steampowered.com/appreviews, which since November 2026
// redirects every request to the store's home page.
func (c *Client) AppReviews(ctx context.Context, appIDs []int64) (map[int64]ReviewSummary, error) {
	if len(appIDs) > MaxReviewBatch {
		return nil, fmt.Errorf("steam: at most %d apps per review request, got %d", MaxReviewBatch, len(appIDs))
	}
	type id struct {
		AppID int64 `json:"appid"`
	}
	in := struct {
		IDs     []id `json:"ids"`
		Context struct {
			Language    string `json:"language"`
			CountryCode string `json:"country_code"`
		} `json:"context"` // without it Steam answers {"response":{}}
		DataRequest struct {
			IncludeReviews bool `json:"include_reviews"`
		} `json:"data_request"`
	}{}
	for _, a := range appIDs {
		in.IDs = append(in.IDs, id{a})
	}
	in.Context.Language = c.language
	in.Context.CountryCode = c.country
	in.DataRequest.IncludeReviews = true
	body, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("input_json", string(body))
	u := c.apiBase + "/IStoreBrowseService/GetItems/v1/?" + q.Encode()

	var r struct {
		Response struct {
			StoreItems []struct {
				AppID   int64 `json:"appid"`
				Success int   `json:"success"`
				Reviews *struct {
					// All languages, without off-topic review bombs: the
					// numbers on the store page.
					SummaryFiltered struct {
						ReviewCount      int    `json:"review_count"`
						PercentPositive  int    `json:"percent_positive"`
						ReviewScore      int    `json:"review_score"`
						ReviewScoreLabel string `json:"review_score_label"`
					} `json:"summary_filtered"`
				} `json:"reviews"`
			} `json:"store_items"`
		} `json:"response"`
	}
	if err := c.getStore(ctx, u, &r); err != nil {
		return nil, err
	}
	out := make(map[int64]ReviewSummary, len(appIDs))
	for _, it := range r.Response.StoreItems {
		if it.Success != 1 || it.AppID == 0 {
			continue
		}
		var s ReviewSummary
		if it.Reviews != nil {
			f := it.Reviews.SummaryFiltered
			s = ReviewSummary{
				Score:      f.ReviewScore,
				ScoreDesc:  f.ReviewScoreLabel,
				Positive:   int(math.Round(float64(f.ReviewCount) * float64(f.PercentPositive) / 100)),
				TotalCount: f.ReviewCount,
			}
		}
		out[it.AppID] = s
	}
	return out, nil
}
