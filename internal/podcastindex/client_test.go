package podcastindex

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/renyijiu/tingyu/internal/version"
)

func TestAuthHeader(t *testing.T) {
	const key, secret = "UXKCGDSYGUUEVQJSYDZH", "yzJe2eE7XV-3eY576dyRZ6wXyAbndh6LUrCZ8KN|"
	const unix int64 = 1613713388
	got := AuthHeader(key, secret, unix)
	sum := sha1.Sum([]byte(key + secret + strconv.FormatInt(unix, 10)))
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if got != stringsToLower(got) {
		t.Fatalf("hash must be lowercase hex: %s", got)
	}
	if len(got) != 40 {
		t.Fatalf("sha1 hex length: %d", len(got))
	}
}

func stringsToLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func TestSearchSignsRequestAndParsesChinese(t *testing.T) {
	body, err := os.ReadFile("testdata/search.json")
	if err != nil {
		t.Fatal(err)
	}
	var gotUA, gotKey, gotDate, gotAuth, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotKey = r.Header.Get("X-Auth-Key")
		gotDate = r.Header.Get("X-Auth-Date")
		gotAuth = r.Header.Get("Authorization")
		gotQuery = r.URL.Query().Get("q")
		if r.URL.Path != "/api/1.0/search/byterm" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	when := time.Unix(1_700_000_000, 0)
	c := &Client{
		Key: "test-key", Secret: "test-secret", BaseURL: srv.URL + "/api/1.0",
		HTTP: srv.Client(), Now: func() time.Time { return when },
	}
	shows, err := c.Search(context.Background(), "故事 FM")
	if err != nil {
		t.Fatal(err)
	}
	if gotUA != version.UserAgent() {
		t.Fatalf("user agent %q", gotUA)
	}
	if gotKey != "test-key" || gotDate != "1700000000" {
		t.Fatalf("auth headers key=%q date=%q", gotKey, gotDate)
	}
	if gotAuth != AuthHeader("test-key", "test-secret", when.Unix()) {
		t.Fatalf("authorization %q", gotAuth)
	}
	if gotQuery != "故事 FM" {
		t.Fatalf("query %q", gotQuery)
	}
	if len(shows) != 2 {
		t.Fatalf("shows: %+v", shows)
	}
	if shows[0].Title != "故事 FM" || shows[0].FeedID != 75075 || shows[0].FeedURL != "https://example.com/gushi.xml" {
		t.Fatalf("first show: %+v", shows[0])
	}
	if shows[0].Image != "https://example.com/art.jpg" || shows[0].ITunesID != 1001 {
		t.Fatalf("image/itunes: %+v", shows[0])
	}
	if shows[0].ID == "" || shows[1].Author != "Owner" || shows[1].Image != "https://example.com/only-image.jpg" {
		t.Fatalf("second show: %+v", shows[1])
	}
}

func TestEpisodesBooleanStatus(t *testing.T) {
	body, err := os.ReadFile("testdata/episodes.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "75075" {
			t.Errorf("id %s", r.URL.Query().Get("id"))
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	c := &Client{Key: "k", Secret: "s", BaseURL: srv.URL, HTTP: srv.Client(), Now: func() time.Time { return time.Unix(10, 0) }}
	eps, err := c.Episodes(context.Background(), 75075)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 {
		t.Fatalf("expected the enclosure item only, got %+v", eps)
	}
	if eps[0].Title != "第一集" || eps[0].GUID != "ep-1" || eps[0].Duration != 3723*time.Second {
		t.Fatalf("%+v", eps[0])
	}
	if eps[0].AudioURL != "https://cdn.example.com/ep1.mp3" {
		t.Fatalf("audio %q", eps[0].AudioURL)
	}
	if eps[0].Published.Unix() != 1672671845 {
		t.Fatalf("published %v", eps[0].Published)
	}
}

func TestStatusFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"false","description":"Not authorized"}`))
	}))
	defer srv.Close()
	c := &Client{Key: "k", Secret: "s", BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := c.Search(context.Background(), "serial")
	if err == nil || !stringsContains(err.Error(), "Not authorized") {
		t.Fatalf("err %v", err)
	}
}

func TestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()
	c := &Client{Key: "k", Secret: "s", BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := c.Search(context.Background(), "x")
	if err == nil || !stringsContains(err.Error(), "401") {
		t.Fatalf("err %v", err)
	}
}

func stringsContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || len(s) > 0 && contains(s, sub)))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
