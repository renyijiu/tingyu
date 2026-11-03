package store

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/renyijiu/tingyu/internal/model"
)

func TestSubscriptionsPersist(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	show := model.Podcast{Title: "故事会", FeedURL: "https://example.com/a.xml", Author: "甲"}
	show.EnsureIDs()
	if err := s.Subscribe(show); err != nil {
		t.Fatal(err)
	}
	other := model.Podcast{Title: "Serial", FeedURL: "https://example.com/b.xml"}
	other.EnsureIDs()
	if err := s.Subscribe(other); err != nil {
		t.Fatal(err)
	}
	if !s.IsSubscribed(show.ID) || len(s.Subscriptions()) != 2 {
		t.Fatalf("%+v", s.Subscriptions())
	}
	if s.Subscriptions()[0].ID != other.ID {
		t.Fatal("newest subscription should be first")
	}
	// Updating keeps the original timestamp and does not duplicate.
	firstAt := s.Subscriptions()[1].SubscribedAt
	show.Title = "故事会（更新）"
	if err := s.Subscribe(show); err != nil {
		t.Fatal(err)
	}
	got := find(t, s.Subscriptions(), show.ID)
	if got.Title != "故事会（更新）" || !got.SubscribedAt.Equal(firstAt) {
		t.Fatalf("%+v", got)
	}
	if len(s.Subscriptions()) != 2 {
		t.Fatalf("duplicated: %+v", s.Subscriptions())
	}
	if err := s.Unsubscribe(show.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Unsubscribe(show.ID); err != nil {
		t.Fatal(err)
	}

	re, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if re.IsSubscribed(show.ID) || len(re.Subscriptions()) != 1 || re.Subscriptions()[0].Title != "Serial" {
		t.Fatalf("reloaded %+v", re.Subscriptions())
	}
	if _, err := os.Stat(filepath.Join(dir, "library.json")); err != nil {
		t.Fatal(err)
	}
}

func TestProgress(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := model.EpisodeKey("https://example.com/a.xml", "ep", "https://cdn/a.mp3")
	if _, ok := s.Progress(key); ok {
		t.Fatal("unexpected progress")
	}
	want := model.Progress{Position: 90 * time.Second, Duration: 30 * time.Minute, Updated: time.Unix(1_700_000_000, 0).UTC()}
	if err := s.SaveProgress(key, want); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Progress(key)
	if !ok || got.Position != want.Position || got.Duration != want.Duration {
		t.Fatalf("%+v %v", got, ok)
	}
	dir := s.path
	re, err := Open(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	got, ok = re.Progress(key)
	if !ok || got.Position != want.Position {
		t.Fatalf("reloaded %+v", got)
	}
}

func TestCorruptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "library.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("expected error")
	}
}

func TestFailedWriteKeepsLibraryUnchanged(t *testing.T) {
	for _, operation := range []string{"subscribe", "update", "unsubscribe", "progress"} {
		t.Run(operation, func(t *testing.T) {
			dir := t.TempDir()
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			show := model.Podcast{ID: "original", Title: "Original"}
			if err := s.Subscribe(show); err != nil {
				t.Fatal(err)
			}
			progress := model.Progress{Position: time.Minute}
			if err := s.SaveProgress("episode", progress); err != nil {
				t.Fatal(err)
			}
			before := s.Subscriptions()
			beforeProgress, _ := s.Progress("episode")
			// A directory at the temporary file path deterministically fails writes.
			if err := os.Mkdir(s.path+".tmp", 0o700); err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "subscribe":
				err = s.Subscribe(model.Podcast{ID: "new", Title: "New"})
			case "update":
				show.Title = "Changed"
				err = s.Subscribe(show)
			case "unsubscribe":
				err = s.Unsubscribe(show.ID)
			case "progress":
				progress.Position = 2 * time.Minute
				err = s.SaveProgress("episode", progress)
			}
			if err == nil {
				t.Fatal("expected a write failure")
			}
			if got := s.Subscriptions(); !reflect.DeepEqual(got, before) {
				t.Errorf("failed write changed subscriptions: got %+v, want %+v", got, before)
			}
			if got, _ := s.Progress("episode"); got != beforeProgress {
				t.Errorf("failed write changed progress: got %+v, want %+v", got, beforeProgress)
			}
			if err := os.Remove(s.path + ".tmp"); err != nil {
				t.Fatal(err)
			}
			// A later successful write must not commit a previously failed mutation.
			if err := s.SaveProgress("other", model.Progress{Position: time.Second}); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if got := reopened.Subscriptions(); !reflect.DeepEqual(got, before) {
				t.Errorf("later write persisted failed subscription change: %+v", got)
			}
			if got, _ := reopened.Progress("episode"); got != beforeProgress {
				t.Errorf("later write persisted failed progress change: %+v", got)
			}
		})
	}
}

func find(t *testing.T, list []model.Podcast, id string) model.Podcast {
	t.Helper()
	for _, p := range list {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("missing %s", id)
	return model.Podcast{}
}

func TestRecentPersistsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	show := model.Podcast{ID: "feed:a", Title: "故事会", Description: "很长的简介"}
	play := func(id string) model.Play {
		return model.Play{Show: show, Episode: model.Episode{ID: id, Title: "第 " + id + " 集", Description: "单集简介"}}
	}
	for i := 0; i < MaxRecent+2; i++ {
		if err := s.AddRecent(play(fmt.Sprint(i))); err != nil {
			t.Fatal(err)
		}
	}
	// Playing one again moves it first instead of listing it twice.
	if err := s.AddRecent(play("4")); err != nil {
		t.Fatal(err)
	}
	re, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range re.Recent() {
		ids = append(ids, p.Episode.ID)
		if p.Show.Description != "" || p.Episode.Description != "" {
			t.Errorf("descriptions kept: %+v", p)
		}
		if p.Show.Title != "故事会" {
			t.Errorf("show lost: %+v", p.Show)
		}
	}
	if want := []string{"4", "7", "6", "5", "3", "2"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("recent %v, want %v", ids, want)
	}
	if err := s.AddRecent(model.Play{}); err == nil {
		t.Fatal("empty episode accepted")
	}
}
