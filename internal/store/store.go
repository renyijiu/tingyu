// Package store persists subscriptions and playback progress as one JSON file.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/renyijiu/tingyu/internal/model"
)

const fileName = "library.json"

// Store is the user's library.
type Store struct {
	mu   sync.Mutex
	path string
	data fileData
}

type fileData struct {
	Subscriptions []model.Podcast           `json:"subscriptions"`
	Progress      map[string]model.Progress `json:"progress,omitempty"`
}

// Open loads the library in dir, creating the directory if needed.
// A missing file starts empty. Corrupt JSON is returned as an error so a
// bad write is not replaced with an empty library.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{
		path: filepath.Join(dir, fileName),
		data: fileData{Progress: map[string]model.Progress{}},
	}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, err
	}
	if s.data.Progress == nil {
		s.data.Progress = map[string]model.Progress{}
	}
	return s, nil
}

// Subscriptions returns a copy of the subscribed shows, newest first.
func (s *Store) Subscriptions() []model.Podcast {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.Podcast, len(s.data.Subscriptions))
	copy(out, s.data.Subscriptions)
	return out
}

// IsSubscribed reports whether id is in the library.
func (s *Store) IsSubscribed(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.index(id) >= 0
}

// Subscribe adds or updates a show. The original subscription time is kept.
func (s *Store) Subscribe(p model.Podcast) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.EnsureIDs()
	if p.SubscribedAt.IsZero() {
		p.SubscribedAt = time.Now().UTC()
	}
	if i := s.index(p.ID); i >= 0 {
		if !s.data.Subscriptions[i].SubscribedAt.IsZero() {
			p.SubscribedAt = s.data.Subscriptions[i].SubscribedAt
		}
		s.data.Subscriptions[i] = p
	} else {
		s.data.Subscriptions = append([]model.Podcast{p}, s.data.Subscriptions...)
	}
	return s.saveLocked()
}

// Unsubscribe removes a show. Removing one that is not there succeeds.
func (s *Store) Unsubscribe(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.index(id)
	if i < 0 {
		return nil
	}
	s.data.Subscriptions = append(s.data.Subscriptions[:i], s.data.Subscriptions[i+1:]...)
	return s.saveLocked()
}

// Progress returns the saved position for an episode key.
func (s *Store) Progress(key string) (model.Progress, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.data.Progress[key]
	return p, ok
}

// SaveProgress writes the position for an episode key.
func (s *Store) SaveProgress(key string, p model.Progress) error {
	if key == "" {
		return errors.New("store: empty episode key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Progress == nil {
		s.data.Progress = map[string]model.Progress{}
	}
	if p.Updated.IsZero() {
		p.Updated = time.Now().UTC()
	}
	s.data.Progress[key] = p
	return s.saveLocked()
}

func (s *Store) index(id string) int {
	for i, p := range s.data.Subscriptions {
		if p.ID == id {
			return i
		}
	}
	return -1
}

func (s *Store) saveLocked() error {
	if s.data.Progress == nil {
		s.data.Progress = map[string]model.Progress{}
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
