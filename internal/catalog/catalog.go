// Package catalog chooses where a search or an episode list comes from.
// Podcast Index is preferred. iTunes fills in when it is unavailable or empty.
// Episode lists prefer the show's own RSS feed.
package catalog

import (
	"context"
	"fmt"

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

// FeedSource downloads and parses a podcast RSS feed.
type FeedSource interface {
	Fetch(ctx context.Context, feedURL string) (feed.Show, []model.Episode, error)
}

// Catalog is the read path for search and episodes.
type Catalog struct {
	Index  Index
	ITunes Directory
	Feeds  FeedSource
}

// Search tries Podcast Index, then iTunes. source says which one answered.
func (c *Catalog) Search(ctx context.Context, query string) (shows []model.Podcast, source string, err error) {
	var indexErr error
	if c.Index != nil {
		shows, indexErr = c.Index.Search(ctx, query)
		if indexErr == nil && len(shows) > 0 {
			return shows, model.SourcePodcastIndex, nil
		}
	}
	if c.ITunes != nil {
		itunes, itErr := c.ITunes.Search(ctx, query)
		if itErr == nil && len(itunes) > 0 {
			return itunes, model.SourceITunes, nil
		}
		if indexErr != nil {
			return nil, "", indexErr
		}
		if itErr != nil {
			return nil, "", itErr
		}
		if c.Index != nil {
			return shows, model.SourcePodcastIndex, nil
		}
		return itunes, model.SourceITunes, nil
	}
	if indexErr != nil {
		return nil, "", indexErr
	}
	return shows, model.SourcePodcastIndex, nil
}

// Episodes loads a show's episodes. RSS is preferred so a subscription still
// works without an API key. Podcast Index and iTunes are the fallbacks.
func (c *Catalog) Episodes(ctx context.Context, show model.Podcast) ([]model.Episode, error) {
	var fallback error
	if c.Feeds != nil && show.FeedURL != "" {
		_, eps, err := c.Feeds.Fetch(ctx, show.FeedURL)
		if err == nil && len(eps) > 0 {
			return tag(show, eps), nil
		}
		fallback = err
	}
	if c.Index != nil && show.FeedID != 0 {
		eps, err := c.Index.Episodes(ctx, show.FeedID)
		if err == nil && len(eps) > 0 {
			return tag(show, eps), nil
		}
		if fallback == nil {
			fallback = err
		}
	}
	if c.ITunes != nil && show.ITunesID != 0 {
		eps, err := c.ITunes.Episodes(ctx, show.ITunesID)
		if err == nil {
			return tag(show, eps), nil
		}
		if fallback == nil {
			fallback = err
		}
	}
	if fallback != nil {
		return nil, fallback
	}
	return nil, fmt.Errorf("没有可用的节目源")
}

func tag(show model.Podcast, eps []model.Episode) []model.Episode {
	for i := range eps {
		eps[i].PodcastID = show.ID
		eps[i].EnsureID(show.FeedURL)
	}
	return eps
}
