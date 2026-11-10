package steam

import (
	"context"
	"net/url"
	"strconv"
)

// ReviewSummary is the user rating shown on a store page.
type ReviewSummary struct {
	Score      int    // 1..9; 0 when the app has no reviews
	ScoreDesc  string // e.g. "Very Positive"
	Positive   int
	Negative   int
	TotalCount int
}

// AppReviews fetches the review summary for one app, counting reviews in all
// languages and from all purchase types.
func (c *Client) AppReviews(ctx context.Context, appID int64) (*ReviewSummary, error) {
	q := url.Values{}
	q.Set("json", "1")
	q.Set("num_per_page", "0") // summary only, no review texts
	q.Set("language", "all")
	q.Set("purchase_type", "all")
	u := c.storeBase + "/appreviews/" + strconv.FormatInt(appID, 10) + "?" + q.Encode()

	var r struct {
		Success      int `json:"success"`
		QuerySummary struct {
			ReviewScore     int    `json:"review_score"`
			ReviewScoreDesc string `json:"review_score_desc"`
			TotalPositive   int    `json:"total_positive"`
			TotalNegative   int    `json:"total_negative"`
			TotalReviews    int    `json:"total_reviews"`
		} `json:"query_summary"`
	}
	if err := c.getStore(ctx, u, &r); err != nil {
		return nil, err
	}
	if r.Success != 1 {
		return nil, ErrAppUnavailable
	}
	s := r.QuerySummary
	return &ReviewSummary{
		Score:      s.ReviewScore,
		ScoreDesc:  s.ReviewScoreDesc,
		Positive:   s.TotalPositive,
		Negative:   s.TotalNegative,
		TotalCount: s.TotalReviews,
	}, nil
}
