package ui

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
)

func (v *View) sidebar(c *ui.Context, p palette, st player.State, reduce bool) {
	// The panel sits below the window controls rather than under them, and
	// keeps the same 10pt clear of the window's bottom edge as of its left.
	// The strip above it is the title bar, so it drags the window.
	bar := c.TitleBar()
	ui.Column(c).Padding(0, 0, 10, 10).Children(func() {
		ui.Box(c).Height(max(bar.Height, 4) + 6).FillWidth().DragWindow()
		v.sidebarPanel(c, p, st, reduce)
	})
}

func (v *View) sidebarPanel(c *ui.Context, p palette, st player.State, reduce bool) {
	panel := ui.Column(c).Width(244).Grow(1).Radius(22).Padding(18, 10, 10, 10).Gap(2)
	panel.Material(glass.Glass{Clarity: 0.6})
	panel.Children(func() {
		v.brand(c, p)
		discover := v.home || (v.App.Selected == nil && !v.expanded)
		if v.navRow(c, p, "discover", iconDiscover, "发现", discover, nil).Clicked() {
			v.home = true
			v.expanded = false
		}
		if st.Episode != nil {
			now := v.navRow(c, p, "now", iconOrbit, "正在播放", v.expanded, func() {
				equalizer(c, "now-eq", p.accent.Mix(p.cyan, 0.4), st.Playing, reduce, 14, 12)
			})
			if now.Clicked() {
				v.expanded = true
				v.home = false
			}
		}
		ui.Scroll(c.Key("library")).Grow(1).FillWidth().Margin(8, 0, 0, 0).Children(func() {
			ui.Column(c).FillWidth().Gap(2).Padding(0, 0, 8, 0).Children(func() {
				v.section(c, p, iconLibrary, "我的订阅", len(v.App.Subs))
				if len(v.App.Subs) == 0 {
					v.libraryEmpty(c, p)
				}
				for _, show := range v.App.Subs {
					v.subRow(c, p, show)
				}
				if len(v.App.Recent) > 0 {
					v.section(c, p, iconHistory, "最近播放", 0)
					for _, item := range v.App.Recent {
						v.recentRow(c, p, st, item, reduce)
					}
				}
			})
		})
		ui.Box(c).Height(1).FillWidth().Margin(4, 6).Background(p.rim)
		v.settingsEntry(c, p)
	})
}

// brand is the mark and the name, at the top of the sidebar, which drags
// the window as its title bar would.
func (v *View) brand(c *ui.Context, p palette) {
	ui.Row(c).Gap(10).Padding(2, 8, 14, 8).DragWindow().Children(func() {
		ui.Box(c).Size(34, 34).Draw(func(pt *ui.Painter, r ui.Rect) { drawMark(pt, r, p) })
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, "听宇").FontSize(19).Bold().LetterSpacing(1)
			ui.Text(c, "TINGYU · 听见宇宙").FontSize(9).LetterSpacing(1.6).TextColor(p.faint)
		})
	})
}

// navRow is a row of the sidebar: chosen, it is lit and carries the mark
// that slides from row to row.
func (v *View) navRow(c *ui.Context, p palette, key string, glyph *ui.SVG, label string, on bool, trailing func()) ui.Element {
	row := ui.ButtonBase(c.Key("nav:"+key)).FillWidth().Height(36).Radius(11).Padding(0, 10).Gap(11)
	v.rowLight(row, p, on)
	row.Children(func() {
		if on {
			v.navMark(c, p)
		}
		color := p.muted
		if on {
			color = p.text
		}
		ui.Icon(c, glyph).FontSize(17).TextColor(color)
		ui.Text(c, label).FontSize(13).Bold().TextColor(color).Grow(1)
		if trailing != nil {
			trailing()
		}
	})
	return row
}

// rowLight fills a row under the pointer, and a chosen one with a wash of
// the aurora.
func (v *View) rowLight(row ui.Element, p palette, on bool) {
	bg := p.panelHi.Alpha(0)
	switch {
	case on:
		row.LinearGradient(ui.LinearGradient{Angle: 90, Oklab: true, From: p.accent.Alpha(0.26), To: p.cyan.Alpha(0.06)})
	case row.Pressed():
		bg = p.panelHi
	case row.Hovered():
		bg = p.panel
	}
	if !on {
		row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	}
}

// navMark is the glowing bar along the chosen row's edge. There is one at
// a time, and it flies to the row chosen next.
func (v *View) navMark(c *ui.Context, p palette) {
	ui.Box(c.Key("mark")).Absolute().Left(-10).Top(9).Bottom(9).Width(3).Radius(2).
		LinearGradient(ui.LinearGradient{Angle: 180, From: p.cyan, To: p.accent}).
		Shadow(0, 0, 10, 0, p.accent.Alpha(0.8)).
		Transition(ui.ElementTransition{Animation: &glide, Match: navMatch{}})
}

func (v *View) section(c *ui.Context, p palette, glyph *ui.SVG, title string, count int) {
	ui.Row(c).Gap(6).Padding(14, 10, 6, 10).Children(func() {
		ui.Icon(c, glyph).FontSize(12).TextColor(p.faint)
		ui.Text(c, title).FontSize(11).Bold().LetterSpacing(1).TextColor(p.faint).Grow(1)
		if count > 0 {
			ui.Text(c, itoa(count)).FontSize(10).Bold().FontFeatures("tnum").TextColor(p.faint).
				Padding(1, 6).Radius(999).Background(p.panel)
		}
	})
}

func (v *View) libraryEmpty(c *ui.Context, p palette) {
	ui.Column(c).Margin(2, 4).Padding(14, 12).Gap(4).Radius(14).Border(1, p.rim).BorderStyle(ui.BorderDashed).Children(func() {
		ui.Text(c, "还没有订阅").FontSize(12).Bold().TextColor(p.muted)
		ui.Text(c, "在「发现」里找到喜欢的节目，订阅后会出现在这里").FontSize(11).TextColor(p.faint)
	})
}

func (v *View) subRow(c *ui.Context, p palette, show model.Podcast) {
	on := v.onShow() && v.App.Selected.ID == show.ID
	row := ui.ButtonBase(c.Key("sub:"+show.ID)).FillWidth().Radius(12).Padding(5, 8).Gap(10)
	v.rowLight(row, p, on)
	var match any
	if (row.Hovered() || row.Focused()) && !on {
		match = coverMatch{show.ID}
	}
	row.Children(func() {
		if on {
			v.navMark(c, p)
		}
		v.cover(c, p, show.Image, show.Title, 34, 9, match)
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, show.Title).SingleLine().FontSize(13).Bold()
			if show.Author != "" {
				ui.Text(c, show.Author).SingleLine().FontSize(11).TextColor(p.muted)
			}
		})
	})
	if row.Clicked() {
		v.show(show)
	}
}

func (v *View) recentRow(c *ui.Context, p palette, st player.State, item model.Play, reduce bool) {
	current := st.Episode != nil && st.Episode.ID == item.Episode.ID
	row := ui.ButtonBase(c.Key("recent:"+item.Episode.ID)).FillWidth().Radius(12).Padding(6, 10).Gap(10)
	v.rowLight(row, p, false)
	row.Children(func() {
		ui.Box(c).Size(14, 14).Center().Children(func() {
			if current {
				equalizer(c, "eq", p.accent.Mix(p.cyan, 0.4), st.Playing, reduce, 12, 11)
			} else {
				ui.Box(c).Size(5, 5).Radius(3).Background(p.faint)
			}
		})
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			title := ui.Text(c, item.Episode.Title).SingleLine().FontSize(12)
			if current {
				title.Bold()
			}
			label := item.Show.Title
			if current && st.Playing {
				label = "正在播放"
			}
			ui.Text(c, label).SingleLine().FontSize(11).TextColor(p.faint)
		})
	})
	if row.Clicked() {
		v.show(item.Show)
		v.play(item.Show, item.Episode)
	}
}
