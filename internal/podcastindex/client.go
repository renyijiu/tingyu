// Package podcastindex is a small client for the Podcast Index API.
package podcastindex

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
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

const defaultBase = "https://api.podcastindex.org/api/1.0"

// Client calls Podcast Index. Key and Secret come from the environment or
// the local config file; they are never built into the binary.
type Client struct {
	Key       string
	Secret    string
	BaseURL   string
	UserAgent string
	HTTP      *http.Client
	Now       func() time.Time
}

// AuthHeader is the Authorization value: lowercase hex SHA-1 of key+secret+unix.
func AuthHeader(key, secret string, unix int64) string {
	sum := sha1.Sum([]byte(key + secret + strconv.FormatInt(unix, 10)))
	return hex.EncodeToString(sum[:])
}

// Search returns feeds matching q. Chinese and other UTF-8 queries are sent as-is.
func (c *Client) Search(ctx context.Context, query string) ([]model.Podcast, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Set("max", "25")
	var body searchResponse
	if err := c.get(ctx, "/search/byterm", q, &body); err != nil {
		return nil, err
	}
	out := make([]model.Podcast, 0, len(body.Feeds))
	for _, f := range body.Feeds {
		out = append(out, f.podcast())
	}
	return out, nil
}

// Episodes returns the newest episodes for a Podcast Index feed id.
func (c *Client) Episodes(ctx context.Context, feedID int64) ([]model.Episode, error) {
	if feedID == 0 {
		return nil, fmt.Errorf("podcastindex: missing feed id")
	}
	q := url.Values{}
	q.Set("id", strconv.FormatInt(feedID, 10))
	q.Set("max", "200")
	var body episodeResponse
	if err := c.get(ctx, "/episodes/byfeedid", q, &body); err != nil {
		return nil, err
	}
	out := make([]model.Episode, 0, len(body.Items))
	for _, it := range body.Items {
		if ep, ok := it.episode(); ok {
			out = append(out, ep)
		}
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values, dest any) error {
	if strings.TrimSpace(c.Key) == "" || strings.TrimSpace(c.Secret) == "" {
		return fmt.Errorf("podcastindex: 缺少 API key 或 secret")
	}
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
	now := c.Now
	if now == nil {
		now = time.Now
	}
	unix := now().Unix()
	ua := c.UserAgent
	if ua == "" {
		ua = version.UserAgent()
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("X-Auth-Key", c.Key)
	req.Header.Set("X-Auth-Date", strconv.FormatInt(unix, 10))
	req.Header.Set("Authorization", AuthHeader(c.Key, c.Secret, unix))
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
		return fmt.Errorf("podcastindex: HTTP %d: %s", resp.StatusCode, trim(raw))
	}
	var probe statusEnvelope
	if err := json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("podcastindex: %w", err)
	}
	if !probe.Status.ok() {
		msg := probe.Description
		if msg == "" {
			msg = trim(raw)
		}
		return fmt.Errorf("podcastindex: %s", msg)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("podcastindex: %w", err)
	}
	return nil
}

func trim(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

type apiStatus string

func (s *apiStatus) UnmarshalJSON(b []byte) error {
	text := strings.Trim(strings.TrimSpace(string(b)), `"`)
	*s = apiStatus(text)
	return nil
}

func (s apiStatus) ok() bool {
	return strings.EqualFold(string(s), "true")
}

type statusEnvelope struct {
	Status      apiStatus `json:"status"`
	Description string    `json:"description"`
}

type searchResponse struct {
	Feeds []feedJSON `json:"feeds"`
}

type feedJSON struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Link        string `json:"link"`
	Description string `json:"description"`
	Author      string `json:"author"`
	OwnerName   string `json:"ownerName"`
	Image       string `json:"image"`
	Artwork     string `json:"artwork"`
	ITunesID    *int64 `json:"itunesId"`
}

func (f feedJSON) podcast() model.Podcast {
	image := f.Artwork
	if image == "" {
		image = f.Image
	}
	author := f.Author
	if author == "" {
		author = f.OwnerName
	}
	var itunes int64
	if f.ITunesID != nil {
		itunes = *f.ITunesID
	}
	p := model.Podcast{
		Title:       f.Title,
		Author:      author,
		Description: f.Description,
		Image:       image,
		FeedURL:     f.URL,
		Link:        f.Link,
		FeedID:      f.ID,
		ITunesID:    itunes,
		Source:      model.SourcePodcastIndex,
	}
	p.EnsureIDs()
	return p
}

type episodeResponse struct {
	Items []itemJSON `json:"items"`
}

type itemJSON struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	GUID          string `json:"guid"`
	DatePublished int64  `json:"datePublished"`
	EnclosureURL  string `json:"enclosureUrl"`
	EnclosureType string `json:"enclosureType"`
	Duration      *int64 `json:"duration"`
	Image         string `json:"image"`
	FeedID        int64  `json:"feedId"`
}

func (it itemJSON) episode() (model.Episode, bool) {
	if strings.TrimSpace(it.EnclosureURL) == "" {
		return model.Episode{}, false
	}
	var dur time.Duration
	if it.Duration != nil && *it.Duration > 0 {
		dur = time.Duration(*it.Duration) * time.Second
	}
	var published time.Time
	if it.DatePublished > 0 {
		published = time.Unix(it.DatePublished, 0).UTC()
	}
	ep := model.Episode{
		Title:       it.Title,
		Description: it.Description,
		AudioURL:    it.EnclosureURL,
		MIME:        it.EnclosureType,
		GUID:        it.GUID,
		Published:   published,
		Duration:    dur,
		Image:       it.Image,
	}
	return ep, true
}
