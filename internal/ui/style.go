package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/model"
)

// palette is the window's colors for this frame. Dark and light both follow
// the desktop; the dark one is the design the spacing and accent were made for.
type palette struct {
	dark       bool
	bg         ui.Color
	sidebar    ui.Color
	card       ui.Color
	cardHover  ui.Color
	text       ui.Color
	muted      ui.Color
	faint      ui.Color
	line       ui.Color
	accent     ui.Color
	accentText ui.Color
	shadow     ui.Color
}

func theme(c *ui.Context) palette {
	sys := *c.Theme()
	var p palette
	p.dark = sys.Dark
	if sys.Dark {
		p.bg = ui.Hex("#0c0c0f")
		p.sidebar = ui.Hex("#111114")
		p.card = ui.Hex("#1a1a20")
		p.cardHover = ui.Hex("#23232c")
		p.text = ui.Hex("#f5f5f7")
		p.muted = ui.Hex("#a1a1aa")
		p.faint = ui.Hex("#6f6f78")
		p.line = ui.Hex("#2a2a32")
		p.accent = ui.Hex("#8b7cff")
		p.accentText = ui.Hex("#ffffff")
		p.shadow = ui.RGBA(0, 0, 0, 0.45)
	} else {
		p.bg = ui.Hex("#f3f3f6")
		p.sidebar = ui.Hex("#ffffff")
		p.card = ui.Hex("#ffffff")
		p.cardHover = ui.Hex("#f7f7fb")
		p.text = ui.Hex("#18181b")
		p.muted = ui.Hex("#71717a")
		p.faint = ui.Hex("#a1a1aa")
		p.line = ui.Hex("#e4e4e7")
		p.accent = ui.Hex("#6d4aff")
		p.accentText = ui.Hex("#ffffff")
		p.shadow = ui.RGBA(24, 24, 32, 0.16)
	}
	sys.Background = p.bg
	sys.Surface = p.card
	sys.Text = p.text
	sys.TextMuted = p.muted
	sys.Border = p.line
	sys.Accent = p.accent
	sys.AccentText = p.accentText
	sys.Radius = 14
	c.SetTheme(&sys)
	return p
}

func pageEnter() ui.ElementTransition {
	return ui.ElementTransition{
		Duration: 240 * time.Millisecond,
		Ease:     ui.EaseOut,
		Enter:    &ui.Motion{X: 22, Opacity: 0},
		Exit:     &ui.Motion{Opacity: 0},
	}
}

func nowEnter() ui.ElementTransition {
	return ui.ElementTransition{
		Duration: 280 * time.Millisecond,
		Ease:     ui.EaseOut,
		Enter:    &ui.Motion{Y: 36, Opacity: 0, Scale: 0.97},
		Exit:     &ui.Motion{Y: 20, Opacity: 0},
	}
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

func seconds(t time.Time) float64 {
	return float64(t.UnixNano()) / 1e9
}

func initial(title string) string {
	r := []rune(strings.TrimSpace(title))
	if len(r) == 0 {
		return "听"
	}
	return string(r[0])
}
