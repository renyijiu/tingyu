package ui

import (
	_ "embed"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

//go:embed nebula.kage
var nebulaSource string

var (
	nebula = shader.MustCompile(nebulaSource)

	nebulaTime   = nebula.Uniform("Time")
	nebulaEnergy = nebula.Uniform("Energy")
	nebulaBase   = nebula.Uniform("Base")
	nebulaTint   = nebula.Uniform("Tint")
	nebulaViolet = nebula.Uniform("Violet")
	nebulaCyan   = nebula.Uniform("Cyan")
	nebulaLight  = nebula.Uniform("Light")
)

// nebulaScale is how much smaller than the window the clouds are drawn:
// they are soft, so stretching them back up costs nothing to the eye and
// saves the GPU most of its work.
const nebulaScale = 5

// cosmos is the sky behind the window. Its clouds drift only while
// something plays, so a paused window draws nothing at all, and the time
// they have drifted carries over from one play to the next.
type cosmos struct {
	drift   float64   // seconds drifted before since
	since   time.Time // when the current drift began, while flowing
	flowing bool

	canvas *ui.Canvas
	values *shader.Values
	drawn  struct {
		t, energy float32
		w, h      int
		tint      ui.Color
		light     bool
	}
	stars []star
}

type star struct{ x, y, size, phase, speed, bright float32 }

// flow starts or stops the drift at now.
func (s *cosmos) flow(on bool, now time.Time) {
	if on == s.flowing {
		return
	}
	if on {
		s.since = now
	} else {
		s.drift += now.Sub(s.since).Seconds()
	}
	s.flowing = on
}

func (s *cosmos) at(now time.Time) float64 {
	if s.flowing {
		return s.drift + now.Sub(s.since).Seconds()
	}
	return s.drift
}

// paint draws the sky in r: the clouds, then the stars over them.
func (s *cosmos) paint(p *ui.Painter, r ui.Rect, pal palette, tint ui.Color, energy float32) {
	if !p.Accelerated() {
		// Without a GPU (and in tests) a gradient stands in for the clouds.
		p.Fill(r, pal.void, 0)
		p.FillGradient(r, ui.LinearGradient{
			Angle: 150, Oklab: true,
			Stops: []ui.GradientStop{
				{Color: tint.Alpha(0.22), At: 0},
				{Color: pal.accent.Alpha(0.08), At: 0.55},
				{Color: pal.void.Alpha(0), At: 1},
			},
		}, 0)
		return
	}
	now := p.Now()
	t := float32(math.Mod(s.at(now), 10000))
	w := max(int(math.Ceil(float64(r.W)/nebulaScale)), 8)
	h := max(int(math.Ceil(float64(r.H)/nebulaScale)), 8)
	if s.canvas == nil || s.drawn.w != w || s.drawn.h != h {
		s.canvas = ui.NewCanvas(w, h)
		s.drawn.w, s.drawn.h = w, h
		s.drawn.t = -1
	}
	if s.values == nil {
		s.values = nebula.NewValues()
	}
	if s.drawn.t != t || s.drawn.energy != energy || s.drawn.tint != tint || s.drawn.light != !pal.dark {
		s.drawn.t, s.drawn.energy, s.drawn.tint, s.drawn.light = t, energy, tint, !pal.dark
		light := float32(0)
		if !pal.dark {
			light = 1
		}
		s.values.Float(nebulaTime, t).Float(nebulaEnergy, energy).
			Color(nebulaBase, pal.void).Color(nebulaTint, tint).
			Color(nebulaViolet, pal.accent).Color(nebulaCyan, pal.cyan).
			Float(nebulaLight, light)
		s.canvas.Draw(p, func(cp *ui.Painter) {
			nebula.Draw(cp, ui.Rect{W: float32(w), H: float32(h)}, shader.Options{Values: s.values})
		})
	}
	p.Image(s.canvas.Bitmap(), r, ui.FillBox)
	if pal.dark {
		s.paintStars(p, r, now)
	}
	if s.flowing {
		p.AnimationFrameRate(30)
	}
}

// paintStars scatters the same stars at every size of the window, each
// twinkling at a pace of its own while the sky drifts.
func (s *cosmos) paintStars(p *ui.Painter, r ui.Rect, now time.Time) {
	if s.stars == nil {
		seed := uint32(0x7f4a7c15)
		next := func() float32 {
			seed ^= seed << 13
			seed ^= seed >> 17
			seed ^= seed << 5
			return float32(seed%10000) / 10000
		}
		for i := 0; i < 140; i++ {
			s.stars = append(s.stars, star{
				x: next(), y: next(),
				size:   0.5 + 1.3*next()*next(),
				phase:  next() * 6.28,
				speed:  0.6 + 1.8*next(),
				bright: 0.25 + 0.6*next(),
			})
		}
	}
	t := float32(s.at(now))
	for _, st := range s.stars {
		a := st.bright * (0.65 + 0.35*float32(math.Sin(float64(st.phase+t*st.speed))))
		x, y := r.X+st.x*r.W, r.Y+st.y*r.H
		if st.size > 1.3 {
			p.Fill(ui.Rect{X: x - st.size*2, Y: y - st.size*2, W: st.size * 4, H: st.size * 4}, ui.RGBA(200, 210, 255, a*0.12), st.size*2)
		}
		p.Fill(ui.Rect{X: x - st.size/2, Y: y - st.size/2, W: st.size, H: st.size}, ui.RGBA(255, 255, 255, a), st.size/2)
	}
}
