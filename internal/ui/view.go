// Package ui is the native window. It draws internal/app and does not own
// the podcast rules, so those stay testable without a window.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/app"
	"github.com/renyijiu/tingyu/internal/model"
	"github.com/renyijiu/tingyu/internal/player"
)

// View is the window contents.
type View struct {
	App *app.App

	subs     ui.ListState
	results  ui.ListState
	episodes ui.ListState
	seek     float64
}

// Build draws one frame.
func (v *View) Build(c *ui.Context) {
	st := v.App.Player.State()
	if st.Playing || st.Buffering {
		c.After(200 * time.Millisecond)
		if st.Playing {
			v.App.Player.Tick()
		}
	}
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		if v.App.Notice != "" {
			ui.Text(c, v.App.Notice).Padding(10, 16).TextColor(t.TextMuted)
			ui.Divider(c)
		}
		ui.Row(c).Grow(1).Children(func() {
			v.sidebar(c)
			ui.Divider(c)
			v.detail(c)
		})
		ui.Divider(c)
		v.transport(c, st)
	})
}

func (v *View) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(300).FillHeight().Padding(12).Gap(8).Background(t.Surface).Children(func() {
		ui.Text(c, "听宇").FontSize(22).Bold()
		ui.Text(c, "Tingyu").FontSize(12).TextColor(t.TextMuted)
		field := ui.TextInput(c.Key("search"), &v.App.Query).
			Placeholder("搜索播客，中文或英文").
			Label("搜索播客").
			FillWidth()
		submitted := field.Submitted()
		if ui.PrimaryButton(c, "搜索").FillWidth().Clicked() || submitted {
			v.App.Search()
		}
		if v.App.Searching {
			ui.Text(c, "正在搜索…").TextColor(t.TextMuted)
		}
		if v.App.SearchErr != "" {
			ui.Text(c, v.App.SearchErr).TextColor(t.Danger)
		}

		ui.Text(c, "我的订阅").Bold()
		if len(v.App.Subs) == 0 {
			ui.Text(c, "还没有订阅").FontSize(12).TextColor(t.TextMuted)
		} else {
			ui.List(c, &v.subs, len(v.App.Subs), func(i int) {
				p := v.App.Subs[i]
				if ui.Button(c.Key(p.ID), p.Title).FillWidth().Clicked() {
					v.App.Open(p)
				}
			}).Grow(1)
		}

		if len(v.App.Results) > 0 {
			ui.Text(c, "搜索结果 · "+sourceName(v.App.ResultSource)).Bold()
			ui.List(c, &v.results, len(v.App.Results), func(i int) {
				p := v.App.Results[i]
				if ui.Button(c.Key("result:"+p.ID), p.Title).FillWidth().Clicked() {
					v.App.Open(p)
				}
			}).Grow(1)
		}
	})
}

func (v *View) detail(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Padding(20).Gap(8).Children(func() {
		if v.App.Selected == nil {
			ui.Text(c, "从左侧选择订阅，或搜索一个播客。").TextColor(t.TextMuted).Padding(8)
			return
		}
		p := *v.App.Selected
		ui.Text(c, p.Title).FontSize(24).Bold()
		if p.Author != "" {
			ui.Text(c, p.Author).TextColor(t.TextMuted)
		}
		if desc := truncate(p.Description, 180); desc != "" {
			ui.Text(c, desc).TextColor(t.TextMuted)
		}
		if v.App.Subscribed(p.ID) {
			if ui.Button(c, "取消订阅").Clicked() {
				_ = v.App.Unsubscribe(p.ID)
			}
		} else if ui.PrimaryButton(c, "订阅这个节目").Clicked() {
			_ = v.App.Subscribe(p)
		}
		if v.App.Loading {
			ui.Text(c, "正在加载单集…").TextColor(t.TextMuted)
		}
		if v.App.EpErr != "" {
			ui.Text(c, v.App.EpErr).TextColor(t.Danger)
		}
		ui.Text(c, "单集").Bold()
		ui.List(c, &v.episodes, len(v.App.Episodes), func(i int) {
			ep := v.App.Episodes[i]
			if ui.Button(c.Key("ep:"+ep.ID), ep.Title).FillWidth().Clicked() {
				v.App.PlayEpisode(ep)
			}
			if meta := episodeMeta(v.App, ep); meta != "" {
				ui.Text(c, meta).FontSize(12).TextColor(t.TextMuted).Padding(0, 4, 8, 4)
			}
		}).Grow(1)
	})
}

func (v *View) transport(c *ui.Context, st player.State) {
	t := c.Theme()
	ui.Column(c).Padding(10, 16).Gap(6).Background(t.Surface).Children(func() {
		title := "没有正在播放的单集"
		if st.Episode != nil {
			title = st.Episode.Title
			if st.ShowTitle != "" {
				title = st.ShowTitle + " · " + title
			}
		}
		ui.Text(c, title).SingleLine()
		if st.Err != nil {
			ui.Text(c, st.Err.Error()).TextColor(t.Danger)
		} else if st.Buffering {
			ui.Text(c, "正在缓冲…").TextColor(t.TextMuted)
		}
		ui.Row(c).Gap(12).Children(func() {
			label := "播放"
			if st.Playing {
				label = "暂停"
			}
			if ui.PrimaryButton(c, label).Disabled(st.Episode == nil || st.Buffering).Clicked() {
				v.App.Toggle()
			}
			hi := st.Duration.Seconds()
			if hi < 1 {
				hi = 1
			}
			if st.Episode == nil {
				v.seek = 0
			} else {
				v.seek = st.Position.Seconds()
			}
			slider := ui.Slider(c, &v.seek, 0, hi).Label("播放进度").Grow(1)
			if st.Episode == nil {
				slider.Disabled(true)
			}
			if slider.Changed() {
				v.App.SeekTo(time.Duration(v.seek * float64(time.Second)))
			}
			ui.Text(c, formatClock(st.Position)+" / "+formatClock(st.Duration)).TextColor(t.TextMuted)
		})
	})
}

func sourceName(source string) string {
	switch source {
	case model.SourceITunes:
		return "iTunes"
	case model.SourcePodcastIndex:
		return "Podcast Index"
	default:
		if source == "" {
			return "搜索"
		}
		return source
	}
}

func episodeMeta(a *app.App, ep model.Episode) string {
	var parts []string
	if !ep.Published.IsZero() {
		parts = append(parts, ep.Published.Local().Format("2006-01-02"))
	}
	if ep.Duration > 0 {
		parts = append(parts, formatClock(ep.Duration))
	}
	if a.Store != nil {
		if prog, ok := a.Store.Progress(ep.ID); ok && prog.Position > 0 {
			parts = append(parts, "听到 "+formatClock(prog.Position))
		}
	}
	return strings.Join(parts, "  ·  ")
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func formatClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	sec := int(d.Round(time.Second) / time.Second)
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
