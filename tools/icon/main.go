// Command icon draws the app's icon, resources/icon.png, from the mark the
// sidebar shows:
//
//	go run ./tools/icon
package main

import (
	"image/png"
	"log"
	"os"

	"github.com/egoist/mygo/ui"

	tingui "github.com/renyijiu/tingyu/internal/ui"
)

func main() {
	img := ui.Render(tingui.Icon, 1024, 1024, 1)
	f, err := os.Create("resources/icon.png")
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}
