// Tingyu is a macOS podcast client. The window is native UI; search,
// subscriptions and playback live in internal packages.
package main

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/artwork"
	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/config"
	"github.com/renyijiu/tingyu/internal/feed"
	"github.com/renyijiu/tingyu/internal/itunes"
	"github.com/renyijiu/tingyu/internal/media"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/paths"
	"github.com/renyijiu/tingyu/internal/player"
	"github.com/renyijiu/tingyu/internal/player/mygoengine"
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
	settings, err := config.LoadSettings(dir)
	if err != nil {
		log.Fatal(err)
	}
	lib, err := store.Open(dir)
	if err != nil {
		log.Fatalf("无法读取订阅库 %s: %v", filepath.Join(dir, "library.json"), err)
	}
	apple := &itunes.Client{}
	cat := &catalog.Catalog{
		ITunes: apple,
		Charts: apple,
		Feeds:  &feed.Fetcher{},
		Cache:  &catalog.DiskCache{Dir: filepath.Join(media.CacheDir(dir), "feeds")},
	}
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
	application.Configure(dir, settings)
	playback.OnChange = func() {
		if application.OnUI != nil {
			application.OnUI(func() {})
		}
	}
	view := &tingui.View{
		App:    application,
		Covers: &artwork.Loader{Dir: filepath.Join(media.CacheDir(dir), "covers")},
	}
	mygo.App.WhenReady(func() {
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { playback.Close() })
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:         "听宇",
			Width:         1100,
			Height:        740,
			MinWidth:      860,
			MinHeight:     560,
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(view.Build),
		})
		application.OnUI = func(fn func()) { win.Update(fn) }
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleAbout},
				mygo.Separator(),
				{Label: "设置…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) {
					win.Update(view.OpenSettings)
				}},
				mygo.Separator(),
				{Role: mygo.RoleServices},
				mygo.Separator(),
				{Role: mygo.RoleHide},
				{Role: mygo.RoleHideOthers},
				{Role: mygo.RoleUnhide},
				mygo.Separator(),
				{Role: mygo.RoleQuit},
			}},
			{Role: mygo.RoleFileMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleViewMenu},
			{Role: mygo.RoleWindowMenu},
		}))
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
