// Package paths locates Tingyu's on-disk data.
//
// On macOS that is ~/Library/Application Support/Tingyu, which is what the
// app documents. Other systems use the user config directory so tests and
// Linux builds have a place to write. TINGYU_DATA_DIR overrides both.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// Dir is the directory that holds config.json, library.json and the play cache.
func Dir() (string, error) {
	if d := os.Getenv("TINGYU_DATA_DIR"); d != "" {
		return d, nil
	}
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "Tingyu"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Tingyu"), nil
}
