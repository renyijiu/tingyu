package config

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestSaveSettingsRoundTripAndOverrides(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(EnvKey, "launch-key")
	t.Setenv(EnvSecret, "")
	if err := os.WriteFile(".env", []byte(EnvSecret+"=launch-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	want := Config{PodcastIndexKey: "local-key", PodcastIndexSecret: "local-secret"}
	if err := Save(dir, Config{PodcastIndexKey: " local-key ", PodcastIndexSecret: " local-secret\n"}); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSettings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Saved != want || !s.HasOverrides() || s.Effective().PodcastIndexKey != "launch-key" || s.Effective().PodcastIndexSecret != "launch-secret" {
		t.Fatal("saved credentials or launch overrides were not preserved")
	}
	info, err := os.Stat(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("credential file mode: %v", info.Mode())
	}
	for _, invalid := range []Config{{PodcastIndexKey: "only-key"}, {PodcastIndexSecret: "only-secret"}} {
		if err := Save(dir, invalid); err == nil {
			t.Fatal("accepted incomplete credentials")
		}
		after, err := LoadSettings(dir)
		if err != nil || after.Saved != want {
			t.Fatal("invalid save changed existing credentials")
		}
	}
	if err := Save(dir, Config{}); err != nil {
		t.Fatal(err)
	}
	s, err = LoadSettings(dir)
	if err != nil || s.Saved.Ready() || !s.Effective().Ready() {
		t.Fatal("clearing local credentials should retain launch overrides")
	}
	t.Setenv(EnvKey, "")
	if err := os.Remove(".env"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil || cfg.Ready() {
		t.Fatal("cleared credentials did not persist across reload")
	}
}

func TestSaveFailureCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("existing data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, Config{PodcastIndexKey: "key", PodcastIndexSecret: "secret"}); err == nil {
		t.Fatal("expected replacement to fail")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != FileName {
		t.Fatalf("temporary credentials remained after failure: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(path, "keep")); err != nil || string(data) != "existing data" {
		t.Fatal("failed save changed the existing destination")
	}
}
