package version

import (
	"strings"
	"testing"
)

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
