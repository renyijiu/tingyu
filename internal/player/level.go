package player

import (
	"math"
	"sync/atomic"
)

// Leveler is a Handle that can tell how loud the audio being heard is.
type Leveler interface {
	// Level is the loudness at the position heard, from 0 (silence) to 1.
	Level() float32
}

// MeterBlock is how many frames one level covers: about 23 ms at 44.1 kHz.
const MeterBlock = 1024

const meterSlots = 512 // about 12 s of blocks, far more than an engine decodes ahead

// Meter keeps the loudness of the blocks an engine decoded, by the frame
// each starts at, so the level of what is heard can be looked up later:
// engines decode ahead of the speakers. One goroutine (the audio thread)
// records; any may read. Recording neither locks nor allocates.
type Meter struct {
	slots [meterSlots]atomic.Uint64 // start frame << 16 | level
	next  atomic.Uint64

	// The block being summed, owned by the recording goroutine.
	start int64
	n     int
	sum   float64
}

// Reset starts the blocks again at frame, as after a seek.
func (m *Meter) Reset(frame int64) {
	m.start, m.n, m.sum = frame, 0, 0
}

// Record adds decoded interleaved samples of ch channels.
func (m *Meter) Record(samples []float32, ch int) {
	if ch < 1 {
		ch = 1
	}
	for i := 0; i+ch <= len(samples); i += ch {
		var sq float64
		for c := 0; c < ch; c++ {
			s := float64(samples[i+c])
			sq += s * s
		}
		m.sum += sq / float64(ch)
		m.n++
		if m.n == MeterBlock {
			m.push()
		}
	}
}

func (m *Meter) push() {
	rms := math.Sqrt(m.sum / float64(m.n))
	level := uint64(math.Round(loudness(rms) * 0xffff))
	i := m.next.Add(1) - 1
	m.slots[i%meterSlots].Store(uint64(m.start)<<16 | level)
	m.start += int64(m.n)
	m.n, m.sum = 0, 0
}

// At returns the level of the block holding frame, the newest one
// recorded, so blocks decoded after a seek win over older ones.
func (m *Meter) At(frame int64) (float32, bool) {
	n := m.next.Load()
	for k := uint64(0); k < meterSlots && k < n; k++ {
		v := m.slots[(n-1-k)%meterSlots].Load()
		start := int64(v >> 16)
		if frame >= start && frame < start+MeterBlock {
			return float32(v&0xffff) / 0xffff, true
		}
	}
	return 0, false
}

// loudness maps an RMS amplitude to 0..1 over 50 dB, where speech sits
// in the upper half and silence at 0.
func loudness(rms float64) float64 {
	if rms <= 0 {
		return 0
	}
	db := 20 * math.Log10(rms)
	return min(max((db+50)/50, 0), 1)
}

// Level is the loudness of what plays, and false when nothing plays or
// the engine cannot tell.
func (c *Controller) Level() (float32, bool) {
	c.mu.Lock()
	h := c.handle
	c.mu.Unlock()
	l, ok := h.(Leveler)
	if !ok || !h.Playing() {
		return 0, false
	}
	return l.Level(), true
}
