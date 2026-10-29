package itunes

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/renyijiu/tingyu/internal/model"
)

// defaultCharts serves Apple's charts as JSON. Apple publishes it for its
// marketing tools without documenting it as an API, so callers should
// expect it to fail and do without.
const defaultCharts = "https://rss.marketingtools.apple.com"

// Top returns the podcasts on Apple's top chart for a store country, such
// as "cn", in chart order. The chart names the shows; one lookup in the
// same store fills in their feeds. Shows without a public feed are left
// out, as they could not be played.
func (c *Client) Top(ctx context.Context, country string, n int) ([]model.Podcast, error) {
	base := c.ChartsURL
	if base == "" {
		base = defaultCharts
	}
	var chart struct {
		Feed struct {
			Results []struct {
				ID string `json:"id"`
			} `json:"results"`
		} `json:"feed"`
	}
	u := fmt.Sprintf("%s/api/v2/%s/podcasts/top/%d/podcasts.json", strings.TrimRight(base, "/"), url.PathEscape(country), n)
	if err := c.fetch(ctx, u, &chart); err != nil {
		return nil, err
	}
	var ids []string
	for _, r := range chart.Feed.Results {
		if _, err := strconv.ParseInt(r.ID, 10, 64); err == nil {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	q := url.Values{}
	q.Set("id", strings.Join(ids, ","))
	q.Set("entity", "podcast")
	q.Set("country", country)
	var body searchResponse
	if err := c.get(ctx, "/lookup", q, &body); err != nil {
		return nil, err
	}
	found := map[int64]model.Podcast{}
	for _, r := range body.Results {
		if p, ok := r.podcast(); ok && p.FeedURL != "" {
			found[p.ITunesID] = p
		}
	}
	out := make([]model.Podcast, 0, len(ids))
	for _, id := range ids {
		n, _ := strconv.ParseInt(id, 10, 64)
		if p, ok := found[n]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}
