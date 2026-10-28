// Tingyu is a macOS podcast client. The window is native UI; search,
// subscriptions and playback live in internal packages.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/config"
	"github.com/renyijiu/tingyu/internal/feed"
	"github.com/renyijiu/tingyu/internal/itunes"
	"github.com/renyijiu/tingyu/internal/media"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/paths"
	"github.com/renyijiu/tingyu/internal/player"
	"github.com/renyijiu/tingyu/internal/player/mygoengine"
	"github.com/renyijiu/tingyu/internal/podcastindex"
	"github.com/renyijiu/tingyu/internal/store"
	tingui "github.com/renyijiu/tingyu/internal/ui"
)

func main() {
	dir, err := paths.Dir()
	if err != nil {
		log.Fatal(err)
	}
	if err := config.WriteTemplate(dir); err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		log.Fatal(err)
	}
	lib, err := store.Open(dir)
	if err != nil {
		log.Fatalf("无法读取订阅库 %s: %v", filepath.Join(dir, "library.json"), err)
	}
	var index catalog.Index
	if cfg.Ready() {
		index = &podcastindex.Client{Key: cfg.PodcastIndexKey, Secret: cfg.PodcastIndexSecret}
	}
	cat := &catalog.Catalog{Index: index, ITunes: &itunes.Client{}, Feeds: &feed.Fetcher{}}
	playback := &player.Controller{
		Engine: mygoengine.New(),
		Open: func(ctx context.Context, audioURL string) (io.ReadSeeker, error) {
			return media.Open(ctx, nil, audioURL, media.CacheDir(dir))
		},
		Save: func(key string, pos, dur time.Duration) error {
			return lib.SaveProgress(key, model.Progress{Position: pos, Duration: dur})
		},
	}
	application := app.New(lib, cat, playback)
	playback.OnChange = func() {
		if application.OnUI != nil {
			application.OnUI(func() {})
		}
	}
	if !cfg.Ready() {
		application.Notice = fmt.Sprintf(
			"未配置 Podcast Index。搜索会使用 iTunes。请设置环境变量 %s 和 %s，或填写 %s。",
			config.EnvKey, config.EnvSecret, filepath.Join(dir, config.FileName),
		)
	}
	view := &tingui.View{App: application}
	mygo.App.WhenReady(func() {
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { playback.Close() })
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "听宇",
			Width:     1100,
			Height:    740,
			MinWidth:  860,
			MinHeight: 560,
			Content:   ui.View(view.Build),
		})
		application.OnUI = func(fn func()) { win.Update(fn) }
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
