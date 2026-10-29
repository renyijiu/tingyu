package mygoengine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/renyijiu/tingyu/internal/player"
)

// src is a download the player must close when it is done.
type src struct {
	*bytes.Reader
	closed atomic.Bool
}

func (s *src) Close() error {
	s.closed.Store(true)
	return nil
}

// wav is 1 s of a 440 Hz tone at half scale, 16-bit stereo at 44.1 kHz.
func wav() []byte {
	const rate, frames = 44100, 44100
	var data bytes.Buffer
	for i := range frames {
		s := int16(0.5 * math.MaxInt16 * math.Sin(2*math.Pi*440*float64(i)/rate))
		binary.Write(&data, binary.LittleEndian, [2]int16{s, s})
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+data.Len()))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, struct {
		Size             uint32
		Format, Channels uint16
		Rate, ByteRate   uint32
		Align, Bits      uint16
	}{Size: 16, Format: 1, Channels: 2, Rate: rate, ByteRate: rate * 4, Align: 4, Bits: 16})
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(data.Len()))
	b.Write(data.Bytes())
	return b.Bytes()
}

func TestOpenPlaysMetersAndCloses(t *testing.T) {
	m4a, err := os.ReadFile("m4a/testdata/faststart.m4a")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
		dur  time.Duration
	}{
		{"wav", wav(), time.Second},
		{"m4a", m4a, 2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "m4a" && runtime.GOOS != "darwin" {
				t.Skip("M4A decodes with macOS AudioToolbox")
			}
			s := &src{Reader: bytes.NewReader(tc.data)}
			h, err := New().Open(s, player.Meta{})
			if err != nil {
				t.Fatal(err)
			}
			if got := h.Duration(); got != tc.dur {
				t.Errorf("Duration = %v, want %v", got, tc.dur)
			}
			// The player decodes ahead while paused; the tone at the
			// start is about -9 dBFS, high on the meter.
			l := h.(player.Leveler)
			deadline := time.Now().Add(5 * time.Second)
			for l.Level() < 0.7 {
				if time.Now().After(deadline) {
					t.Fatalf("Level = %.2f, want the tone's", l.Level())
				}
				time.Sleep(10 * time.Millisecond)
			}
			h.Close()
			for !s.closed.Load() {
				if time.Now().After(deadline) {
					t.Fatal("closing the player left the download open")
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

func TestOpenUnknownFormat(t *testing.T) {
	_, err := New().Open(bytes.NewReader([]byte("not audio at all")), player.Meta{})
	var notice player.Notice
	if !errors.As(err, &notice) || !strings.Contains(string(notice), "M4A") {
		t.Fatalf("Open = %v, want a notice naming the supported formats", err)
	}
}
