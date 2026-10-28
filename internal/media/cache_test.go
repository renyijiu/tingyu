package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestStreamBeforeDownloadFinishes(t *testing.T) {
	payload := []byte("0123456789abcdefghij")
	release := make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing user agent")
		}
		w.Header().Set("Content-Length", "20")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload[:4])
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-release
		_, _ = w.Write(payload[4:])
	}))
	defer srv.Close()
	t.Cleanup(func() { once.Do(func() { close(release) }) })

	r, err := Open(context.Background(), srv.Client(), srv.URL+"/ep.mp3", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "0123" {
		t.Fatalf("got %q", buf)
	}
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		t.Fatal(err)
	}
	if end != 20 {
		t.Fatalf("length %d", end)
	}
	if _, err := r.Seek(4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(rest) != string(payload[4:]) {
		t.Fatalf("rest %q", rest)
	}
}

func TestCompleteCacheIsReused(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte("audio-bytes"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	first, err := Open(context.Background(), srv.Client(), srv.URL+"/a.mp3", dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(context.Background(), srv.Client(), srv.URL+"/a.mp3", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	b, err := io.ReadAll(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(got) {
		t.Fatalf("cached %q", b)
	}
	if hits != 1 {
		t.Fatalf("cache miss, hits=%d", hits)
	}
}

func TestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	r, err := Open(context.Background(), srv.Client(), srv.URL, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	_, err = io.ReadAll(r)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCanceledDownloadIsNotCached(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var requests atomic.Int32
	payload := "0123456789abcdefghij"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "20")
		if requests.Add(1) == 1 {
			_, _ = io.WriteString(w, payload[:4])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		_, _ = io.WriteString(w, payload)
	}))
	defer func() {
		cancel()
		srv.Close()
	}()
	dir := t.TempDir()
	r, err := Open(ctx, srv.Client(), srv.URL, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := io.ReadFull(r, make([]byte, 4)); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := io.ReadAll(r); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled download returned %v, want context.Canceled", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("canceled download left cache files: %v", entries)
	}
	retry, err := Open(context.Background(), srv.Client(), srv.URL, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer retry.Close()
	got, err := io.ReadAll(retry)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != payload || requests.Load() != 2 {
		t.Errorf("retry reused partial audio: body=%q requests=%d", got, requests.Load())
	}
}

func TestHashStable(t *testing.T) {
	if hashName("https://cdn/a.mp3") != hashName("https://cdn/a.mp3") {
		t.Fatal("hash changed")
	}
	if hashName("https://cdn/a.mp3") == hashName("https://cdn/b.mp3") {
		t.Fatal("collision")
	}
}
