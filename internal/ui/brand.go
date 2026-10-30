package ui

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// The mark of 听宇: a planet with a ring around it, the ring's far side
// behind the planet and its near side in front, and a star above.
// The sidebar draws it small; Icon draws the app's icon from it.

// ringPath is the part of a tilted ellipse around (cx, cy) from angle a0
// to a1 (radians).
func ringPath(cx, cy, rx, ry, tilt, a0, a1 float32) *ui.Path {
	var path ui.Path
	const steps = 96
	sin, cos := float32(math.Sin(float64(tilt))), float32(math.Cos(float64(tilt)))
	for i := 0; i <= steps; i++ {
		a := a0 + (a1-a0)*float32(i)/steps
		x := rx * float32(math.Cos(float64(a)))
		y := ry * float32(math.Sin(float64(a)))
		px, py := cx+x*cos-y*sin, cy+x*sin+y*cos
		if i == 0 {
			path.MoveTo(px, py)
		} else {
			path.LineTo(px, py)
		}
	}
	return &path
}

// drawMark paints the mark in the square r.
func drawMark(p *ui.Painter, r ui.Rect, pal palette) {
	s := min(r.W, r.H)
	cx, cy := r.X+r.W/2, r.Y+r.H/2+s*0.02
	planet := s * 0.25
	rx, ry, tilt := s*0.47, s*0.15, float32(-0.42)
	width := max(s*0.042, 1.4)
	ring := ui.LinearGradient{Angle: 90, Oklab: true, Stops: []ui.GradientStop{
		{Color: pal.cyan, At: 0}, {Color: pal.accent.Mix(pal.rose, 0.4), At: 1},
	}}

	// The far side of the ring, behind the planet.
	p.StrokePathGradient(ringPath(cx, cy, rx, ry, tilt, math.Pi, 2*math.Pi), width, ring)

	// The planet: lit from the top left, its night side falling into
	// violet, with a soft shine where the light is.
	box := ui.Rect{X: cx - planet, Y: cy - planet, W: 2 * planet, H: 2 * planet}
	var body ui.Path
	body.Circle(cx, cy, planet)
	p.FillPathGradient(&body, ui.LinearGradient{Angle: 145, Oklab: true, Stops: []ui.GradientStop{
		{Color: ui.Hex("#d9ceff"), At: 0},
		{Color: ui.Hex("#9a7dff"), At: 0.42},
		{Color: ui.Hex("#5b3fd9"), At: 0.75},
		{Color: ui.Hex("#22125e"), At: 1},
	}})
	p.Clip(box, planet, func() {
		const steps = 16
		for i := 0; i < steps; i++ {
			k := float32(i) / steps
			d := planet * (1.2 - k*1.05)
			p.Fill(ui.Rect{X: cx - planet*0.38 - d/2, Y: cy - planet*0.42 - d/2, W: d, H: d}, ui.RGBA(255, 255, 255, 0.022), d/2)
		}
	})

	// The near side, in front.
	p.StrokePathGradient(ringPath(cx, cy, rx, ry, tilt, 0, math.Pi), width, ring)

	// A star, four-pointed, at the top right.
	sx, sy, sr := r.X+r.W*0.82, r.Y+r.H*0.16, s*0.09
	var star ui.Path
	star.MoveTo(sx, sy-sr).
		QuadTo(sx, sy, sx+sr, sy).
		QuadTo(sx, sy, sx, sy+sr).
		QuadTo(sx, sy, sx-sr, sy).
		QuadTo(sx, sy, sx, sy-sr).Close()
	p.FillPath(&star, ui.RGBA(255, 255, 255, 0.95))
}

// Icon draws the app's icon at size×size pixels: the mark on a night sky
// in a rounded square, on the grid macOS icons share (a body of 824 in
// 1024, with room for its shadow).
func Icon(c *ui.Context) {
	w, _ := c.Size()
	k := w / 1024
	pal := palette{
		dark:   true,
		accent: ui.Hex("#8b6dff"),
		cyan:   ui.Hex("#3fd5f0"),
		rose:   ui.Hex("#f472b6"),
	}
	inset, body, radius := 100*k, 824*k, 186*k
	root := c.Root()
	root.Background(ui.Transparent)
	tile := ui.Box(c).Absolute().Left(inset).Top(inset).Size(body, body).Radius(radius).Clip().
		Shadow(0, 10*k, 28*k, 0, ui.RGBA(0, 0, 0, 0.35))
	tile.LinearGradient(ui.LinearGradient{Angle: 160, Oklab: true, Stops: []ui.GradientStop{
		{Color: ui.Hex("#1c1147"), At: 0},
		{Color: ui.Hex("#0b0820"), At: 0.6},
		{Color: ui.Hex("#05040b"), At: 1},
	}})
	tile.Children(func() {
		// Nebulae: soft clouds of violet, cyan and rose.
		for _, cloud := range []struct {
			x, y, d float32
			col     ui.Color
			a       float32
		}{
			{520, -80, 560, pal.accent, 0.55},
			{-120, 470, 520, pal.cyan, 0.32},
			{430, 520, 420, pal.rose, 0.28},
		} {
			ui.Box(c).Absolute().Left(cloud.x*k).Top(cloud.y*k).Size(cloud.d*k, cloud.d*k).
				Radius(cloud.d * k / 2).Background(cloud.col.Alpha(cloud.a)).Blur(110 * k)
		}
		ui.Box(c).Absolute().Left(0).Top(0).Size(body, body).Draw(func(p *ui.Painter, r ui.Rect) {
			seed := uint32(0x9e3779b9)
			next := func() float32 {
				seed ^= seed << 13
				seed ^= seed >> 17
				seed ^= seed << 5
				return float32(seed%10000) / 10000
			}
			for i := 0; i < 70; i++ {
				x, y := r.X+next()*r.W, r.Y+next()*r.H
				d := (1.5 + 4*next()*next()) * k
				p.Fill(ui.Rect{X: x - d/2, Y: y - d/2, W: d, H: d}, ui.RGBA(255, 255, 255, 0.25+0.55*next()), d/2)
			}
		})
		// The planet's glow, then the mark.
		ui.Box(c).Absolute().Left(body/2-230*k).Top(body/2-220*k).Size(460*k, 460*k).
			Radius(230 * k).Background(pal.accent.Alpha(0.45)).Blur(70 * k)
		ui.Box(c).Absolute().Left(body*0.09).Top(body*0.09).Size(body*0.82, body*0.82).Draw(func(p *ui.Painter, r ui.Rect) {
			drawMark(p, r, pal)
		})
		// A hairline of light along the top edge, as glass catches it.
		ui.Box(c).Absolute().Left(0).Top(0).Size(body, body).Radius(radius).
			InsetShadow(0, 2*k, 2*k, 0, ui.RGBA(255, 255, 255, 0.16))
	})
}
