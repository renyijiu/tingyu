// Package m4a decodes AAC in MP4 files (.m4a) for package audio, with the
// AudioToolbox that ships with macOS. Import it for its side effect, as
// audio/mp3 is imported:
//
//	import _ "github.com/renyijiu/tingyu/internal/player/mygoengine/m4a"
//
// AudioToolbox reads the file through callbacks, so a file still
// downloading can play: a read past what has arrived waits for it. When
// the index (the moov box) comes first, as on 小宇宙, playback starts with
// the first bytes; when it comes last, opening waits for the whole file.
//
// Elsewhere than macOS the format is recognized but does not open.
package m4a

import (
	"io"

	"github.com/egoist/mygo/audio"
)

func init() {
	// Every MP4 file starts with a box of type ftyp, after its size.
	audio.RegisterFormat("m4a", "????ftyp", func(r io.ReadSeeker) (audio.Decoder, error) {
		return NewDecoder(r)
	})
}
