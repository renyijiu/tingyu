package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/player"
)

// nowMatch is the playing cover, which flies between the dock and the
// player.
var nowMatch = coverMatch{"now"}

// status floats what goes wrong with playback, and buffering, above the
// dock, without taking room from the page.
func (v *View) status(c *ui.Context, p palette, st player.State) {
	type line struct {
		text   string
		glyph  *ui.SVG
		color  ui.Color
		urgent bool
	}
	var lines []line
	if st.Err != nil {
		lines = append(lines, line{app.Explain(st.Err, "无法播放这一集"), iconAlert, p.danger, true})
	} else if st.Buffering {
		lines = append(lines, line{"正在缓冲…", iconWave, p.cyan, false})
	}
	if st.SaveErr != nil {
		lines = append(lines, line{"无法保存播放进度：" + st.SaveErr.Error(), iconAlert, p.danger, true})
	}
	ui.Column(c).Absolute().Left(0).Right(0).Bottom(100).AlignItems(ui.Center).Gap(8).PassThrough().Children(func() {
		for _, l := range lines {
			t := ui.Row(c.Key(l.text)).Gap(8).Padding(9, 16, 9, 12).Radius(999).MaxWidth(560).
				Material(glass.Glass{Tint: l.color.Alpha(0.18)}).Shadow(0, 10, 30, 0, p.shadow)
			t.Transition(ui.ElementTransition{Animation: &pop, Enter: &ui.Motion{Y: 14, Opacity: 0, Scale: 0.95}, Exit: &ui.Motion{Y: 8, Opacity: 0}})
			t.Children(func() {
				ui.Icon(c, l.glyph).FontSize(15).TextColor(l.color)
				ui.Text(c, l.text).FontSize(12).Bold().TextColor(p.text).MaxLines(2)
			})
		}
	})
}

// dock is the player along the bottom: a capsule of glass with what plays,
// its controls and its progress.
func (v *View) dock(c *ui.Context, p palette, st player.State, reduce bool) {
	off := st.Episode == nil
	d := ui.Row(c.Key("dock")).Height(76).Margin(10, 10, 10, 10).Radius(26).Padding(0, 14, 0, 12).Gap(14)
	d.Material(glass.Glass{}).Shadow(0, 14, 40, -10, p.shadow)
	d.Transition(ui.ElementTransition{Animation: &rise, Enter: &ui.Motion{Y: 50, Opacity: 0}, Exit: &ui.Motion{Y: 50, Opacity: 0}})
	d.Children(func() {
		open := ui.ButtonBase(c.Key("dock-open")).Width(250).Gap(12).Radius(16).Padding(5, 6).FocusRing(!off)
		if open.Hovered() && !off {
			open.Background(p.panel)
		}
		open.Children(func() {
			if off {
				ui.Box(c).Size(50, 50).Radius(14).Center().Border(1, p.rim).BorderStyle(ui.BorderDashed).Children(func() {
					ui.Icon(c, iconHeadset).FontSize(22).TextColor(p.faint)
				})
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "没有正在播放的单集").SingleLine().FontSize(13).Bold().TextColor(p.muted)
					ui.Text(c, "挑一集，开始收听").SingleLine().FontSize(11).TextColor(p.faint)
				})
				return
			}
			image, showTitle := v.playingArt(st)
			ui.Box(c).Size(50, 50).Children(func() {
				v.cover(c, p, image, st.Episode.Title, 50, 13, nowMatch)
				if st.Playing {
					ui.Box(c).Absolute().Right(-3).Bottom(-3).Size(20, 20).Radius(10).Center().
						LinearGradient(p.aurora(135)).Shadow(0, 2, 6, 0, p.shadow).Children(func() {
						equalizer(c, "eq", p.accentText, true, reduce, 10, 9)
					})
				}
			})
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, st.Episode.Title).SingleLine().FontSize(13).Bold()
				if showTitle != "" {
					ui.Text(c, showTitle).SingleLine().FontSize(11).TextColor(p.muted)
				}
			})
		})
		if open.Clicked() && !off {
			v.expanded = true
			v.home = false
		}
		ui.Row(c).Gap(6).Children(func() {
			if skipButton(c, p, "back15", -15, 36, off).Clicked() {
				v.App.Player.Skip(-15 * time.Second)
			}
			v.playButton(c, p, st, "transport", 46)
			if skipButton(c, p, "fwd30", 30, 36, off).Clicked() {
				v.App.Player.Skip(30 * time.Second)
			}
		})
		ui.Row(c).Grow(1).Gap(8).MinWidth(180).Children(func() {
			clock(c, p, st.Position, false)
			v.scrubber(c, p, st, "seek")
			clock(c, p, st.Duration, false)
		})
		v.speedChip(c, p, st, off)
		if iconButton(c, p, "expand", iconUp, "展开播放页", 34, off).Clicked() {
			v.expanded = true
			v.home = false
		}
	})
}

func clock(c *ui.Context, p palette, d time.Duration, large bool) {
	size := float32(11)
	if large {
		size = 12
	}
	ui.Text(c, formatClock(d)).FontSize(size).FontFeatures("tnum").TextColor(p.muted).MinWidth(34).TextAlign(ui.Center)
}

// speedChip shows the speed and steps through the speeds on a click.
func (v *View) speedChip(c *ui.Context, p palette, st player.State, off bool) {
	b := ui.ButtonBase(c.Key("speed")).Height(28).Padding(0, 10).Radius(999).Label("播放速度").Disabled(off)
	bg := p.panel
	if b.Hovered() && !off {
		bg = p.panelHi
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond})).Border(1, p.rim)
	b.Children(func() {
		color := p.text
		if off {
			color = p.faint
		}
		ui.Text(c.Key(speedLabel(st.Speed)), speedLabel(st.Speed)).FontSize(12).Bold().FontFeatures("tnum").TextColor(color).
			Transition(ui.ElementTransition{Animation: &pop, Enter: &ui.Motion{Y: -8, Opacity: 0}, Exit: &ui.Motion{Y: 8, Opacity: 0}})
	})
	if b.Clicked() {
		speeds := []float64{0.8, 1, 1.25, 1.5, 2}
		next := speeds[0]
		for i, s := range speeds {
			if nearly(st.Speed, s) {
				next = speeds[(i+1)%len(speeds)]
			}
		}
		v.App.Player.SetSpeed(next)
	}
}

// nowPlaying is the player, full size: the cover as a planet, its progress
// an orbit around it that a moon of light travels, and a halo of sound
// rising off it while it plays.
func (v *View) nowPlaying(c *ui.Context, p palette, st player.State, reduce bool) {
	ep := st.Episode
	image, showTitle := v.playingArt(st)
	tint := v.tintOf(p, image, showTitle)
	v.topBar(c, func() {
		if iconButton(c, p, "collapse", iconDown, "收起", 34, false).Clicked() {
			v.expanded = false
		}
		ui.Spacer(c)
		overline(c, p, iconOrbit, "NOW PLAYING")
		ui.Spacer(c)
		ui.Box(c).Width(34)
	})
	_, h := c.Size()
	planet := min(max((h-400)*0.75, 150), 280)
	ui.Column(c).Grow(1).FillWidth().Center().Gap(18).Padding(0, 36, 28, 36).Children(func() {
		orbit := planet + 76
		ui.Box(c).Size(orbit, orbit).Center().Draw(func(pt *ui.Painter, r ui.Rect) {
			_, live := v.App.Player.Level()
			drawHalo(pt, r, p, tint, planet, st, reduce, live, v.loudness(pt, reduce))
		}).Children(func() {
			art := ui.Box(c).Radius(planet/2).Shadow(0, 20, 60, 0, tint.Alpha(0.45))
			breathe(c, art, st.Playing, reduce)
			art.Children(func() {
				v.cover(c, p, image, ep.Title, planet, planet/2, nowMatch)
			})
		})
		ui.Column(c).Gap(6).AlignItems(ui.Center).MaxWidth(620).Children(func() {
			ui.Text(c, ep.Title).FontSize(24).Bold().MaxLines(2).TextAlign(ui.Center).LetterSpacing(-0.2)
			if showTitle != "" {
				ui.Text(c, showTitle).FontSize(14).Bold().TextColor(tint.Mix(p.text, 0.45))
			}
		})
		ui.Row(c).Gap(12).FillWidth().MaxWidth(620).Children(func() {
			clock(c, p, st.Position, true)
			v.scrubber(c, p, st, "seek-now")
			left := st.Duration - st.Position
			ui.Text(c, "-"+formatClock(left)).FontSize(12).FontFeatures("tnum").TextColor(p.muted).MinWidth(40).TextAlign(ui.Center)
		})
		ui.Row(c).Gap(22).Children(func() {
			if skipButton(c, p, "back15", -15, 50, false).Clicked() {
				v.App.Player.Skip(-15 * time.Second)
			}
			v.playButton(c, p, st, "transport-now", 74)
			if skipButton(c, p, "fwd30", 30, 50, false).Clicked() {
				v.App.Player.Skip(30 * time.Second)
			}
		})
		v.speeds(c, p, st)
	})
}

// drawHalo draws around the planet: a halo of bars rising and falling with
// the voice (live, at its loudness level; otherwise as a voice would), the
// orbit of the episode's progress, and its moon.
func drawHalo(pt *ui.Painter, r ui.Rect, p palette, tint ui.Color, planet float32, st player.State, reduce, live bool, level float32) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	moving := st.Playing && !reduce && pt.Accelerated()
	t := 0.0
	if moving {
		t = seconds(pt.Now())
		pt.AnimationFrameRate(60)
	}
	// The halo: bars around the planet, their lengths a few slow waves
	// folded together, as tall as the voice is loud.
	gain := 1.0
	if live {
		gain = 0.3 + 1.5*float64(level)
	}
	const bars = 96
	inner := planet/2 + 10
	for i := 0; i < bars; i++ {
		a := float64(i) / bars * 2 * math.Pi
		w := math.Sin(a*3+t*2.1)*0.5 + math.Sin(a*5-t*3.3)*0.3 + math.Sin(a*11+t*5.7)*0.2
		lv := min((0.25+0.75*math.Abs(w))*gain, 1.6)
		if !st.Playing {
			lv *= 0.35
		}
		l := float32(lv) * 14
		x0, y0 := cx+inner*float32(math.Cos(a)), cy+inner*float32(math.Sin(a))
		x1, y1 := cx+(inner+l)*float32(math.Cos(a)), cy+(inner+l)*float32(math.Sin(a))
		col := tint.Mix(p.cyan, float32(0.5+0.5*math.Sin(a*2+t*0.6))*0.5).Alpha(min(0.25+0.5*float32(lv), 0.9))
		pt.Line(x0, y0, x1, y1, 2, col)
	}
	// The orbit, and the part of it the episode has come.
	orbit := planet/2 + 32
	var ring ui.Path
	ring.Circle(cx, cy, orbit)
	pt.StrokePath(ring.Dash(0, 2, 5), 1.2, p.text.Alpha(0.16))
	frac := float32(0)
	if st.Duration > 0 {
		frac = float32(min(st.Position.Seconds()/st.Duration.Seconds(), 1))
	}
	start := float32(-math.Pi / 2)
	end := start + 2*math.Pi*frac
	if frac > 0 {
		pt.StrokePathConicGradient(arcPath(cx, cy, orbit, start, end), 2.4, ui.ConicGradient{
			Center: ui.Point{X: cx, Y: cy}, Oklab: true,
			Stops: []ui.GradientStop{{Color: p.accent, At: 0}, {Color: p.cyan, At: 0.5}, {Color: p.rose, At: 1}},
		})
	}
	// The moon, where the episode has got to.
	mx, my := cx+orbit*float32(math.Cos(float64(end))), cy+orbit*float32(math.Sin(float64(end)))
	dot(pt, mx, my, 13, p.cyan.Alpha(0.12))
	dot(pt, mx, my, 7, p.cyan.Alpha(0.3))
	dot(pt, mx, my, 4, ui.Hex("#ffffff"))
}

// speeds is the row of playback speeds, a pill of the aurora sliding to
// the one chosen.
func (v *View) speeds(c *ui.Context, p palette, st player.State) {
	type speedMatch struct{}
	ui.Row(c).Gap(2).Padding(4).Radius(999).Background(p.panel).Border(1, p.rim).Children(func() {
		for _, speed := range []float64{0.8, 1, 1.25, 1.5, 2} {
			label := speedLabel(speed)
			on := nearly(st.Speed, speed)
			btn := ui.ButtonBase(c.Key(label)).Height(28).Padding(0, 13).Radius(999)
			fg := p.muted
			if on {
				fg = p.accentText
			} else if btn.Hovered() {
				fg = p.text
			}
			btn.Children(func() {
				if on {
					ui.Box(c.Key("pill")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Radius(999).
						LinearGradient(p.aurora(120)).Shadow(0, 4, 12, 0, p.accent.Alpha(0.35)).
						Transition(ui.ElementTransition{Animation: &pop, Match: speedMatch{}})
				}
				// The label in the flow sizes the button; the one shown is
				// placed as the pill is, so it paints over it.
				ui.Text(c, label).FontSize(12).Bold().FontFeatures("tnum").Invisible()
				ui.Text(c, label).FontSize(12).Bold().FontFeatures("tnum").TextColor(fg).Attach(ui.AnchorCenter, ui.AnchorCenter)
			})
			if btn.Clicked() {
				v.App.Player.SetSpeed(speed)
			}
		}
	})
}

func speedLabel(speed float64) string {
	switch {
	case nearly(speed, 0.8):
		return "0.8×"
	case nearly(speed, 1.25):
		return "1.25×"
	case nearly(speed, 1.5):
		return "1.5×"
	case nearly(speed, 2):
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
