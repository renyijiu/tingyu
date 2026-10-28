package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/player"
)

func (v *View) playbackStatus(c *ui.Context, p palette, st player.State) {
	if st.Err != nil {
		ui.Text(c, st.Err.Error()).Padding(8, 20).TextColor(c.Theme().Danger)
	} else if st.Buffering {
		ui.Text(c, "正在缓冲…").Padding(8, 20).TextColor(p.muted)
	}
	if st.SaveErr != nil {
		ui.Text(c, "无法保存播放进度："+st.SaveErr.Error()).Padding(8, 20).TextColor(c.Theme().Danger)
	}
}

func (v *View) miniPlayer(c *ui.Context, p palette, st player.State) {
	ui.Row(c).Padding(10, 16).Gap(12).Background(p.sidebar).Border(1, p.line).Children(func() {
		open := ui.ButtonBase(c.Key("mini-open")).Grow(1).Gap(12).Radius(12).Padding(4, 6)
		if open.Hovered() {
			open.Background(p.cardHover)
		}
		open.Children(func() {
			if st.Episode == nil {
				ui.Column(c).Size(48, 48).Radius(10).Background(p.card)
				ui.Text(c, "没有正在播放的单集").TextColor(p.muted)
				return
			}
			title := st.Episode.Title
			image := st.Episode.Image
			if image == "" && v.App.Selected != nil {
				image = v.App.Selected.Image
			}
			v.cover(c, p, image, title, 48, 10)
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, title).SingleLine().Bold()
				if st.ShowTitle != "" {
					ui.Text(c, st.ShowTitle).SingleLine().FontSize(12).TextColor(p.muted)
				}
			})
		})
		if open.Clicked() && st.Episode != nil {
			v.expanded = true
			v.home = false
		}
		v.playButton(c, p, st)
		v.scrubber(c, p, st)
		ui.Text(c, formatClock(st.Position)+" / "+formatClock(st.Duration)).FontSize(12).TextColor(p.muted)
	})
}

func (v *View) nowPlaying(c *ui.Context, p palette, st player.State, reduce bool) {
	ep := st.Episode
	image := ep.Image
	showTitle := st.ShowTitle
	if v.App.Selected != nil {
		if image == "" {
			image = v.App.Selected.Image
		}
		if showTitle == "" {
			showTitle = v.App.Selected.Title
		}
	}
	_, wash, has := v.coverBitmap(image)
	if !has {
		wash = p.accent
	}
	angle := float32(150)
	if !reduce {
		angle = 130 + 50*float32(0.5+0.5*sin(seconds(c.Now())*0.45))
	}
	page := ui.Column(c).Fill().Center().Gap(18).Padding(28)
	page.Gradient(wash.Alpha(0.55), p.bg, angle)
	page.Children(func() {
		if ui.Button(c, "收起").Radius(999).Padding(8, 18).MinHeight(36).Clicked() {
			v.expanded = false
		}
		cover := ui.Column(c)
		cover.Shadow(0, 24, 60, 0, p.shadow).Radius(24)
		cover.Scale(breathe(c, st.Playing, reduce), breathe(c, st.Playing, reduce))
		cover.Children(func() {
			v.cover(c, p, image, ep.Title, 280, 24)
		})
		ui.Column(c).Gap(4).Center().Children(func() {
			ui.Text(c, ep.Title).FontSize(28).Bold()
			if showTitle != "" {
				ui.Text(c, showTitle).FontSize(15).TextColor(p.muted)
			}
		})
		v.scrubber(c, p, st)
		ui.Text(c, formatClock(st.Position)+" / "+formatClock(st.Duration)).TextColor(p.muted)
		ui.Row(c).Gap(12).Center().Children(func() {
			if ui.Button(c, "−15s").Radius(999).Clicked() {
				v.App.Player.Skip(-15 * time.Second)
			}
			v.playButton(c, p, st)
			if ui.Button(c, "+30s").Radius(999).Clicked() {
				v.App.Player.Skip(30 * time.Second)
			}
		})
		v.speeds(c, p, st)
	})
}

func (v *View) playButton(c *ui.Context, p palette, st player.State) {
	label := "播放"
	if st.Playing {
		label = "暂停"
	}
	btn := ui.PrimaryButton(c.Key("transport"), label).MinHeight(40).Radius(999).Padding(8, 18)
	btn.Disabled(st.Episode == nil || st.Buffering)
	if btn.Clicked() {
		v.App.Toggle()
	}
	_ = p
}

func (v *View) scrubber(c *ui.Context, p palette, st player.State) {
	hi := st.Duration.Seconds()
	if hi < 1 {
		hi = 1
	}
	if st.Episode == nil {
		v.seek = 0
	} else {
		v.seek = st.Position.Seconds()
	}
	slider := ui.Slider(c.Key("seek"), &v.seek, 0, hi).Label("播放进度").Grow(1).MinWidth(160)
	if st.Episode == nil {
		slider.Disabled(true)
	}
	if slider.Changed() {
		v.App.SeekTo(time.Duration(v.seek * float64(time.Second)))
	}
	_ = p
}

func (v *View) speeds(c *ui.Context, p palette, st player.State) {
	ui.Row(c).Gap(8).Center().Children(func() {
		for _, speed := range []float64{0.8, 1, 1.25, 1.5, 2} {
			label := speedLabel(speed)
			btn := ui.ButtonBase(c.Key(label)).Radius(999).Padding(6, 12)
			if nearly(st.Speed, speed) {
				btn.Background(p.accent)
			} else if btn.Hovered() {
				btn.Background(p.cardHover)
			} else {
				btn.Background(p.card)
			}
			fg := p.text
			if nearly(st.Speed, speed) {
				fg = p.accentText
			}
			btn.Children(func() {
				ui.Text(c, label).FontSize(12).TextColor(fg)
			})
			if btn.Clicked() {
				v.App.Player.SetSpeed(speed)
			}
		}
	})
}

func speedLabel(speed float64) string {
	switch speed {
	case 0.8:
		return "0.8×"
	case 1:
		return "1×"
	case 1.25:
		return "1.25×"
	case 1.5:
		return "1.5×"
	case 2:
		return "2×"
	default:
		return "1×"
	}
}

func nearly(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.01
}
