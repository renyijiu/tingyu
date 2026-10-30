package ui

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

func picture() image.Image { return image.NewRGBA(image.Rect(0, 0, 2, 2)) }

func TestCoversLetGoOfTheOneUsedLongestAgo(t *testing.T) {
	v := &View{}
	a := v.arts()
	for i := 0; i < keptCovers; i++ {
		a.put(itoa(i), ui.NewBitmap(picture()), ui.Color{})
	}
	if _, _, ok := v.coverBitmap("0"); !ok {
		t.Fatal("cover 0 missing")
	}
	a.put("new", ui.NewBitmap(picture()), ui.Color{})
	if len(a.bmp) != keptCovers || len(a.used) != keptCovers {
		t.Fatalf("%d covers kept, %d tracked", len(a.bmp), len(a.used))
	}
	for _, url := range []string{"0", "new", "2"} {
		if _, ok := a.bmp[url]; !ok {
			t.Fatalf("%s let go", url)
		}
	}
	if _, ok := a.bmp["1"]; ok {
		t.Fatal("1, used longest ago, is still kept")
	}
}
