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
	d, _, err := audio.NewDecoder(r)
	if err != nil {
		return nil, fmt.Errorf("无法解码音频（当前支持 MP3 和 WAV）：%w", err)
	}
	m := &metered{Decoder: d}
	var dec audio.Decoder = m
	if c, ok := r.(io.Closer); ok {
		dec = closing{m, c}
	}
	return &handle{p: audio.NewPlayer(dec), m: m, rate: d.SampleRate()}, nil
}

// metered passes the decoded samples on, measuring their loudness on the
// way. The player calls it from one goroutine at a time, ahead of what is
// heard; the handle looks the level up by the position heard.
type metered struct {
	audio.Decoder
	meter player.Meter
}

func (m *metered) Read(buf []float32) (int, error) {
	n, err := m.Decoder.Read(buf)
	m.meter.Record(buf[:n], m.Decoder.Channels())
	return n, err
}

func (m *metered) SetPosition(frame int64) error {
	err := m.Decoder.SetPosition(frame)
	if err == nil {
		m.meter.Reset(frame)
	}
	return err
}

type closing struct {
	*metered
	io.Closer
}

type handle struct {
	p    *audio.Player
	m    *metered
	rate int
}

func (h *handle) Play()                   { h.p.Play() }
func (h *handle) Pause()                  { h.p.Pause() }
func (h *handle) Seek(d time.Duration)    { h.p.Seek(d) }
func (h *handle) Position() time.Duration { return h.p.Position() }
func (h *handle) Duration() time.Duration { return h.p.Duration() }
func (h *handle) Playing() bool           { return h.p.Playing() }
func (h *handle) SetSpeed(speed float64)  { h.p.SetSpeed(speed) }
func (h *handle) Close() error {
	h.p.Close()
	return nil
}

// Level is the loudness of the block being heard. Position is in the
// episode's own time, so it holds at any speed.
func (h *handle) Level() float32 {
	frame := int64(h.p.Position().Seconds() * float64(h.rate))
	l, _ := h.m.meter.At(frame)
	return l
}
