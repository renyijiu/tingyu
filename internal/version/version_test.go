package version

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The app's bundle takes its version from mygo.json, and a release tag
// must match it; both have to say the same as Version.
func TestVersionMatchesMygoJSON(t *testing.T) {
	b, err := os.ReadFile("../../mygo.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Version != Version {
		t.Fatalf("mygo.json says %q, version.Version %q: change both", cfg.Version, Version)
	}
}

func TestUserAgentNamesTheApp(t *testing.T) {
	ua := UserAgent()
	if !strings.HasPrefix(ua, "Tingyu/"+Version+" ") {
		t.Fatalf("user agent %q does not name the app and its version", ua)
	}
}

// Most 小宇宙 shows are hosted on feed.xyzfm.space, whose CDN refuses any
// User-Agent with github.com in it, whatever its case.
func TestUserAgentPassesXiaoyuzhouFeeds(t *testing.T) {
	if ua := UserAgent(); strings.Contains(strings.ToLower(ua), "github.com") {
		t.Fatalf("user agent %q is refused by feed.xyzfm.space", ua)
	}
}
