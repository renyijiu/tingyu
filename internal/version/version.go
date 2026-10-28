// Package version is the app version shared by the User-Agent and mygo.json.
package version

// Version is the Tingyu release shown to Podcast Index and in the about line.
const Version = "0.1.0"

// UserAgent identifies this app. Podcast Index rejects requests that do not
// send a specific product name. The repository is named without its host:
// the CDN in front of 小宇宙's feeds (feed.xyzfm.space) answers 403 to any
// User-Agent that contains "github.com".
func UserAgent() string {
	return "Tingyu/" + Version + " (renyijiu/tingyu)"
}
