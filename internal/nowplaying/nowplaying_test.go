package nowplaying

import (
	"context"
	"errors"
	"image"
	"image/color"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
)

type call struct {
	kind  string
	info  mygo.NowPlayingInfo
	state mygo.PlaybackState
	pos   time.Duration
}

type fakeSystem struct {
	mu    sync.Mutex
	calls []call
	set   chan struct{}
}

func (s *fakeSystem) Set(info mygo.NowPlayingInfo) {
	s.mu.Lock()
	s.calls = append(s.calls, call{kind: "set", info: info})
	s.mu.Unlock()
	if s.set != nil {
		s.set <- struct{}{}
	}
}

func (s *fakeSystem) SetPlayback(state mygo.PlaybackState, pos time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call{kind: "playback", state: state, pos: pos})
}

func (s *fakeSystem) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call{kind: "clear"})
}

// take returns the calls since the last take.
func (s *fakeSystem) take() []call {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.calls
	s.calls = nil
	return out
}

type fakePlayer struct {
	st    player.State
	moves []string
}

func (p *fakePlayer) State() player.State { return p.st }
func (p *fakePlayer) Toggle() {
	p.st.Playing = !p.st.Playing
	p.moves = append(p.moves, "toggle")
}
func (p *fakePlayer) Seek(d time.Duration) {
	p.st.Position = d
	p.moves = append(p.moves, "seek "+d.String())
}
func (p *fakePlayer) Skip(d time.Duration) { p.moves = append(p.moves, "skip "+d.String()) }

type fakeCovers struct{ err error }

func (c fakeCovers) Load(context.Context, string) (image.Image, error) {
	if c.err != nil {
		return nil, c.err
	}
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{200, 100, 50, 255})
	return img, nil
}

func playing(id string, pos time.Duration) player.State {
	return player.State{
		Episode:  &model.Episode{ID: id, Title: "第 " + id + " 集"},
		Playing:  true,
		Position: pos,
		Duration: time.Hour,
		Speed:    1,
	}
}

func TestSyncTellsOnlyWhatChanged(t *testing.T) {
	sys := &fakeSystem{}
	now := time.Unix(1000, 0)
	b := &Bridge{System: sys, Now: func() time.Time { return now }}

	b.Sync(playing("1", 0), "", "故事会")
	got := sys.take()
	if len(got) != 1 || got[0].kind != "set" || got[0].info.Title != "第 1 集" || got[0].info.Artist != "故事会" ||
		got[0].info.State != mygo.PlaybackPlaying || got[0].info.Duration != time.Hour || got[0].info.Commands != commands {
		t.Fatalf("first sync %+v", got)
	}

	// Playback moving on as the system expects says nothing.
	now = now.Add(10 * time.Second)
	b.Sync(playing("1", 10*time.Second), "", "故事会")
	if got := sys.take(); len(got) != 0 {
		t.Fatalf("steady playback told the system %+v", got)
	}

	// A jump is told.
	b.Sync(playing("1", 5*time.Minute), "", "故事会")
	if got := sys.take(); len(got) != 1 || got[0].kind != "playback" || got[0].pos != 5*time.Minute {
		t.Fatalf("seek %+v", got)
	}

	// So is a pause, and the paused position then stays put.
	paused := playing("1", 5*time.Minute)
	paused.Playing = false
	b.Sync(paused, "", "故事会")
	if got := sys.take(); len(got) != 1 || got[0].kind != "playback" || got[0].state != mygo.PlaybackPaused {
		t.Fatalf("pause %+v", got)
	}
	now = now.Add(time.Minute)
	b.Sync(paused, "", "故事会")
	if got := sys.take(); len(got) != 0 {
		t.Fatalf("paused playback told the system %+v", got)
	}

	// A new speed is a new rate.
	fast := playing("1", 5*time.Minute)
	fast.Speed = 1.5
	b.Sync(fast, "", "故事会")
	if got := sys.take(); len(got) != 1 || got[0].kind != "set" || got[0].info.Rate != 1.5 {
		t.Fatalf("speed %+v", got)
	}

	// Nothing playing clears the controls, once.
	b.Sync(player.State{}, "", "")
	b.Sync(player.State{}, "", "")
	if got := sys.take(); len(got) != 1 || got[0].kind != "clear" {
		t.Fatalf("clear %+v", got)
	}
}

func TestSyncShowsTheCoverOnceLoaded(t *testing.T) {
	sys := &fakeSystem{set: make(chan struct{}, 4)}
	b := &Bridge{System: sys, Covers: fakeCovers{}}
	b.Sync(playing("1", 0), "https://example.com/cover.jpg", "故事会")
	<-sys.set
	select {
	case <-sys.set:
	case <-time.After(5 * time.Second):
		t.Fatal("cover never shown")
	}
	got := sys.take()
	if len(got) != 2 || len(got[0].info.Artwork) != 0 || len(got[1].info.Artwork) == 0 {
		t.Fatalf("calls %+v", got)
	}
	// The cover stays with the episode; it is not loaded again.
	b.Sync(playing("1", 0), "https://example.com/cover.jpg", "故事会")
	b.mu.Lock()
	art := len(b.shown.art)
	b.mu.Unlock()
	if art == 0 {
		t.Fatal("cover forgotten")
	}
}

func TestCoverOfAnEpisodeNoLongerPlayingIsDropped(t *testing.T) {
	sys := &fakeSystem{}
	b := &Bridge{System: sys}
	b.Sync(playing("2", 0), "https://example.com/two.jpg", "故事会")
	sys.take()
	b.Covers = fakeCovers{}
	b.loadArt("1", "https://example.com/one.jpg")
	if got := sys.take(); len(got) != 0 {
		t.Fatalf("stale cover shown: %+v", got)
	}
	b.Covers = fakeCovers{err: errors.New("offline")}
	b.loadArt("2", "https://example.com/two.jpg")
	if got := sys.take(); len(got) != 0 {
		t.Fatalf("failed cover shown: %+v", got)
	}
}

func TestCommandsDriveThePlayer(t *testing.T) {
	p := &fakePlayer{st: playing("1", time.Minute)}
	changed := 0
	b := &Bridge{System: &fakeSystem{}, Player: p, Changed: func() { changed++ }}

	b.Command(mygo.MediaPlay, 0) // already playing
	b.Command(mygo.MediaPause, 0)
	b.Command(mygo.MediaPause, 0) // already paused
	b.Command(mygo.MediaPlay, 0)
	b.Command(mygo.MediaSeek, 3*time.Minute)
	b.Command(mygo.MediaNext, 0)
	b.Command(mygo.MediaPrevious, 0)
	b.Command(mygo.MediaStop, 0)

	want := []string{"toggle", "toggle", "seek 3m0s", "skip 30s", "skip -15s", "toggle"}
	if len(p.moves) != len(want) {
		t.Fatalf("moves %v, want %v", p.moves, want)
	}
	for i := range want {
		if p.moves[i] != want[i] {
			t.Fatalf("moves %v, want %v", p.moves, want)
		}
	}
	if changed != 8 {
		t.Fatalf("window told %d times", changed)
	}
}
