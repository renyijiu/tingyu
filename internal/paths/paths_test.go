package paths

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "data")
	t.Setenv("TINGYU_DATA_DIR", want)
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDirDefaultEndsWithTingyu(t *testing.T) {
	t.Setenv("TINGYU_DATA_DIR", "")
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "Tingyu" {
		t.Fatalf("got %q", got)
	}
}

func TestCacheDirFollowsOverride(t *testing.T) {
	data := filepath.Join(t.TempDir(), "data")
	t.Setenv("TINGYU_DATA_DIR", data)
	got, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(data, "cache"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCacheDirDefaultIsOutsideTheData(t *testing.T) {
	t.Setenv("TINGYU_DATA_DIR", "")
	data, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	cache, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(cache) != "Tingyu" || strings.HasPrefix(cache, data) {
		t.Fatalf("cache %q, data %q", cache, data)
	}
}

func TestMoveCacheMovesTheOldCache(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	writeFile(t, filepath.Join(data, "cache", "feeds", "a.json"))
	writeFile(t, filepath.Join(data, "library.json"))
	cache := filepath.Join(root, "caches", "Tingyu")
	if err := MoveCache(data, cache); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, "feeds", "a.json")); err != nil {
		t.Fatalf("cache not moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(data, "cache")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old cache still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(data, "library.json")); err != nil {
		t.Fatalf("library touched: %v", err)
	}
}

func TestMoveCacheRemovesTheOldCacheWhenTheNewOneIsThere(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	writeFile(t, filepath.Join(data, "cache", "old.audio"))
	cache := filepath.Join(root, "caches", "Tingyu")
	writeFile(t, filepath.Join(cache, "new.audio"))
	if err := MoveCache(data, cache); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "cache")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old cache still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cache, "new.audio")); err != nil {
		t.Fatalf("new cache touched: %v", err)
	}
}

func TestMoveCacheLeavesAnOverriddenCacheAlone(t *testing.T) {
	data := t.TempDir()
	writeFile(t, filepath.Join(data, "cache", "a.audio"))
	if err := MoveCache(data, filepath.Join(data, "cache")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "cache", "a.audio")); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
