package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
)

func (v *View) sidebar(c *ui.Context, p palette, st player.State) {
	ui.Column(c).Width(276).FillHeight().Padding(18, 14).Gap(6).Background(p.sidebar).Children(func() {
		ui.Text(c, "听宇").FontSize(22).Bold().Padding(6, 8, 0, 8)
		ui.Text(c, "Tingyu").FontSize(12).TextColor(p.faint).Padding(0, 8, 10, 8)

		v.nav(c, p, "发现", v.home || (v.App.Selected == nil && !v.expanded), func() {
			v.home = true
			v.expanded = false
		})
		ui.Scroll(c.Key("library")).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, "我的订阅").FontSize(12).Bold().TextColor(p.muted).Padding(14, 8, 4, 8)
				if len(v.App.Subs) == 0 {
					ui.Text(c, "还没有订阅").FontSize(12).TextColor(p.faint).Padding(2, 8)
				} else {
					ui.List(c, &v.subs, len(v.App.Subs), func(i int) {
						v.subRow(c, p, v.App.Subs[i])
					}).MaxHeight(280)
				}

				if len(v.recent) > 0 {
					ui.Text(c, "最近播放").FontSize(12).Bold().TextColor(p.muted).Padding(14, 8, 4, 8)
					for _, item := range v.recent {
						v.recentRow(c, p, st, item)
					}
				}
			})
		})
		ui.Divider(c)
		v.settingsEntry(c, p)
	})
}

func (v *View) nav(c *ui.Context, p palette, label string, on bool, fn func()) {
	row := ui.ButtonBase(c.Key("nav:"+label)).FillWidth().Radius(10).Padding(8, 10)
	if on {
		row.Background(p.accent.Alpha(0.18))
	} else if row.Hovered() {
		row.Background(p.cardHover)
	}
	row.Children(func() {
		color := p.text
		if on {
			color = p.accent
		}
		ui.Text(c, label).Bold().TextColor(color)
	})
	if row.Clicked() {
		fn()
	}
}

func (v *View) subRow(c *ui.Context, p palette, show model.Podcast) {
	on := v.App.Selected != nil && v.App.Selected.ID == show.ID && !v.home && !v.expanded
	row := ui.ButtonBase(c.Key("sub:"+show.ID)).FillWidth().Radius(12).Padding(6, 8).Gap(10)
	if on {
		row.Background(p.accent.Alpha(0.16))
	} else if row.Hovered() {
		row.Background(p.cardHover)
	}
	row.Children(func() {
		v.cover(c, p, show.Image, show.Title, 36, 8)
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, show.Title).SingleLine().FontSize(13)
			if show.Author != "" {
				ui.Text(c, show.Author).SingleLine().FontSize(11).TextColor(p.muted)
			}
		})
	})
	if row.Clicked() {
		v.show(show)
	}
}

func (v *View) recentRow(c *ui.Context, p palette, st player.State, item recentPlay) {
	playing := st.Episode != nil && st.Episode.ID == item.Episode.ID && st.Playing
	row := ui.ButtonBase(c.Key("recent:"+item.Episode.ID)).FillWidth().Radius(10).Padding(6, 8)
	if row.Hovered() {
		row.Background(p.cardHover)
	}
	row.Children(func() {
		ui.Text(c, item.Episode.Title).SingleLine().FontSize(12)
		label := item.Show.Title
		if playing {
			label = "正在播放"
		}
		ui.Text(c, label).SingleLine().FontSize(11).TextColor(p.muted)
	})
	if row.Clicked() {
		v.show(item.Show)
		v.play(item.Show, item.Episode)
	}
}
