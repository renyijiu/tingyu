// Package feed parses podcast RSS and Atom feeds with gofeed.
package feed

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/version"
)

// Show is the channel-level metadata of a feed.
type Show struct {
	Title       string
	Author      string
	Description string
	Image       string
	Link        string
}

// Parse reads a feed and returns the show plus its playable episodes,
// newest first. feedURL is stored in each episode key.
func Parse(r io.Reader, feedURL string) (Show, []model.Episode, error) {
	parsed, err := gofeed.NewParser().Parse(r)
	if err != nil {
		return Show{}, nil, fmt.Errorf("feed: %w", err)
	}
	show := Show{
		Title:       strings.TrimSpace(parsed.Title),
		Description: plain(parsed.Description),
		Link:        parsed.Link,
	}
	if parsed.Author != nil {
		show.Author = parsed.Author.Name
	}
	if parsed.ITunesExt != nil && show.Author == "" {
		show.Author = parsed.ITunesExt.Author
	}
	if parsed.Image != nil {
		show.Image = parsed.Image.URL
	}
	if parsed.ITunesExt != nil && parsed.ITunesExt.Image != "" {
		show.Image = parsed.ITunesExt.Image
	}
	eps := make([]model.Episode, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		if item == nil {
			continue
		}
		enc := pickEnclosure(item)
		if enc == nil || strings.TrimSpace(enc.URL) == "" {
			continue
		}
		ep := model.Episode{
			Title:       strings.TrimSpace(item.Title),
			Description: plain(firstText(item.Description, item.Content)),
			AudioURL:    enc.URL,
			MIME:        enc.Type,
			Published:   published(item),
			Duration:    itemDuration(item),
			Image:       itemImage(item, show.Image),
		}
		if item.GUID != "" {
			ep.GUID = item.GUID
		}
		ep.EnsureID(feedURL)
		eps = append(eps, ep)
	}
	sort.SliceStable(eps, func(i, j int) bool {
		if eps[i].Published.IsZero() {
			return false
		}
		if eps[j].Published.IsZero() {
			return true
		}
		return eps[i].Published.After(eps[j].Published)
	})
	return show, eps, nil
}

// MaxSize is the largest feed Fetch reads unless told otherwise. Some shows
// keep every episode in their feed and run to tens of megabytes.
const MaxSize = 64 << 20

// Validators identify the copy of a feed a client holds: the ETag and
// Last-Modified its server sent with it.
type Validators struct {
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
}

// Result is a fetched feed.
type Result struct {
	Show     Show
	Episodes []model.Episode
	// Validators identify this copy, for asking next time whether it
	// changed.
	Validators Validators
	// NotModified says the feed has not changed since the copy the fetch
	// named. Show and Episodes are then empty.
	NotModified bool
}

// Fetcher downloads a feed and parses it.
type Fetcher struct {
	HTTP      *http.Client
	UserAgent string
	// Limit is the largest feed read, MaxSize when zero.
	Limit int64
}

// Fetch downloads feedURL. With since set, a server that supports it
// answers that the feed has not changed instead of sending it again.
func (f *Fetcher) Fetch(ctx context.Context, feedURL string, since Validators) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return Result{}, err
	}
	ua := f.UserAgent
	if ua == "" {
		ua = version.UserAgent()
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*")
	if since.ETag != "" {
		req.Header.Set("If-None-Match", since.ETag)
	}
	if since.LastModified != "" {
		req.Header.Set("If-Modified-Since", since.LastModified)
	}
	hc := f.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified && since != (Validators{}) {
		return Result{NotModified: true, Validators: since}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("feed: HTTP %d", resp.StatusCode)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = MaxSize
	}
	body := &counter{r: io.LimitReader(resp.Body, limit+1)}
	show, eps, err := Parse(body, feedURL)
	if body.n > limit {
		return Result{}, fmt.Errorf("feed: larger than %d bytes", limit)
	}
	if err != nil {
		return Result{}, err
	}
	return Result{
		Show:       show,
		Episodes:   eps,
		Validators: Validators{ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")},
	}, nil
}

// counter counts the bytes read through it.
type counter struct {
	r io.Reader
	n int64
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func pickEnclosure(item *gofeed.Item) *gofeed.Enclosure {
	var fallback *gofeed.Enclosure
	for _, enc := range item.Enclosures {
		if enc == nil || enc.URL == "" {
			continue
		}
		if fallback == nil {
			fallback = enc
		}
		mt := strings.ToLower(enc.Type)
		u := strings.ToLower(enc.URL)
		if strings.HasPrefix(mt, "audio/") || strings.Contains(u, ".mp3") || strings.Contains(u, ".m4a") || strings.Contains(u, ".ogg") || strings.Contains(u, ".aac") {
			return enc
		}
	}
	return fallback
}

func itemDuration(item *gofeed.Item) time.Duration {
	if item.ITunesExt != nil {
		if d, ok := ParseDuration(item.ITunesExt.Duration); ok {
			return d
		}
	}
	return 0
}

func itemImage(item *gofeed.Item, fallback string) string {
	if item.ITunesExt != nil && item.ITunesExt.Image != "" {
		return item.ITunesExt.Image
	}
	if item.Image != nil && item.Image.URL != "" {
		return item.Image.URL
	}
	return fallback
}

func published(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return item.PublishedParsed.UTC()
	}
	if item.UpdatedParsed != nil {
		return item.UpdatedParsed.UTC()
	}
	return time.Time{}
}

func firstText(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

var tagRE = regexp.MustCompile(`<[^>]+>`)

func plain(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

// ParseDuration accepts itunes:duration as seconds, mm:ss or hh:mm:ss.
func ParseDuration(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if !strings.Contains(s, ":") {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return 0, false
		}
		return time.Duration(n) * time.Second, true
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, false
		}
		nums[i] = n
	}
	var h, m, sec int
	if len(nums) == 3 {
		h, m, sec = nums[0], nums[1], nums[2]
	} else {
		m, sec = nums[0], nums[1]
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, true
}
