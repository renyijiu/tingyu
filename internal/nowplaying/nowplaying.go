// Package nowplaying shows what plays where macOS shows media, in Control
// Center and the menu bar, and lets the media keys of keyboards and
// headsets drive the player.
package nowplaying

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/renyijiu/tingyu/internal/player"
)

// The next and previous track keys move within the episode, by as much as
// the player bar's buttons do.
const (
	SkipBack    = 15 * time.Second
	SkipForward = 30 * time.Second
)

// commands are what the system's controls offer.
const commands = mygo.MediaPlay | mygo.MediaPause | mygo.MediaSeek | mygo.MediaNext | mygo.MediaPrevious

// System is where the system shows media: mygo.NowPlaying in the app.
type System interface {
	Set(mygo.NowPlayingInfo)
	SetPlayback(mygo.PlaybackState, time.Duration)
	Clear()
}

// Player is what the media keys drive: a *player.Controller in the app.
type Player interface {
	State() player.State
	Toggle()
	Seek(time.Duration)
	Skip(time.Duration)
}

// Covers loads a cover: an *artwork.Loader in the app.
type Covers interface {
	Load(ctx context.Context, url string) (image.Image, error)
}

// Bridge keeps the system's media controls in step with the player.
type Bridge struct {
	System System
	Player Player
	Covers Covers
	// Changed runs after a media key moved the player, so the window
	// draws it.
	Changed func()
	Now     func() time.Time

	mu    sync.Mutex
	shown shown
}

// shown is what the system was last told.
type shown struct {
	id, title, artist, cover string
	art                      []byte
	duration                 time.Duration
	speed                    float64
	playing                  bool
	// pos was where playback was at at. The system moves it on while
	// playing, so a position only needs telling when it jumps.
	pos time.Duration
	at  time.Time
}

// Sync tells the system what st plays, from the show titled show with the
// cover at cover, and only what changed since it was last told. The window
// calls it as it draws.
func (b *Bridge) Sync(st player.State, cover, show string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if st.Episode == nil {
		if b.shown.id != "" {
			b.System.Clear()
			b.shown = shown{}
		}
		return
	}
	was := b.shown
	now := b.now()
	next := shown{
		id: st.Episode.ID, title: st.Episode.Title, artist: show, cover: cover,
		duration: st.Duration, speed: st.Speed, playing: st.Playing,
		pos: st.Position, at: now,
	}
	sameCover := next.id == was.id && next.cover == was.cover
	if sameCover {
		next.art = was.art
	}
	switch {
	case next.id != was.id || next.title != was.title || next.artist != was.artist || !sameCover ||
		next.speed != was.speed || absDur(next.duration-was.duration) > time.Second:
		b.System.Set(next.info())
		if !sameCover && cover != "" && b.Covers != nil {
			go b.loadArt(next.id, cover)
		}
	case next.playing != was.playing || absDur(next.pos-was.expected(now)) > 2*time.Second:
		b.System.SetPlayback(next.state(), next.pos)
	default:
		return
	}
	b.shown = next
}

// Command carries out a command of the media keys or the system's
// controls. The system calls it on the main thread.
func (b *Bridge) Command(cmd mygo.MediaCommand, position time.Duration) {
	playing := b.Player.State().Playing
	switch cmd {
	case mygo.MediaPlay:
		if !playing {
			b.Player.Toggle()
		}
	case mygo.MediaPause, mygo.MediaStop:
		if playing {
			b.Player.Toggle()
		}
	case mygo.MediaSeek:
		b.Player.Seek(position)
	case mygo.MediaNext:
		b.Player.Skip(SkipForward)
	case mygo.MediaPrevious:
		b.Player.Skip(-SkipBack)
	default:
		return
	}
	if b.Changed != nil {
		b.Changed()
	}
}

// loadArt shows the cover once it is loaded, if the same episode still
// plays. A cover that cannot be loaded leaves the system's own picture.
func (b *Bridge) loadArt(id, cover string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	img, err := b.Covers.Load(ctx, cover)
	if err != nil {
		return
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.shown.id != id || b.shown.cover != cover {
		return
	}
	now := b.now()
	b.shown.pos, b.shown.at = b.shown.expected(now), now
	b.shown.art = buf.Bytes()
	b.System.Set(b.shown.info())
}

func (s shown) info() mygo.NowPlayingInfo {
	return mygo.NowPlayingInfo{
		Title:    s.title,
		Artist:   s.artist,
		Artwork:  s.art,
		Duration: s.duration,
		Position: s.pos,
		Rate:     s.speed,
		State:    s.state(),
		Commands: commands,
	}
}

func (s shown) state() mygo.PlaybackState {
	if s.playing {
		return mygo.PlaybackPlaying
	}
	return mygo.PlaybackPaused
}

// expected is where the system believes playback is at now.
func (s shown) expected(now time.Time) time.Duration {
	if !s.playing {
		return s.pos
	}
	speed := s.speed
	if speed <= 0 {
		speed = 1
	}
	return s.pos + time.Duration(float64(now.Sub(s.at))*speed)
}

func (b *Bridge) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
