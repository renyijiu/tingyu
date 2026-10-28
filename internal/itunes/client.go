// Package itunes searches the iTunes Search API. No key is required, so it
// is the fallback when Podcast Index is not configured or returns nothing.
package itunes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/version"
)

const defaultBase = "https://itunes.apple.com"

// Client calls the iTunes Search API.
type Client struct {
	BaseURL   string
	UserAgent string
	HTTP      *http.Client
}

// Search looks up podcasts. The term is UTF-8, so Chinese queries work.
func (c *Client) Search(ctx context.Context, term string) ([]model.Podcast, error) {
	q := url.Values{}
	q.Set("term", term)
	q.Set("media", "podcast")
	q.Set("entity", "podcast")
	q.Set("limit", "25")
	var body searchResponse
	if err := c.get(ctx, "/search", q, &body); err != nil {
		return nil, err
	}
	out := make([]model.Podcast, 0, len(body.Results))
	for _, r := range body.Results {
		if p, ok := r.podcast(); ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// Episodes loads episodes for an iTunes collection id.
func (c *Client) Episodes(ctx context.Context, collectionID int64) ([]model.Episode, error) {
	if collectionID == 0 {
		return nil, fmt.Errorf("itunes: missing collection id")
	}
	q := url.Values{}
	q.Set("id", strconv.FormatInt(collectionID, 10))
	q.Set("entity", "podcastEpisode")
	q.Set("limit", "200")
	var body searchResponse
	if err := c.get(ctx, "/lookup", q, &body); err != nil {
		return nil, err
	}
	out := make([]model.Episode, 0, len(body.Results))
	for _, r := range body.Results {
		if ep, ok := r.episode(); ok {
			out = append(out, ep)
		}
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values, dest any) error {
	base := c.BaseURL
	if base == "" {
		base = defaultBase
	}
	u, err := url.Parse(strings.TrimRight(base, "/") + path)
	if err != nil {
		return err
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	ua := c.UserAgent
	if ua == "" {
		ua = version.UserAgent()
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("itunes: HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("itunes: %w", err)
	}
	return nil
}

type searchResponse struct {
	ResultCount int          `json:"resultCount"`
	Results     []resultJSON `json:"results"`
}

type resultJSON struct {
	WrapperType       string `json:"wrapperType"`
	Kind              string `json:"kind"`
	CollectionID      int64  `json:"collectionId"`
	CollectionName    string `json:"collectionName"`
	ArtistName        string `json:"artistName"`
	FeedURL           string `json:"feedUrl"`
	ArtworkURL600     string `json:"artworkUrl600"`
	ArtworkURL100     string `json:"artworkUrl100"`
	CollectionViewURL string `json:"collectionViewUrl"`
	TrackID           int64  `json:"trackId"`
	TrackName         string `json:"trackName"`
	Description       string `json:"description"`
	EpisodeURL        string `json:"episodeUrl"`
	ReleaseDate       string `json:"releaseDate"`
	TrackTimeMillis   int64  `json:"trackTimeMillis"`
	ArtworkURL160     string `json:"artworkUrl160"`
}

func (r resultJSON) podcast() (model.Podcast, bool) {
	if r.Kind != "" && r.Kind != "podcast" {
		return model.Podcast{}, false
	}
	if r.WrapperType != "" && r.WrapperType != "track" {
		return model.Podcast{}, false
	}
	if r.CollectionName == "" {
		return model.Podcast{}, false
	}
	image := r.ArtworkURL600
	if image == "" {
		image = r.ArtworkURL100
	}
	p := model.Podcast{
		Title:    r.CollectionName,
		Author:   r.ArtistName,
		Image:    image,
		FeedURL:  r.FeedURL,
		Link:     r.CollectionViewURL,
		ITunesID: r.CollectionID,
		Source:   model.SourceITunes,
	}
	p.EnsureIDs()
	return p, true
}

func (r resultJSON) episode() (model.Episode, bool) {
	if r.Kind != "podcast-episode" && r.WrapperType != "podcastEpisode" {
		return model.Episode{}, false
	}
	if strings.TrimSpace(r.EpisodeURL) == "" {
		return model.Episode{}, false
	}
	var published time.Time
	if r.ReleaseDate != "" {
		if t, err := time.Parse(time.RFC3339, r.ReleaseDate); err == nil {
			published = t
		}
	}
	var dur time.Duration
	if r.TrackTimeMillis > 0 {
		dur = time.Duration(r.TrackTimeMillis) * time.Millisecond
	}
	image := r.ArtworkURL160
	if image == "" {
		image = r.ArtworkURL600
	}
	ep := model.Episode{
		Title:       r.TrackName,
		Description: r.Description,
		AudioURL:    r.EpisodeURL,
		GUID:        strconv.FormatInt(r.TrackID, 10),
		Published:   published,
		Duration:    dur,
		Image:       image,
		MIME:        "audio/mpeg",
	}
	return ep, true
}
