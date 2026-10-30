// Package catalog chooses where a search or an episode list comes from.
// Podcast Index is preferred. iTunes fills in when it is unavailable or empty.
// Episode lists prefer the show's own RSS feed, and the last one loaded of
// each show is kept to open it at once.
package catalog

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/renyijiu/tingyu/internal/feed"
	"github.com/renyijiu/tingyu/internal/model"
)

// Index looks up Podcast Index.
type Index interface {
	Search(ctx context.Context, query string) ([]model.Podcast, error)
	Episodes(ctx context.Context, feedID int64) ([]model.Episode, error)
}

// Directory looks up iTunes.
type Directory interface {
	Search(ctx context.Context, term string) ([]model.Podcast, error)
	Episodes(ctx context.Context, collectionID int64) ([]model.Episode, error)
}

// Charts lists the shows most listened to in a store.
type Charts interface {
	Top(ctx context.Context, country string, n int) ([]model.Podcast, error)
}

// FeedSource downloads and parses a podcast RSS feed. Given the validators
// of a copy already held, it may answer that the feed has not changed.
type FeedSource interface {
	Fetch(ctx context.Context, feedURL string, since feed.Validators) (feed.Result, error)
}

// Cache keeps the last episode list of each show.
type Cache interface {
	Load(key string) (Entry, bool)
	Save(key string, e Entry) error
}

// Entry is a show's episode list as last loaded.
type Entry struct {
	// Source is where the episodes came from. Only an RSS copy has
	// validators to ask its feed whether it changed.
	Source     string          `json:"source"`
	FeedURL    string          `json:"feedUrl,omitempty"`
	Show       feed.Show       `json:"show"`
	Episodes   []model.Episode `json:"episodes"`
	Validators feed.Validators `json:"validators"`
}

// Catalog is the read path for search and episodes.
type Catalog struct {
	Index  Index
	ITunes Directory
	Feeds  FeedSource
	Cache  Cache
	Charts Charts
}

// Popular is what most listen to now: Apple's chart for China, the store
// whose shows a listener of a Chinese app most likely follows.
func (c *Catalog) Popular(ctx context.Context) ([]model.Podcast, error) {
	if c.Charts == nil {
		return nil, nil
	}
	return c.Charts.Top(ctx, "cn", 24)
}

// searchGrace is how long a search waits for the slower catalog once the
// other has found something.
var searchGrace = 2 * time.Second

// Search asks Podcast Index and iTunes at once and merges what they find.
// A show both know comes once, with what each knows of it, and shows
// titled as the query come first; the rest take turns, Podcast Index
// first. source names who answered.
func (c *Catalog) Search(ctx context.Context, query string) (shows []model.Podcast, source string, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type answer struct {
		shows []model.Podcast
		err   error
	}
	ask := func(search func(context.Context, string) ([]model.Podcast, error)) chan answer {
		ch := make(chan answer, 1)
		go func() {
			shows, err := search(ctx, query)
			ch <- answer{shows, err}
		}()
		return ch
	}
	var piCh, itCh chan answer
	pending := 0
	if c.Index != nil {
		piCh = ask(c.Index.Search)
		pending++
	}
	if c.ITunes != nil {
		itCh = ask(c.ITunes.Search)
		pending++
	}
	var pi, it answer
	var grace <-chan time.Time
	for pending > 0 {
		select {
		case pi = <-piCh:
			piCh = nil
		case it = <-itCh:
			itCh = nil
		case <-grace:
			pending = 1
		case <-ctx.Done():
			pending = 1
		}
		pending--
		if grace == nil && (len(pi.shows) > 0 || len(it.shows) > 0) {
			grace = time.After(searchGrace)
		}
	}
	switch {
	case len(pi.shows) > 0 && len(it.shows) > 0:
		return merge(query, pi.shows, it.shows), model.SourceBoth, nil
	case len(pi.shows) > 0:
		return merge(query, pi.shows, nil), model.SourcePodcastIndex, nil
	case len(it.shows) > 0:
		return merge(query, it.shows, nil), model.SourceITunes, nil
	case pi.err != nil:
		return nil, "", pi.err
	case it.err != nil:
		return nil, "", it.err
	case ctx.Err() != nil:
		return nil, "", ctx.Err()
	case c.Index != nil:
		return nil, model.SourcePodcastIndex, nil
	}
	return nil, model.SourceITunes, nil
}

// merge takes turns between a and b, a first, keeping one of each show:
// shows are the same when their feeds are, whatever the scheme or case.
// Shows titled as the query move to the front.
func merge(query string, a, b []model.Podcast) []model.Podcast {
	var out []model.Podcast
	at := map[string]int{}
	add := func(p model.Podcast) {
		key := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(p.FeedURL)), "https://"), "http://"), "/")
		if key == "" {
			key = "id:" + p.ID
		}
		if i, ok := at[key]; ok {
			out[i] = combine(out[i], p)
			return
		}
		at[key] = len(out)
		out = append(out, p)
	}
	for i := 0; i < max(len(a), len(b)); i++ {
		if i < len(a) {
			add(a[i])
		}
		if i < len(b) {
			add(b[i])
		}
	}
	want := plainTitle(query)
	sort.SliceStable(out, func(i, j int) bool {
		return plainTitle(out[i].Title) == want && plainTitle(out[j].Title) != want
	})
	return out
}

// combine fills what one catalog left out of a show with what the other
// knows.
func combine(p, q model.Podcast) model.Podcast {
	if p.FeedID == 0 {
		p.FeedID = q.FeedID
	}
	if p.ITunesID == 0 {
		p.ITunesID = q.ITunesID
	}
	for _, f := range []struct {
		to   *string
		from string
	}{
		{&p.Author, q.Author}, {&p.Description, q.Description}, {&p.Image, q.Image}, {&p.Link, q.Link},
	} {
		if *f.to == "" {
			*f.to = f.from
		}
	}
	return p
}

// plainTitle is a title as typed in a search: in lower case, without
// spaces.
func plainTitle(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), "")
}

// Episodes loads a show's episodes, newest first, and returns the show with
// what its feed tells of it: its own description, which iTunes leaves out
// and Podcast Index cuts short, and any author, cover or link it lacked.
//
// RSS is preferred so a subscription still works without an API key. A
// show loaded before is first passed to early, as it was, while its feed
// is asked whether it changed; when the feed cannot be read, that copy
// stands, so episode ids stay those of the feed. Podcast Index and iTunes
// are the fallbacks.
func (c *Catalog) Episodes(ctx context.Context, show model.Podcast, early func(model.Podcast, []model.Episode)) (model.Podcast, []model.Episode, error) {
	var kept Entry
	have := false
	if c.Cache != nil {
		kept, have = c.Cache.Load(show.ID)
		if have && early != nil {
			early(withFeed(show, kept.Show), tag(show, kept.Episodes))
		}
	}
	fromFeed := have && kept.Source == model.SourceRSS && kept.FeedURL == show.FeedURL
	var fallback error
	if c.Feeds != nil && show.FeedURL != "" {
		var since feed.Validators
		if fromFeed {
			since = kept.Validators
		}
		res, err := c.Feeds.Fetch(ctx, show.FeedURL, since)
		switch {
		case err == nil && res.NotModified && fromFeed:
			return withFeed(show, kept.Show), tag(show, kept.Episodes), nil
		case err == nil && len(res.Episodes) > 0:
			eps := tag(show, res.Episodes)
			c.keep(show, Entry{Source: model.SourceRSS, FeedURL: show.FeedURL, Show: res.Show, Episodes: eps, Validators: res.Validators})
			return withFeed(show, res.Show), eps, nil
		case fromFeed:
			return withFeed(show, kept.Show), tag(show, kept.Episodes), nil
		}
		fallback = err
	}
	if c.Index != nil && show.FeedID != 0 {
		eps, err := c.Index.Episodes(ctx, show.FeedID)
		if err == nil && len(eps) > 0 {
			eps = tag(show, eps)
			c.keep(show, Entry{Source: model.SourcePodcastIndex, Episodes: eps})
			return show, eps, nil
		}
		if fallback == nil {
			fallback = err
		}
	}
	if c.ITunes != nil && show.ITunesID != 0 {
		eps, err := c.ITunes.Episodes(ctx, show.ITunesID)
		if err == nil {
			eps = tag(show, eps)
			c.keep(show, Entry{Source: model.SourceITunes, Episodes: eps})
			return show, eps, nil
		}
		if fallback == nil {
			fallback = err
		}
	}
	if have {
		return withFeed(show, kept.Show), tag(show, kept.Episodes), nil
	}
	if fallback != nil {
		return show, nil, fallback
	}
	return show, nil, fmt.Errorf("没有可用的节目源")
}

// keep caches what was loaded. A cache that cannot be written only means
// the next open waits for the network.
func (c *Catalog) keep(show model.Podcast, e Entry) {
	if c.Cache != nil && len(e.Episodes) > 0 {
		_ = c.Cache.Save(show.ID, e)
	}
}

// withFeed fills in show from its feed. The feed's description is the
// show's own words, so it wins over a directory's.
func withFeed(show model.Podcast, f feed.Show) model.Podcast {
	if f.Description != "" {
		show.Description = f.Description
	}
	if show.Author == "" {
		show.Author = f.Author
	}
	if show.Image == "" {
		show.Image = f.Image
	}
	if show.Link == "" {
		show.Link = f.Link
	}
	return show
}

func tag(show model.Podcast, eps []model.Episode) []model.Episode {
	for i := range eps {
		eps[i].PodcastID = show.ID
		eps[i].EnsureID(show.FeedURL)
	}
	return eps
}
