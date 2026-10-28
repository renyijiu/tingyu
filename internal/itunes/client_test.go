package itunes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/renyijiu/tingyu/internal/version"
)

func TestSearchParsesAndSkipsNonPodcasts(t *testing.T) {
	body, err := os.ReadFile("testdata/search.json")
	if err != nil {
		t.Fatal(err)
	}
	var term, ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		term = r.URL.Query().Get("term")
		ua = r.Header.Get("User-Agent")
		if r.URL.Query().Get("media") != "podcast" || r.URL.Query().Get("entity") != "podcast" {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	shows, err := c.Search(context.Background(), "故事 FM")
	if err != nil {
		t.Fatal(err)
	}
	if term != "故事 FM" {
		t.Fatalf("term %q", term)
	}
	if ua != version.UserAgent() {
		t.Fatalf("ua %q", ua)
	}
	if len(shows) != 1 {
		t.Fatalf("%+v", shows)
	}
	if shows[0].Title != "故事 FM" || shows[0].ITunesID != 123456 || shows[0].FeedURL != "https://example.com/gushi.xml" {
		t.Fatalf("%+v", shows[0])
	}
	if shows[0].Image != "https://example.com/art600.jpg" {
		t.Fatalf("image %q", shows[0].Image)
	}
}

func TestEpisodes(t *testing.T) {
	body, err := os.ReadFile("testdata/lookup.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" || r.URL.Query().Get("id") != "123456" {
			t.Errorf("%s %s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	eps, err := c.Episodes(context.Background(), 123456)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 {
		t.Fatalf("%+v", eps)
	}
	if eps[0].Title != "英文小集" || eps[0].Duration != 90*time.Second || eps[0].GUID != "99" {
		t.Fatalf("%+v", eps[0])
	}
	if !eps[0].Published.Equal(time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("published %v", eps[0].Published)
	}
}

func TestLiveSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("live iTunes search")
	}
	c := &Client{}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for _, term := range []string{"serial", "故事FM"} {
		shows, err := c.Search(ctx, term)
		if err != nil {
			t.Skipf("iTunes unreachable for %q: %v", term, err)
		}
		if len(shows) == 0 {
			t.Fatalf("iTunes returned no podcasts for %q", term)
		}
		if shows[0].Title == "" {
			t.Fatalf("empty title for %q: %+v", term, shows[0])
		}
		t.Logf("%q -> %s", term, shows[0].Title)
	}
}
