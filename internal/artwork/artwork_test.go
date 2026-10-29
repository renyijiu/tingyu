package artwork

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func picture(w, h int, alpha uint8) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 200, A: alpha})
		}
	}
	return img
}

func server(t *testing.T, body []byte, hits *int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLoadShrinksAndKeepsTheCover(t *testing.T) {
	var src bytes.Buffer
	if err := jpeg.Encode(&src, picture(2000, 1500, 255), nil); err != nil {
		t.Fatal(err)
	}
	hits := 0
	srv := server(t, src.Bytes(), &hits)
	l := &Loader{Dir: t.TempDir(), HTTP: srv.Client()}

	img, err := l.Load(context.Background(), srv.URL+"/cover.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 640 || b.Dy() != 480 {
		t.Fatalf("size %v, want 640×480", b)
	}
	files, _ := filepath.Glob(filepath.Join(l.Dir, "*.jpg"))
	if len(files) != 1 {
		t.Fatalf("kept %v", files)
	}

	again, err := l.Load(context.Background(), srv.URL+"/cover.jpg")
	if err != nil || hits != 1 {
		t.Fatalf("second load: %v, %d downloads", err, hits)
	}
	if b := again.Bounds(); b.Dx() != 640 || b.Dy() != 480 {
		t.Fatalf("kept size %v", b)
	}
}

func TestLoadKeepsTransparencyAsPNG(t *testing.T) {
	var src bytes.Buffer
	if err := png.Encode(&src, picture(800, 1600, 128)); err != nil {
		t.Fatal(err)
	}
	hits := 0
	srv := server(t, src.Bytes(), &hits)
	l := &Loader{Dir: t.TempDir(), HTTP: srv.Client()}
	img, err := l.Load(context.Background(), srv.URL+"/logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 320 || b.Dy() != 640 {
		t.Fatalf("size %v, want 320×640", b)
	}
	if files, _ := filepath.Glob(filepath.Join(l.Dir, "*.png")); len(files) != 1 {
		t.Fatalf("kept %v", files)
	}
}

func TestLoadLeavesSmallCoversAndReportsFailures(t *testing.T) {
	var src bytes.Buffer
	if err := png.Encode(&src, picture(300, 300, 255)); err != nil {
		t.Fatal(err)
	}
	hits := 0
	srv := server(t, src.Bytes(), &hits)
	l := &Loader{HTTP: srv.Client()}
	img, err := l.Load(context.Background(), srv.URL+"/small.png")
	if err != nil || img.Bounds().Dx() != 300 {
		t.Fatalf("%v %v", err, img.Bounds())
	}
	if _, err := l.Load(context.Background(), srv.URL+"/missing"); err == nil {
		t.Fatal("a 404 loaded")
	}
}

func TestKeptCoverThatNoLongerDecodesIsFetchedAgain(t *testing.T) {
	var src bytes.Buffer
	if err := jpeg.Encode(&src, picture(100, 100, 255), nil); err != nil {
		t.Fatal(err)
	}
	hits := 0
	srv := server(t, src.Bytes(), &hits)
	l := &Loader{Dir: t.TempDir(), HTTP: srv.Client()}
	url := srv.URL + "/c.jpg"
	if err := os.WriteFile(l.path(url, ".jpg"), []byte("not a jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Load(context.Background(), url); err != nil || hits != 1 {
		t.Fatalf("%v, %d downloads", err, hits)
	}
}
