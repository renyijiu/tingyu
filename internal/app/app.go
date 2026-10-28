// Package app is the UI-independent state of Tingyu: search, subscriptions,
// episode lists and playback.
package app

import (
	"context"
	"strings"
	"time"

	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
	"github.com/renyijiu/tingyu/internal/store"
)

// App is the model the window draws.
type App struct {
	Store   *store.Store
	Catalog *catalog.Catalog
	Player  *player.Controller

	// Notice is a startup message, such as a missing Podcast Index key.
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
	a.async(func() {
		shows, src, err := a.Catalog.Search(context.Background(), q)
		a.publish(func() {
			if gen != a.searchGen {
				return
			}
			a.Searching = false
			a.ResultSource = src
			if err != nil {
				a.SearchErr = err.Error()
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

// Open shows a podcast's episodes.
func (a *App) Open(p model.Podcast) {
	p.EnsureIDs()
	a.Selected = &p
	a.Episodes = nil
	a.EpErr = ""
	a.Loading = true
	a.openGen++
	gen := a.openGen
	a.async(func() {
		eps, err := a.Catalog.Episodes(context.Background(), p)
		a.publish(func() {
			if gen != a.openGen {
				return
			}
			a.Loading = false
			if err != nil {
				a.EpErr = err.Error()
				return
			}
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
