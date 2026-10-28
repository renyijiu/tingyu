package ui

import "github.com/egoist/mygo/ui"

// The icon set: one grid of 24, strokes of 1.8 with round ends, drawn for
// 听宇 so they sit together. Icons take the color of the text around them.

func icon(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	iconDiscover = icon(`<circle cx="12" cy="12" r="9"/><path d="M15.6 8.4l-2.1 5.1-5.1 2.1 2.1-5.1z" fill="currentColor" stroke-width="1.4"/>`)
	iconOrbit    = icon(`<circle cx="12" cy="12" r="4.2"/><path d="M4.6 16.9C2.4 19 1.9 20.6 3.3 21c2.4.8 8.1-1.9 12.6-6.4S22.1 4.6 21.3 3.3c-.5-.8-2-.6-4 .5"/><circle cx="19.4" cy="4.6" r=".6" fill="currentColor"/>`)
	iconLibrary  = icon(`<path d="M12 3.2l8.6 4.6L12 12.4 3.4 7.8z"/><path d="M3.4 12.2L12 16.8l8.6-4.6"/><path d="M3.4 16.4L12 21l8.6-4.6"/>`)
	iconHistory  = icon(`<path d="M3.6 12a8.4 8.4 0 1 0 2.5-6"/><path d="M3.4 3.6v4.2h4.2"/><path d="M12 7.6V12l3 1.9"/>`)
	iconSearch   = icon(`<circle cx="10.8" cy="10.8" r="6.6"/><path d="M20 20l-4.4-4.4"/>`)
	iconPlay     = icon(`<path d="M8 5.6v12.8a1.1 1.1 0 0 0 1.7.9l10-6.4a1.1 1.1 0 0 0 0-1.8l-10-6.4A1.1 1.1 0 0 0 8 5.6z" fill="currentColor" stroke-width="1.2"/>`)
	iconPause    = icon(`<rect x="6.2" y="5" width="4" height="14" rx="1.4" fill="currentColor" stroke="none"/><rect x="13.8" y="5" width="4" height="14" rx="1.4" fill="currentColor" stroke="none"/>`)
	iconBack     = icon(`<path d="M4.2 12.6A7.9 7.9 0 1 0 6.6 6.3"/><path d="M4.4 2.8v4.4h4.4"/>`)
	iconForward  = icon(`<path d="M19.8 12.6a7.9 7.9 0 1 1-2.4-6.3"/><path d="M19.6 2.8v4.4h-4.4"/>`)
	iconDown     = icon(`<path d="M6 9.5l6 6 6-6"/>`)
	iconUp       = icon(`<path d="M6 14.5l6-6 6 6"/>`)
	iconLeft     = icon(`<path d="M14.5 6l-6 6 6 6"/>`)
	iconRight    = icon(`<path d="M9.5 6l6 6-6 6"/>`)
	iconSettings = icon(`<path d="M10.3 2.8h3.4l.5 2.4 1.9 1.1 2.3-.8 1.7 3-1.8 1.6v2.2l1.8 1.6-1.7 3-2.3-.8-1.9 1.1-.5 2.4h-3.4l-.5-2.4-1.9-1.1-2.3.8-1.7-3 1.8-1.6v-2.2L4 8.5l1.7-3 2.3.8 1.9-1.1z"/><circle cx="12" cy="12" r="2.9"/>`)
	iconCheck    = icon(`<path d="M5 12.6l4.4 4.4L19 7.4"/>`)
	iconPlus     = icon(`<path d="M12 5v14M5 12h14"/>`)
	iconClose    = icon(`<path d="M6.5 6.5l11 11M17.5 6.5l-11 11"/>`)
	iconSparkle  = icon(`<path d="M12 2.5c.7 5 4.5 8.8 9.5 9.5-5 .7-8.8 4.5-9.5 9.5-.7-5-4.5-8.8-9.5-9.5 5-.7 8.8-4.5 9.5-9.5z" fill="currentColor" stroke-width="1"/>`)
	iconEye      = icon(`<path d="M2.4 12S5.9 5.6 12 5.6 21.6 12 21.6 12 18.1 18.4 12 18.4 2.4 12 2.4 12z"/><circle cx="12" cy="12" r="2.9"/>`)
	iconEyeOff   = icon(`<path d="M9.9 5.8A9.7 9.7 0 0 1 12 5.6c6.1 0 9.6 6.4 9.6 6.4a17 17 0 0 1-2.6 3.4M6.6 7.3C3.9 9 2.4 12 2.4 12s3.5 6.4 9.6 6.4a9.3 9.3 0 0 0 4.4-1.1"/><path d="M9.9 9.9a2.9 2.9 0 0 0 4.2 4.1"/><path d="M3.5 3.5l17 17"/>`)
	iconKey      = icon(`<circle cx="7.8" cy="15.8" r="4.4"/><path d="M11 12.6l8.6-8.6M16.2 7.4l2.4 2.4M13.8 9.8l2 2"/>`)
	iconHeadset  = icon(`<path d="M4 15.5V12a8 8 0 0 1 16 0v3.5"/><rect x="3" y="14" width="4.6" height="6.6" rx="1.8"/><rect x="16.4" y="14" width="4.6" height="6.6" rx="1.8"/>`)
	iconSignal   = icon(`<circle cx="12" cy="12" r="1.8" fill="currentColor"/><path d="M8.6 15.4a4.8 4.8 0 0 1 0-6.8M15.4 8.6a4.8 4.8 0 0 1 0 6.8M5.6 18.4a9 9 0 0 1 0-12.8M18.4 5.6a9 9 0 0 1 0 12.8"/>`)
	iconMic      = icon(`<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5.5 11.5a6.5 6.5 0 0 0 13 0M12 18v3"/>`)
	iconWave     = icon(`<path d="M3 12h1.6M7 8.4v7.2M11 5v14M15 8v8M19 10.4v3.2M21 12h0"/>`)
	iconAlert    = icon(`<circle cx="12" cy="12" r="8.6"/><path d="M12 7.6v5.2M12 16.2v.1"/>`)
)
