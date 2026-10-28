package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/config"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
	"github.com/renyijiu/tingyu/internal/podcastindex"
)

func TestSettingsEntryStaysBelowLibrary(t *testing.T) {
	for _, count := range []int{0, 30} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			a := app.New(nil, &catalog.Catalog{}, &player.Controller{})
			a.Query = "保留搜索"
			v := &View{App: a}
			for i := 0; i < count; i++ {
				a.Subs = append(a.Subs, model.Podcast{ID: fmt.Sprint(i), Title: fmt.Sprintf("节目 %d", i)})
			}
			if count > 0 {
				for i := 0; i < 6; i++ {
					v.recent = append(v.recent, recentPlay{Episode: model.Episode{ID: fmt.Sprint(i), Title: fmt.Sprintf("最近单集 %d", i)}})
				}
			}
			tt := ui.NewTester(v.Build, 860, 560)
			entry, ok := tt.Find("设置")
			if !ok {
				t.Fatal("settings utility missing")
			}
			library, _ := tt.Find("我的订阅")
			transport, _ := tt.Find("没有正在播放的单集")
			if entry.Y < 400 || entry.Y <= library.Y || entry.Y+entry.H > transport.Y {
				t.Fatalf("settings entry is not pinned above the player: entry=%+v library=%+v player=%+v", entry, library, transport)
			}
			tt.Scroll(130, 300, 0, 500)
			after, _ := tt.Find("设置")
			if after != entry {
				t.Fatal("library scrolling moved the settings utility")
			}
			if err := tt.Click("设置"); err != nil {
				t.Fatal(err)
			}
			if !v.settingsOpen || a.Query != "保留搜索" {
				t.Fatal("settings did not open independently of browsing")
			}
			v.settingsDraft.PodcastIndexKey = "unsaved"
			v.OpenSettings()
			if v.settingsDraft.PodcastIndexKey != "unsaved" {
				t.Fatal("opening settings again discarded the active draft")
			}
		})
	}
}

func TestSettingsDialogSaveCancelAndFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(config.EnvKey, "")
	t.Setenv(config.EnvSecret, "")
	dir := t.TempDir()
	a := app.New(nil, &catalog.Catalog{}, &player.Controller{})
	a.Configure(dir, config.Settings{})
	v := &View{App: a}
	tt := ui.NewTester(v.Build, 860, 560)
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	click("设置")
	click("API Key")
	tt.Type("test-key")
	click("保存")
	if !tt.HasText("请同时填写") || a.Settings.Saved.Ready() {
		t.Fatal("incomplete credentials were not rejected in the dialog")
	}
	click("API Secret")
	tt.Type("test-secret")
	if tt.HasText("无法保存设置") {
		t.Fatal("editing did not clear the stale error")
	}
	click("显示密钥")
	if !v.showCredentials {
		t.Fatal("credential visibility toggle did not work")
	}
	click("保存")
	if !tt.HasText("已保存") || !a.Settings.Saved.Ready() || v.showCredentials || a.Catalog.Index == nil {
		t.Fatalf("save: ready=%v draftReady=%v visible=%v index=%v error=%q texts=%q", a.Settings.Saved.Ready(), v.settingsDraft.Ready(), v.showCredentials, a.Catalog.Index != nil, v.settingsErr, tt.Texts())
	}
	saved, err := config.Load(dir)
	if err != nil || saved.PodcastIndexKey != "test-key" || saved.PodcastIndexSecret != "test-secret" {
		t.Fatal("dialog values did not persist")
	}
	click("API Key")
	tt.Type("-unsaved")
	click("关闭")
	click("设置")
	if v.settingsDraft != saved {
		t.Fatal("closing the dialog saved an uncommitted draft")
	}
	path := filepath.Join(dir, config.FileName)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	click("保存")
	if !tt.HasText("无法保存设置") || tt.HasText("已保存") || a.Settings.Saved != saved {
		t.Fatal("failed write was not surfaced or changed active credentials")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	click("保存")
	if tt.HasText("无法保存设置") || !tt.HasText("已保存") {
		t.Fatal("retry did not clear the save error")
	}
}

func TestSettingsExplainsOverridesAtMinimumWindowSize(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(config.EnvKey, "")
	t.Setenv(config.EnvSecret, "")
	for _, mode := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(mode.name, func(t *testing.T) {
			dir := t.TempDir()
			a := app.New(nil, &catalog.Catalog{}, &player.Controller{})
			a.Configure(dir, config.Settings{Overrides: config.Config{PodcastIndexKey: "launch-key", PodcastIndexSecret: "launch-secret"}})
			v := &View{App: a}
			tt := ui.NewTester(v.Build, 860, 560)
			tt.SetDark(mode.dark)
			if err := tt.Click("设置"); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText("密钥优先生效") || v.settingsDraft != (config.Config{}) {
				t.Fatal("launch overrides were not explained or were copied into the editor")
			}
			if err := tt.Click("保存"); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText("已保存，将优先使用 Podcast Index") {
				t.Fatal("save did not reflect the effective search provider")
			}
			client := a.Catalog.Index.(*podcastindex.Client)
			if client.Key != "launch-key" || client.Secret != "launch-secret" {
				t.Fatal("save replaced launch overrides")
			}
			stored, err := config.Load(dir)
			if err != nil || stored != (config.Config{}) {
				t.Fatal("saving the empty editor persisted launch credentials")
			}
		})
	}
}
