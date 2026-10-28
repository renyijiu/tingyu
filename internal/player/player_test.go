package player

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/renyijiu/tingyu/internal/model"
)

type mem struct{ *bytes.Reader }

func (mem) Close() error { return nil }

type fakeHandle struct {
	pos     time.Duration
	dur     time.Duration
	playing bool
	closed  bool
}

func (h *fakeHandle) Play()                   { h.playing = true }
func (h *fakeHandle) Pause()                  { h.playing = false }
func (h *fakeHandle) Seek(d time.Duration)    { h.pos = d }
func (h *fakeHandle) Position() time.Duration { return h.pos }
func (h *fakeHandle) Duration() time.Duration { return h.dur }
func (h *fakeHandle) Playing() bool           { return h.playing }
func (h *fakeHandle) Close() error            { h.closed = true; h.playing = false; return nil }

type fakeEngine struct {
	h   *fakeHandle
	err error
	n   int
}

func (e *fakeEngine) Open(io.ReadSeeker, Meta) (Handle, error) {
	e.n++
	if e.err != nil {
		return nil, e.err
	}
	e.h = &fakeHandle{dur: time.Minute}
	return e.h, nil
}

func TestPlayPauseSeekSaves(t *testing.T) {
	var saved []time.Duration
	eng := &fakeEngine{}
	now := time.Unix(1_700_000_000, 0)
	c := &Controller{
		Engine: eng,
		Open: func(context.Context, string) (io.ReadSeeker, error) {
			return mem{bytes.NewReader([]byte("x"))}, nil
		},
		Save: func(key string, pos, dur time.Duration) {
			if key != "ep-1" {
				t.Errorf("key %s", key)
			}
			if dur != time.Minute {
				t.Errorf("dur %v", dur)
			}
			saved = append(saved, pos)
		},
		Now: func() time.Time { return now },
	}
	ep := model.Episode{ID: "ep-1", Title: "第一集", AudioURL: "https://cdn/a.mp3", Duration: 30 * time.Minute}
	var order []string
	opened := false
	c.OnChange = func() { order = append(order, "ui") }
	c.Open = func(context.Context, string) (io.ReadSeeker, error) {
		if !opened && (len(order) == 0 || order[0] != "ui") {
			t.Fatalf("window was not asked to redraw before the download: %v", order)
		}
		opened = true
		return mem{bytes.NewReader([]byte("x"))}, nil
	}
	c.Play(context.Background(), "故事会", ep, 15*time.Second)
	st := c.State()
	if st.Err != nil || !st.Playing || st.Buffering {
		t.Fatalf("%+v", st)
	}
	if eng.h.pos != 15*time.Second || !eng.h.playing {
		t.Fatalf("engine pos %v playing %v", eng.h.pos, eng.h.playing)
	}
	eng.h.pos = 40 * time.Second
	c.Toggle()
	if eng.h.playing {
		t.Fatal("still playing")
	}
	if len(saved) == 0 || saved[len(saved)-1] != 40*time.Second {
		t.Fatalf("pause save %v", saved)
	}
	c.Seek(5 * time.Second)
	if eng.h.pos != 5*time.Second {
		t.Fatalf("seek %v", eng.h.pos)
	}
	c.Toggle()
	if !eng.h.playing {
		t.Fatal("did not resume")
	}
	eng.h.pos = 8 * time.Second
	now = now.Add(3 * time.Second)
	c.Tick()
	if saved[len(saved)-1] != 8*time.Second {
		t.Fatalf("tick %v", saved)
	}
	n := len(saved)
	c.Tick()
	if len(saved) != n {
		t.Fatal("tick should throttle")
	}
	now = now.Add(3 * time.Second)
	eng.h.pos = 11 * time.Second
	c.Tick()
	if len(saved) != n+1 || saved[len(saved)-1] != 11*time.Second {
		t.Fatalf("tick after 2s: %v", saved)
	}
}

func TestPlayErrorAndStaleOpen(t *testing.T) {
	eng := &fakeEngine{err: errors.New("bad format")}
	c := &Controller{
		Engine: eng,
		Open: func(context.Context, string) (io.ReadSeeker, error) {
			return mem{bytes.NewReader(nil)}, nil
		},
	}
	c.Play(context.Background(), "", model.Episode{ID: "e", AudioURL: "https://cdn/a.mp3"}, 0)
	st := c.State()
	if st.Err == nil || st.Playing || st.Buffering {
		t.Fatalf("%+v", st)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	eng.err = nil
	c.Open = func(_ context.Context, audioURL string) (io.ReadSeeker, error) {
		if strings.Contains(audioURL, "slow") {
			started <- struct{}{}
			<-release
		}
		return mem{bytes.NewReader([]byte("a"))}, nil
	}
	go c.Play(context.Background(), "show", model.Episode{ID: "slow", Title: "慢", AudioURL: "https://cdn/slow.mp3"}, time.Second)
	<-started
	c.Play(context.Background(), "show", model.Episode{ID: "fast", Title: "快", AudioURL: "https://cdn/fast.mp3"}, 0)
	close(release)
	// Let the slow open finish and drop itself.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.State().Episode != nil && c.State().Episode.ID == "fast" && c.State().Playing {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("stale open won: %+v", c.State())
}

func TestMissingAudio(t *testing.T) {
	c := &Controller{}
	c.Play(context.Background(), "", model.Episode{Title: "x"}, 0)
	if c.State().Err == nil {
		t.Fatal("expected error")
	}
}
