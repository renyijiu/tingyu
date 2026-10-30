package ui

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

func (v *View) settingsEntry(c *ui.Context, p palette) {
	button := ui.ButtonBase(c.Key("settings-entry")).Label("设置").FillWidth().Height(38).Padding(0, 10).Gap(11).Radius(11)
	v.rowLight(button, p, false)
	turn := float32(0)
	if button.Hovered() {
		turn = 60
	}
	button.Children(func() {
		gear := ui.Icon(c, iconSettings).FontSize(17).TextColor(p.muted)
		gear.Rotate(gear.AnimateTo("turn", turn, pop))
		ui.Text(c, "设置").FontSize(13).Bold().TextColor(p.muted).Grow(1)
		ui.Text(c, "⌘,").FontSize(11).TextColor(p.faint).Padding(1, 6).Radius(6).Border(1, p.rim)
	})
	if button.Clicked() {
		v.OpenSettings()
	}
}

func (v *View) OpenSettings() {
	if v.settingsOpen {
		return
	}
	v.settingsDraft = v.App.Settings.Saved
	v.showCredentials = false
	v.settingsErr, v.settingsMessage = "", ""
	v.settingsOpen = true
	v.settingsRuns++
}

// settings is a panel over the frosted window: where searches come from,
// and the keys of Podcast Index.
func (v *View) settings(c *ui.Context, p palette) {
	ui.DialogBase(c.Key("settings"), &v.settingsOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(p.void.Alpha(0.45)).Material(glass.Blur{Radius: 8})
		in := panel.Keyframes("in", []ui.Keyframe{{At: 0, Value: 0}, {At: 360 * time.Millisecond, Value: 1, Ease: ui.CubicBezier(0.2, 1.3, 0.4, 1)}},
			ui.KeyframeOptions{Run: v.settingsRuns})
		panel.Width(460).Padding(24).Gap(14).Radius(26).Background(p.solid).
			Border(1, ui.Color{}).BorderLinearGradient(ui.LinearGradient{Angle: 160, Stops: []ui.GradientStop{
			{Color: p.accent.Alpha(0.6), At: 0}, {Color: p.rim, At: 0.4}, {Color: p.cyan.Alpha(0.35), At: 1},
		}}).
			Shadow(0, 30, 80, -10, p.shadow).Shadow(0, 0, 60, 0, p.accent.Alpha(0.12)).
			Opacity(min(in*1.5, 1)).Scale(0.94+0.06*in, 0.94+0.06*in)
		panel.Children(func() { v.settingsBody(c, p) })
	})
}

func (v *View) settingsBody(c *ui.Context, p palette) {
	ready := v.App.Settings.Effective().Ready()
	ui.Row(c).Gap(14).Children(func() {
		ui.Box(c).Size(44, 44).Radius(14).Center().LinearGradient(p.aurora(135)).
			Shadow(0, 8, 20, 0, p.accent.Alpha(0.35)).Children(func() {
			ui.Icon(c, iconKey).FontSize(21).TextColor(p.accentText)
		})
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, "设置").FontSize(20).Bold()
			ui.Text(c, "搜索来源与密钥").FontSize(12).TextColor(p.muted)
		})
		if ready {
			chip(c, p, iconSignal, "Podcast Index 已启用", p.ok)
		} else {
			chip(c, p, iconSignal, "iTunes 搜索中", p.warn)
		}
	})
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, "Podcast Index").FontSize(14).Bold()
		ui.Text(c, "填写 API Key 和 API Secret，保存后用于后续搜索。清空两项可移除本机密钥。").FontSize(12).LineHeight(1.5).TextColor(p.muted)
		if v.App.Settings.HasOverrides() {
			ui.Text(c, "环境变量或 .env 中的密钥优先生效，此处仅修改本机保存的配置。").FontSize(12).LineHeight(1.5).TextColor(p.warn)
		}
	})
	keyChanged := v.credentialField(c, p, "API Key", &v.settingsDraft.PodcastIndexKey)
	secretChanged := v.credentialField(c, p, "API Secret", &v.settingsDraft.PodcastIndexSecret)
	if keyChanged || secretChanged {
		v.settingsErr, v.settingsMessage = "", ""
	}
	ui.Row(c).Gap(10).Children(func() {
		reveal := ui.ButtonBase(c.Key("reveal")).Gap(6).Padding(5, 10, 5, 8).Radius(999)
		if reveal.Hovered() {
			reveal.Background(p.panelHi)
		}
		reveal.Children(func() {
			glyph := iconEye
			if v.showCredentials {
				glyph = iconEyeOff
			}
			ui.Icon(c, glyph).FontSize(15).TextColor(p.muted)
			ui.Text(c, "显示密钥").FontSize(12).TextColor(p.muted)
		})
		if reveal.Clicked() {
			v.showCredentials = !v.showCredentials
		}
		ui.Spacer(c)
		ui.Text(c, "密钥仅保存在本机。保存不会验证密钥是否有效。").FontSize(11).TextColor(p.faint).Shrink(1)
	})
	if v.settingsErr != "" {
		ui.Row(c).Gap(6).Children(func() {
			ui.Icon(c, iconAlert).FontSize(14).TextColor(p.danger)
			ui.Text(c, v.settingsErr).FontSize(12).TextColor(p.danger)
		})
	}
	if v.settingsMessage != "" {
		ui.Row(c.Key("saved")).Gap(6).Transition(ui.ElementTransition{Animation: &pop, Enter: &ui.Motion{Y: 6, Opacity: 0}}).Children(func() {
			ui.Icon(c, iconCheck).FontSize(14).TextColor(p.ok)
			ui.Text(c, v.settingsMessage).FontSize(12).TextColor(p.ok)
		})
	}
	ui.Row(c).Gap(10).Margin(4, 0, 0, 0).Justify(ui.End).Children(func() {
		if pill(c, p, "close", nil, "关闭", false).Clicked() {
			v.settingsOpen = false
		}
		if pill(c, p, "save", iconCheck, "保存", true).Clicked() {
			v.saveSettings()
		}
	})
}

func (v *View) saveSettings() {
	v.settingsErr, v.settingsMessage = "", ""
	if err := v.App.SaveSettings(v.settingsDraft); err != nil {
		v.settingsErr = "无法保存设置：" + err.Error()
		return
	}
	v.settingsDraft = v.App.Settings.Saved
	v.showCredentials = false
	v.settingsMessage = "已保存，当前使用 iTunes。"
	if v.App.Settings.Effective().Ready() {
		v.settingsMessage = "已保存，将优先使用 Podcast Index。"
	}
}

func (v *View) credentialField(c *ui.Context, p palette, label string, value *string) bool {
	changed := false
	ui.Field(c.Key(label), label, func() {
		field := ui.TextInput(c, value).Placeholder("输入 "+label).FillWidth().MinHeight(40).Radius(12).Padding(9, 12).
			Background(p.panel).Border(1, p.rim).FontFeatures("tnum")
		if field.Focused() {
			field.Border(1.5, p.accent).Shadow(0, 0, 0, 3, p.accent.Alpha(0.18))
		}
		if !v.showCredentials {
			field.Password()
		}
		changed = field.Changed()
	})
	return changed
}
