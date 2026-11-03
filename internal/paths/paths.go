// Package paths locates Tingyu's on-disk data.
//
// On macOS that is ~/Library/Application Support/Tingyu, which is what the
// app documents, and what can be downloaded again is in
// ~/Library/Caches/Tingyu, which Time Machine leaves out. Other systems use
// the user config and cache directories so tests and Linux builds have a
// place to write. TINGYU_DATA_DIR overrides both: the cache is then its
// cache folder, so one variable keeps a test apart from the real app.
package paths

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const envDir = "TINGYU_DATA_DIR"

// Dir is the directory that holds config.json and library.json.
func Dir() (string, error) {
	if d := os.Getenv(envDir); d != "" {
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

// CacheDir is the directory that holds played audio, episode lists and
// covers: everything the app can download again.
func CacheDir() (string, error) {
	if d := os.Getenv(envDir); d != "" {
		return filepath.Join(d, "cache"), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Tingyu"), nil
}

// MoveCache moves the cache earlier versions kept in the data directory
// to cacheDir. When it cannot be moved, as when cacheDir is already there,
// the old one is removed: it holds nothing that cannot be downloaded again,
// and left behind it would take space no one sees.
func MoveCache(dataDir, cacheDir string) error {
	old := filepath.Join(dataDir, "cache")
	if filepath.Clean(old) == filepath.Clean(cacheDir) {
		return nil
	}
	if _, err := os.Stat(old); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if _, err := os.Stat(cacheDir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
			return err
		}
		if os.Rename(old, cacheDir) == nil {
			return nil
		}
	}
	return os.RemoveAll(old)
}
