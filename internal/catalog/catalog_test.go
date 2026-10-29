package catalog

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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
	// unchanged answers 304 to a fetch that names this ETag.
	unchanged string
	desc      string
	asked     *[]feed.Validators
}

func (f fakeFeed) Fetch(_ context.Context, _ string, since feed.Validators) (feed.Result, error) {
	if f.asked != nil {
		*f.asked = append(*f.asked, since)
	}
	if f.err != nil {
		return feed.Result{}, f.err
	}
	if f.unchanged != "" && since.ETag == f.unchanged {
		return feed.Result{NotModified: true, Validators: since}, nil
	}
	return feed.Result{
		Show:       feed.Show{Title: "rss", Description: f.desc, Author: "RSS 作者"},
		Episodes:   append([]model.Episode(nil), f.eps...),
		Validators: feed.Validators{ETag: f.unchanged},
	}, nil
}

func TestSearchFallsBackToITunes(t *testing.T) {
	pi := model.Podcast{Title: "来自索引", FeedURL: "https://example.com/pi.xml"}
	it := model.Podcast{Title: "来自 iTunes", FeedURL: "https://example.com/it.xml"}
	c := &Catalog{Index: fakeIndex{shows: []model.Podcast{pi}}, ITunes: fakeIT{shows: []model.Podcast{it}}}
	got, src, err := c.Search(context.Background(), "故事")
	if err != nil || src != model.SourceBoth || len(got) != 2 || got[0].Title != "来自索引" || got[1].Title != "来自 iTunes" {
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

	c = &Catalog{Index: fakeIndex{err: errors.New("index down")}, ITunes: fakeIT{err: errors.New("itunes down")}}
	if _, _, err = c.Search(context.Background(), "both down"); err == nil || err.Error() != "index down" {
		t.Fatalf("both down: %v", err)
	}
}

func TestSearchMergesShowsBothCatalogsKnow(t *testing.T) {
	pi := []model.Podcast{
		{Title: "故事会", FeedURL: "https://example.com/story.xml", FeedID: 7, Description: "索引的简介", Source: model.SourcePodcastIndex},
		{Title: "故事FM", FeedURL: "https://feed.example.com/gushi/", FeedID: 8, Source: model.SourcePodcastIndex},
		{Title: "故事", FeedURL: "https://example.com/other.xml", FeedID: 9, Source: model.SourcePodcastIndex},
	}
	it := []model.Podcast{
		{Title: "故事FM", FeedURL: "http://FEED.example.com/gushi", ITunesID: 42, Image: "https://img/600.jpg", Author: "爱哲", Source: model.SourceITunes},
		{Title: "故事会", FeedURL: "https://example.com/story.xml", ITunesID: 43, Source: model.SourceITunes},
	}
	c := &Catalog{Index: fakeIndex{shows: pi}, ITunes: fakeIT{shows: it}}
	got, src, err := c.Search(context.Background(), "故事 fm")
	if err != nil || src != model.SourceBoth {
		t.Fatalf("%v %s", err, src)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 shows once each, got %+v", got)
	}
	// The show titled as typed comes first, with what both know of it. The
	// copy seen first, iTunes' here as the two take turns, keeps its feed.
	first := got[0]
	if first.Title != "故事FM" || first.FeedID != 8 || first.ITunesID != 42 || first.Image != "https://img/600.jpg" || first.Author != "爱哲" || first.FeedURL != it[0].FeedURL {
		t.Fatalf("merged show %+v", first)
	}
	if got[1].Title != "故事会" || got[1].ITunesID != 43 || got[1].Description != "索引的简介" || got[2].Title != "故事" {
		t.Fatalf("order %+v", got)
	}
}

type slowIndex struct{}

func (slowIndex) Search(ctx context.Context, _ string) ([]model.Podcast, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (slowIndex) Episodes(context.Context, int64) ([]model.Episode, error) { return nil, nil }

func TestSearchDoesNotWaitLongForASlowCatalog(t *testing.T) {
	old := searchGrace
	searchGrace = 50 * time.Millisecond
	defer func() { searchGrace = old }()
	it := model.Podcast{Title: "来自 iTunes", FeedURL: "https://example.com/it.xml"}
	c := &Catalog{Index: slowIndex{}, ITunes: fakeIT{shows: []model.Podcast{it}}}
	start := time.Now()
	got, src, err := c.Search(context.Background(), "故事")
	if err != nil || src != model.SourceITunes || len(got) != 1 {
		t.Fatalf("%v %s %+v", err, src, got)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("waited %v for the slow catalog", d)
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
	_, got, err := c.Episodes(context.Background(), show, nil)
	if err != nil || len(got) != 1 || got[0].Title != "RSS" {
		t.Fatalf("%v %+v", err, got)
	}
	if got[0].ID != model.EpisodeKey(show.FeedURL, "g", rssEp.AudioURL) || got[0].PodcastID != show.ID {
		t.Fatalf("ids %+v", got[0])
	}

	c.Feeds = fakeFeed{err: errors.New("rss down")}
	_, got, err = c.Episodes(context.Background(), show, nil)
	if err != nil || got[0].Title != "PI" {
		t.Fatalf("pi fallback %v %+v", err, got)
	}

	c.Feeds = nil
	c.Index = fakeIndex{err: errors.New("pi down")}
	_, got, err = c.Episodes(context.Background(), show, nil)
	if err != nil || got[0].Title != "IT" {
		t.Fatalf("itunes fallback %v %+v", err, got)
	}
}

// iTunes gives no description and Podcast Index cuts its own short: the
// feed's is the show's own.
func TestEpisodesFillTheShowFromItsFeed(t *testing.T) {
	show := model.Podcast{ID: "feed:https://example.com/a.xml", Title: "节目", FeedURL: "https://example.com/a.xml", Description: "被截断的…", Image: "https://img/itunes.jpg"}
	c := &Catalog{Feeds: fakeFeed{eps: []model.Episode{{Title: "一", GUID: "1", AudioURL: "https://cdn/1.mp3"}}, desc: "完整的节目简介"}}
	got, _, err := c.Episodes(context.Background(), show, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "完整的节目简介" || got.Author != "RSS 作者" || got.Image != "https://img/itunes.jpg" || got.ID != show.ID || got.Title != "节目" {
		t.Fatalf("show %+v", got)
	}

	c.Feeds = fakeFeed{err: errors.New("rss down")}
	c.ITunes = fakeIT{eps: []model.Episode{{Title: "IT", AudioURL: "https://cdn/b.mp3"}}}
	show.ITunesID = 9
	got, _, err = c.Episodes(context.Background(), show, nil)
	if err != nil || got.Description != "被截断的…" {
		t.Fatalf("without a feed the show stays as it was: %v %+v", err, got)
	}
}

func TestEpisodesOpenFromTheCacheAndAskWhetherTheFeedChanged(t *testing.T) {
	show := model.Podcast{ID: "feed:https://example.com/a.xml", FeedURL: "https://example.com/a.xml", ITunesID: 9}
	var asked []feed.Validators
	rss := fakeFeed{eps: []model.Episode{{Title: "RSS", GUID: "g", AudioURL: "https://cdn/a.mp3"}}, desc: "简介", unchanged: `"v1"`, asked: &asked}
	cache := &DiskCache{Dir: t.TempDir()}
	c := &Catalog{Feeds: rss, Cache: cache, ITunes: fakeIT{eps: []model.Episode{{Title: "IT", AudioURL: "https://cdn/b.mp3"}}}}

	early := 0
	_, first, err := c.Episodes(context.Background(), show, func(model.Podcast, []model.Episode) { early++ })
	if err != nil || early != 0 || len(first) != 1 {
		t.Fatalf("first open: %v, early %d, %+v", err, early, first)
	}

	// Opened again, the kept copy shows at once; the feed answers 304.
	var shown []model.Episode
	var shownShow model.Podcast
	got, eps, err := c.Episodes(context.Background(), show, func(s model.Podcast, e []model.Episode) { shownShow, shown = s, e })
	if err != nil || len(shown) != 1 || shown[0].ID != first[0].ID || shownShow.Description != "简介" {
		t.Fatalf("early copy %v %+v %+v", err, shownShow, shown)
	}
	if len(eps) != 1 || eps[0].ID != first[0].ID || got.Description != "简介" {
		t.Fatalf("not modified %+v %+v", got, eps)
	}
	if asked[0] != (feed.Validators{}) || asked[1].ETag != `"v1"` {
		t.Fatalf("validators sent %+v", asked)
	}

	// When the feed cannot be read, the kept RSS copy stands rather than
	// iTunes, whose episode ids differ.
	c.Feeds = fakeFeed{err: errors.New("rss down")}
	_, eps, err = c.Episodes(context.Background(), show, nil)
	if err != nil || eps[0].Title != "RSS" || eps[0].ID != first[0].ID {
		t.Fatalf("rss down %v %+v", err, eps)
	}
}

func TestDiskCacheKeepsTheShowsUsedLast(t *testing.T) {
	d := &DiskCache{Dir: t.TempDir(), Max: 2}
	entry := func(title string) Entry {
		return Entry{Source: model.SourceRSS, Episodes: []model.Episode{{ID: title, Title: title}}}
	}
	if _, ok := d.Load("a"); ok {
		t.Fatal("empty cache loaded something")
	}
	for i, key := range []string{"a", "b"} {
		if err := d.Save(key, entry(key)); err != nil {
			t.Fatal(err)
		}
		// Each later than the last, whatever the file system's clock.
		old := time.Now().Add(time.Duration(i-10) * time.Minute)
		os.Chtimes(d.path(key), old, old)
	}
	if e, ok := d.Load("a"); !ok || e.Episodes[0].Title != "a" {
		t.Fatalf("load a: %v %+v", ok, e)
	}
	if err := d.Save("c", entry("c")); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Load("b"); ok {
		t.Fatal("b, used longest ago, should be gone")
	}
	for _, key := range []string{"a", "c"} {
		if _, ok := d.Load(key); !ok {
			t.Fatalf("%s should be kept", key)
		}
	}
	if err := os.WriteFile(d.path("bad"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Load("bad"); ok {
		t.Fatal("a broken file loaded")
	}
}
