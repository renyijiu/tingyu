// Package config reads the Podcast Index credentials.
//
// Precedence, highest first: process environment, a .env file in the working
// directory, then config.json in the data directory.
package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	EnvKey    = "PODCASTINDEX_API_KEY"
	EnvSecret = "PODCASTINDEX_API_SECRET"
	FileName  = "config.json"
)

// Config is the local configuration. The key and secret are secrets and
// must not be committed.
type Config struct {
	PodcastIndexKey    string `json:"podcastIndexKey"`
	PodcastIndexSecret string `json:"podcastIndexSecret"`
}

// Ready reports whether Podcast Index can be called.
func (c Config) Ready() bool {
	return strings.TrimSpace(c.PodcastIndexKey) != "" && strings.TrimSpace(c.PodcastIndexSecret) != ""
}

// Load reads config.json from dir, then applies .env and the environment.
// A missing file is not an error.
func Load(dir string) (Config, error) {
	var cfg Config
	path := filepath.Join(dir, FileName)
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}
	if err == nil && len(strings.TrimSpace(string(b))) > 0 {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	}
	fileEnv, err := parseEnvFile(".env")
	if err != nil {
		return cfg, err
	}
	cfg.PodcastIndexKey = pick(os.Getenv(EnvKey), fileEnv[EnvKey], cfg.PodcastIndexKey)
	cfg.PodcastIndexSecret = pick(os.Getenv(EnvSecret), fileEnv[EnvSecret], cfg.PodcastIndexSecret)
	return cfg, nil
}

// WriteTemplate creates an empty config.json when the user does not have one yet.
func WriteTemplate(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, FileName)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	body := "{\n  \"podcastIndexKey\": \"\",\n  \"podcastIndexSecret\": \"\"\n}\n"
	return os.WriteFile(path, []byte(body), 0o600)
}

func pick(env, file, disk string) string {
	if v := strings.TrimSpace(env); v != "" {
		return v
	}
	if v := strings.TrimSpace(file); v != "" {
		return v
	}
	return strings.TrimSpace(disk)
}

// parseEnvFile reads KEY=VALUE lines. Missing file yields an empty map.
// Values already in the process environment are not returned, so the
// caller can treat the map as lower priority than the environment.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if key != "" {
			out[key] = val
		}
	}
	return out, sc.Err()
}
