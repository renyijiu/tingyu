package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseChineseFeed(t *testing.T) {
	f, err := os.Open("testdata/show.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	show, eps, err := Parse(f, "https://example.com/feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	if show.Title != "故事会" || show.Author != "主播甲" || show.Description != "中文播客" {
		t.Fatalf("show %+v", show)
	}
	if show.Image != "https://example.com/cover.jpg" {
		t.Fatalf("image %q", show.Image)
	}
	if len(eps) != 3 {
		t.Fatalf("episodes: %+v", eps)
	}
	if eps[0].Title != "较新的一集" {
		t.Fatalf("order: %s then %s", eps[0].Title, eps[1].Title)
	}
	if eps[0].Description != "你好&世界" {
		t.Fatalf("description %q", eps[0].Description)
	}
	if eps[0].Duration != 12*time.Minute+34*time.Second {
		t.Fatalf("duration %v", eps[0].Duration)
	}
	if eps[0].ID != "https://example.com/feed.xml|new" {
		t.Fatalf("id %q", eps[0].ID)
	}
	if eps[1].Title != "较早的一集" || eps[1].AudioURL != "https://cdn.example.com/old.mp3" {
		t.Fatalf("picked non-audio enclosure: %+v", eps[1])
	}
	if eps[1].Duration != time.Hour+2*time.Minute+3*time.Second {
		t.Fatalf("hms %v", eps[1].Duration)
	}
	if eps[2].Duration != 125*time.Second {
		t.Fatalf("seconds %v", eps[2].Duration)
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"90", 90 * time.Second, true},
		{"12:34", 12*time.Minute + 34*time.Second, true},
		{"1:02:03", time.Hour + 2*time.Minute + 3*time.Second, true},
		{"", 0, false},
		{"nope", 0, false},
	}
	for _, tc := range cases {
		got, ok := ParseDuration(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%q -> %v %v, want %v %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestFetchUsesFixtureServer(t *testing.T) {
	body, err := os.ReadFile("testdata/show.xml")
	if err != nil {
		t.Fatal(err)
	}
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	res, err := (&Fetcher{HTTP: srv.Client()}).Fetch(context.Background(), srv.URL, Validators{})
	if err != nil {
		t.Fatal(err)
	}
	if ua == "" {
		t.Fatal("missing user agent")
	}
	if len(res.Episodes) != 3 || res.Show.Description != "中文播客" {
		t.Fatalf("%d episodes, show %+v", len(res.Episodes), res.Show)
	}
}

func TestFetchAsksWhetherTheFeedChanged(t *testing.T) {
	body, err := os.ReadFile("testdata/show.xml")
	if err != nil {
		t.Fatal(err)
	}
	const etag, modified = `"v1"`, "Wed, 28 Oct 2026 10:00:00 GMT"
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.Header.Get("If-None-Match")+"|"+r.Header.Get("If-Modified-Since"))
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", modified)
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	f := &Fetcher{HTTP: srv.Client()}

	first, err := f.Fetch(context.Background(), srv.URL, Validators{})
	if err != nil || first.NotModified || len(first.Episodes) != 3 {
		t.Fatalf("first fetch %v %+v", err, first)
	}
	if first.Validators != (Validators{ETag: etag, LastModified: modified}) {
		t.Fatalf("validators %+v", first.Validators)
	}
	again, err := f.Fetch(context.Background(), srv.URL, first.Validators)
	if err != nil || !again.NotModified || len(again.Episodes) != 0 || again.Validators != first.Validators {
		t.Fatalf("second fetch %v %+v", err, again)
	}
	if asked[0] != "|" || asked[1] != etag+"|"+modified {
		t.Fatalf("conditional headers %q", asked)
	}
}

func TestFetchRefusesAFeedOverTheLimit(t *testing.T) {
	body, err := os.ReadFile("testdata/show.xml")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	_, err = (&Fetcher{HTTP: srv.Client(), Limit: int64(len(body)) / 2}).Fetch(context.Background(), srv.URL, Validators{})
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("got %v, want a size error", err)
	}
	if _, err := (&Fetcher{HTTP: srv.Client(), Limit: int64(len(body))}).Fetch(context.Background(), srv.URL, Validators{}); err != nil {
		t.Fatalf("a feed right at the limit: %v", err)
	}
}
