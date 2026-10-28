package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/model"
)

func (v *View) discover(c *ui.Context, p palette, reduce bool) {
	ui.Column(c).Fill().Padding(28, 32, 16, 32).Gap(18).Children(func() {
		ui.Text(c, "发现").FontSize(32).Bold()
		ui.Text(c, "搜一部中文或英文播客").TextColor(p.muted)
		v.searchRow(c, p)
		if v.App.SearchErr != "" {
			ui.Text(c, v.App.SearchErr).TextColor(c.Theme().Danger)
		}
		if v.App.Searching {
			v.skeletons(c, p, reduce)
			return
		}
		if len(v.App.Results) == 0 {
			v.discoverEmpty(c, p)
			return
		}
		ui.Text(c, "搜索结果 · "+sourceName(v.App.ResultSource)).FontSize(13).TextColor(p.muted)
		ui.GridView(c, &v.grid, len(v.App.Results), 168, 232, func(i int) {
			v.resultCard(c, p, v.App.Results[i], reduce)
		}).Grow(1)
	})
}

func (v *View) searchRow(c *ui.Context, p palette) {
	ui.Row(c).Gap(10).Children(func() {
		field := ui.TextInput(c.Key("search"), &v.App.Query).
			Placeholder("搜索播客，中文或英文").
			Label("搜索播客").
			Grow(1)
		field.MinHeight(44).Radius(14).Padding(10, 14).Background(p.card).Border(1, p.line)
		if field.Submitted() {
			v.App.Search()
		}
		if ui.PrimaryButton(c, "搜索").MinHeight(44).Radius(14).Padding(10, 18).Clicked() {
			v.App.Search()
		}
	})
}

func (v *View) discoverEmpty(c *ui.Context, p palette) {
	ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
		ui.Text(c, "封面会从这里铺开").FontSize(20).Bold().TextColor(p.muted)
		ui.Text(c, "试试「故事FM」或 Serial").TextColor(p.faint)
	})
}

func (v *View) skeletons(c *ui.Context, p palette, reduce bool) {
	ui.Text(c, "正在搜索…").TextColor(p.muted)
	op := shimmer(c, reduce)
	ui.Grid(c).Columns(4).Gap(16).Children(func() {
		for i := 0; i < 8; i++ {
			card := ui.Column(c.Key("sk:" + string(rune('a'+i)))).Height(220).Radius(16).Background(p.card).Padding(12).Gap(10)
			card.Opacity(op)
			card.Children(func() {
				ui.Column(c).Height(140).Radius(12).Background(p.line)
				ui.Column(c).Height(12).Width(90).Radius(6).Background(p.line)
				ui.Column(c).Height(10).Width(60).Radius(6).Background(p.line)
			})
		}
	})
}

func (v *View) resultCard(c *ui.Context, p palette, show model.Podcast, reduce bool) {
	card := ui.Column(c.Key("card:" + show.ID)).Fill().Padding(10).Gap(8).Radius(16).Clip().Background(p.card)
	hover := card.Hovered() && !reduce
	lift := float32(0)
	scale := float32(1)
	shadow := float32(10)
	if hover {
		lift = -4
		scale = 1.035
		shadow = 26
		card.Background(p.cardHover)
	}
	card.AnimatePose(ui.Pose{Y: lift, Scale: scale}, ui.Animation{Duration: 180 * time.Millisecond, Ease: ui.EaseOut})
	card.Shadow(0, 8, card.Animate("shadow", shadow, 180*time.Millisecond), 0, p.shadow)
	card.Children(func() {
		v.cover(c, p, show.Image, show.Title, 148, 12)
		ui.Text(c, show.Title).SingleLine().Bold()
		author := show.Author
		if author == "" {
			author = " "
		}
		ui.Text(c, author).SingleLine().FontSize(12).TextColor(p.muted)
	})
	if card.Clicked() {
		v.show(show)
	}
}
