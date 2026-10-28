package ui

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/version"
)

// artCache downloads cover images once and keeps a dominant color for the wash.
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
	col := dominant(img)
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

func dominant(img image.Image) ui.Color {
	b := img.Bounds()
	if b.Empty() {
		return ui.Hex("#6d4aff")
	}
	stepX := max(b.Dx()/20, 1)
	stepY := max(b.Dy()/20, 1)
	var r, g, bl, n uint64
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			r += uint64(cr)
			g += uint64(cg)
			bl += uint64(cb)
			n++
		}
	}
	if n == 0 {
		return ui.Hex("#6d4aff")
	}
	return ui.RGBA(uint8(r/n>>8), uint8(g/n>>8), uint8(bl/n>>8), 1)
}

// cover draws a rounded cover, or a letter tile while the image is on its way.
func (v *View) cover(c *ui.Context, p palette, url, title string, size float32, radius float32) {
	if bmp, _, ok := v.coverBitmap(url); ok && bmp != nil {
		ui.Image(c, bmp).Size(size, size).Fit(ui.Cover).Radius(radius).Clip()
		return
	}
	tile := ui.Column(c).Size(size, size).Radius(radius).Background(p.accent.Alpha(0.85)).Center()
	tile.Children(func() {
		ui.Text(c, initial(title)).FontSize(size * 0.38).Bold().TextColor(p.accentText)
	})
}
