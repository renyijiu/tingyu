// Package model holds the podcast types shared by the API clients, storage and UI.
package model

import (
	"strings"
	"time"
)

// Source names where a podcast record came from.
const (
	SourcePodcastIndex = "podcastindex"
	SourceITunes       = "itunes"
	SourceRSS          = "rss"
	// SourceBoth names search results merged from Podcast Index and iTunes.
	SourceBoth = "podcastindex+itunes"
)

// Podcast is a show the user can subscribe to.
type Podcast struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Author       string    `json:"author,omitempty"`
	Description  string    `json:"description,omitempty"`
	Image        string    `json:"image,omitempty"`
	FeedURL      string    `json:"feedUrl,omitempty"`
	Link         string    `json:"link,omitempty"`
	FeedID       int64     `json:"feedId,omitempty"`
	ITunesID     int64     `json:"itunesId,omitempty"`
	Source       string    `json:"source,omitempty"`
	SubscribedAt time.Time `json:"subscribedAt,omitempty"`
}

// Episode is one playable item of a podcast.
type Episode struct {
	ID          string        `json:"id"`
	PodcastID   string        `json:"podcastId,omitempty"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	AudioURL    string        `json:"audioUrl"`
	MIME        string        `json:"mime,omitempty"`
	GUID        string        `json:"guid,omitempty"`
	Published   time.Time     `json:"published,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	Image       string        `json:"image,omitempty"`
}

// Play is an episode played, with its show, as the sidebar lists it under
// 最近播放.
type Play struct {
	Show    Podcast `json:"show"`
	Episode Episode `json:"episode"`
}

// Progress is how far the user got in an episode.
type Progress struct {
	Position time.Duration `json:"position"`
	Duration time.Duration `json:"duration,omitempty"`
	Updated  time.Time     `json:"updated"`
}

// PodcastID is a stable id. Shows that share a feed URL collapse into one
// subscription whether they were found on Podcast Index or iTunes.
func PodcastID(feedURL string, feedID, itunesID int64, title string) string {
	if u := strings.TrimSpace(feedURL); u != "" {
		return "feed:" + strings.ToLower(u)
	}
	if feedID != 0 {
		return "pi:" + itoa(feedID)
	}
	if itunesID != 0 {
		return "itunes:" + itoa(itunesID)
	}
	return "title:" + strings.TrimSpace(title)
}

// EpisodeKey identifies playback progress. The feed URL plus the item GUID
// stays the same when the enclosure URL is rewritten.
func EpisodeKey(feedURL, guid, audioURL string) string {
	guid = strings.TrimSpace(guid)
	feedURL = strings.TrimSpace(feedURL)
	if guid != "" && feedURL != "" {
		return feedURL + "|" + guid
	}
	if guid != "" {
		return "guid:" + guid
	}
	return "audio:" + strings.TrimSpace(audioURL)
}

// EnsureIDs fills ID fields that callers left empty.
func (p *Podcast) EnsureIDs() {
	if p.ID == "" {
		p.ID = PodcastID(p.FeedURL, p.FeedID, p.ITunesID, p.Title)
	}
}

// EnsureID fills Episode.ID.
func (e *Episode) EnsureID(feedURL string) {
	if e.ID == "" {
		e.ID = EpisodeKey(feedURL, e.GUID, e.AudioURL)
	}
}

// ResumeFrom is where playback should start. An episode left in the last
// few seconds starts over.
func ResumeFrom(p Progress) time.Duration {
	if p.Position <= 0 {
		return 0
	}
	if p.Duration > 0 && p.Duration-p.Position <= 10*time.Second {
		return 0
	}
	if p.Position > p.Duration && p.Duration > 0 {
		return 0
	}
	return p.Position
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
