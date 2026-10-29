package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/player"
)

// The controls 听宇 draws itself, on ui's widgets without a look.

// roundButton is a round button without content, named by label, and the
// color its content takes.
func roundButton(c *ui.Context, p palette, key, label string, size float32, off bool) (ui.Element, ui.Color) {
	btn := ui.ButtonBase(c.Key(key)).Label(label).Size(size, size).Radius(size / 2).Center().Disabled(off)
	scale, color, bg := float32(1), p.text, p.panelHi.Alpha(0)
	switch {
	case off:
		color = p.faint
	case btn.Pressed():
		scale, bg = 0.9, p.panelHi
	case btn.Hovered():
		scale, bg = 1.06, p.panelHi
	}
	btn.Background(btn.AnimateColor("bg", bg, ui.Animation{Duration: 140 * time.Millisecond}))
	btn.AnimatePose(ui.Pose{Scale: scale}, pop)
	return btn, color
}

// iconButton is a round button showing an icon, named by label.
func iconButton(c *ui.Context, p palette, key string, glyph *ui.SVG, label string, size float32, off bool) ui.Element {
	btn, color := roundButton(c, p, key, label, size, off)
	btn.Children(func() {
		ui.Icon(c, glyph).FontSize(size * 0.5).TextColor(color)
	})
	return btn
}

// skipButton is the arrow around a number of seconds, as Apple's players
// draw it.
func skipButton(c *ui.Context, p palette, key string, delta int, size float32, off bool) ui.Element {
	glyph, label, n := iconForward, "前进 30 秒", delta
	if delta < 0 {
		glyph, label, n = iconBack, "后退 15 秒", -delta
	}
	btn, color := roundButton(c, p, key, label, size, off)
	btn.Children(func() {
		ui.Icon(c, glyph).FontSize(size * 0.62).TextColor(color)
		ui.Text(c, itoa(n)).FontSize(size*0.2).Bold().TextColor(color).FontFeatures("tnum").
			Attach(ui.AnchorCenter, ui.AnchorCenter).Top(size * 0.03).PassThrough()
	})
	return btn
}

// playButton is the round button of the aurora that plays and pauses. Its
// glyph turns into the other one, and a ring spins around it while the
// episode buffers.
func (v *View) playButton(c *ui.Context, p palette, st player.State, key string, size float32) {
	label, glyph, name := "播放", iconPlay, "play"
	if st.Playing {
		label, glyph, name = "暂停", iconPause, "pause"
	}
	off := st.Episode == nil
	btn := ui.ButtonBase(c.Key(key)).Label(label).Size(size, size).Radius(size / 2).Disabled(off || st.Buffering)
	btn.LinearGradient(p.aurora(135))
	scale, glow := float32(1), float32(0.32)
	switch {
	case off:
		btn.Opacity(0.35)
		glow = 0
	case btn.Pressed():
		scale, glow = 0.92, 0.5
	case btn.Hovered():
		scale, glow = 1.07, 0.6
	}
	glow = btn.Animate("glow", glow, 200*time.Millisecond)
	btn.Shadow(0, size*0.16, size*0.55, 0, p.accent.Alpha(glow))
	btn.InsetShadow(0, 1, 1, 0, ui.RGBA(255, 255, 255, 0.35))
	btn.AnimatePose(ui.Pose{Scale: scale}, pop)
	btn.Children(func() {
		g := ui.Icon(c.Key(name), glyph).FontSize(size*0.44).TextColor(p.accentText).
			Attach(ui.AnchorCenter, ui.AnchorCenter)
		if name == "play" {
			g.Left(size * 0.03) // the triangle's weight sits left of its box
		}
		g.Transition(ui.ElementTransition{
			Animation: &pop,
			Enter:     &ui.Motion{Scale: 0.3, Opacity: 0, Rotate: -70},
			Exit:      &ui.Motion{Scale: 0.3, Opacity: 0, Rotate: 70},
		})
	})
	if st.Buffering {
		btn.DrawOver(func(pt *ui.Painter, r ui.Rect) {
			turn := float32(0)
			if pt.Accelerated() {
				turn = float32(math.Mod(seconds(pt.Now())*1.4, 1))
				pt.AnimationFrame()
			}
			arc := arcPath(r.X+r.W/2, r.Y+r.H/2, r.W/2+4, turn*2*math.Pi, turn*2*math.Pi+math.Pi*1.2)
			pt.StrokePathGradient(arc, 2.5, p.aurora(90))
		})
	}
	if btn.Clicked() {
		v.App.Toggle()
	}
}

// arcPath is the arc of the circle around (cx, cy) from a0 to a1, radians
// clockwise from the right.
func arcPath(cx, cy, r, a0, a1 float32) *ui.Path {
	var path ui.Path
	steps := max(int(math.Abs(float64(a1-a0))*float64(r)/3), 12)
	for i := 0; i <= steps; i++ {
		a := float64(a0 + (a1-a0)*float32(i)/float32(steps))
		x, y := cx+r*float32(math.Cos(a)), cy+r*float32(math.Sin(a))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	return &path
}

// scrubber is the line of the episode's progress: the played part in the
// aurora, and a knob of light that grows under the pointer.
func (v *View) scrubber(c *ui.Context, p palette, st player.State, key string) ui.Element {
	hi := st.Duration.Seconds()
	if hi < 1 {
		hi = 1
	}
	if st.Episode == nil {
		v.seek = 0
	} else {
		v.seek = st.Position.Seconds()
	}
	const pad = 8
	s := ui.SliderBase(c.Key(key), &v.seek, 0, hi).Label("播放进度").Height(20).PaddingX(pad).Grow(1).MinWidth(120)
	off := st.Episode == nil
	s.Disabled(off)
	hot := float32(0)
	if !off && (s.Hovered() || s.Pressed() || s.Focused()) {
		hot = 1
	}
	hot = s.AnimateTo("hot", hot, pop)
	frac := float32(math.Min(v.seek/hi, 1))
	s.Draw(func(pt *ui.Painter, r ui.Rect) {
		x0, w := r.X+pad, r.W-2*pad
		th := 4 + 2*hot
		y := r.Y + r.H/2 - th/2
		track := p.text.Alpha(0.12)
		pt.Fill(ui.Rect{X: x0, Y: y, W: w, H: th}, track, th/2)
		if frac > 0 {
			pt.FillGradient(ui.Rect{X: x0, Y: y, W: max(w*frac, th), H: th}, p.aurora(0), th/2)
		}
		if off {
			return
		}
		x := x0 + w*frac
		d := 8 + 6*hot
		cy := r.Y + r.H/2
		pt.Fill(ui.Rect{X: x - d, Y: cy - d, W: 2 * d, H: 2 * d}, p.cyan.Alpha(0.18+0.12*hot), d)
		pt.Fill(ui.Rect{X: x - d/2, Y: cy - d/2, W: d, H: d}, ui.Hex("#ffffff"), d/2)
	})
	if s.Changed() {
		v.App.SeekTo(time.Duration(v.seek * float64(time.Second)))
	}
	return s
}

// equalizer is four bars dancing while something plays, at rest when not.
func equalizer(c *ui.Context, key string, color ui.Color, on, reduce bool, w, h float32) ui.Element {
	return ui.Box(c.Key(key)).Size(w, h).Draw(func(pt *ui.Painter, r ui.Rect) {
		const n = 4
		gap := r.W * 0.14
		bw := (r.W - gap*(n-1)) / n
		moving := on && !reduce && pt.Accelerated()
		t := seconds(pt.Now())
		for i := 0; i < n; i++ {
			lv := []float32{0.45, 0.8, 0.6, 0.35}[i]
			if moving {
				f := float64(i)
				lv = 0.22 + 0.78*float32(math.Abs(math.Sin(t*(2.6+f*0.55)+f*1.7)*math.Sin(t*(1.3+f*0.3)+f)))
			}
			bh := max(r.H*lv, bw)
			pt.Fill(ui.Rect{X: r.X + float32(i)*(bw+gap), Y: r.Y + r.H - bh, W: bw, H: bh}, color, bw/2)
		}
		if moving {
			pt.AnimationFrameRate(30)
		}
	})
}

// chip is a small pill of text, with an icon before it or not.
func chip(c *ui.Context, p palette, glyph *ui.SVG, text string, color ui.Color) ui.Element {
	e := ui.Row(c).Gap(5).Padding(4, 10).Radius(999).Background(color.Alpha(0.12)).Border(1, color.Alpha(0.18))
	e.Children(func() {
		if glyph != nil {
			ui.Icon(c, glyph).FontSize(12).TextColor(color)
		}
		ui.Text(c, text).FontSize(11).Bold().TextColor(color).SingleLine()
	})
	return e
}

// overline is the small spaced label above a title.
func overline(c *ui.Context, p palette, glyph *ui.SVG, text string) {
	ui.Row(c).Gap(6).Children(func() {
		if glyph != nil {
			ui.Icon(c, glyph).FontSize(12).TextColor(p.accent.Mix(p.cyan, 0.3))
		}
		ui.Text(c, text).FontSize(11).Bold().LetterSpacing(2.2).TextColor(p.accent.Mix(p.cyan, 0.3))
	})
}

// pill is a rounded button with an icon and a label: filled with the
// aurora when primary, or a quiet outline.
func pill(c *ui.Context, p palette, key string, glyph *ui.SVG, label string, primary bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Radius(999).Padding(0, 18, 0, 14).Height(40).Gap(8)
	fg := p.text
	scale := float32(1)
	if primary {
		fg = p.accentText
		btn.LinearGradient(p.aurora(120))
		glow := float32(0.3)
		if btn.Hovered() {
			glow = 0.55
		}
		btn.Shadow(0, 8, 22, 0, p.accent.Alpha(btn.Animate("glow", glow, 180*time.Millisecond)))
		btn.InsetShadow(0, 1, 1, 0, ui.RGBA(255, 255, 255, 0.3))
	} else {
		bg := p.panel
		if btn.Hovered() {
			bg = p.panelHi
		}
		btn.Background(btn.AnimateColor("bg", bg, ui.Animation{Duration: 140 * time.Millisecond})).Border(1, p.rim)
	}
	if btn.Pressed() {
		scale = 0.95
	} else if btn.Hovered() {
		scale = 1.03
	}
	btn.AnimatePose(ui.Pose{Scale: scale}, pop)
	btn.Children(func() {
		if glyph != nil {
			ui.Icon(c, glyph).FontSize(16).TextColor(fg)
		}
		ui.Text(c, label).Bold().FontSize(13).TextColor(fg).SingleLine()
	})
	return btn
}

// dot fills a circle of radius r at a point that moves from frame to
// frame. Fill rounds a rectangle to whole pixels to keep its edges crisp,
// so a small circle drawn with it on the move steps from pixel to pixel; a
// path is smoothed where it falls between them.
func dot(pt *ui.Painter, cx, cy, r float32, c ui.Color) {
	var circle ui.Path
	circle.Circle(cx, cy, r)
	pt.FillPath(&circle, c)
}

// sparkles bursts small stars out of r for a moment after at.
func sparkles(pt *ui.Painter, r ui.Rect, p palette, at time.Time) {
	if at.IsZero() || !pt.Accelerated() {
		return
	}
	const life = 900 * time.Millisecond
	age := pt.Now().Sub(at)
	if age < 0 || age > life {
		return
	}
	pt.AnimationFrame()
	k := float32(age) / float32(life)
	ease := 1 - (1-k)*(1-k)*(1-k)
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	colors := []ui.Color{p.accent, p.cyan, p.rose, ui.Hex("#ffffff")}
	for i := 0; i < 16; i++ {
		a := float64(i)/16*2*math.Pi + float64(i%3)*0.21
		dist := (r.W*0.5 + 10) + float32(28+(i*13)%30)*ease
		x := cx + dist*float32(math.Cos(a))
		y := cy + dist*float32(math.Sin(a))*0.7 - 10*ease
		s := (2.4 + float32(i%3)) * (1 - k)
		col := colors[i%len(colors)].Alpha(1 - k)
		var star ui.Path
		star.MoveTo(x, y-s*2).QuadTo(x, y, x+s*2, y).QuadTo(x, y, x, y+s*2).QuadTo(x, y, x-s*2, y).QuadTo(x, y, x, y-s*2).Close()
		pt.FillPath(&star, col)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
