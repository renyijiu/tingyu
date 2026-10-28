package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/renyijiu/tingyu/internal/feed"
	"github.com/renyijiu/tingyu/internal/model"
)

type fakeIndex struct {
	shows []model.Podcast
	eps   []model.Episode
	err   error
}

func (f fakeIndex) Search(context.Context, string) ([]model.Podcast, error) {
	return f.shows, f.err
}
func (f fakeIndex) Episodes(context.Context, int64) ([]model.Episode, error) {
	return f.eps, f.err
}

type fakeIT struct {
	shows []model.Podcast
	eps   []model.Episode
	err   error
}

func (f fakeIT) Search(context.Context, string) ([]model.Podcast, error) {
	return f.shows, f.err
}
func (f fakeIT) Episodes(context.Context, int64) ([]model.Episode, error) {
	return f.eps, f.err
}

type fakeFeed struct {
	eps []model.Episode
	err error
}

func (f fakeFeed) Fetch(context.Context, string) (feed.Show, []model.Episode, error) {
	return feed.Show{Title: "rss"}, f.eps, f.err
}

func TestSearchFallsBackToITunes(t *testing.T) {
	pi := model.Podcast{Title: "来自索引", FeedURL: "https://example.com/pi.xml"}
	it := model.Podcast{Title: "来自 iTunes", FeedURL: "https://example.com/it.xml"}
	c := &Catalog{Index: fakeIndex{shows: []model.Podcast{pi}}, ITunes: fakeIT{shows: []model.Podcast{it}}}
	got, src, err := c.Search(context.Background(), "故事")
	if err != nil || src != model.SourcePodcastIndex || got[0].Title != "来自索引" {
		t.Fatalf("%v %s %+v", err, src, got)
	}

	c.Index = fakeIndex{err: errors.New("boom")}
	got, src, err = c.Search(context.Background(), "serial")
	if err != nil || src != model.SourceITunes || got[0].Title != "来自 iTunes" {
		t.Fatalf("fallback %v %s %+v", err, src, got)
	}

	c.Index = fakeIndex{}
	got, src, err = c.Search(context.Background(), "empty")
	if err != nil || src != model.SourceITunes {
		t.Fatalf("empty index should supplement with iTunes: %v %s %+v", err, src, got)
	}

	c = &Catalog{ITunes: fakeIT{shows: []model.Podcast{it}}}
	got, src, err = c.Search(context.Background(), "only")
	if err != nil || src != model.SourceITunes || len(got) != 1 {
		t.Fatalf("itunes only %v %s %+v", err, src, got)
	}
}

func TestEpisodesPreferRSS(t *testing.T) {
	show := model.Podcast{ID: "feed:https://example.com/a.xml", FeedURL: "https://example.com/a.xml", FeedID: 5, ITunesID: 9}
	rssEp := model.Episode{Title: "RSS", GUID: "g", AudioURL: "https://cdn/a.mp3"}
	piEp := model.Episode{Title: "PI", GUID: "g", AudioURL: "https://cdn/a.mp3"}
	c := &Catalog{
		Index:  fakeIndex{eps: []model.Episode{piEp}},
		ITunes: fakeIT{eps: []model.Episode{{Title: "IT", AudioURL: "https://cdn/b.mp3"}}},
		Feeds:  fakeFeed{eps: []model.Episode{rssEp}},
	}
	got, err := c.Episodes(context.Background(), show)
	if err != nil || len(got) != 1 || got[0].Title != "RSS" {
		t.Fatalf("%v %+v", err, got)
	}
	if got[0].ID != model.EpisodeKey(show.FeedURL, "g", rssEp.AudioURL) || got[0].PodcastID != show.ID {
		t.Fatalf("ids %+v", got[0])
	}

	c.Feeds = fakeFeed{err: errors.New("rss down")}
	got, err = c.Episodes(context.Background(), show)
	if err != nil || got[0].Title != "PI" {
		t.Fatalf("pi fallback %v %+v", err, got)
	}

	c.Feeds = nil
	c.Index = fakeIndex{err: errors.New("pi down")}
	got, err = c.Episodes(context.Background(), show)
	if err != nil || got[0].Title != "IT" {
		t.Fatalf("itunes fallback %v %+v", err, got)
	}
}
