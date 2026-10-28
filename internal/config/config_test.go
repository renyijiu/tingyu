package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	body := `{"podcastIndexKey":"from-file","podcastIndexSecret":"file-secret"}`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvKey, "from-env")
	t.Setenv(EnvSecret, "")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PodcastIndexKey != "from-env" {
		t.Fatalf("key: %q", cfg.PodcastIndexKey)
	}
	if cfg.PodcastIndexSecret != "file-secret" {
		t.Fatalf("secret: %q", cfg.PodcastIndexSecret)
	}
	if !cfg.Ready() {
		t.Fatal("expected ready")
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Setenv(EnvKey, "")
	t.Setenv(EnvSecret, "")
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ready() {
		t.Fatal("empty config should not be ready")
	}
}

func TestDotEnvFillsGaps(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	env := "PODCASTINDEX_API_KEY=dotenv-key\nPODCASTINDEX_API_SECRET=\"dotenv-secret\"\n# comment\n\n"
	if err := os.WriteFile(".env", []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvKey, "")
	t.Setenv(EnvSecret, "")
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PodcastIndexKey != "dotenv-key" || cfg.PodcastIndexSecret != "dotenv-secret" {
		t.Fatalf("%+v", cfg)
	}
}

func TestWriteTemplateOnce(t *testing.T) {
	dir := t.TempDir()
	if err := WriteTemplate(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"podcastIndexKey":"keep","podcastIndexSecret":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteTemplate(dir); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PodcastIndexKey != "keep" {
		t.Fatalf("template overwrote config: %+v", cfg)
	}
}
