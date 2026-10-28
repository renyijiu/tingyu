// Package mygoengine plays audio with MyGo. It is the only package that
// imports the audio engine, so the rest of the player can be tested without one.
package mygoengine

import (
	"fmt"
	"io"
	"time"

	"github.com/egoist/mygo/audio"
	_ "github.com/egoist/mygo/audio/mp3"

	"github.com/renyijiu/tingyu/internal/player"
)

// Engine opens streams through package audio.
type Engine struct{}

// New sets a playback latency suited to long episodes.
func New() *Engine {
	audio.SetLatency(audio.LatencyPlayback)
	return &Engine{}
}

// Open decodes r. MP3 is registered; WAV is built in. Other formats return an error.
func (e *Engine) Open(r io.ReadSeeker, _ player.Meta) (player.Handle, error) {
	p, err := audio.OpenStream(r)
	if err != nil {
		return nil, fmt.Errorf("无法解码音频（当前支持 MP3 和 WAV）：%w", err)
	}
	return &handle{p: p}, nil
}

type handle struct {
	p *audio.Player
}

func (h *handle) Play()                   { h.p.Play() }
func (h *handle) Pause()                  { h.p.Pause() }
func (h *handle) Seek(d time.Duration)    { h.p.Seek(d) }
func (h *handle) Position() time.Duration { return h.p.Position() }
func (h *handle) Duration() time.Duration { return h.p.Duration() }
func (h *handle) Playing() bool           { return h.p.Playing() }
func (h *handle) Close() error {
	h.p.Close()
	return nil
}
