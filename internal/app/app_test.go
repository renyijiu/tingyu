package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/feed"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
	"github.com/renyijiu/tingyu/internal/store"
)

type fakeIndex struct {
	shows []model.Podcast
	eps   []model.Episode
}

func (f fakeIndex) Search(context.Context, string) ([]model.Podcast, error) {
	return f.shows, nil
}
func (f fakeIndex) Episodes(context.Context, int64) ([]model.Episode, error) {
	return f.eps, nil
}

type fakeIT struct{}

func (fakeIT) Search(context.Context, string) ([]model.Podcast, error) {
	return nil, errors.New("itunes unused")
}
func (fakeIT) Episodes(context.Context, int64) ([]model.Episode, error) {
	return nil, errors.New("itunes unused")
}

type fakeFeed struct{ eps []model.Episode }

func (f fakeFeed) Fetch(context.Context, string) (feed.Show, []model.Episode, error) {
	return feed.Show{}, f.eps, nil
}

type fakeEngine struct{ h *fakeHandle }

type fakeHandle struct {
	pos     time.Duration
	playing bool
}

func (h *fakeHandle) Play()                   { h.playing = true }
func (h *fakeHandle) Pause()                  { h.playing = false }
func (h *fakeHandle) Seek(d time.Duration)    { h.pos = d }
func (h *fakeHandle) Position() time.Duration { return h.pos }
func (h *fakeHandle) Duration() time.Duration { return 30 * time.Minute }
func (h *fakeHandle) Playing() bool           { return h.playing }
func (h *fakeHandle) Close() error            { h.playing = false; return nil }

func (e *fakeEngine) Open(io.ReadSeeker, player.Meta) (player.Handle, error) {
	e.h = &fakeHandle{}
	return e.h, nil
}

func TestSubscribeSearchPlay(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	show := model.Podcast{Title: "故事会", Author: "甲", FeedURL: "https://example.com/gushi.xml", FeedID: 7}
	show.EnsureIDs()
	ep := model.Episode{Title: "第一集", GUID: "ep-1", AudioURL: "https://cdn.example.com/1.mp3", Duration: 30 * time.Minute}
	ep.EnsureID(show.FeedURL)
	cat := &catalog.Catalog{
		Index:  fakeIndex{shows: []model.Podcast{show}, eps: []model.Episode{ep}},
		ITunes: fakeIT{},
		Feeds:  fakeFeed{eps: []model.Episode{ep}},
	}
	eng := &fakeEngine{}
	pl := &player.Controller{
		Engine: eng,
		Open: func(context.Context, string) (io.ReadSeeker, error) {
			return bytes.NewReader([]byte("abc")), nil
		},
		Save: func(key string, pos, dur time.Duration) {
			_ = st.SaveProgress(key, model.Progress{Position: pos, Duration: dur, Updated: time.Now()})
		},
	}
	a := New(st, cat, pl)
	a.Sync = true
	a.Notice = "未配置"
	a.Query = "故事"
	a.Search()
	if a.Searching || a.SearchErr != "" || len(a.Results) != 1 || a.Results[0].Title != "故事会" {
		t.Fatalf("%+v err %q", a.Results, a.SearchErr)
	}
	if err := a.Subscribe(a.Results[0]); err != nil {
		t.Fatal(err)
	}
	if len(a.Subs) != 1 || !a.Subscribed(show.ID) {
		t.Fatalf("subs %+v", a.Subs)
	}
	a.Open(a.Subs[0])
	if a.Loading || len(a.Episodes) != 1 || a.Episodes[0].Title != "第一集" {
		t.Fatalf("eps %+v %s", a.Episodes, a.EpErr)
	}
	a.PlayEpisode(a.Episodes[0])
	if !a.Player.State().Playing || eng.h.pos != 0 {
		t.Fatalf("play %+v pos %v", a.Player.State(), eng.h.pos)
	}
	eng.h.pos = 2 * time.Minute
	a.Toggle()
	prog, ok := st.Progress(a.Episodes[0].ID)
	if !ok || prog.Position != 2*time.Minute {
		t.Fatalf("progress %+v %v", prog, ok)
	}
	a.PlayEpisode(a.Episodes[0])
	if eng.h.pos != 2*time.Minute {
		t.Fatalf("resume %v", eng.h.pos)
	}
	eng.h.pos = 30*time.Minute - time.Second
	a.Toggle()
	a.PlayEpisode(a.Episodes[0])
	if eng.h.pos != 0 {
		t.Fatalf("near end should restart, pos %v", eng.h.pos)
	}
	if err := a.Unsubscribe(show.ID); err != nil {
		t.Fatal(err)
	}
	if a.Subscribed(show.ID) {
		t.Fatal("still subscribed")
	}
	re, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(re.Subscriptions()) != 0 {
		t.Fatal("subscription survived")
	}
	if _, ok := re.Progress(ep.ID); !ok {
		t.Fatal("progress should remain after unsubscribe")
	}
}

func TestEmptyQuery(t *testing.T) {
	a := New(nil, &catalog.Catalog{}, nil)
	a.Sync = true
	a.Search()
	if a.SearchErr == "" {
		t.Fatal("expected a prompt")
	}
}
