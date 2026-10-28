package model

import (
	"testing"
	"time"
)

func TestPodcastIDPrefersFeedURL(t *testing.T) {
	a := PodcastID("https://Example.com/feed.xml", 9, 8, "标题")
	b := PodcastID("https://example.com/feed.xml", 1, 2, "其他")
	if a != b {
		t.Fatalf("feed URL should collapse sources: %q vs %q", a, b)
	}
	if got := PodcastID("", 42, 0, ""); got != "pi:42" {
		t.Fatalf("feed id: %q", got)
	}
	if got := PodcastID("", 0, 7, ""); got != "itunes:7" {
		t.Fatalf("itunes id: %q", got)
	}
}

func TestEpisodeKey(t *testing.T) {
	if got := EpisodeKey("https://ex/feed", "guid-1", "https://cdn/a.mp3"); got != "https://ex/feed|guid-1" {
		t.Fatalf("guid key: %q", got)
	}
	if got := EpisodeKey("", "", "https://cdn/a.mp3"); got != "audio:https://cdn/a.mp3" {
		t.Fatalf("audio key: %q", got)
	}
}

func TestResumeFrom(t *testing.T) {
	dur := 30 * time.Minute
	if got := ResumeFrom(Progress{Position: time.Minute, Duration: dur}); got != time.Minute {
		t.Fatalf("mid episode: %v", got)
	}
	if got := ResumeFrom(Progress{Position: dur - 3*time.Second, Duration: dur}); got != 0 {
		t.Fatalf("near end should restart, got %v", got)
	}
	if got := ResumeFrom(Progress{}); got != 0 {
		t.Fatalf("empty: %v", got)
	}
}
