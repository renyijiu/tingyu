package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/model"
)

// suggestions are searches to start from, for a sky with nothing in it yet.
var suggestions = []string{"故事FM", "忽左忽右", "随机波动", "日谈公园", "Serial", "The Daily"}

func (v *View) discover(c *ui.Context, p palette, reduce bool) {
	v.topBar(c, func() {})
	// With results to show, the head steps back to make room for them.
	busy := v.App.Searching || len(v.App.Results) > 0
	v.App.LoadChart()
	ui.Column(c).Grow(1).FillWidth().Padding(0, 36, 0, 36).Gap(16).Children(func() {
		ui.Column(c.Key("head")).Gap(6).Transition(ui.ElementTransition{Animation: &glide, Size: true}).Children(func() {
			if !busy {
				overline(c, p, iconSparkle, "DISCOVER")
			}
			ui.Text(c, "发现").FontSize(40).Bold().LetterSpacing(-0.5).Hidden(busy)
			if !busy {
				ui.Text(c, "在声音的宇宙里，找到下一档常听的播客").FontSize(14).TextColor(p.muted)
			}
		})
		v.searchBar(c, p)
		if v.App.Notice != "" && !busy {
			v.notice(c, p)
		}
		if v.App.SearchErr != "" {
			ui.Row(c).Gap(8).Children(func() {
				ui.Icon(c, iconAlert).FontSize(15).TextColor(p.danger)
				ui.Text(c, v.App.SearchErr).TextColor(p.danger)
			})
		}
		switch {
		case v.App.Searching:
			v.skeletons(c, p, reduce)
		case len(v.App.Results) == 0 && len(v.App.Chart) > 0:
			v.discoverChart(c, p, reduce)
		case len(v.App.Results) == 0:
			v.discoverEmpty(c, p, reduce)
		default:
			v.results(c, p, reduce)
		}
	})
}

// searchBar is a capsule of glass with the field in it; focused it is
// ringed with the aurora, and while it searches the ring turns.
func (v *View) searchBar(c *ui.Context, p palette) {
	bar := ui.Row(c.Key("searchbar")).Height(56).Radius(28).Padding(0, 6, 0, 20).Gap(12).MaxWidth(720)
	focused := bar.FocusWithin()
	bg := p.panel
	if focused || bar.Hovered() {
		bg = p.panelHi
	}
	bar.Background(bar.AnimateColor("bg", bg, ui.Animation{Duration: 160 * time.Millisecond}))
	switch {
	case v.App.Searching:
		turn := bar.Loop("turn", 1400*time.Millisecond, ui.Linear)
		bar.Border(1.5, ui.Color{}).BorderConicGradient(ui.ConicGradient{Stops: p.halo(), Angle: turn * 360, Oklab: true})
		bar.Shadow(0, 0, 26, 0, p.accent.Alpha(0.35))
	case focused:
		bar.Border(1.5, ui.Color{}).BorderLinearGradient(p.aurora(0))
		bar.Shadow(0, 10, 30, 0, p.accent.Alpha(0.22))
	default:
		bar.Border(1, p.rim).Shadow(0, 10, 30, 0, p.shadow.Alpha(0.12))
	}
	bar.Children(func() {
		color := p.muted
		if focused {
			color = p.accent.Mix(p.cyan, 0.3)
		}
		ui.Icon(c, iconSearch).FontSize(19).TextColor(color)
		field := ui.TextInputBase(c.Key("search"), &v.App.Query).
			Placeholder("搜索播客，中文或英文").
			Label("搜索播客").
			Grow(1).FontSize(15)
		if field.Submitted() {
			v.search()
		}
		if v.App.Query != "" {
			if iconButton(c, p, "clear", iconClose, "清除", 30, false).Clicked() {
				v.App.Query = ""
			}
		}
		btn := ui.ButtonBase(c.Key("go")).Height(44).Padding(0, 22).Radius(22).Gap(6)
		btn.LinearGradient(p.aurora(120)).InsetShadow(0, 1, 1, 0, ui.RGBA(255, 255, 255, 0.3))
		scale := float32(1)
		if btn.Pressed() {
			scale = 0.95
		} else if btn.Hovered() {
			scale = 1.04
		}
		btn.AnimatePose(ui.Pose{Scale: scale}, pop)
		btn.Children(func() {
			ui.Text(c, "搜索").Bold().FontSize(14).TextColor(p.accentText)
		})
		if btn.Clicked() {
			v.search()
		}
	})
}

func (v *View) search() {
	v.searches++
	v.App.Search()
}

// notice says which catalog searches, and opens the settings to change it.
func (v *View) notice(c *ui.Context, p palette) {
	n := ui.ButtonBase(c.Key("notice")).AlignSelf(ui.Start).Gap(8).Padding(6, 12, 6, 10).Radius(999).Background(p.warn.Alpha(0.1)).Border(1, p.warn.Alpha(0.22))
	if n.Hovered() {
		n.Background(p.warn.Alpha(0.16))
	}
	n.Children(func() {
		ui.Icon(c, iconSignal).FontSize(14).TextColor(p.warn)
		ui.Text(c, v.App.Notice).FontSize(12).TextColor(p.text.Mix(p.warn, 0.35))
		ui.Icon(c, iconRight).FontSize(13).TextColor(p.warn)
	})
	if n.Clicked() {
		v.OpenSettings()
	}
}

// discoverEmpty is a little solar system and the searches it suggests.
func (v *View) discoverEmpty(c *ui.Context, p palette, reduce bool) {
	ui.Column(c).Grow(1).Center().Gap(14).Padding(0, 0, 40, 0).Children(func() {
		ui.Box(c).Size(260, 150).Draw(func(pt *ui.Painter, r ui.Rect) { drawOrbits(pt, r, p, reduce) })
		ui.Text(c, "宇宙很大，从一个关键词开始").FontSize(20).Bold()
		ui.Text(c, "封面会从这里铺开。试试：").FontSize(13).TextColor(p.muted)
		ui.Row(c).Gap(8).Wrap().Justify(ui.Center).MaxWidth(520).Children(func() { v.suggestionChips(c, p) })
	})
}

// suggestionChips are searches to start from, one click each.
func (v *View) suggestionChips(c *ui.Context, p palette) {
	for _, s := range suggestions {
		b := ui.ButtonBase(c.Key("try:"+s)).Padding(7, 14).Radius(999).Border(1, p.rim)
		bg := p.panel
		if b.Hovered() {
			bg = p.panelHi
			b.Border(1, p.accent.Alpha(0.45))
		}
		b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
		b.Children(func() { ui.Text(c, s).FontSize(13) })
		if b.Clicked() {
			v.App.Query = s
			v.search()
		}
	}
}

// discoverChart is what most listen to now, ranked, under the searches to
// start from.
func (v *View) discoverChart(c *ui.Context, p palette, reduce bool) {
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Text(c, "试试：").FontSize(13).TextColor(p.muted)
		v.suggestionChips(c, p)
	})
	ui.Row(c).Gap(10).Padding(6, 0, 0, 0).Children(func() {
		ui.Text(c, "大家在听").FontSize(17).Bold()
		ui.Text(c, itoa(len(v.App.Chart))+" 档").FontSize(12).FontFeatures("tnum").TextColor(p.muted)
		ui.Spacer(c)
		chip(c, p, iconSignal, "Apple 播客 · 中国区榜单", p.cyan)
	})
	v.showGrid(c, p, &v.chartGrid, v.App.Chart, true, reduce)
}

// drawOrbits draws three planets on tilted orbits around a sun of the
// aurora, turning while the window has a GPU to turn them with.
func drawOrbits(pt *ui.Painter, r ui.Rect, p palette, reduce bool) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	moving := !reduce && pt.Accelerated()
	t := 0.0
	if moving {
		t = seconds(pt.Now())
		pt.AnimationFrameRate(30)
	}
	orbits := []struct {
		rx, ry, speed, size float32
		col                 ui.Color
	}{
		{56, 20, 0.7, 7, p.cyan},
		{92, 34, 0.42, 10, p.rose},
		{126, 50, 0.26, 8, p.accent.Mix(p.cyan, 0.5)},
	}
	for i, o := range orbits {
		pt.StrokePath(ringPath(cx, cy, o.rx, o.ry, -0.18, 0, 2*math.Pi), 1, p.text.Alpha(0.1))
		a := float64(i)*2.1 + t*float64(o.speed)
		sin, cos := float32(math.Sin(-0.18)), float32(math.Cos(-0.18))
		x, y := o.rx*float32(math.Cos(a)), o.ry*float32(math.Sin(a))
		px, py := cx+x*cos-y*sin, cy+x*sin+y*cos
		pt.Fill(ui.Rect{X: px - o.size*1.6, Y: py - o.size*1.6, W: o.size * 3.2, H: o.size * 3.2}, o.col.Alpha(0.16), o.size*1.6)
		pt.Fill(ui.Rect{X: px - o.size/2, Y: py - o.size/2, W: o.size, H: o.size}, o.col, o.size/2)
	}
	for _, g := range []struct{ d, a float32 }{{70, 0.1}, {46, 0.18}} {
		pt.Fill(ui.Rect{X: cx - g.d/2, Y: cy - g.d/2, W: g.d, H: g.d}, p.accent.Alpha(g.a), g.d/2)
	}
	var sun ui.Path
	sun.Circle(cx, cy, 15)
	pt.FillPathGradient(&sun, p.aurora(135))
}

func (v *View) skeletons(c *ui.Context, p palette, reduce bool) {
	ui.Row(c).Gap(8).Children(func() {
		equalizer(c, "searching", p.accent.Mix(p.cyan, 0.4), true, reduce, 14, 12)
		ui.Text(c, "正在搜索…").FontSize(13).TextColor(p.muted)
	})
	ui.Grid(c).Columns(4).Gap(16).Children(func() {
		for i := 0; i < 8; i++ {
			card := ui.Column(c.Key("sk:" + itoa(i))).Height(236).Radius(20).Background(p.panel).Padding(10).Gap(10).Clip()
			card.Children(func() {
				ui.Box(c).Height(150).Radius(14).Background(p.text.Alpha(0.06))
				ui.Box(c).Height(12).Width(110).Radius(6).Background(p.text.Alpha(0.06))
				ui.Box(c).Height(10).Width(70).Radius(5).Background(p.text.Alpha(0.05))
			})
			delay := float64(i%4) * 0.12
			card.DrawOver(func(pt *ui.Painter, r ui.Rect) {
				if reduce || !pt.Accelerated() {
					return
				}
				pt.AnimationFrame()
				k := float32(math.Mod(seconds(pt.Now())/1.4-delay, 1))
				x := r.X - r.W + k*r.W*3
				pt.Clip(r, 20, func() {
					pt.FillGradient(ui.Rect{X: x, Y: r.Y, W: r.W, H: r.H}, ui.LinearGradient{
						Angle: 100,
						Stops: []ui.GradientStop{{Color: p.text.Alpha(0), At: 0}, {Color: p.text.Alpha(0.07), At: 0.5}, {Color: p.text.Alpha(0), At: 1}},
					}, 0)
				})
			})
		}
	})
}

func (v *View) results(c *ui.Context, p palette, reduce bool) {
	ui.Row(c).Gap(10).Children(func() {
		ui.Text(c, "搜索结果").FontSize(17).Bold()
		ui.Text(c, itoa(len(v.App.Results))+" 档").FontSize(12).FontFeatures("tnum").TextColor(p.muted)
		ui.Spacer(c)
		chip(c, p, iconSignal, "来自 "+sourceName(v.App.ResultSource), p.cyan)
	})
	v.showGrid(c, p, &v.grid, v.App.Results, false, reduce)
}

// showGrid lays shows out as cards, numbered when ranked.
func (v *View) showGrid(c *ui.Context, p palette, state *ui.GridState, shows []model.Podcast, ranked, reduce bool) {
	// The covers fill their cards, so the rows are as high as the cards
	// are wide: work out how wide the grid makes them.
	const minWidth, gap = 168, 16
	w, _ := c.Size()
	inner := max(w-326, minWidth)
	cols := max(float32(int((inner+gap)/(minWidth+gap))), 1)
	art := (inner-gap*(cols-1))/cols - 20
	ui.GridView(c, state, len(shows), minWidth, art+74, func(i int) {
		rank := 0
		if ranked {
			rank = i + 1
		}
		v.resultCard(c, p, i, shows[i], art, rank, reduce)
	}).Gap(gap).Grow(1).Margin(0, -8).Padding(4, 8, 24, 8)
}

// resultCard is a show in the results or the chart, with its place on the
// chart when rank is set. Cards rise in one after another; under the
// pointer, one lifts toward the viewer and glows in its cover's color, and
// its cover is the one that flies to the show's page.
func (v *View) resultCard(c *ui.Context, p palette, i int, show model.Podcast, art float32, rank int, reduce bool) {
	card := ui.ButtonBase(c.Key("card:" + show.ID)).Fill().Padding(10).Radius(20).FocusRing(false)
	hot := card.Hovered() || card.Focused()
	tint := v.tintOf(p, show.Image, show.Title)
	delay := time.Duration(min(i, 11)) * 40 * time.Millisecond
	in := card.Keyframes("in", []ui.Keyframe{{At: 0, Value: 0}, {At: 520 * time.Millisecond, Value: 1, Ease: ui.EaseOut}},
		ui.KeyframeOptions{Run: v.searches, Delay: delay})
	card.Opacity(in).Translate(0, (1-in)*18)

	lift, scale, glow := float32(0), float32(1), float32(0)
	bg, rim := p.panel, p.rim
	if hot && !reduce {
		lift, scale, glow = -6, 1.03, 0.42
	}
	if hot {
		bg, rim = p.panelHi, tint.Alpha(0.45)
	}
	if card.Pressed() {
		scale = 0.98
	}
	card.AnimatePose(ui.Pose{Y: lift, Scale: scale}, pop)
	card.Background(card.AnimateColor("bg", bg, ui.Animation{Duration: 160 * time.Millisecond}))
	card.Border(1, card.AnimateColor("rim", rim, ui.Animation{Duration: 160 * time.Millisecond}))
	glow = card.Animate("glow", glow, 220*time.Millisecond)
	card.Shadow(0, 18, 40, -6, tint.Alpha(glow))
	card.Children(func() {
		ui.Column(c).Fill().Gap(9).Children(func() {
			var match any
			if hot {
				match = coverMatch{show.ID}
			}
			v.artwork(c, p, show.Image, show.Title, art, 14, match).FillWidth().AspectRatio(1)
			if rank > 0 {
				badge := ui.Row(c).Height(24).MinWidth(24).Padding(0, 7).Radius(12).Justify(ui.Center).
					Attach(ui.AnchorTopLeft, ui.AnchorTopLeft).Left(8).Top(8).PassThrough()
				if rank <= 3 {
					badge.LinearGradient(p.aurora(120))
				} else {
					badge.Background(ui.RGBA(10, 8, 22, 0.55)).Border(1, ui.RGBA(255, 255, 255, 0.18))
				}
				badge.Children(func() {
					ui.Text(c, itoa(rank)).FontSize(12).Bold().FontFeatures("tnum").TextColor(ui.RGBA(255, 255, 255, 0.96))
				})
			}
			ui.Column(c).Gap(2).Padding(0, 2).Children(func() {
				ui.Text(c, show.Title).SingleLine().Bold().FontSize(14)
				author := show.Author
				if author == "" {
					author = " "
				}
				ui.Text(c, author).SingleLine().FontSize(12).TextColor(p.muted)
			})
		})
	})
	if card.Clicked() {
		v.show(show)
	}
}
