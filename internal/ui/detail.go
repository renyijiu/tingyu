package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
)

func (v *View) detail(c *ui.Context, p palette, st player.State, reduce bool) {
	show := *v.App.Selected
	_, wash, has := v.coverBitmap(show.Image)
	if !has {
		wash = p.accent
	}
	body := ui.Column(c).Fill().Clip()
	body.Gradient(wash.Alpha(0.42), p.bg, 168)
	body.Children(func() {
		if bmp, _, ok := v.coverBitmap(show.Image); ok && bmp != nil {
			ui.Image(c, bmp).Absolute().Right(-30).Top(-20).Size(340, 340).Fit(ui.Cover).Blur(28).Opacity(0.28)
		}
		ui.Column(c).Fill().Padding(28, 32, 12, 32).Gap(16).Children(func() {
			ui.Row(c).Gap(22).Children(func() {
				cover := ui.Column(c)
				cover.Shadow(0, 16, 36, 0, p.shadow).Radius(18)
				cover.Children(func() {
					v.cover(c, p, show.Image, show.Title, 168, 18)
				})
				ui.Column(c).Grow(1).Gap(8).Padding(6, 0, 0, 0).Children(func() {
					ui.Text(c, show.Title).FontSize(32).Bold()
					if show.Author != "" {
						ui.Text(c, show.Author).FontSize(15).TextColor(p.muted)
					}
					if desc := truncate(show.Description, 140); desc != "" {
						ui.Text(c, desc).MaxLines(3).TextColor(p.muted)
					}
					v.subscribeButton(c, p, show)
				})
			})
			if v.App.Loading {
				ui.Text(c, "正在加载单集…").TextColor(p.muted)
			}
			if v.App.EpErr != "" {
				ui.Text(c, v.App.EpErr).TextColor(c.Theme().Danger)
			}
			ui.Text(c, "单集").Bold()
			ui.List(c, &v.episodes, len(v.App.Episodes), func(i int) {
				v.episodeRow(c, p, st, show, v.App.Episodes[i], reduce)
			}).Grow(1)
		})
	})
}

func (v *View) subscribeButton(c *ui.Context, p palette, show model.Podcast) {
	on := v.App.Subscribed(show.ID)
	label := "订阅这个节目"
	if on {
		label = "取消订阅"
	}
	btn := ui.ButtonBase(c.Key("subbtn")).Radius(999).Padding(10, 18).MinHeight(40)
	bg := p.accent
	fg := p.accentText
	if on {
		bg = p.card
		fg = p.text
		btn.Border(1, p.line)
	}
	bg = btn.AnimateColor("sub", bg, ui.Animation{Duration: 220 * time.Millisecond, Ease: ui.EaseOut})
	btn.Background(bg)
	scale := float32(1)
	if btn.Hovered() {
		scale = 1.04
	}
	btn.AnimatePose(ui.Pose{Scale: scale}, ui.Snappy)
	btn.Children(func() {
		ui.Text(c, label).Bold().TextColor(fg)
	})
	if btn.Clicked() {
		if on {
			_ = v.App.Unsubscribe(show.ID)
		} else {
			_ = v.App.Subscribe(show)
		}
	}
}

func (v *View) episodeRow(c *ui.Context, p palette, st player.State, show model.Podcast, ep model.Episode, reduce bool) {
	current := st.Episode != nil && st.Episode.ID == ep.ID
	row := ui.Column(c.Key("ep:"+ep.ID)).FillWidth().Padding(8, 4).Radius(12).Gap(6)
	if current {
		row.Background(p.accent.Alpha(0.12))
	} else if row.Hovered() {
		row.Background(p.cardHover)
	}
	row.Children(func() {
		ui.Row(c).Gap(12).Children(func() {
			if current && (st.Playing || st.Buffering) {
				v.equalizer(c, p, reduce)
			} else {
				ui.Column(c).Width(16)
			}
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, ep.Title).SingleLine()
				if meta := episodeMeta(v.App, ep); meta != "" {
					ui.Text(c, meta).FontSize(12).TextColor(p.muted)
				}
			})
			if ep.Duration > 0 {
				ui.Text(c, formatClock(ep.Duration)).FontSize(12).TextColor(p.faint)
			}
		})
		if frac := playedFrac(v.App, ep); frac > 0 {
			ui.Row(c).Height(3).Radius(2).Background(p.line).Children(func() {
				ui.Column(c).Height(3).Width(float32(frac) * 220).Radius(2).Background(p.accent)
			})
		}
	})
	if row.Clicked() {
		v.play(show, ep)
	}
}

func (v *View) equalizer(c *ui.Context, p palette, reduce bool) {
	ui.Row(c).Width(16).Height(14).Gap(2).AlignItems(ui.End).Children(func() {
		now := seconds(c.Now())
		for i := 0; i < 3; i++ {
			h := float32(5)
			if !reduce {
				h = 4 + 9*float32(0.5+0.5*sin(now*6+float64(i)*0.9))
			}
			bar := ui.Column(c.Key(i)).Width(3).Radius(2).Background(p.accent)
			bar.Height(bar.Animate("eq", h, 90*time.Millisecond))
		}
	})
}

func episodeMeta(a *app.App, ep model.Episode) string {
	var parts []string
	if !ep.Published.IsZero() {
		parts = append(parts, ep.Published.Local().Format("2006-01-02"))
	}
	if ep.Duration > 0 {
		parts = append(parts, formatClock(ep.Duration))
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

func sin(v float64) float64 { return math.Sin(v) }
