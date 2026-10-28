package ui

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/feed"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
	"github.com/renyijiu/tingyu/internal/store"
)

type index struct {
	shows []model.Podcast
	eps   []model.Episode
}

func (f index) Search(context.Context, string) ([]model.Podcast, error) {
	return f.shows, nil
}
func (f index) Episodes(context.Context, int64) ([]model.Episode, error) { return f.eps, nil }

type itunes struct{}

func (itunes) Search(context.Context, string) ([]model.Podcast, error) { return nil, nil }
func (itunes) Episodes(context.Context, int64) ([]model.Episode, error) {
	return nil, nil
}

type feeds struct{ eps []model.Episode }

func (f feeds) Fetch(context.Context, string) (feed.Show, []model.Episode, error) {
	return feed.Show{}, f.eps, nil
}

type handle struct {
	pos     time.Duration
	playing bool
}

func (h *handle) Play()                   { h.playing = true }
func (h *handle) Pause()                  { h.playing = false }
func (h *handle) Seek(d time.Duration)    { h.pos = d }
func (h *handle) Position() time.Duration { return h.pos }
func (h *handle) Duration() time.Duration { return 30 * time.Minute }
func (h *handle) Playing() bool           { return h.playing }
func (h *handle) Close() error            { h.playing = false; return nil }

type engine struct{ h *handle }

func (e *engine) Open(io.ReadSeeker, player.Meta) (player.Handle, error) {
	e.h = &handle{}
	return e.h, nil
}

func TestViewSearchSubscribeAndPlay(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	show := model.Podcast{
		Title: "故事会", Author: "甲", Description: "中文播客",
		FeedURL: "https://example.com/gushi.xml", FeedID: 7,
	}
	show.EnsureIDs()
	ep := model.Episode{
		Title: "第一集", GUID: "ep-1", AudioURL: "https://cdn.example.com/1.mp3",
		Duration: 12*time.Minute + 34*time.Second, Published: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	ep.EnsureID(show.FeedURL)
	a := app.New(st, &catalog.Catalog{
		Index:  index{shows: []model.Podcast{show}, eps: []model.Episode{ep}},
		ITunes: itunes{},
		Feeds:  feeds{eps: []model.Episode{ep}},
	}, &player.Controller{
		Engine: &engine{},
		Open: func(context.Context, string) (io.ReadSeeker, error) {
			return bytes.NewReader([]byte("abc")), nil
		},
		Save: func(key string, pos, dur time.Duration) error {
			return st.SaveProgress(key, model.Progress{Position: pos, Duration: dur})
		},
	})
	a.Sync = true
	a.Notice = "未配置 Podcast Index。搜索会使用 iTunes。"
	v := &View{App: a}
	tt := ui.NewTester(v.Build, 1100, 760)
	if !tt.HasText("听宇") || !tt.HasText("iTunes") || !tt.HasText("还没有订阅") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("搜索播客"); err != nil {
		t.Fatal(err)
	}
	tt.Type("故事")
	if a.Query != "故事" {
		t.Fatalf("query %q", a.Query)
	}
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Podcast Index") || !tt.HasText("故事会") {
		t.Fatalf("after search %q", tt.Texts())
	}
	if err := tt.Click("故事会"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("第一集") || !tt.HasText("12:34") {
		t.Fatalf("episodes %q", tt.Texts())
	}
	blockWrites := func() {
		t.Helper()
		if err := os.Mkdir(filepath.Join(dir, "library.json.tmp"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	allowWrites := func() {
		t.Helper()
		if err := os.Remove(filepath.Join(dir, "library.json.tmp")); err != nil {
			t.Fatal(err)
		}
	}
	blockWrites()
	if err := tt.Click("订阅这个节目"); err != nil {
		t.Fatal(err)
	}
	if a.Subscribed(show.ID) || !tt.HasText("无法保存订阅") {
		t.Errorf("failed subscription should stay unsubscribed and show an error: %q", tt.Texts())
	}
	allowWrites()
	if err := tt.Click("订阅这个节目"); err != nil {
		t.Fatal(err)
	}
	if !a.Subscribed(show.ID) || !tt.HasText("取消订阅") || tt.HasText("还没有订阅") {
		t.Fatalf("subscribe texts %q subscribed %v", tt.Texts(), a.Subscribed(show.ID))
	}
	if tt.HasText("无法保存订阅") {
		t.Error("successful subscription did not clear the save error")
	}
	if err := tt.Click("第一集"); err != nil {
		t.Fatal(err)
	}
	if !a.Player.State().Playing || !tt.HasText("暂停") {
		t.Fatalf("playing %v texts %q", a.Player.State().Playing, tt.Texts())
	}
	blockWrites()
	if err := tt.Click("暂停"); err != nil {
		t.Fatal(err)
	}
	if a.Player.State().Playing {
		t.Fatal("still playing")
	}
	if !tt.HasText("无法保存播放进度") {
		t.Errorf("failed progress save was hidden: %q", tt.Texts())
	}
	allowWrites()
	if err := tt.Click("播放"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("暂停"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("无法保存播放进度") {
		t.Error("successful progress save did not clear the error")
	}
	blockWrites()
	if err := tt.Click("取消订阅"); err != nil {
		t.Fatal(err)
	}
	if !a.Subscribed(show.ID) || !tt.HasText("无法保存订阅") {
		t.Errorf("failed unsubscribe should preserve subscription and show an error: %q", tt.Texts())
	}
	allowWrites()
	if err := tt.Click("取消订阅"); err != nil {
		t.Fatal(err)
	}
	if a.Subscribed(show.ID) {
		t.Fatal("still subscribed")
	}
}
