package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

func svg(viewBox, body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="` + viewBox +
		`" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	keyboardIcon   = svg("0 0 24 24", `<rect x="2.5" y="6" width="19" height="12" rx="2.5"/><path d="M6 10h.01M9.33 10h.01M12.66 10h.01M16 10h.01M18 10h.01M6 13.5h.01M18 13.5h.01M8.5 14h7"/>`)
	mouseIcon      = svg("0 0 24 24", `<rect x="6.5" y="3" width="11" height="18" rx="5.5"/><path d="M12 7v3"/>`)
	headphonesIcon = svg("0 0 24 24", `<path d="M4 15v-3a8 8 0 0 1 16 0v3"/><rect x="3" y="14" width="4.5" height="6.5" rx="1.8"/><rect x="16.5" y="14" width="4.5" height="6.5" rx="1.8"/>`)
	trackpadIcon   = svg("0 0 24 24", `<rect x="3" y="5" width="18" height="14" rx="2.5"/>`)
	gamepadIcon    = svg("0 0 24 24", `<path d="M7 7h10a4.5 4.5 0 0 1 4.4 5.4l-.8 4a2.6 2.6 0 0 1-4.5 1.2L14.5 16h-5l-1.6 1.6a2.6 2.6 0 0 1-4.5-1.2l-.8-4A4.5 4.5 0 0 1 7 7z"/><path d="M8 10v3M6.5 11.5h3M15.5 11h.01M17.5 12.5h.01"/>`)
	genericIcon    = svg("0 0 24 24", `<rect x="3" y="7" width="16" height="10" rx="2.5"/><path d="M21.5 10.5v3"/>`)

	refreshIcon = svg("0 0 16 16", `<path d="M13.5 8a5.5 5.5 0 1 1-1.6-3.9"/><path d="M13.5 2.5v3h-3"/>`)
	plugIcon    = svg("0 0 24 24", `<path d="M9 3v4M15 3v4M6 7h12v3a6 6 0 0 1-12 0z"/><path d="M12 16v5"/>`)
	alertIcon   = svg("0 0 16 16", `<circle cx="8" cy="8" r="6.5"/><path d="M8 4.8v3.6M8 11.2h.01"/>`)
	boltIcon    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 9 12" fill="currentColor"><path d="M5.2 0 0 6.8h3.6L2.9 12 9 4.9H5.3z"/></svg>`))
)

// deviceIcon guesses the kind of accessory from its name; pmset reports no
// device type.
func deviceIcon(name string) *ui.SVG {
	n := strings.ToLower(name)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(n, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("trackpad", "触控板"):
		return trackpadIcon
	case has("keyboard", "键盘", "qinglong", "keychron", "hhkb"):
		return keyboardIcon
	case has("mouse", "鼠标", "mx master", "mx anywhere", "mx vertical"):
		return mouseIcon
	case has("airpods", "buds", "headphone", "headset", "beats", "耳机"):
		return headphonesIcon
	case has("controller", "gamepad", "dualsense", "joy-con", "手柄"):
		return gamepadIcon
	}
	return genericIcon
}
