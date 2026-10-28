package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/config"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/podcastindex"
)

func TestSettingsSaveAppliesAndFailureKeepsActiveConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(config.EnvKey, "")
	t.Setenv(config.EnvSecret, "")
	dir := t.TempDir()
	a := New(nil, &catalog.Catalog{ITunes: fakeIT{}}, nil)
	a.Configure(dir, config.Settings{})
	a.Sync = true
	if a.Catalog.Index != nil || a.Notice == "" {
		t.Fatal("empty settings should use the fallback")
	}
	if err := a.SaveSettings(config.Config{PodcastIndexKey: " key ", PodcastIndexSecret: " secret "}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Key") != "key" || r.Header.Get("Authorization") != podcastindex.AuthHeader("key", "secret", 1234) {
			t.Error("search did not authenticate with the saved credentials")
		}
		_, _ = w.Write([]byte(`{"status":true,"feeds":[{"id":1,"title":"New source"}]}`))
	}))
	defer srv.Close()
	client := a.Catalog.Index.(*podcastindex.Client)
	client.BaseURL = srv.URL
	client.Now = func() time.Time { return time.Unix(1234, 0) }
	a.Query = "test"
	a.Search()
	if a.SearchErr != "" || a.ResultSource != model.SourcePodcastIndex || len(a.Results) != 1 || a.Notice != "" {
		t.Fatal("saved settings did not immediately enable Podcast Index")
	}
	reloaded, err := config.LoadSettings(dir)
	if err != nil || reloaded.Saved != a.Settings.Saved {
		t.Fatal("saved settings did not survive reload")
	}
	oldCatalog, oldSettings := a.Catalog, a.Settings
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.settingsDir = blocked
	if err := a.SaveSettings(config.Config{PodcastIndexKey: "other", PodcastIndexSecret: "other"}); err == nil {
		t.Fatal("expected save failure")
	}
	if a.Catalog != oldCatalog || a.Settings != oldSettings {
		t.Fatal("failed save changed the active settings")
	}
	a.settingsDir = dir
	if err := a.SaveSettings(config.Config{}); err != nil {
		t.Fatal(err)
	}
	if a.Catalog.Index != nil || a.Catalog.ITunes == nil || a.Notice == "" {
		t.Fatal("clearing settings did not restore the fallback")
	}
}

func TestSettingsChangeDiscardsPendingSearch(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	updates := make(chan func(), 1)
	old := &catalog.Catalog{Index: searchIndexFunc(func(context.Context, string) ([]model.Podcast, error) {
		close(started)
		<-release
		return []model.Podcast{{Title: "Old source"}}, nil
	})}
	a := New(nil, old, nil)
	a.settingsDir = t.TempDir()
	a.OnUI = func(fn func()) { updates <- fn }
	a.Query = "test"
	a.Search()
	<-started
	err := a.SaveSettings(config.Config{PodcastIndexKey: "new", PodcastIndexSecret: "new"})
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case update := <-updates:
		update()
	case <-time.After(2 * time.Second):
		t.Fatal("pending search did not finish")
	}
	if a.Catalog == old || old.Index == nil || a.Searching || len(a.Results) != 0 || a.ResultSource != "" {
		t.Fatal("old request mutated the new search state")
	}
}
