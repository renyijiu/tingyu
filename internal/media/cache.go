// Package media opens remote audio as an io.ReadSeeker.
//
// Playback can start before the download finishes. Bytes that are not on
// disk yet block the reader. A finished file is reused from the cache, which
// is how seek and resume stay local. This is a play cache, not a download
// manager: it keeps the episodes played most recently, up to MaxCache.
package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/renyijiu/tingyu/internal/version"
)

// Open returns a ReadSeeker for audioURL, downloading into cacheDir.
// The caller must Close it.
func Open(ctx context.Context, client *http.Client, audioURL, cacheDir string) (io.ReadSeekCloser, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	name := hashName(audioURL)
	final := filepath.Join(cacheDir, name)
	if st, err := os.Stat(final); err == nil && st.Size() > 0 {
		f, err := os.Open(final)
		if err != nil {
			return nil, err
		}
		// Playing it again counts as using it, so it is pruned last.
		now := time.Now()
		_ = os.Chtimes(final, now, now)
		return f, nil
	}
	f, err := os.CreateTemp(cacheDir, name+".part-")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	r := &Reader{
		file:   f,
		final:  final,
		cancel: cancel,
		size:   -1,
	}
	r.cond = sync.NewCond(&r.mu)
	if client == nil {
		client = &http.Client{}
	}
	go r.download(ctx, client, audioURL)
	return r, nil
}

func hashName(audioURL string) string {
	sum := sha256.Sum256([]byte(audioURL))
	return hex.EncodeToString(sum[:]) + audioExt
}

// Reader is a growing file. Read blocks until the download reaches the offset.
type Reader struct {
	file   *os.File
	final  string
	cancel context.CancelFunc

	mu     sync.Mutex
	cond   *sync.Cond
	ready  int64
	size   int64
	pos    int64
	done   bool
	err    error
	closed bool
}

func (r *Reader) download(ctx context.Context, client *http.Client, audioURL string) {
	err := r.fetch(ctx, client, audioURL)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
	r.done = true
	if r.err == nil && !r.closed {
		_ = r.file.Sync()
		// Rename keeps the open descriptor on the same inode, so playback
		// can continue while the next play finds the finished file.
		if os.Rename(r.file.Name(), r.final) == nil {
			_ = Prune(filepath.Dir(r.final), MaxCache, r.final)
		}
	} else if r.file != nil {
		name := r.file.Name()
		_ = os.Remove(name)
	}
	r.cond.Broadcast()
}

func (r *Reader) fetch(ctx context.Context, client *http.Client, audioURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", version.UserAgent())
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("media: HTTP %d", resp.StatusCode)
	}
	size := int64(-1)
	if resp.ContentLength >= 0 {
		size = resp.ContentLength
	}
	r.mu.Lock()
	r.size = size
	r.cond.Broadcast()
	r.mu.Unlock()

	buf := make([]byte, 32*1024)
	off := int64(0)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := r.file.WriteAt(buf[:n], off); werr != nil {
				return werr
			}
			off += int64(n)
			r.mu.Lock()
			r.ready = off
			r.cond.Broadcast()
			closed := r.closed
			r.mu.Unlock()
			if closed {
				return context.Canceled
			}
		}
		if rerr == io.EOF {
			r.mu.Lock()
			if r.size < 0 {
				r.size = off
			}
			r.mu.Unlock()
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

// Read reads at the current offset, waiting for bytes still downloading.
func (r *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	r.mu.Lock()
	for {
		if r.closed {
			r.mu.Unlock()
			return 0, os.ErrClosed
		}
		if r.pos < r.ready {
			pos := r.pos
			n := r.ready - r.pos
			if int64(len(p)) < n {
				n = int64(len(p))
			}
			r.mu.Unlock()
			got, err := r.file.ReadAt(p[:n], pos)
			r.mu.Lock()
			if got > 0 {
				r.pos = pos + int64(got)
				r.mu.Unlock()
				return got, nil
			}
			if err != nil && !errors.Is(err, io.EOF) {
				r.mu.Unlock()
				return 0, err
			}
			continue
		}
		if r.done && r.pos >= r.ready {
			err := r.err
			r.mu.Unlock()
			if err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		if r.err != nil && r.done {
			err := r.err
			r.mu.Unlock()
			return 0, err
		}
		r.cond.Wait()
	}
}

// Seek moves the offset. Seeking to the end uses Content-Length when the
// server sent one, without waiting for the body.
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = r.pos + offset
	case io.SeekEnd:
		for r.size < 0 && !r.done {
			r.cond.Wait()
		}
		if r.size < 0 {
			if r.err != nil {
				return 0, r.err
			}
			return 0, fmt.Errorf("media: unknown length")
		}
		next = r.size + offset
	default:
		return 0, fmt.Errorf("media: invalid whence %d", whence)
	}
	if next < 0 {
		return 0, fmt.Errorf("media: negative offset")
	}
	r.pos = next
	return next, nil
}

// Close stops the download and closes the file.
func (r *Reader) Close() error {
	r.mu.Lock()
	r.closed = true
	r.cond.Broadcast()
	r.mu.Unlock()
	r.cancel()
	return r.file.Close()
}

// MaxCache is how many bytes of played audio the cache keeps: some twenty
// to forty episodes.
const MaxCache = 2 << 30

const audioExt = ".audio"

// Prune removes the audio used longest ago until what is left fits in
// limit. keep, the file just played, stays whatever its size.
func Prune(dir string, limit int64, keep string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type file struct {
		path string
		size int64
		used time.Time
	}
	var files []file
	var total int64
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), audioExt) {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		files = append(files, file{filepath.Join(dir, de.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].used.Before(files[j].used) })
	for _, f := range files {
		if total <= limit {
			break
		}
		if f.path == keep {
			continue
		}
		if err := os.Remove(f.path); err == nil {
			total -= f.size
		}
	}
	return nil
}

// Tidy runs at launch, before anything plays: it removes the downloads a
// quit or crash cut short, which no one will finish, and prunes the cache
// to MaxCache.
func Tidy(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, de := range entries {
		if !de.IsDir() && strings.Contains(de.Name(), audioExt+".part-") {
			_ = os.Remove(filepath.Join(dir, de.Name()))
		}
	}
	return Prune(dir, MaxCache, "")
}
