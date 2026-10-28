package ui

import "github.com/egoist/mygo/ui"

var settingsIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"><path d="M10 2h4l.6 2.6 2.1 1.2 2.6-.8 2 3.5-2 1.8v3.4l2 1.8-2 3.5-2.6-.8-2.1 1.2L14 22h-4l-.6-2.6-2.1-1.2-2.6.8-2-3.5 2-1.8v-3.4l-2-1.8 2-3.5 2.6.8 2.1-1.2z"/><circle cx="12" cy="12" r="3.2"/></svg>`))

func (v *View) settingsEntry(c *ui.Context, p palette) {
	button := ui.ButtonBase(c.Key("settings-entry")).Label("设置").FillWidth().MinHeight(40).Padding(8, 10).Gap(10).Radius(10)
	if button.Hovered() {
		button.Background(p.cardHover)
	}
	button.Children(func() {
		ui.Icon(c, settingsIcon).Size(18, 18).TextColor(p.muted)
		ui.Text(c, "设置").FontSize(13).TextColor(p.muted).Grow(1)
		ui.Text(c, "⌘,").FontSize(12).TextColor(p.faint)
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
}

func (v *View) settings(c *ui.Context) {
	t := c.Theme()
	ui.Modal(c.Key("settings"), &v.settingsOpen, func() {
		ui.Column(c).Width(440).Gap(12).Children(func() {
			ui.Text(c, "设置").FontSize(22).Bold()
			ui.Text(c, "Podcast Index").Bold()
			ui.Text(c, "填写 API Key 和 API Secret，保存后用于后续搜索。清空两项可移除本机密钥。").TextColor(t.TextMuted)
			if v.App.Settings.HasOverrides() {
				ui.Text(c, "环境变量或 .env 中的密钥优先生效，此处仅修改本机保存的配置。").TextColor(t.TextMuted)
			}
			keyChanged := v.credentialField(c, "API Key", &v.settingsDraft.PodcastIndexKey)
			secretChanged := v.credentialField(c, "API Secret", &v.settingsDraft.PodcastIndexSecret)
			ui.Checkbox(c, &v.showCredentials, "显示密钥")
			if keyChanged || secretChanged {
				v.settingsErr, v.settingsMessage = "", ""
			}
			ui.Text(c, "密钥仅保存在本机。保存不会验证密钥是否有效。").FontSize(12).TextColor(t.TextMuted)
			if v.settingsErr != "" {
				ui.Text(c, v.settingsErr).TextColor(t.Danger)
			}
			if v.settingsMessage != "" {
				ui.Text(c, v.settingsMessage).TextColor(t.TextMuted)
			}
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "关闭").Clicked() {
					v.settingsOpen = false
				}
				if ui.PrimaryButton(c, "保存").Clicked() {
					v.saveSettings()
				}
			})
		})
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

func (v *View) credentialField(c *ui.Context, label string, value *string) bool {
	changed := false
	ui.Field(c.Key(label), label, func() {
		field := ui.TextInput(c, value).Placeholder("输入 " + label).FillWidth()
		if !v.showCredentials {
			field.Password()
		}
		changed = field.Changed()
	})
	return changed
}
