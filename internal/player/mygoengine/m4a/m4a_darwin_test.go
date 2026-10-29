package m4a

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/audio"
)

// The fixtures are 2 s of AAC-LC stereo at 44.1 kHz, 64 kb/s: a 440 Hz
// tone at half scale for the first second, then silence. faststart.m4a
// puts the moov box before the audio, as 小宇宙 does; moov-last.m4a after
// it, as ffmpeg does by default. Both were made with:
//
//	E='if(lt(t\,1)\,0.5*sin(2*PI*440*t)\,0)'
//	ffmpeg -f lavfi -i "aevalsrc=exprs=${E}|${E}:s=44100" -t 2 \
//		-c:a aac -b:a 64k [-movflags +faststart] -map_metadata -1 -fflags +bitexact out.m4a

const (
	rate    = 44100
	toneRMS = 0.5 / math.Sqrt2
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func open(t *testing.T, r io.ReadSeeker) audio.Decoder {
	t.Helper()
	d, name, err := audio.NewDecoder(r)
	if err != nil {
		t.Fatalf("NewDecoder: %v", err)
	}
	if name != "m4a" {
		t.Fatalf("format = %q, want m4a", name)
	}
	t.Cleanup(func() { d.(io.Closer).Close() })
	return d
}

// rms reads n frames from d.
func rms(t *testing.T, d audio.Decoder, n int) float64 {
	t.Helper()
	buf := make([]float32, 1024*d.Channels())
	var sum float64
	got := 0
	for got < n {
		k, err := d.Read(buf[:min(len(buf), (n-got)*d.Channels())])
		for _, s := range buf[:k] {
			sum += float64(s) * float64(s)
		}
		got += k / d.Channels()
		if err != nil {
			t.Fatalf("Read after %d frames: %v", got, err)
		}
	}
	return math.Sqrt(sum / float64(n*d.Channels()))
}

func TestDecodesBothLayouts(t *testing.T) {
	for _, name := range []string{"faststart.m4a", "moov-last.m4a"} {
		t.Run(name, func(t *testing.T) {
			d := open(t, bytes.NewReader(fixture(t, name)))
			if d.SampleRate() != rate || d.Channels() != 2 {
				t.Fatalf("format = %d Hz × %d, want %d Hz stereo", d.SampleRate(), d.Channels(), rate)
			}
			// The encoder's priming frames are trimmed: exactly 2 s.
			if d.Len() != 2*rate {
				t.Fatalf("Len = %d, want %d", d.Len(), 2*rate)
			}
			if got := rms(t, d, rate*9/10); math.Abs(got-toneRMS) > 0.03 {
				t.Errorf("tone RMS = %.3f, want about %.3f", got, toneRMS)
			}
			rms(t, d, rate/5)
			if got := rms(t, d, rate*9/10-1); got > 0.01 {
				t.Errorf("silence RMS = %.3f", got)
			}
			buf := make([]float32, 4096)
			total := 2*rate - 1
			for {
				n, err := d.Read(buf)
				total += n / 2
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if total != 2*rate {
				t.Errorf("decoded %d frames, want %d", total, 2*rate)
			}
		})
	}
}

func TestSetPosition(t *testing.T) {
	d := open(t, bytes.NewReader(fixture(t, "faststart.m4a")))
	if err := d.SetPosition(rate * 3 / 2); err != nil {
		t.Fatal(err)
	}
	if got := rms(t, d, 4096); got > 0.01 {
		t.Errorf("RMS at 1.5 s = %.3f, want silence", got)
	}
	if err := d.SetPosition(rate / 2); err != nil {
		t.Fatal(err)
	}
	if got := rms(t, d, 4096); math.Abs(got-toneRMS) > 0.03 {
		t.Errorf("RMS at 0.5 s = %.3f, want the tone", got)
	}
	// From the end back to the start, as a player looping or replaying does.
	if err := d.SetPosition(d.Len()); err != nil {
		t.Fatal(err)
	}
	if n, err := d.Read(make([]float32, 4096)); n != 0 || err != io.EOF {
		t.Fatalf("Read at the end = %d, %v; want 0, EOF", n, err)
	}
	if err := d.SetPosition(0); err != nil {
		t.Fatal(err)
	}
	if got := rms(t, d, 4096); math.Abs(got-toneRMS) > 0.05 {
		t.Errorf("RMS at 0 = %.3f after replay", got)
	}
}

// download is a file arriving: reads past ready wait for release, or fail
// with err when it is set.
type download struct {
	data []byte
	pos  int64
	err  error

	mu      sync.Mutex
	ready   int64
	release chan struct{}
}

func newDownload(data []byte, ready int64) *download {
	return &download{data: data, ready: ready, release: make(chan struct{})}
}

func (r *download) Read(p []byte) (int, error) {
	if r.pos >= int64(len(r.data)) {
		return 0, io.EOF
	}
	r.mu.Lock()
	ready := r.ready
	r.mu.Unlock()
	if r.pos >= ready {
		if r.err != nil {
			return 0, r.err
		}
		<-r.release
		ready = int64(len(r.data))
	}
	n := copy(p, r.data[r.pos:ready])
	r.pos += int64(n)
	return n, nil
}

// Seek knows the length at once, as media.Reader does from Content-Length.
func (r *download) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.pos = off
	case io.SeekCurrent:
		r.pos += off
	case io.SeekEnd:
		r.pos = int64(len(r.data)) + off
	}
	return r.pos, nil
}

func (r *download) finish() {
	r.mu.Lock()
	r.ready = int64(len(r.data))
	r.mu.Unlock()
	close(r.release)
}

func openAsync(r io.ReadSeeker) chan error {
	done := make(chan error, 1)
	go func() {
		d, _, err := audio.NewDecoder(r)
		if err == nil {
			d.(io.Closer).Close()
		}
		done <- err
	}()
	return done
}

func TestOpensBeforeDownloadEnds(t *testing.T) {
	// AudioFile first reads 8 KiB from the start; the moov box of
	// faststart.m4a is within it, and moov-last.m4a's is not.
	const head = 8192

	fast := newDownload(fixture(t, "faststart.m4a"), head)
	defer fast.finish()
	select {
	case err := <-openAsync(fast):
		if err != nil {
			t.Fatalf("faststart: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("faststart: opening waited for the rest of the file")
	}

	last := newDownload(fixture(t, "moov-last.m4a"), head)
	done := openAsync(last)
	select {
	case err := <-done:
		t.Fatalf("moov-last opened without its moov box: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	last.finish()
	if err := <-done; err != nil {
		t.Fatalf("moov-last after the download: %v", err)
	}
}

func TestDownloadErrorPassesThrough(t *testing.T) {
	broken := errors.New("connection reset")

	last := newDownload(fixture(t, "moov-last.m4a"), 8192)
	last.err = broken
	if _, _, err := audio.NewDecoder(last); !errors.Is(err, broken) {
		t.Fatalf("open = %v, want the download's error", err)
	}

	before := registered()
	fast := newDownload(fixture(t, "faststart.m4a"), 8192)
	fast.err = broken
	d := open(t, fast)
	buf := make([]float32, 4096)
	for {
		_, err := d.Read(buf)
		if err == nil {
			continue
		}
		if !errors.Is(err, broken) {
			t.Fatalf("Read = %v, want the download's error", err)
		}
		break
	}
	if n := registered(); n != before+1 {
		t.Errorf("%d sources registered, want %d", n, before+1)
	}
}

func TestClose(t *testing.T) {
	before := registered()
	d, err := NewDecoder(bytes.NewReader(fixture(t, "faststart.m4a")))
	if err != nil {
		t.Fatal(err)
	}
	c := d.(io.Closer)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := d.Read(make([]float32, 64)); err == nil {
		t.Error("Read after Close succeeded")
	}
	if err := d.SetPosition(0); err == nil {
		t.Error("SetPosition after Close succeeded")
	}
	if n := registered(); n != before {
		t.Errorf("%d sources registered after Close, want %d", n, before)
	}
}

// DecodeSound reads a decoder to its end and drops it without Close.
func TestDecodeSoundFreesWhenCollected(t *testing.T) {
	before := registered()
	s, err := audio.DecodeSound(fixture(t, "faststart.m4a"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Duration() != 2*time.Second {
		t.Errorf("Duration = %v, want 2s", s.Duration())
	}
	deadline := time.Now().Add(5 * time.Second)
	for registered() != before {
		if time.Now().After(deadline) {
			t.Fatalf("%d sources registered long after the decoder was dropped, want %d", registered(), before)
		}
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
}

func TestNotMP4(t *testing.T) {
	// The ftyp box, then nothing.
	data := append(fixture(t, "faststart.m4a")[:28], make([]byte, 4096)...)
	before := registered()
	if _, _, err := audio.NewDecoder(bytes.NewReader(data)); err == nil {
		t.Fatal("opened an MP4 of no boxes after ftyp")
	}
	if n := registered(); n != before {
		t.Errorf("a failed open left %d sources registered, want %d", n, before)
	}
}

func registered() int {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	return len(sources)
}
