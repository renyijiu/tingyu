package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DiskCache keeps each show's last episode list as a JSON file in Dir. It
// keeps the Max shows opened most recently and forgets the rest.
type DiskCache struct {
	Dir string
	Max int
}

const defaultMaxShows = 60

func (d *DiskCache) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(d.Dir, hex.EncodeToString(sum[:16])+".json")
}

// Load reads the entry for key. Reading it counts as using it.
func (d *DiskCache) Load(key string) (Entry, bool) {
	p := d.path(key)
	b, err := os.ReadFile(p)
	if err != nil {
		return Entry{}, false
	}
	var e Entry
	if err := json.Unmarshal(b, &e); err != nil || len(e.Episodes) == 0 {
		return Entry{}, false
	}
	now := time.Now()
	_ = os.Chtimes(p, now, now)
	return e, true
}

// Save writes the entry for key in one piece, then forgets the shows used
// longest ago beyond Max.
func (d *DiskCache) Save(key string, e Entry) error {
	if err := os.MkdirAll(d.Dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(d.Dir, "feed-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), d.path(key)); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	d.prune()
	return nil
}

func (d *DiskCache) prune() {
	limit := d.Max
	if limit <= 0 {
		limit = defaultMaxShows
	}
	entries, err := os.ReadDir(d.Dir)
	if err != nil {
		return
	}
	type file struct {
		name string
		used time.Time
	}
	var files []file
	for _, de := range entries {
		if !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		if info, err := de.Info(); err == nil {
			files = append(files, file{de.Name(), info.ModTime()})
		}
	}
	if len(files) <= limit {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].used.After(files[j].used) })
	for _, f := range files[limit:] {
		os.Remove(filepath.Join(d.Dir, f.name))
	}
}
