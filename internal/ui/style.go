package ui

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"

	"github.com/renyijiu/tingyu/internal/model"
)

// palette is the window's colors for this frame. 听宇 is "listening to the
// cosmos": the dark theme is a night sky the panels float over, the light
// one the same sky at dawn.
type palette struct {
	dark bool
	// void is the sky behind everything, under the nebula.
	void ui.Color
	// panel fills cards and fields over the sky, translucent;
	// panelHi is the same under the pointer, and rim their hairline.
	panel   ui.Color
	panelHi ui.Color
	rim     ui.Color
	// solid is an opaque surface, for dialogs.
	solid ui.Color

	text  ui.Color
	muted ui.Color
	faint ui.Color

	// The aurora: violet, cyan and rose. Accent is the violet.
	accent     ui.Color
	cyan       ui.Color
	rose       ui.Color
	accentText ui.Color

	shadow ui.Color
	danger ui.Color
	ok     ui.Color
	warn   ui.Color
}

func theme(c *ui.Context) palette {
	sys := *c.Theme()
	var p palette
	p.dark = sys.Dark
	if sys.Dark {
		p.void = ui.Hex("#05040b")
		p.panel = ui.RGBA(255, 255, 255, 0.045)
		p.panelHi = ui.RGBA(255, 255, 255, 0.085)
		p.rim = ui.RGBA(255, 255, 255, 0.09)
		p.solid = ui.Hex("#110f1d")
		p.text = ui.Hex("#f4f2ff")
		p.muted = ui.Hex("#aaa6c8")
		p.faint = ui.Hex("#6c6889")
		p.accent = ui.Hex("#8b6dff")
		p.cyan = ui.Hex("#3fd5f0")
		p.rose = ui.Hex("#f472b6")
		p.shadow = ui.RGBA(0, 0, 0, 0.5)
		p.danger = ui.Hex("#ff6b8a")
		p.ok = ui.Hex("#4ade9c")
		p.warn = ui.Hex("#fbbf5a")
	} else {
		p.void = ui.Hex("#f5f3fc")
		p.panel = ui.RGBA(255, 255, 255, 0.62)
		p.panelHi = ui.RGBA(255, 255, 255, 0.86)
		p.rim = ui.RGBA(40, 30, 90, 0.10)
		p.solid = ui.Hex("#fcfbff")
		p.text = ui.Hex("#15122a")
		p.muted = ui.Hex("#5d5878")
		p.faint = ui.Hex("#9a96b4")
		p.accent = ui.Hex("#6a4cff")
		p.cyan = ui.Hex("#0ea5c6")
		p.rose = ui.Hex("#e0458f")
		p.shadow = ui.RGBA(40, 24, 110, 0.16)
		p.danger = ui.Hex("#e11d48")
		p.ok = ui.Hex("#059669")
		p.warn = ui.Hex("#d97706")
	}
	p.accentText = ui.Hex("#ffffff")
	sys.Background = p.void
	sys.Surface = p.solid
	sys.SurfaceHover = p.panelHi.Over(p.solid)
	sys.Text = p.text
	sys.TextMuted = p.muted
	sys.Border = p.rim.Over(p.solid)
	sys.Accent = p.accent
	sys.AccentHover = p.accent.Mix(p.cyan, 0.18)
	sys.AccentPressed = p.accent.Mix(p.void, 0.2)
	sys.AccentText = p.accentText
	sys.Focus = p.accent
	sys.Danger = p.danger
	sys.Selection = p.accent.Alpha(0.32)
	sys.Radius = 12
	if sys.Dark {
		sys.Inverse, sys.InverseText = ui.Hex("#262238"), p.text
	}
	c.SetTheme(&sys)
	return p
}

// aurora is the brand's gradient, violet through cyan, for what plays and
// what matters most on a page.
func (p palette) aurora(angle float32) ui.LinearGradient {
	return ui.LinearGradient{
		Angle: angle,
		Oklab: true,
		Stops: []ui.GradientStop{{Color: p.accent, At: 0}, {Color: p.accent.Mix(p.cyan, 0.45), At: 0.55}, {Color: p.cyan, At: 1}},
	}
}

// halo is the aurora around a loop, for rings that glow and turn.
func (p palette) halo() []ui.GradientStop {
	return []ui.GradientStop{
		{Color: p.accent, At: 0},
		{Color: p.cyan, At: 0.33},
		{Color: p.rose, At: 0.66},
		{Color: p.accent, At: 1},
	}
}

// Motion. Pages glide in on a spring; the player rises from the dock.
var (
	glide = ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond, Bounce: 0.08}}
	pop   = ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond, Bounce: 0.28}}
	rise  = ui.Animation{Spring: ui.SpringOptions{Duration: 600 * time.Millisecond, Bounce: 0.12}}
)

func pageEnter() ui.ElementTransition {
	return ui.ElementTransition{
		Animation: &glide,
		Enter:     &ui.Motion{Y: 18, Opacity: 0, Scale: 0.985},
		Exit:      &ui.Motion{Opacity: 0, Scale: 0.99},
	}
}

func nowEnter() ui.ElementTransition {
	return ui.ElementTransition{
		Animation: &rise,
		Enter:     &ui.Motion{Y: 70, Opacity: 0, Scale: 0.96},
		Exit:      &ui.Motion{Y: 50, Opacity: 0, Scale: 0.97},
	}
}

// coverMatch is the identity a cover shares between where it was clicked
// and where it lands, so it flies across.
type coverMatch struct{ id string }

// navMatch is the glowing mark of the chosen sidebar row.
type navMatch struct{}

func sourceName(source string) string {
	switch source {
	case model.SourceITunes:
		return "iTunes"
	case model.SourcePodcastIndex:
		return "Podcast Index"
	case model.SourceBoth:
		return "Podcast Index 和 iTunes"
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

// formatLength is a duration as people say it: "1 小时 12 分", "38 分钟".
func formatLength(d time.Duration) string {
	m := int(d.Round(time.Minute) / time.Minute)
	switch {
	case m >= 60 && m%60 == 0:
		return fmt.Sprintf("%d 小时", m/60)
	case m >= 60:
		return fmt.Sprintf("%d 小时 %d 分", m/60, m%60)
	case m > 0:
		return fmt.Sprintf("%d 分钟", m)
	default:
		return formatClock(d)
	}
}

func seconds(t time.Time) float64 {
	return float64(t.UnixNano()) / 1e9
}

// initial is the first letter or digit of a title, past brackets such as
// those of "【番外】".
func initial(title string) string {
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	return "听"
}

// hue is a stable angle for a title, so a show without a cover still has
// colors of its own.
func hue(s string) float32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return float32(h.Sum32() % 360)
}
