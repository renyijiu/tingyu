package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
)

const (
	offline = "网络连接失败，请检查网络后重试"
	slow    = "连接超时，请检查网络后重试"
)

// closedAddr is an address nothing listens on.
func closedAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func TestExplainNetworkFailures(t *testing.T) {
	addr := closedAddr(t)
	_, refused := http.Get("http://" + addr + "/search")

	proxied := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: addr})}}
	_, viaProxy := proxied.Get("https://itunes.apple.com/search?term=Serial")

	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer hang.Close()
	_, timedOut := (&http.Client{Timeout: 50 * time.Millisecond}).Get(hang.URL)

	unknown := &url.Error{Op: "Get", URL: "https://feeds.example/rss", Err: &net.DNSError{Err: "no such host", Name: "feeds.example", IsNotFound: true}}

	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"refused", refused, offline},
		{"proxy refused", viaProxy, offline},
		{"no such host", unknown, offline},
		{"timeout", timedOut, slow},
	} {
		if got := Explain(tc.err, "失败"); got != tc.want {
			t.Errorf("%s: Explain(%v) = %q, want %q", tc.name, tc.err, got, tc.want)
		}
	}
}

func TestExplainKeepsNoticesAndHidesTheRest(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{player.Notice("这一集没有音频地址"), "这一集没有音频地址"},
		{fmt.Errorf("%w: %w", player.Notice("无法解码音频"), errors.New("audio: unknown format")), "无法解码音频"},
		{errors.New("feed: HTTP 404"), "失败"},
		{errors.New("feed: XML syntax error on line 1"), "失败"},
	} {
		if got := Explain(tc.err, "失败"); got != tc.want {
			t.Errorf("Explain(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestSearchAndOpenShowPlainErrors(t *testing.T) {
	down := &url.Error{Op: "Get", URL: "https://itunes.apple.com/search?term=x", Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}}
	a := New(nil, &catalog.Catalog{Index: searchIndexFunc(func(context.Context, string) ([]model.Podcast, error) {
		return nil, down
	})}, nil)
	a.Sync = true
	a.Query = "x"
	a.Search()
	if a.SearchErr != offline {
		t.Fatalf("SearchErr = %q, want %q", a.SearchErr, offline)
	}

	a.Open(model.Podcast{Title: "无处可读"})
	if a.EpErr != "无法读取这个节目的单集" || strings.Contains(a.EpErr, "节目源") {
		t.Fatalf("EpErr = %q", a.EpErr)
	}
}
