// Package player tracks play, pause, seek and the position written to disk.
// The audio engine is an interface so tests do not open a sound device.
package player

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/renyijiu/tingyu/internal/model"
)

// Handle is one open piece of audio.
type Handle interface {
	Play()
	Pause()
	Seek(time.Duration)
	Position() time.Duration
	Duration() time.Duration
	Playing() bool
	Close() error
}

// Meta is shown with the audio. Engines may ignore it.
type Meta struct {
	Title    string
	Artist   string
	Duration time.Duration
}

// Engine opens a stream. r may still be downloading.
type Engine interface {
	Open(r io.ReadSeeker, meta Meta) (Handle, error)
}

// Opener turns an episode URL into something the engine can seek.
// It must honor ctx during opening and any subsequent network reads.
type Opener func(ctx context.Context, audioURL string) (io.ReadSeeker, error)

// SaveFunc persists a position. It may be nil.
type SaveFunc func(episodeKey string, position, duration time.Duration) error

// Controller is the playback session.
type Controller struct {
	Engine Engine
	Open   Opener
	Save   SaveFunc
	// OnChange runs after playback state changes from a background open,
	// so the window can draw "buffering" before the file arrives.
	// It is called without the controller lock and must not re-enter Play.
	OnChange func()
	Now      func() time.Time

	mu        sync.Mutex
	gen       int
	handle    Handle
	closer    io.Closer
	cancel    context.CancelFunc
	episode   *model.Episode
	showTitle string
	key       string
	buffering bool
	playErr   error
	saveErr   error
	pending   *time.Duration
	lastSave  time.Time
	speed     float64
}

// State is a snapshot the UI can draw.
type State struct {
	Episode   *model.Episode
	ShowTitle string
	Playing   bool
	Position  time.Duration
	Duration  time.Duration
	Speed     float64
	Buffering bool
	Err       error
	SaveErr   error
}

// Play starts episode at resume, replacing whatever is playing.
func (c *Controller) Play(ctx context.Context, showTitle string, ep model.Episode, resume time.Duration) {
	c.mu.Lock()
	c.rememberLocked()
	c.gen++
	gen := c.gen
	c.closeLocked()
	ctx, c.cancel = context.WithCancel(ctx)
	epCopy := ep
	c.episode = &epCopy
	c.showTitle = showTitle
	c.key = ep.ID
	c.buffering = true
	c.playErr = nil
	at := resume
	c.pending = &at
	open := c.Open
	engine := c.Engine
	c.mu.Unlock()
	c.notify()

	if open == nil || engine == nil {
		c.fail(gen, errString("播放器未就绪"))
		return
	}
	if ep.AudioURL == "" {
		c.fail(gen, errString("这一集没有音频地址"))
		return
	}
	src, err := open(ctx, ep.AudioURL)
	if err != nil {
		c.fail(gen, err)
		return
	}
	c.mu.Lock()
	stale := gen != c.gen
	c.mu.Unlock()
	if stale || ctx.Err() != nil {
		closeIf(src)
		c.fail(gen, ctx.Err())
		return
	}
	h, err := engine.Open(src, Meta{Title: ep.Title, Artist: showTitle, Duration: ep.Duration})
	if err != nil {
		closeIf(src)
		c.fail(gen, err)
		return
	}
	c.mu.Lock()
	if gen != c.gen || ctx.Err() != nil {
		c.mu.Unlock()
		h.Close()
		closeIf(src)
		c.fail(gen, ctx.Err())
		return
	}
	c.handle = h
	if cl, ok := src.(io.Closer); ok {
		c.closer = cl
	}
	seek := resume
	if c.pending != nil {
		seek = *c.pending
	}
	c.pending = nil
	c.buffering = false
	if seek > 0 {
		h.Seek(seek)
	}
	applySpeed(h, c.speed)
	h.Play()
	c.mu.Unlock()
	c.notify()
}

// SetSpeed changes playback speed. 1 is normal. Engines that cannot change
// speed keep playing at their own rate; the choice is still remembered.
func (c *Controller) SetSpeed(speed float64) {
	if speed < 0.5 || speed > 2.5 {
		return
	}
	c.mu.Lock()
	c.speed = speed
	h := c.handle
	c.mu.Unlock()
	applySpeed(h, speed)
	c.notify()
}

// Skip moves the current episode by delta, staying inside the episode.
func (c *Controller) Skip(delta time.Duration) {
	st := c.State()
	if st.Episode == nil {
		return
	}
	at := st.Position + delta
	if at < 0 {
		at = 0
	}
	if st.Duration > 0 && at > st.Duration {
		at = st.Duration
	}
	c.Seek(at)
}

// Toggle pauses or resumes the current episode.
func (c *Controller) Toggle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle == nil {
		return
	}
	if c.handle.Playing() {
		c.handle.Pause()
		c.rememberLocked()
		return
	}
	c.handle.Play()
}

// Seek moves the current episode.
func (c *Controller) Seek(at time.Duration) {
	if at < 0 {
		at = 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = &at
	if c.handle == nil {
		return
	}
	c.handle.Seek(at)
	c.rememberLocked()
}

// Tick stores the position, at most once every two seconds.
func (c *Controller) Tick() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle == nil || !c.handle.Playing() {
		return
	}
	now := c.now()
	if !c.lastSave.IsZero() && now.Sub(c.lastSave) < 2*time.Second {
		return
	}
	c.rememberLocked()
}

// State returns the current playback snapshot.
func (c *Controller) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := State{
		Episode:   c.episode,
		ShowTitle: c.showTitle,
		Buffering: c.buffering,
		Err:       c.playErr,
		SaveErr:   c.saveErr,
	}
	if c.handle != nil {
		st.Playing = c.handle.Playing()
		st.Position = c.handle.Position()
		st.Duration = c.handle.Duration()
	} else if c.pending != nil {
		st.Position = *c.pending
	}
	if st.Duration <= 0 && c.episode != nil {
		st.Duration = c.episode.Duration
	}
	st.Speed = c.speed
	if st.Speed == 0 {
		st.Speed = 1
	}
	return st
}

// Close stops playback and stores the position.
func (c *Controller) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	c.rememberLocked()
	c.closeLocked()
	c.buffering = false
}

func (c *Controller) fail(gen int, err error) {
	c.mu.Lock()
	if gen != c.gen {
		c.mu.Unlock()
		return
	}
	c.buffering = false
	c.playErr = err
	c.pending = nil
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.mu.Unlock()
	c.notify()
}

func (c *Controller) notify() {
	if c.OnChange != nil {
		c.OnChange()
	}
}

func (c *Controller) rememberLocked() {
	if c.handle == nil || c.key == "" || c.Save == nil {
		return
	}
	pos := c.handle.Position()
	dur := c.handle.Duration()
	if dur <= 0 && c.episode != nil {
		dur = c.episode.Duration
	}
	c.lastSave = c.now()
	// Save runs while the controller lock is held. It must not call back in.
	c.saveErr = c.Save(c.key, pos, dur)
}

func (c *Controller) closeLocked() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.handle != nil {
		c.handle.Close()
		c.handle = nil
	}
	if c.closer != nil {
		c.closer.Close()
		c.closer = nil
	}
}

func (c *Controller) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func closeIf(r io.ReadSeeker) {
	if c, ok := r.(io.Closer); ok {
		c.Close()
	}
}

type speedControl interface {
	SetSpeed(float64)
}

func applySpeed(h Handle, speed float64) {
	if h == nil {
		return
	}
	if speed == 0 {
		speed = 1
	}
	if s, ok := h.(speedControl); ok {
		s.SetSpeed(speed)
	}
}

type stringErr string

func (e stringErr) Error() string { return string(e) }

func errString(s string) error { return stringErr(s) }
