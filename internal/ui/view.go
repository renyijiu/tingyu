// Package ui is the native window. Podcast rules stay in internal/app so
// they can be tested without a window.
package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/config"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
)

// View is the window contents: a sky the panels float over, a sidebar of
// glass on the left, the page in the middle and the player docked below.
type View struct {
	App *app.App

	art        *artCache
	sky        cosmos
	voice      voice
	episodes   ui.ListState
	grid       ui.GridState
	seek       float64
	expanded   bool
	home       bool
	recent     []recentPlay
	libraryErr error

	// burst is when a subscription last went through, for its sparkles.
	burst time.Time
	// searches counts the searches, so the results come in anew each time.
	searches uint64

	settingsOpen    bool
	settingsRuns    uint64
	settingsDraft   config.Config
	showCredentials bool
	settingsErr     string
	settingsMessage string
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
	if st.Playing || st.Buffering {
		// The clock and the scrubber move a pixel every few seconds: four
		// builds a second keep them current. What moves smoothly draws itself.
		c.After(250 * time.Millisecond)
		if st.Playing {
			v.App.Player.Tick()
		}
	}
	v.sky.flow(st.Playing && !reduce, c.Now())

	root := ui.Column(c).Fill()
	tint := root.AnimateColor("tint", v.skyTint(p, st), ui.Animation{Duration: 1200 * time.Millisecond, Ease: ui.EaseInOut})
	energy := float32(0)
	if st.Playing {
		energy = 1
	}
	energy = root.AnimateTo("energy", energy, ui.Animation{Duration: 1500 * time.Millisecond, Ease: ui.EaseInOut})
	root.Draw(func(pt *ui.Painter, r ui.Rect) { v.sky.paint(pt, r, p, tint, energy, v.loudness(pt, reduce)) })
	root.Children(func() {
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			v.sidebar(c, p, st, reduce)
			v.stage(c, p, st, reduce)
		})
		if !v.expanded || st.Episode == nil {
			v.dock(c, p, st, reduce)
		}
		v.status(c, p, st)
	})
	v.settings(c, p)
}

// skyTint is the color the sky takes: the playing show's on the player,
// the open show's on its page, and otherwise what plays, if anything.
func (v *View) skyTint(p palette, st player.State) ui.Color {
	switch {
	case v.expanded && st.Episode != nil:
		img, show := v.playingArt(st)
		return v.tintOf(p, img, show)
	case v.onShow():
		return v.tintOf(p, v.App.Selected.Image, v.App.Selected.Title)
	case st.Episode != nil:
		img, show := v.playingArt(st)
		return v.tintOf(p, img, show)
	}
	return p.accent
}

func (v *View) onShow() bool {
	return !v.home && !v.expanded && v.App.Selected != nil
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

// topBar is the strip along the top of the page, under the window's title
// bar: it drags the window, and holds the page's way back.
func (v *View) topBar(c *ui.Context, fn func()) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(bar.Height, 18)+22).Padding(10, 28, 0, 20).Gap(10).DragWindow().Children(fn)
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

// playingArt is the cover and show title of what plays: the episode's own
// cover, or its show's.
func (v *View) playingArt(st player.State) (image, show string) {
	if st.Episode == nil {
		return "", ""
	}
	image, show = st.Episode.Image, st.ShowTitle
	for _, item := range v.recent {
		if item.Episode.ID == st.Episode.ID {
			if image == "" {
				image = item.Show.Image
			}
			if show == "" {
				show = item.Show.Title
			}
			return image, show
		}
	}
	if v.App.Selected != nil {
		if image == "" {
			image = v.App.Selected.Image
		}
		if show == "" {
			show = v.App.Selected.Title
		}
	}
	return image, show
}

// breathe swells e and lets it go, slowly, while something plays, as if
// the planet were alive. The view builds four times a second while playing
// anyway; it only turns the swell around, and the frames between only
// paint.
func breathe(c *ui.Context, e ui.Element, playing, reduce bool) {
	const half = 2200 * time.Millisecond
	scale := float32(1)
	if playing && !reduce && int(math.Floor(seconds(c.Now())/half.Seconds()))%2 == 0 {
		scale = 1.025
	}
	e.AnimatePose(ui.Pose{Scale: scale}, ui.Animation{Duration: half, Ease: ui.EaseInOut})
}

// loudness is how loud what plays is now, smoothed, or 0 when the engine
// cannot tell, motion is reduced or nothing plays.
func (v *View) loudness(pt *ui.Painter, reduce bool) float32 {
	level, ok := v.App.Player.Level()
	if !ok || reduce {
		level = 0
	}
	return v.voice.follow(pt.Now(), level)
}
