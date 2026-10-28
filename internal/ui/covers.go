package ui

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/version"
)

// artCache downloads cover images once and keeps the color the sky takes
// from each.
type artCache struct {
	mu   sync.Mutex
	bmp  map[string]*ui.Bitmap
	col  map[string]ui.Color
	busy map[string]bool
	bad  map[string]bool
}

func (v *View) arts() *artCache {
	if v.art == nil {
		v.art = &artCache{
			bmp:  map[string]*ui.Bitmap{},
			col:  map[string]ui.Color{},
			busy: map[string]bool{},
			bad:  map[string]bool{},
		}
	}
	return v.art
}

func (v *View) coverBitmap(url string) (*ui.Bitmap, ui.Color, bool) {
	if url == "" {
		return nil, ui.Color{}, false
	}
	a := v.arts()
	a.mu.Lock()
	bmp, ok := a.bmp[url]
	col := a.col[url]
	busy, failed := a.busy[url], a.bad[url]
	if ok || busy || failed {
		a.mu.Unlock()
		return bmp, col, ok
	}
	a.busy[url] = true
	a.mu.Unlock()
	go v.fetchCover(url)
	return nil, ui.Color{}, false
}

func (v *View) fetchCover(url string) {
	a := v.arts()
	defer func() {
		a.mu.Lock()
		delete(a.busy, url)
		a.mu.Unlock()
		if v.App != nil && v.App.OnUI != nil {
			v.App.OnUI(func() {})
		}
	}()
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		v.markBad(url)
		return
	}
	req.Header.Set("User-Agent", version.UserAgent())
	resp, err := client.Do(req)
	if err != nil {
		v.markBad(url)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		v.markBad(url)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		v.markBad(url)
		return
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		v.markBad(url)
		return
	}
	bmp := ui.NewBitmap(img)
	col := vibrant(img)
	a.mu.Lock()
	a.bmp[url] = bmp
	a.col[url] = col
	a.mu.Unlock()
}

func (v *View) markBad(url string) {
	a := v.arts()
	a.mu.Lock()
	a.bad[url] = true
	a.mu.Unlock()
}

// vibrant picks the color a cover is remembered by: the most common hue
// among its lively pixels, rather than the average, which turns most
// covers to mud. A cover in grays keeps the brand's violet.
func vibrant(img image.Image) ui.Color {
	fallback := ui.Hex("#8b6dff")
	b := img.Bounds()
	if b.Empty() {
		return fallback
	}
	const bins = 24
	var weight [bins]float64
	var sumR, sumG, sumB [bins]float64
	stepX := max(b.Dx()/32, 1)
	stepY := max(b.Dy()/32, 1)
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			r, g, bl := float64(cr)/65535, float64(cg)/65535, float64(cb)/65535
			h, s, l := hsl(r, g, bl)
			if s < 0.22 || l < 0.12 || l > 0.9 {
				continue
			}
			w := s * (1 - math.Abs(l-0.55)*1.4)
			if w <= 0 {
				continue
			}
			i := int(h*bins) % bins
			weight[i] += w
			sumR[i] += r * w
			sumG[i] += g * w
			sumB[i] += bl * w
		}
	}
	best, total := 0, 0.0
	for i := range weight {
		total += weight[i]
		if weight[i] > weight[best] {
			best = i
		}
	}
	if weight[best] == 0 || weight[best] < total*0.04 {
		return fallback
	}
	r, g, bl := sumR[best]/weight[best], sumG[best]/weight[best], sumB[best]/weight[best]
	// Lift dark colors so the sky they light still glows.
	h, s, l := hsl(r, g, bl)
	l = math.Max(l, 0.5)
	s = math.Min(math.Max(s, 0.45), 0.9)
	r, g, bl = rgb(h, s, l)
	return ui.RGBA(uint8(r*255), uint8(g*255), uint8(bl*255), 1)
}

func hsl(r, g, b float64) (h, s, l float64) {
	hi := math.Max(r, math.Max(g, b))
	lo := math.Min(r, math.Min(g, b))
	l = (hi + lo) / 2
	if hi == lo {
		return 0, 0, l
	}
	d := hi - lo
	if l > 0.5 {
		s = d / (2 - hi - lo)
	} else {
		s = d / (hi + lo)
	}
	switch hi {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

func rgb(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		return l, l, l
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	return channel(p, q, h+1.0/3), channel(p, q, h), channel(p, q, h-1.0/3)
}

func channel(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	}
	return p
}

// tintOf is the color a show lights the sky with: its cover's once it has
// come, else a hue of its own.
func (v *View) tintOf(p palette, url, title string) ui.Color {
	if _, col, ok := v.coverBitmap(url); ok {
		return col
	}
	if url == "" && title == "" {
		return p.accent
	}
	return ui.Oklch(0.68, 0.16, hue(title))
}

// cover draws a cover of size, or a tile of the show's own gradient with
// its first letter while the image is on its way. match, when set, is the
// identity the cover flies with between pages.
func (v *View) cover(c *ui.Context, p palette, url, title string, size, radius float32, match any) ui.Element {
	return v.artwork(c, p, url, title, size, radius, match).Size(size, size)
}

// artwork is a cover without a size; letter is about how large it will show,
// for the placeholder's letter.
func (v *View) artwork(c *ui.Context, p palette, url, title string, letter, radius float32, match any) ui.Element {
	var e ui.Element
	if bmp, _, ok := v.coverBitmap(url); ok && bmp != nil {
		e = ui.Image(c, bmp).Fit(ui.Cover).Radius(radius).Clip()
	} else {
		h := hue(title)
		e = ui.Column(c).Radius(radius).Center().Clip().LinearGradient(ui.LinearGradient{
			Angle: 135, Oklab: true,
			Stops: []ui.GradientStop{
				{Color: ui.Oklch(0.62, 0.17, h), At: 0},
				{Color: ui.Oklch(0.42, 0.16, h+50), At: 1},
			},
		})
		e.Children(func() {
			ui.Text(c, initial(title)).FontSize(letter * 0.4).Bold().TextColor(ui.RGBA(255, 255, 255, 0.92))
		})
	}
	if match != nil {
		e.Transition(ui.ElementTransition{Animation: &glide, Match: match, Position: true, Size: true})
	}
	return e
}
