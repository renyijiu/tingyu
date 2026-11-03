package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestPruneRemovesWhatWasUsedLongestAgo(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := writeAudio(t, dir, "old"+audioExt, 40, now.Add(-3*time.Hour))
	mid := writeAudio(t, dir, "mid"+audioExt, 40, now.Add(-2*time.Hour))
	recent := writeAudio(t, dir, "recent"+audioExt, 40, now.Add(-time.Hour))
	other := writeAudio(t, dir, "feeds.json", 500, now.Add(-4*time.Hour))
	if err := Prune(dir, 90, ""); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{old: false, mid: true, recent: true, other: true} {
		if _, err := os.Stat(path); (err == nil) != want {
			t.Errorf("%s kept=%v, want %v", filepath.Base(path), err == nil, want)
		}
	}
}

func TestPruneKeepsTheFileJustPlayed(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	big := writeAudio(t, dir, "big"+audioExt, 200, now.Add(-time.Hour))
	small := writeAudio(t, dir, "small"+audioExt, 10, now)
	if err := Prune(dir, 100, big); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(big); err != nil {
		t.Errorf("file just played was removed: %v", err)
	}
	if _, err := os.Stat(small); err == nil {
		t.Errorf("older audio kept while over the limit")
	}
}

func TestReplayCountsAsUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("audio-bytes"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	url := srv.URL + "/a.mp3"
	path := writeAudio(t, dir, hashName(url), 11, time.Now().Add(-48*time.Hour))
	r, err := Open(context.Background(), srv.Client(), url, dir)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(st.ModTime()) > time.Hour {
		t.Errorf("replayed audio still looks unused: %v", st.ModTime())
	}
}

func TestTidyRemovesUnfinishedDownloads(t *testing.T) {
	dir := t.TempDir()
	part := writeAudio(t, dir, "abc"+audioExt+".part-123", 10, time.Now())
	done := writeAudio(t, dir, "abc"+audioExt, 10, time.Now())
	if err := os.MkdirAll(filepath.Join(dir, "covers"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Tidy(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(part); err == nil {
		t.Error("unfinished download left behind")
	}
	if _, err := os.Stat(done); err != nil {
		t.Errorf("finished audio removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "covers")); err != nil {
		t.Errorf("covers removed: %v", err)
	}
	if err := Tidy(filepath.Join(dir, "missing")); err != nil {
		t.Errorf("missing cache: %v", err)
	}
}

func writeAudio(t *testing.T, dir, name string, size int, used time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, used, used); err != nil {
		t.Fatal(err)
	}
	return path
}
