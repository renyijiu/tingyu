package ui

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/catalog"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
)

type charts struct{ shows []model.Podcast }

func (f charts) Top(_ context.Context, country string, _ int) ([]model.Podcast, error) {
	if country != "cn" {
		return nil, nil
	}
	return f.shows, nil
}

func TestDiscoverShowsTheChartRanked(t *testing.T) {
	var shows []model.Podcast
	for _, title := range []string{"声动早咖啡", "忽左忽右", "岩中花述"} {
		p := model.Podcast{Title: title, FeedURL: "https://example.com/" + title + ".xml"}
		p.EnsureIDs()
		shows = append(shows, p)
	}
	ep := model.Episode{Title: "最新一集", GUID: "1", AudioURL: "https://cdn.example.com/1.mp3"}
	a := app.New(nil, &catalog.Catalog{
		ITunes: itunes{},
		Charts: charts{shows},
		Feeds:  feeds{eps: []model.Episode{ep}},
	}, &player.Controller{})
	a.Sync = true
	v := &View{App: a}
	tt := ui.NewTester(v.Build, 1100, 760)
	if !tt.HasText("大家在听") || !tt.HasText("Apple 播客 · 中国区榜单") {
		t.Fatalf("chart missing: %q", tt.Texts())
	}
	for _, want := range []string{"声动早咖啡", "忽左忽右", "岩中花述", "1", "2", "3", "故事FM"} {
		if !tt.HasText(want) {
			t.Fatalf("%q missing: %q", want, tt.Texts())
		}
	}
	if tt.HasText("宇宙很大，从一个关键词开始") {
		t.Fatal("the empty sky shows beside the chart")
	}
	// 忽左忽右 is a suggested search too; 岩中花述 is only on the chart.
	if err := tt.Click("岩中花述"); err != nil {
		t.Fatal(err)
	}
	if a.Selected == nil || a.Selected.Title != "岩中花述" || !tt.HasText("最新一集") {
		t.Fatalf("opened %+v, texts %q", a.Selected, tt.Texts())
	}
}

func TestDiscoverWithoutAChartKeepsTheEmptySky(t *testing.T) {
	a := app.New(nil, &catalog.Catalog{ITunes: itunes{}}, &player.Controller{})
	a.Sync = true
	tt := ui.NewTester((&View{App: a}).Build, 1100, 760)
	if !tt.HasText("宇宙很大，从一个关键词开始") || tt.HasText("大家在听") {
		t.Fatalf("texts %q", tt.Texts())
	}
}
