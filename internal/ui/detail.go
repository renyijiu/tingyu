package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
)

// detail is a show's page: its cover, large, glowing in its color over a
// blur of itself, and its episodes below, the whole page scrolling as one.
// The list's first rows are the page's head, so only the episodes in view
// are built, however many the feed has.
func (v *View) detail(c *ui.Context, p palette, st player.State, reduce bool) {
	show := *v.App.Selected
	if bmp, _, ok := v.coverBitmap(show.Image); ok && bmp != nil {
		// The cover's own light, blurred, falling on the page from the top.
		ui.Image(c, bmp).Absolute().Right(-60).Top(-120).Size(440, 440).Fit(ui.Cover).Radius(220).
			Blur(56).Opacity(0.3).PassThrough().Cache()
	}
	v.topBar(c, func() {
		back := ui.ButtonBase(c.Key("back")).Gap(4).Padding(5, 12, 5, 8).Radius(999).Label("返回发现")
		bg := p.panel
		if back.Hovered() {
			bg = p.panelHi
		}
		back.Background(back.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond})).Border(1, p.rim)
		back.Children(func() {
			ui.Icon(c, iconLeft).FontSize(15).TextColor(p.muted)
			ui.Text(c, "发现").FontSize(12).Bold().TextColor(p.muted)
		})
		if back.Clicked() {
			v.home = true
		}
	})
	const head = 2
	n := head + len(v.App.Episodes)
	ui.List(c, &v.episodes, n, func(i int) {
		switch i {
		case 0:
			v.hero(c, p, show)
		case 1:
			v.episodesHeader(c, p)
		default:
			v.episodeRow(c, p, st, show, i-head, v.App.Episodes[i-head], reduce)
		}
	}).Grow(1).Padding(0, 0, 24, 0)
}

func (v *View) hero(c *ui.Context, p palette, show model.Podcast) {
	tint := v.tintOf(p, show.Image, show.Title)
	ui.Column(c).FillWidth().Padding(8, 36, 22, 36).Children(func() {
		ui.Row(c).Gap(28).AlignItems(ui.Start).Children(func() {
			art := ui.Box(c).Radius(24).Shadow(0, 22, 50, -6, tint.Alpha(0.55)).Shadow(0, 2, 6, 0, p.shadow)
			art.Children(func() {
				v.cover(c, p, show.Image, show.Title, 196, 24, coverMatch{show.ID})
			})
			ui.Column(c).Grow(1).Gap(10).Padding(4, 0, 0, 0).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					overline(c, p, iconMic, "PODCAST")
					if show.Source != "" {
						chip(c, p, nil, sourceName(show.Source), tint.Mix(p.text, 0.25))
					}
				})
				ui.Text(c, show.Title).FontSize(32).Bold().MaxLines(2).LetterSpacing(-0.3)
				if show.Author != "" {
					ui.Text(c, show.Author).FontSize(14).Bold().TextColor(tint.Mix(p.text, 0.45))
				}
				if desc := truncate(show.Description, 160); desc != "" {
					ui.Text(c, desc).FontSize(13).LineHeight(1.55).MaxLines(3).TextColor(p.muted)
				}
				ui.Row(c).Gap(10).Margin(6, 0, 0, 0).Children(func() {
					if len(v.App.Episodes) > 0 {
						if pill(c, p, "latest", iconPlay, "播放最新一集", true).Clicked() {
							v.play(show, v.App.Episodes[0])
						}
					}
					v.subscribeButton(c, p, show)
				})
				if v.libraryErr != nil {
					ui.Row(c).Gap(6).Children(func() {
						ui.Icon(c, iconAlert).FontSize(14).TextColor(p.danger)
						ui.Text(c, "无法保存订阅："+v.libraryErr.Error()).FontSize(12).TextColor(p.danger)
					})
				}
			})
		})
	})
}

// subscribeButton follows the show, and bursts into stars when it has.
// Followed, it says so, beside a quiet way to stop.
func (v *View) subscribeButton(c *ui.Context, p palette, show model.Podcast) {
	if !v.App.Subscribed(show.ID) {
		if pill(c, p, "subscribe", iconPlus, "订阅这个节目", false).Clicked() {
			v.libraryErr = v.App.Subscribe(show)
			if v.libraryErr == nil {
				v.burst = c.Now()
			}
		}
		return
	}
	done := ui.Row(c.Key("subscribed")).Gap(6).Height(40).Padding(0, 16, 0, 12).Radius(999).
		Background(p.ok.Alpha(0.12)).Border(1, p.ok.Alpha(0.3))
	done.Transition(ui.ElementTransition{Animation: &pop, Enter: &ui.Motion{Scale: 0.7, Opacity: 0}})
	at := v.burst
	done.DrawOver(func(pt *ui.Painter, r ui.Rect) { sparkles(pt, r, p, at) })
	done.Children(func() {
		ui.Icon(c, iconCheck).FontSize(16).TextColor(p.ok)
		ui.Text(c, "已订阅").Bold().FontSize(13).TextColor(p.ok)
	})
	stop := ui.ButtonBase(c.Key("unsubscribe")).Height(40).Padding(0, 14).Radius(999)
	color := p.muted
	if stop.Hovered() {
		color = p.danger
		stop.Background(p.danger.Alpha(0.08))
	}
	stop.Children(func() { ui.Text(c, "取消订阅").FontSize(13).TextColor(color) })
	if stop.Clicked() {
		v.libraryErr = v.App.Unsubscribe(show.ID)
	}
}

func (v *View) episodesHeader(c *ui.Context, p palette) {
	ui.Column(c).FillWidth().Padding(6, 36, 8, 36).Gap(10).Children(func() {
		ui.Row(c).Gap(10).Children(func() {
			ui.Text(c, "单集").FontSize(18).Bold()
			if n := len(v.App.Episodes); n > 0 {
				ui.Text(c, "共 "+itoa(n)+" 集").FontSize(12).FontFeatures("tnum").TextColor(p.muted)
			}
		})
		if v.App.Loading {
			for i := 0; i < 4; i++ {
				ui.Row(c.Key("loading:" + itoa(i))).Height(58).Radius(16).Background(p.panel).Opacity(1 - float32(i)*0.2)
			}
			ui.Text(c, "正在加载单集…").FontSize(12).TextColor(p.muted)
		}
		if v.App.EpErr != "" {
			ui.Row(c).Gap(8).Padding(12, 14).Radius(14).Background(p.danger.Alpha(0.08)).Children(func() {
				ui.Icon(c, iconAlert).FontSize(15).TextColor(p.danger)
				ui.Text(c, v.App.EpErr).FontSize(13).TextColor(p.danger)
			})
		}
	})
}

// episodeRow is an episode: its number, or a play button under the
// pointer, or the equalizer while it plays; its title and when it came
// out; and a ring of how much of it was heard.
func (v *View) episodeRow(c *ui.Context, p palette, st player.State, show model.Podcast, i int, ep model.Episode, reduce bool) {
	current := st.Episode != nil && st.Episode.ID == ep.ID
	ui.Box(c).FillWidth().Padding(0, 24).Children(func() {
		row := ui.ButtonBase(c.Key("ep:"+ep.ID)).FillWidth().Padding(10, 12).Gap(14).Radius(16)
		hot := row.Hovered() || row.Focused()
		bg := p.panelHi.Alpha(0)
		switch {
		case current:
			bg = p.accent.Alpha(0.14)
		case row.Pressed():
			bg = p.panelHi
		case hot:
			bg = p.panel
		}
		row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 140 * time.Millisecond}))
		if current {
			row.Border(1, p.accent.Alpha(0.28))
		}
		row.Children(func() {
			ui.Box(c).Size(34, 34).Radius(17).Center().Children(func() {
				switch {
				case current && (st.Playing || st.Buffering):
					equalizer(c, "eq", p.accent.Mix(p.cyan, 0.4), true, reduce, 16, 14)
				case hot || current:
					ui.Box(c.Key("play")).Size(34, 34).Radius(17).Center().LinearGradient(p.aurora(135)).
						Transition(ui.ElementTransition{Animation: &pop, Enter: &ui.Motion{Scale: 0.5, Opacity: 0}}).
						Children(func() { ui.Icon(c, iconPlay).FontSize(14).TextColor(p.accentText).Margin(0, 0, 0, 2) })
				default:
					ui.Text(c, itoa(i+1)).FontSize(13).Bold().FontFeatures("tnum").TextColor(p.faint)
				}
			})
			ui.Column(c).Grow(1).Gap(3).Children(func() {
				title := ui.Text(c, ep.Title).SingleLine().FontSize(14)
				if current {
					title.Bold().TextColor(p.accent.Mix(p.text, 0.35))
				}
				if meta := episodeMeta(v.App, ep); meta != "" {
					ui.Text(c, meta).FontSize(12).FontFeatures("tnum").TextColor(p.muted)
				}
			})
			if frac := playedFrac(v.App, ep); frac > 0 {
				ui.Box(c).Size(18, 18).Draw(func(pt *ui.Painter, r ui.Rect) { progressRing(pt, r, p, float32(frac)) })
			}
			if ep.Duration > 0 {
				ui.Text(c, formatClock(ep.Duration)).FontSize(12).FontFeatures("tnum").TextColor(p.faint).MinWidth(52).TextAlign(ui.End)
			}
		})
		if row.Clicked() {
			v.play(show, ep)
		}
	})
}

// progressRing is how much of an episode was heard, around a circle, or a
// check once it all was.
func progressRing(pt *ui.Painter, r ui.Rect, p palette, frac float32) {
	cx, cy, rad := r.X+r.W/2, r.Y+r.H/2, r.W/2-1.5
	if frac >= 0.97 {
		var dot ui.Path
		dot.Circle(cx, cy, r.W/2)
		pt.FillPath(&dot, p.ok.Alpha(0.16))
		check := (&ui.Path{}).MoveTo(cx-4, cy).LineTo(cx-1, cy+3).LineTo(cx+4.5, cy-3)
		pt.StrokePath(check, 1.8, p.ok)
		return
	}
	var track ui.Path
	track.Circle(cx, cy, rad)
	pt.StrokePath(&track, 2.2, p.text.Alpha(0.12))
	start := float32(-math.Pi / 2)
	pt.StrokePathGradient(arcPath(cx, cy, rad, start, start+2*math.Pi*frac), 2.2, p.aurora(0))
}

func episodeMeta(a *app.App, ep model.Episode) string {
	var parts []string
	if !ep.Published.IsZero() {
		parts = append(parts, ep.Published.Local().Format("2006-01-02"))
	}
	if ep.Duration > 0 {
		parts = append(parts, formatLength(ep.Duration))
	}
	if a != nil && a.Store != nil {
		if prog, ok := a.Store.Progress(ep.ID); ok && prog.Position > 0 {
			parts = append(parts, "听到 "+formatClock(prog.Position))
		}
	}
	return joinParts(parts)
}

func playedFrac(a *app.App, ep model.Episode) float64 {
	if a == nil || a.Store == nil {
		return 0
	}
	prog, ok := a.Store.Progress(ep.ID)
	if !ok || prog.Position <= 0 {
		return 0
	}
	dur := prog.Duration
	if dur <= 0 {
		dur = ep.Duration
	}
	if dur <= 0 {
		return 0
	}
	f := float64(prog.Position) / float64(dur)
	if f > 1 {
		return 1
	}
	return f
}

func joinParts(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "  ·  "
		}
		out += p
	}
	return out
}
