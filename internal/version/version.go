// Package version is the app version shared by the User-Agent and mygo.json.
package version

// Version is the Tingyu release shown to Podcast Index and in the about line.
const Version = "0.1.0"

// UserAgent identifies this app. Podcast Index rejects requests that do not
// send a specific product name.
func UserAgent() string {
	return "Tingyu/" + Version + " (+https://github.com/renyijiu/tingyu)"
}
