// Package ui is the native window. Podcast rules stay in internal/app so
// they can be tested without a window.
package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
)

// View is the window contents.
type View struct {
	App *app.App

	art      *artCache
	subs     ui.ListState
	episodes ui.ListState
	grid     ui.GridState
	seek     float64
	expanded bool
	home     bool
	recent   []recentPlay
}

type recentPlay struct {
	Show    model.Podcast
	Episode model.Episode
}

// Build draws one frame.
func (v *View) Build(c *ui.Context) {
	p := theme(c)
	st := v.App.Player.State()
	reduce := c.Preferences().ReduceMotion
	if st.Playing || st.Buffering || v.App.Searching || (v.expanded && !reduce) {
		c.After(32 * time.Millisecond)
		if st.Playing {
			v.App.Player.Tick()
		}
	}
	ui.Column(c).Fill().Background(p.bg).Children(func() {
		if v.App.Notice != "" {
			ui.Text(c, v.App.Notice).Padding(8, 20).FontSize(12).TextColor(p.muted)
		}
		ui.Row(c).Grow(1).Children(func() {
			v.sidebar(c, p, st)
			v.stage(c, p, st, reduce)
		})
		if !v.expanded {
			v.miniPlayer(c, p, st)
		}
	})
}

func (v *View) stage(c *ui.Context, p palette, st player.State, reduce bool) {
	key := "home"
	trans := pageEnter()
	switch {
	case v.expanded && st.Episode != nil:
		key = "now"
		trans = nowEnter()
	case !v.home && v.App.Selected != nil:
		key = "show:" + v.App.Selected.ID
	}
	ui.Column(c.Key(key)).Grow(1).FillHeight().Transition(trans).Children(func() {
		switch {
		case v.expanded && st.Episode != nil:
			v.nowPlaying(c, p, st, reduce)
		case !v.home && v.App.Selected != nil:
			v.detail(c, p, st, reduce)
		default:
			v.discover(c, p, reduce)
		}
	})
}

func (v *View) show(show model.Podcast) {
	v.home = false
	v.expanded = false
	v.App.Open(show)
}

func (v *View) play(show model.Podcast, ep model.Episode) {
	if show.ID == "" && v.App.Selected != nil {
		show = *v.App.Selected
	}
	v.remember(show, ep)
	v.App.PlayEpisode(ep)
}

func (v *View) remember(show model.Podcast, ep model.Episode) {
	next := []recentPlay{{Show: show, Episode: ep}}
	for _, item := range v.recent {
		if item.Episode.ID == ep.ID || len(next) >= 6 {
			continue
		}
		next = append(next, item)
	}
	v.recent = next
}

func breathe(c *ui.Context, playing, reduce bool) float32 {
	if !playing || reduce {
		return 1
	}
	return 1 + 0.018*float32(math.Sin(seconds(c.Now())*1.35))
}

func shimmer(c *ui.Context, reduce bool) float32 {
	if reduce {
		return 0.72
	}
	return 0.45 + 0.35*float32(0.5+0.5*math.Sin(seconds(c.Now())*3.2))
}
