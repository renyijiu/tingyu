package m4a

import (
	"errors"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo/audio"
)

// AudioFile parses the MP4 and reads it through readProc and sizeProc;
// ExtAudioFile decodes its packets to float32 at the file's own rate,
// trimming the encoder's priming frames. Both are called through purego,
// as package audio calls the output, so the build needs no cgo.

type audioStreamBasicDescription struct {
	sampleRate       float64
	formatID         uint32
	formatFlags      uint32
	bytesPerPacket   uint32
	framesPerPacket  uint32
	bytesPerFrame    uint32
	channelsPerFrame uint32
	bitsPerChannel   uint32
	reserved         uint32
}

// audioBufferList holds one AudioBuffer, which is pointer-aligned.
type audioBufferList struct {
	numberBuffers  uint32
	_              uint32
	numberChannels uint32
	dataByteSize   uint32
	data           unsafe.Pointer
}

const (
	kAudioFileM4AType                      = 0x6d346166 // 'm4af'
	kExtAudioFileProperty_FileDataFormat   = 0x66666d74 // 'ffmt'
	kExtAudioFileProperty_ClientDataFormat = 0x63666d74 // 'cfmt'
	kExtAudioFileProperty_FileLengthFrames = 0x2366726d // '#frm'
	kAudioFormatLinearPCM                  = 0x6c70636d // 'lpcm'
	kAudioFormatFlagIsFloat                = 1
	kAudioFormatFlagIsPacked               = 8
	kAudioFileUnspecifiedError             = 0x7768743f // 'wht?'

	readFrames = 4096 // decoded per call into AudioToolbox
)

var (
	loadOnce sync.Once
	loadErr  error

	audioFileOpenWithCallbacks  func(client, read, write, getSize, setSize uintptr, hint uint32, out *uintptr) int32
	audioFileClose              func(file uintptr) int32
	extAudioFileWrapAudioFileID func(file uintptr, forWriting bool, out *uintptr) int32
	extAudioFileGetProperty     func(ext uintptr, id uint32, size *uint32, data unsafe.Pointer) int32
	extAudioFileSetProperty     func(ext uintptr, id uint32, size uint32, data unsafe.Pointer) int32
	extAudioFileRead            func(ext uintptr, frames *uint32, data *audioBufferList) int32
	extAudioFileSeek            func(ext uintptr, frame int64) int32
	extAudioFileDispose         func(ext uintptr) int32

	// The callbacks are made once; AudioFile passes back the id of the
	// source to read. Go pointers cannot go to C, so sources are looked up.
	readProc, sizeProc uintptr

	sourcesMu sync.Mutex
	sources   = map[uintptr]*source{}
	lastID    uintptr
)

func load() error {
	loadOnce.Do(func() {
		lib, err := purego.Dlopen("/System/Library/Frameworks/AudioToolbox.framework/AudioToolbox", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = err
			return
		}
		purego.RegisterLibFunc(&audioFileOpenWithCallbacks, lib, "AudioFileOpenWithCallbacks")
		purego.RegisterLibFunc(&audioFileClose, lib, "AudioFileClose")
		purego.RegisterLibFunc(&extAudioFileWrapAudioFileID, lib, "ExtAudioFileWrapAudioFileID")
		purego.RegisterLibFunc(&extAudioFileGetProperty, lib, "ExtAudioFileGetProperty")
		purego.RegisterLibFunc(&extAudioFileSetProperty, lib, "ExtAudioFileSetProperty")
		purego.RegisterLibFunc(&extAudioFileRead, lib, "ExtAudioFileRead")
		purego.RegisterLibFunc(&extAudioFileSeek, lib, "ExtAudioFileSeek")
		purego.RegisterLibFunc(&extAudioFileDispose, lib, "ExtAudioFileDispose")
		// AudioFile calls them on the goroutine inside an AudioToolbox
		// call, and waits while they wait for the download.
		readProc = purego.NewCallback(func(id uintptr, pos int64, n uint32, buf unsafe.Pointer, got *uint32) int32 {
			*got = 0
			s := lookup(id)
			if s == nil {
				return kAudioFileUnspecifiedError
			}
			k, err := s.readAt(unsafe.Slice((*byte)(buf), n), pos)
			*got = uint32(k)
			if err != nil {
				s.err = err
				return kAudioFileUnspecifiedError
			}
			return 0
		})
		sizeProc = purego.NewCallback(func(id uintptr) int64 {
			if s := lookup(id); s != nil {
				return s.size
			}
			return 0
		})
	})
	return loadErr
}

func lookup(id uintptr) *source {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	return sources[id]
}

// source is the file AudioFile reads, from where the decoder was opened.
type source struct {
	r     io.ReadSeeker
	start int64
	size  int64
	// err is why the last read failed; AudioToolbox only passes on that
	// one did.
	err error
}

// readAt reads p at pos. What is past the end reads short at once, so
// AudioFile looking for boxes after the last one does not wait for the
// download to finish.
func (s *source) readAt(p []byte, pos int64) (int, error) {
	if pos < 0 {
		return 0, errors.New("m4a: negative offset")
	}
	if pos >= s.size {
		return 0, nil
	}
	p = p[:min(int64(len(p)), s.size-pos)]
	if _, err := s.r.Seek(s.start+pos, io.SeekStart); err != nil {
		return 0, err
	}
	n, err := io.ReadFull(s.r, p)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	return n, err
}

// NewDecoder returns a decoder of the MP4 file r, which decodes to its own
// rate, in mono or stereo. Close frees what AudioToolbox holds.
func NewDecoder(r io.ReadSeeker) (audio.Decoder, error) {
	if err := load(); err != nil {
		return nil, err
	}
	start, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	d := &decoder{src: &source{r: r, start: start, size: end - start}, length: -1}
	sourcesMu.Lock()
	lastID++
	d.id = lastID
	sources[d.id] = d.src
	sourcesMu.Unlock()
	if err := d.open(); err != nil {
		d.Close()
		return nil, err
	}
	// One dropped without Close, as audio.DecodeSound drops its decoder,
	// frees AudioToolbox's file when it is collected.
	d.cleanup = runtime.AddCleanup(d, toolbox.release, d.toolbox)
	return d, nil
}

// toolbox is what AudioToolbox holds for a decoder.
type toolbox struct {
	id   uintptr // of the source
	file uintptr // AudioFileID
	ext  uintptr // ExtAudioFileRef
}

func (t toolbox) release() {
	if t.ext != 0 {
		extAudioFileDispose(t.ext)
	}
	if t.file != 0 {
		audioFileClose(t.file)
	}
	sourcesMu.Lock()
	delete(sources, t.id)
	sourcesMu.Unlock()
}

type decoder struct {
	mu sync.Mutex // one AudioToolbox call at a time, and none after Close
	toolbox
	cleanup runtime.Cleanup
	src     *source
	rate    int
	ch      int
	length  int64
	closed  bool
	buf     []float32 // AudioToolbox decodes into it
}

func (d *decoder) open() error {
	d.src.err = nil
	if st := audioFileOpenWithCallbacks(d.id, readProc, 0, sizeProc, 0, kAudioFileM4AType, &d.file); st != 0 {
		d.file = 0
		return d.fail("AudioFileOpenWithCallbacks", st)
	}
	if st := extAudioFileWrapAudioFileID(d.file, false, &d.ext); st != 0 {
		d.ext = 0
		return d.fail("ExtAudioFileWrapAudioFileID", st)
	}
	var file audioStreamBasicDescription
	size := uint32(unsafe.Sizeof(file))
	if st := extAudioFileGetProperty(d.ext, kExtAudioFileProperty_FileDataFormat, &size, unsafe.Pointer(&file)); st != 0 {
		return d.fail("ExtAudioFileGetProperty(FileDataFormat)", st)
	}
	if file.sampleRate <= 0 || file.channelsPerFrame == 0 {
		return errors.New("m4a: no audio track")
	}
	d.rate = int(math.Round(file.sampleRate))
	d.ch = int(min(file.channelsPerFrame, 2))
	client := audioStreamBasicDescription{
		sampleRate:       file.sampleRate,
		formatID:         kAudioFormatLinearPCM,
		formatFlags:      kAudioFormatFlagIsFloat | kAudioFormatFlagIsPacked, // interleaved
		bytesPerPacket:   uint32(4 * d.ch),
		framesPerPacket:  1,
		bytesPerFrame:    uint32(4 * d.ch),
		channelsPerFrame: uint32(d.ch),
		bitsPerChannel:   32,
	}
	if st := extAudioFileSetProperty(d.ext, kExtAudioFileProperty_ClientDataFormat, uint32(unsafe.Sizeof(client)), unsafe.Pointer(&client)); st != 0 {
		return d.fail("ExtAudioFileSetProperty(ClientDataFormat)", st)
	}
	var frames int64
	size = uint32(unsafe.Sizeof(frames))
	if st := extAudioFileGetProperty(d.ext, kExtAudioFileProperty_FileLengthFrames, &size, unsafe.Pointer(&frames)); st == 0 && frames > 0 {
		d.length = frames
	}
	d.buf = make([]float32, readFrames*d.ch)
	return nil
}

// fail reports the read that failed when there was one, so a cancelled
// download reads as cancelled.
func (d *decoder) fail(call string, st int32) error {
	if err := d.src.err; err != nil {
		d.src.err = nil
		return fmt.Errorf("m4a: %s: %w", call, err)
	}
	return fmt.Errorf("m4a: %s failed (OSStatus %d)", call, st)
}

func (d *decoder) SampleRate() int { return d.rate }
func (d *decoder) Channels() int   { return d.ch }
func (d *decoder) Len() int64      { return d.length }

func (d *decoder) Read(dst []float32) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return 0, errors.New("m4a: decoder closed")
	}
	n := uint32(min(len(dst), len(d.buf)) / d.ch)
	if n == 0 {
		return 0, nil
	}
	// The list passed to C points at buf, so buf is pinned for the call.
	var pin runtime.Pinner
	pin.Pin(&d.buf[0])
	list := audioBufferList{
		numberBuffers:  1,
		numberChannels: uint32(d.ch),
		dataByteSize:   n * uint32(4*d.ch),
		data:           unsafe.Pointer(&d.buf[0]),
	}
	d.src.err = nil
	st := extAudioFileRead(d.ext, &n, &list)
	pin.Unpin()
	if st != 0 {
		return 0, d.fail("ExtAudioFileRead", st)
	}
	if n == 0 {
		return 0, io.EOF
	}
	return copy(dst, d.buf[:int(n)*d.ch]), nil
}

func (d *decoder) SetPosition(frame int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return errors.New("m4a: decoder closed")
	}
	d.src.err = nil
	if st := extAudioFileSeek(d.ext, frame); st != 0 {
		return d.fail("ExtAudioFileSeek", st)
	}
	return nil
}

// Close frees the decoder. It leaves the reader open.
func (d *decoder) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	d.cleanup.Stop()
	d.release()
	return nil
}
