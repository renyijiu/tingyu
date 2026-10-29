// Package artwork downloads cover images, shrinks them to the largest size
// the window shows them at, and keeps the shrunk copies on disk.
package artwork

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif" // covers come as any of these
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/renyijiu/tingyu/internal/version"
)

// MaxSide is the longest side a cover keeps: twice the 280 points of the
// largest cover the window draws, for Retina screens. Feeds' covers are
// often 1400 to 3000 pixels, 8 to 36 MB each once decoded.
const MaxSide = 640

const (
	maxDownload = 16 << 20
	maxKept     = 1000
)

// Loader loads covers. With Dir empty, nothing is kept on disk.
type Loader struct {
	Dir  string
	HTTP *http.Client
}

// Load returns the cover at url, no larger than MaxSide on either side.
func (l *Loader) Load(ctx context.Context, url string) (image.Image, error) {
	if img, ok := l.kept(url); ok {
		return img, nil
	}
	img, err := l.download(ctx, url)
	if err != nil {
		return nil, err
	}
	img = Shrink(img, MaxSide)
	l.keep(url, img)
	return img, nil
}

func (l *Loader) download(ctx context.Context, url string) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.UserAgent())
	hc := l.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artwork: HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload))
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("artwork: %w", err)
	}
	return img, nil
}

// Shrink scales img down to fit side × side, keeping its shape. A smaller
// image comes back as it is.
func Shrink(img image.Image, side int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= side && h <= side {
		return img
	}
	if w >= h {
		w, h = side, max(h*side/w, 1)
	} else {
		w, h = max(w*side/h, 1), side
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

// path is where url's cover is kept: a JPEG, or a PNG for one that shows
// through somewhere.
func (l *Loader) path(url, ext string) string {
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(l.Dir, hex.EncodeToString(sum[:16])+ext)
}

func (l *Loader) kept(url string) (image.Image, bool) {
	if l.Dir == "" {
		return nil, false
	}
	for _, ext := range []string{".jpg", ".png"} {
		p := l.path(url, ext)
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			os.Remove(p)
			continue
		}
		now := time.Now()
		_ = os.Chtimes(p, now, now)
		return img, true
	}
	return nil, false
}

// keep writes the shrunk cover in one piece. A cover that cannot be kept
// is only downloaded again next time.
func (l *Loader) keep(url string, img image.Image) {
	if l.Dir == "" || os.MkdirAll(l.Dir, 0o755) != nil {
		return
	}
	var buf bytes.Buffer
	ext := ".jpg"
	if opaque(img) {
		if jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}) != nil {
			return
		}
	} else {
		ext = ".png"
		if png.Encode(&buf, img) != nil {
			return
		}
	}
	tmp, err := os.CreateTemp(l.Dir, "cover-*.tmp")
	if err != nil {
		return
	}
	_, werr := tmp.Write(buf.Bytes())
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), l.path(url, ext)) != nil {
		os.Remove(tmp.Name())
		return
	}
	l.prune()
}

func opaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}

// prune forgets the covers used longest ago beyond maxKept, about 80 MB.
func (l *Loader) prune() {
	entries, err := os.ReadDir(l.Dir)
	if err != nil || len(entries) <= maxKept {
		return
	}
	type file struct {
		name string
		used time.Time
	}
	var files []file
	for _, de := range entries {
		if ext := filepath.Ext(de.Name()); ext != ".jpg" && ext != ".png" {
			continue
		}
		if info, err := de.Info(); err == nil {
			files = append(files, file{de.Name(), info.ModTime()})
		}
	}
	if len(files) <= maxKept {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].used.After(files[j].used) })
	for _, f := range files[maxKept:] {
		os.Remove(filepath.Join(l.Dir, f.name))
	}
}
