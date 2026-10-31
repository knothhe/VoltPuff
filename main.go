package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/knothhe/VoltPuff/internal/battery"
)

const version = "0.1.1"

//go:embed assets/tray.png
var trayPNG []byte

//go:embed assets/icon.png
var iconPNG []byte

// The panel sizes itself to its content: these are the fixed heights of
// its parts, in DIPs.
const (
	panelWidth   = 300
	headHeight   = 58
	rowHeight    = 48
	listInset    = 6
	noticeHeight = 34
	emptyHeight  = 176
	footHeight   = 37
	maxRows      = 6 // more scroll
	lowLevel     = 20
)

type puff struct {
	tray     *mygo.Tray
	panel    *mygo.Window // the open flyout, or nil
	panelH   int
	mascot   *ui.Bitmap
	ctx      context.Context
	cancel   context.CancelFunc
	read     func(context.Context) ([]battery.Device, error)
	devices  []battery.Device
	updated  time.Time
	err      error
	loading  bool
	quitting bool
}

func main() {
	showPanel := flag.Bool("show", false, "open the panel after launch; the default starts in the menu bar only")
	list := flag.Bool("list", false, "print accessory batteries as JSON without starting the GUI")
	mock := flag.String("mock", "", "preview the panel with mock devices: "+strings.Join(mockNames(), ", "))
	showVersion := flag.Bool("version", false, "print the version")
	flag.Parse()
	if *showVersion {
		fmt.Println("VoltPuff " + version)
		return
	}
	if *list {
		devices, err := battery.Read(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(devices); err != nil {
			log.Fatal(err)
		}
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &puff{ctx: ctx, cancel: cancel, read: battery.Read}
	if *mock != "" {
		scene, ok := mockScenes[*mock]
		if !ok {
			log.Fatalf("unknown mock scene %q; choose one of: %s", *mock, strings.Join(mockNames(), ", "))
		}
		s.read, *showPanel = scene.read, true
		if scene.err != nil && scene.devices != nil {
			// The scene's devices stand for the last successful read.
			s.devices, s.updated = scene.devices, time.Now().Add(-11*time.Minute)
		}
	}
	mygo.App.SetName("VoltPuff")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { s.quitting = true; s.cancel() })
	// The flyout is the only window: closing it leaves the menu bar icon.
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnSecondInstance(func([]string, string) {
		if s.tray != nil {
			s.open()
		}
	})
	mygo.App.WhenReady(func() { s.ready(*showPanel) })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (s *puff) ready(showPanel bool) {
	var err error
	s.mascot, err = ui.DecodeBitmap(iconPNG)
	if err != nil {
		log.Fatal(err)
	}
	s.tray, err = mygo.NewTray(mygo.TrayOptions{
		ID:   "voltpuff.knothhe.com.battery",
		Icon: trayPNG, IconIsTemplate: true,
		ToolTip: "电团团 · 正在读取电量…",
	})
	if err != nil {
		log.Fatal("create tray: ", err)
	}
	s.tray.OnClick(func() {
		if s.panel != nil {
			s.panel.Close()
		} else {
			s.open()
		}
	})
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开电团团", Click: func(*mygo.MenuItem, *mygo.Window) { s.open() }},
		{Label: "刷新电量", Click: func(*mygo.MenuItem, *mygo.Window) { s.refresh() }},
		mygo.Separator(),
		{Label: "VoltPuff v" + version, Disabled: true},
		{Label: "退出电团团", Role: mygo.RoleQuit},
	})
	s.tray.OnRightClick(func() {
		s.closePanel()
		s.tray.SetMenu(menu)
		defer s.tray.SetMenu(nil)
		s.tray.PopUpMenu()
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { s.tray.Destroy() })
	s.refresh()
	if showPanel {
		// A just-created status item may not have a frame to anchor to yet.
		time.AfterFunc(300*time.Millisecond, func() { mygo.RunOnMain(s.open) })
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				mygo.RunOnMain(s.refresh)
			}
		}
	}()
}

// refresh is entered on the main thread. One bounded read runs at a time;
// result publication and every UI mutation also stay on the main thread.
func (s *puff) refresh() {
	if s.loading || s.quitting {
		return
	}
	s.loading = true
	s.changed()
	go func() {
		devices, err := s.read(s.ctx)
		mygo.RunOnMain(func() {
			if s.quitting {
				return
			}
			s.loading, s.err = false, err
			if err == nil {
				s.devices, s.updated = devices, time.Now()
			}
			s.updateTray()
			s.changed()
		})
	}()
}

func (s *puff) updateTray() {
	if s.err != nil {
		s.tray.SetTitle("—")
		s.tray.SetToolTip("电团团 · 电量读取失败，点击查看")
		return
	}
	title := ""
	if low, ok := battery.Lowest(s.devices); ok {
		title = fmt.Sprintf("%d%%", low)
	}
	s.tray.SetTitle(title)
	lines := []string{"电团团 · VoltPuff"}
	for _, d := range s.devices {
		lines = append(lines, d.Name+" · "+percent(d))
	}
	if len(s.devices) == 0 {
		lines = append(lines, "还没有可读取电量的设备")
	}
	s.tray.SetToolTip(strings.Join(lines, "\n"))
}

// open shows the panel below the menu bar icon. The flyout closes itself as
// the user clicks elsewhere or presses Escape; the app keeps running.
func (s *puff) open() {
	if s.panel != nil || s.quitting {
		return
	}
	s.panelH = s.panelHeight()
	s.panel = mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:         s.tray,
		Placement:    mygo.PlacementBottom,
		Gap:          6,
		Width:        panelWidth,
		Height:       s.panelH,
		Focusable:    true,
		Shadow:       true,
		Vibrancy:     mygo.VibrancyPopover,
		CornerRadius: 12,
		Content:      ui.View(s.view),
	})
	s.panel.OnClosed(func() { s.panel = nil })
	s.refresh()
}

func (s *puff) closePanel() {
	if s.panel != nil {
		s.panel.Close()
	}
}

// changed redraws an open panel, resizing it to what it now shows.
func (s *puff) changed() {
	if s.panel == nil {
		return
	}
	if h := s.panelHeight(); h != s.panelH {
		s.panelH = h
		s.panel.SetSize(panelWidth, h)
	}
	s.panel.Invalidate()
}

type panelMode int

const (
	modeFirstLoad panelMode = iota // nothing read yet
	modeEmpty                      // no devices, or a failure with nothing to show
	modeDevices
)

func (s *puff) mode() panelMode {
	switch {
	case len(s.devices) > 0:
		return modeDevices
	case s.err == nil && s.updated.IsZero():
		return modeFirstLoad
	}
	return modeEmpty
}

func (s *puff) panelHeight() int {
	h := headHeight + footHeight
	switch s.mode() {
	case modeFirstLoad:
		h += 2*rowHeight + listInset
	case modeEmpty:
		h += emptyHeight
	default:
		if s.err != nil {
			h += noticeHeight
		}
		h += min(len(s.devices), maxRows)*rowHeight + listInset
	}
	return h
}

func percent(d battery.Device) string {
	if d.Percent == nil {
		return "电量未知"
	}
	return fmt.Sprintf("%d%%", *d.Percent)
}

func isLow(d battery.Device) bool {
	return !d.Charging && d.Percent != nil && *d.Percent <= lowLevel
}

// palette holds the colors of the panel, which draws over the popover
// material: text in translucent inks, so the material shows through.
type palette struct {
	bg, text, muted, faint, hover, tile, line Color
	warn, ok, danger                          Color
}

type Color = ui.Color

func newPalette(dark bool) palette {
	if dark {
		return palette{
			bg: ui.Hex("#24222e"), text: ui.Hex("#f5f5f7"),
			muted: ui.RGBA(235, 235, 245, .6), faint: ui.RGBA(235, 235, 245, .3),
			hover: ui.RGBA(255, 255, 255, .07), tile: ui.RGBA(255, 255, 255, .08), line: ui.RGBA(255, 255, 255, .08),
			warn: ui.Hex("#ff9f43"), ok: ui.Hex("#4cd964"), danger: ui.Hex("#ff6b6b"),
		}
	}
	return palette{
		bg: ui.Hex("#f6f5fa"), text: ui.Hex("#1d1d1f"),
		muted: ui.RGBA(60, 60, 67, .62), faint: ui.RGBA(60, 60, 67, .32),
		hover: ui.RGBA(0, 0, 0, .05), tile: ui.RGBA(0, 0, 0, .055), line: ui.RGBA(0, 0, 0, .08),
		warn: ui.Hex("#e8590c"), ok: ui.Hex("#2f9e44"), danger: ui.Hex("#d93025"),
	}
}

func (s *puff) view(c *ui.Context) {
	c.OnShortcut(ui.Cmd, ui.KeyW, s.closePanel)
	c.OnShortcut(ui.Cmd, ui.KeyQ, mygo.App.Quit)
	t := *c.Theme()
	if t.Dark {
		t.Accent, t.AccentHover, t.AccentText = ui.Hex("#b7a2ff"), ui.Hex("#c7b6ff"), ui.Hex("#1e1a2e")
	} else {
		t.Accent, t.AccentHover = ui.Hex("#7c5cdb"), ui.Hex("#6d4fcc")
	}
	c.SetTheme(&t)
	p := newPalette(t.Dark)

	c.Root().Background(ui.Transparent)
	box := ui.Column(c).Fill().TextColor(p.text)
	if !c.Vibrancy() {
		box.Background(p.bg).Radius(12)
	}
	box.Children(func() {
		s.header(c, p)
		switch s.mode() {
		case modeFirstLoad:
			s.skeleton(c, p)
		case modeEmpty:
			s.emptyState(c, p)
		default:
			s.deviceList(c, p)
		}
		ui.Spacer(c)
		s.footer(c, p)
	})
}

func (s *puff) header(c *ui.Context, p palette) {
	ui.Row(c).Height(headHeight).Shrink(0).PaddingStart(16).PaddingEnd(12).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Image(c, s.mascot).Size(30, 30).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, "电团团").FontSize(13).FontWeight(600)
			lead, alert, ink := s.summary(p)
			ui.Row(c).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, lead).FontSize(11).TextColor(p.muted).SingleLine().Shrink(0)
				if alert != "" {
					ui.Text(c, alert).FontSize(11).FontWeight(500).TextColor(ink).SingleLine().Shrink(1).MinWidth(0)
				}
			})
		})
		b := ui.ButtonBase(c.Key("refresh")).Size(26, 26).Shrink(0).Radius(7).Center().
			TextColor(p.muted).Label("刷新电量").Tooltip("刷新电量").OnClick(s.refresh)
		if b.Hovered() && !s.loading {
			b.Background(p.hover).TextColor(p.text)
		}
		if b.Pressed() {
			b.Background(p.tile)
		}
		b.Children(func() {
			if s.loading {
				ui.Spinner(c).Size(14, 14).Label("正在刷新")
			} else {
				ui.Icon(c, refreshIcon).Size(15, 15)
			}
		})
	})
}

// summary is the line under the title: a muted lead, then what needs
// attention in its own color.
func (s *puff) summary(p palette) (lead, alert string, ink Color) {
	switch s.mode() {
	case modeFirstLoad:
		return "正在读取电量…", "", ink
	case modeEmpty:
		if s.err != nil {
			return "", "暂时读不到电量", p.danger
		}
		return "没有发现设备", "", ink
	}
	if s.err != nil {
		return fmt.Sprintf("%d 台设备 ·", len(s.devices)), "读取失败", p.danger
	}
	lead = fmt.Sprintf("%d 台设备 ·", len(s.devices))
	var low []string
	charging := 0
	for _, d := range s.devices {
		if isLow(d) {
			low = append(low, d.Name)
		}
		if d.Charging {
			charging++
		}
	}
	switch {
	case len(low) == 1:
		return lead, low[0] + " 需要充电", p.warn
	case len(low) > 1:
		return lead, fmt.Sprintf("%d 台需要充电", len(low)), p.warn
	case charging > 0:
		return lead, fmt.Sprintf("%d 台充电中", charging), p.ok
	}
	return lead + " 一切正常", "", ink
}

func (s *puff) deviceList(c *ui.Context, p palette) {
	if s.err != nil {
		ui.Row(c).Height(noticeHeight).Shrink(0).Padding(0, 12, 6, 12).Children(func() {
			ui.Row(c).Grow(1).Height(noticeHeight - 6).PaddingX(10).Gap(7).Radius(8).AlignItems(ui.Center).
				Background(p.danger.Alpha(.1)).TextColor(p.danger).Children(func() {
				ui.Icon(c, alertIcon).Size(12, 12)
				ui.Text(c, "读取失败，以下为上次结果").FontSize(11).TextColor(p.danger)
			})
		})
	}
	rows := min(len(s.devices), maxRows)
	ui.Scroll(c).Height(float32(rows*rowHeight + listInset)).Shrink(0).Children(func() {
		list := ui.Column(c).Padding(0, 6, listInset, 6)
		if s.err != nil {
			list.Opacity(.55)
		}
		list.Children(func() {
			for _, d := range s.devices {
				s.deviceRow(c, p, d)
			}
		})
	})
}

func (s *puff) deviceRow(c *ui.Context, p palette, d battery.Device) {
	status, ink := "正常使用", p.muted
	switch {
	case d.Charging && d.Percent != nil && *d.Percent >= 100:
		status, ink = "已充满", p.ok
	case d.Charging:
		status, ink = "充电中", p.ok
	case d.Percent == nil:
		status = "未报告电量"
	case isLow(d):
		status, ink = "电量低，请及时充电", p.warn
	case *d.Percent >= 80:
		status = "电量充足"
	}
	row := ui.Row(c.Key("device-" + d.ID)).Height(rowHeight).Shrink(0).PaddingX(10).Gap(11).Radius(9).
		AlignItems(ui.Center).Tooltip(d.Name + " · " + percent(d))
	if row.Hovered() {
		row.Background(p.hover)
	}
	row.Children(func() {
		ui.Box(c).Size(32, 32).Shrink(0).Radius(8).Background(p.tile).Center().TextColor(p.text).Children(func() {
			ui.Icon(c, deviceIcon(d.Name)).Size(18, 18).Opacity(.85)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, d.Name).FontSize(13).FontWeight(500).SingleLine()
			ui.Row(c).Gap(3).AlignItems(ui.Center).TextColor(ink).Children(func() {
				if d.Charging {
					ui.Icon(c, boltIcon).Size(7, 10)
				}
				ui.Text(c, status).FontSize(11).TextColor(ink).SingleLine()
			})
		})
		label, labelInk, weight := "—", p.faint, 500
		if d.Percent != nil {
			label, labelInk, weight = fmt.Sprintf("%d%%", *d.Percent), p.text, 600
			if isLow(d) {
				labelInk = p.warn
			}
		}
		ui.Text(c, label).FontSize(13).FontWeight(weight).FontFeatures("tnum").
			TextColor(labelInk).MinWidth(36).TextAlign(ui.End).Shrink(0)
		batteryGlyph(c, p, d)
	})
}

// batteryGlyph draws a battery in the style of the system's, filled to the
// device's level: green while charging, warm when low.
func batteryGlyph(c *ui.Context, p palette, d battery.Device) {
	fill := p.text.Alpha(.8)
	switch {
	case d.Charging:
		fill = p.ok
	case isLow(d):
		fill = p.warn
	}
	ui.Row(c).Shrink(0).Gap(1).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Size(23, 12).Border(1, p.faint).Radius(3.5).Padding(1.5).Children(func() {
			if d.Percent != nil {
				ui.Box(c).FillHeight().WidthPercent(max(float32(*d.Percent), 10)).Radius(1.5).Background(fill)
			}
		})
		ui.Box(c).Size(1.5, 4).Radius(1).Background(p.faint)
	})
}

func (s *puff) skeleton(c *ui.Context, p palette) {
	list := ui.Column(c).Padding(0, 6, listInset, 6).Shrink(0)
	list.Opacity(.55 + .45*list.Loop("pulse", 1200*time.Millisecond, ui.Bounce(ui.EaseInOut)))
	list.Children(func() {
		for i := range 2 {
			ui.Row(c.Key(i)).Height(rowHeight).PaddingX(10).Gap(11).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(32, 32).Radius(8).Background(p.tile)
				ui.Column(c).Grow(1).Gap(6).Children(func() {
					ui.Box(c).Height(9).WidthPercent(60).Radius(4).Background(p.tile)
					ui.Box(c).Height(8).WidthPercent(35).Radius(4).Background(p.tile)
				})
			})
		}
	})
}

func (s *puff) emptyState(c *ui.Context, p palette) {
	icon, title := plugIcon, "还没有可读取电量的设备"
	desc, action := "连接键盘、鼠标等蓝牙外设后再刷新。\n部分设备不会向 macOS 报告电量。", "重新检测"
	if s.err != nil {
		icon, title = alertIcon, "暂时读不到电量"
		desc, action = "系统没有返回配件电源信息，\n稍后会自动重试。", "重试"
	}
	ui.Column(c).Height(emptyHeight).Shrink(0).Padding(0, 24, 8, 24).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Box(c).Size(40, 40).Radius(12).Background(p.tile).Center().TextColor(p.muted).Children(func() {
			ui.Icon(c, icon).Size(20, 20)
		})
		ui.Text(c, title).FontSize(13).FontWeight(500).Margin(10, 0, 4, 0)
		ui.Text(c, desc).FontSize(11).LineHeight(1.5).TextColor(p.muted).TextAlign(ui.Center)
		ui.PrimaryButton(c.Key("retry"), action).Label(action).Disabled(s.loading).Margin(12, 0, 0, 0).OnClick(s.refresh)
	})
}

func (s *puff) footer(c *ui.Context, p palette) {
	ui.Box(c).Height(1).Shrink(0).Background(p.line)
	ui.Row(c).Height(footHeight - 1).Shrink(0).PaddingStart(16).PaddingEnd(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, s.footStatus()).Grow(1).FontSize(11).TextColor(p.muted)
		q := ui.ButtonBase(c.Key("quit")).Height(22).PaddingX(6).Radius(5).Center().TextColor(p.muted).
			Label("退出电团团").Tooltip("VoltPuff v" + version).OnClick(mygo.App.Quit)
		if q.Hovered() {
			q.Background(p.hover).TextColor(p.text)
		}
		q.Children(func() { ui.Text(c, "退出").FontSize(11) })
	})
}

func (s *puff) footStatus() string {
	switch {
	case s.updated.IsZero() && s.err == nil:
		return "每分钟自动刷新"
	case s.err != nil && s.updated.IsZero():
		return "读取失败"
	case s.err != nil:
		return "上次成功 " + s.updated.Format("15:04")
	case s.loading:
		return "正在刷新…"
	}
	if m := int(time.Since(s.updated).Minutes()); m >= 1 {
		return fmt.Sprintf("%d 分钟前更新", m)
	}
	return "刚刚更新"
}
