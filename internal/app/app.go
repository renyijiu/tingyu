// Package app is the UI-independent state of Tingyu: search, subscriptions,
// episode lists and playback.
package app

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/config"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
	"github.com/renyijiu/tingyu/internal/store"
)

// App is the model the window draws.
type App struct {
	Store    *store.Store
	Catalog  *catalog.Catalog
	Player   *player.Controller
	Settings config.Settings

	settingsDir string

	// Notice explains the current search configuration.
	Notice string

	// Sync runs background work inline. Tests set it.
	Sync bool
	// OnUI runs a function on the UI thread. Window.Update in the real app.
	OnUI func(func())

	Query        string
	Results      []model.Podcast
	ResultSource string
	Searching    bool
	SearchErr    string

	// Chart is what most listen to now, for the Discover page.
	Chart        []model.Podcast
	chartLoading bool
	chartFailed  time.Time

	Subs     []model.Podcast
	Selected *model.Podcast
	Episodes []model.Episode
	Loading  bool
	EpErr    string

	searchGen int
	openGen   int
}

// New loads subscriptions from disk.
func New(st *store.Store, cat *catalog.Catalog, pl *player.Controller) *App {
	a := &App{Store: st, Catalog: cat, Player: pl}
	if st != nil {
		a.Subs = st.Subscriptions()
	}
	return a
}

// Search looks up Query.
func (a *App) Search() {
	a.searchGen++
	q := strings.TrimSpace(a.Query)
	if q == "" {
		a.Searching = false
		a.SearchErr = "请输入关键词"
		a.Results = nil
		a.ResultSource = ""
		return
	}
	a.Searching = true
	a.SearchErr = ""
	gen := a.searchGen
	cat := a.Catalog
	a.async(func() {
		shows, src, err := cat.Search(context.Background(), q)
		a.publish(func() {
			if gen != a.searchGen {
				return
			}
			a.Searching = false
			a.ResultSource = src
			if err != nil {
				log.Printf("search %q: %v", q, err)
				a.SearchErr = Explain(err, "搜索失败，请稍后再试")
				a.Results = nil
				return
			}
			a.Results = shows
			if len(shows) == 0 {
				a.SearchErr = "没有找到播客"
			}
		})
	})
}

// chartRetry is how long a chart that could not be loaded waits before it
// is asked for again.
const chartRetry = time.Minute

// LoadChart loads the chart unless it is loaded or loading. One that cannot
// be loaded leaves the Discover page as it was, and is asked for again a
// minute later, the next time the page draws.
func (a *App) LoadChart() {
	if len(a.Chart) > 0 || a.chartLoading || a.Catalog == nil {
		return
	}
	if !a.chartFailed.IsZero() && time.Since(a.chartFailed) < chartRetry {
		return
	}
	a.chartLoading = true
	cat := a.Catalog
	a.async(func() {
		shows, err := cat.Popular(context.Background())
		a.publish(func() {
			a.chartLoading = false
			if err != nil || len(shows) == 0 {
				a.chartFailed = time.Now()
				return
			}
			a.Chart = shows
		})
	})
}

// Open shows a podcast's episodes. A show opened before shows at once as
// it was, then as its feed has it now.
func (a *App) Open(p model.Podcast) {
	p.EnsureIDs()
	a.Selected = &p
	a.Episodes = nil
	a.EpErr = ""
	a.Loading = true
	a.openGen++
	gen := a.openGen
	cat := a.Catalog
	a.async(func() {
		show, eps, err := cat.Episodes(context.Background(), p, func(show model.Podcast, eps []model.Episode) {
			a.publish(func() {
				if gen != a.openGen {
					return
				}
				a.Loading = false
				a.Selected = &show
				a.Episodes = eps
			})
		})
		a.publish(func() {
			if gen != a.openGen {
				return
			}
			a.Loading = false
			if err != nil {
				log.Printf("episodes of %s: %v", p.FeedURL, err)
				a.EpErr = Explain(err, "无法读取这个节目的单集")
				return
			}
			a.Selected = &show
			a.Episodes = eps
			if len(eps) == 0 {
				a.EpErr = "这个节目还没有可播放的单集"
			}
		})
	})
}

// Subscribe adds the show and keeps it on disk.
func (a *App) Subscribe(p model.Podcast) error {
	p.EnsureIDs()
	if err := a.Store.Subscribe(p); err != nil {
		return err
	}
	a.Subs = a.Store.Subscriptions()
	if a.Selected != nil && a.Selected.ID == p.ID {
		cp := p
		if i := indexOf(a.Subs, p.ID); i >= 0 {
			cp = a.Subs[i]
		}
		a.Selected = &cp
	}
	return nil
}

// Unsubscribe removes the show.
func (a *App) Unsubscribe(id string) error {
	if err := a.Store.Unsubscribe(id); err != nil {
		return err
	}
	a.Subs = a.Store.Subscriptions()
	return nil
}

// Subscribed reports whether id is in the library.
func (a *App) Subscribed(id string) bool {
	return a.Store != nil && a.Store.IsSubscribed(id)
}

// PlayEpisode starts playback from the saved position.
func (a *App) PlayEpisode(ep model.Episode) {
	if a.Player == nil {
		return
	}
	show := ""
	if a.Selected != nil {
		show = a.Selected.Title
	}
	var resume model.Progress
	if a.Store != nil {
		if p, ok := a.Store.Progress(ep.ID); ok {
			resume = p
		}
	}
	at := model.ResumeFrom(resume)
	epCopy := ep
	a.async(func() {
		a.Player.Play(context.Background(), show, epCopy, at)
		a.publish(func() {})
	})
}

// Toggle pauses or resumes.
func (a *App) Toggle() {
	if a.Player != nil {
		a.Player.Toggle()
	}
}

// SeekTo moves the current episode.
func (a *App) SeekTo(d time.Duration) {
	if a.Player != nil {
		a.Player.Seek(d)
	}
}

func (a *App) async(fn func()) {
	if a.Sync {
		fn()
		return
	}
	go fn()
}

func (a *App) publish(fn func()) {
	if a.OnUI != nil && !a.Sync {
		a.OnUI(fn)
		return
	}
	fn()
}

func indexOf(list []model.Podcast, id string) int {
	for i, p := range list {
		if p.ID == id {
			return i
		}
	}
	return -1
}
