package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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
	_, eps, err := (&Fetcher{HTTP: srv.Client()}).Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if ua == "" {
		t.Fatal("missing user agent")
	}
	if len(eps) != 3 {
		t.Fatalf("%d episodes", len(eps))
	}
}
