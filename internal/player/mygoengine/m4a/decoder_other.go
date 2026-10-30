//go:build !darwin

package m4a

import (
	"errors"
	"io"

	"github.com/egoist/mygo/audio"
)

// NewDecoder fails: only macOS has AudioToolbox.
func NewDecoder(io.ReadSeeker) (audio.Decoder, error) {
	return nil, errors.New("m4a: decoding needs macOS AudioToolbox")
}
