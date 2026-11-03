// Command notices writes resources/THIRD_PARTY_NOTICES.txt: the licenses
// of Tingyu, of Go and of every module linked into the macOS app, which
// their licenses ask a binary to carry. mygo ships everything in resources/
// inside the app, so the file lands in Contents/Resources:
//
//	go run ./tools/notices
//
// Run it after changing go.mod; CI checks the file is current.
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const out = "resources/THIRD_PARTY_NOTICES.txt"

// files are what a module ships its terms in, in the order written.
var files = []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "NOTICE", "PATENTS"}

func main() {
	var b bytes.Buffer
	b.WriteString("听宇 Tingyu 及其包含的第三方软件的许可声明。\n")
	b.WriteString("Licenses of Tingyu and of the third-party software it includes.\n")
	section(&b, "Tingyu", ".")
	section(&b, "The Go programming language", strings.TrimSpace(goCmd("env", "GOROOT")))

	list := goCmd("list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}}\t{{.Dir}}{{end}}{{end}}", ".")
	lines := strings.Split(strings.TrimSpace(list), "\n")
	slices.Sort(lines)
	for _, line := range slices.Compact(lines) {
		if line == "" {
			continue
		}
		name, dir, _ := strings.Cut(line, "\t")
		section(&b, name, dir)
	}
	if err := os.WriteFile(out, b.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
}

// goCmd runs the go command for the macOS app and returns what it prints.
func goCmd(args ...string) string {
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOOS=darwin", "GOARCH=arm64")
	cmd.Stderr = os.Stderr
	printed, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	return string(printed)
}

// section writes the terms found in dir under name. A module without any
// is an error: its terms would be missing from the app.
func section(b *bytes.Buffer, name, dir string) {
	found := false
	for _, f := range files {
		text, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		if !found {
			fmt.Fprintf(b, "\n%s\n%s\n", strings.Repeat("=", 72), name)
			found = true
		}
		fmt.Fprintf(b, "%s\n-- %s --\n\n%s\n", strings.Repeat("-", 72), f, bytes.TrimSpace(text))
	}
	if !found {
		log.Fatalf("%s: no license in %s", name, dir)
	}
}
