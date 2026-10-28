package app

import (
	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/config"
	"github.com/renyijiu/tingyu/internal/podcastindex"
)

// Configure installs the settings loaded at startup, on the UI thread.
func (a *App) Configure(dir string, settings config.Settings) {
	a.settingsDir = dir
	a.Settings = settings
	a.applySettings()
}

// SaveSettings activates credentials only after they have been persisted.
func (a *App) SaveSettings(cfg config.Config) error {
	cfg = cfg.Normalized()
	if err := config.Save(a.settingsDir, cfg); err != nil {
		return err
	}
	a.Settings.Saved = cfg
	a.applySettings()
	a.searchGen++
	a.Searching = false
	a.SearchErr = ""
	a.Results = nil
	a.ResultSource = ""
	return nil
}

func (a *App) applySettings() {
	next := catalog.Catalog{}
	if a.Catalog != nil {
		next = *a.Catalog
	}
	next.Index = nil
	cfg := a.Settings.Effective()
	a.Notice = "当前使用 iTunes 搜索，可在「设置」中配置 Podcast Index。"
	if cfg.Ready() {
		next.Index = &podcastindex.Client{Key: cfg.PodcastIndexKey, Secret: cfg.PodcastIndexSecret}
		a.Notice = ""
	}
	a.Catalog = &next
}
