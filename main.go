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

var dismissIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16"><path d="M4 4L12 12M12 4L4 12" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>`))

//go:embed assets/tray.png
var trayPNG []byte

//go:embed assets/icon.png
var iconPNG []byte

type puff struct {
	win                  *mygo.Window
	tray                 *mygo.Tray
	mascot               *ui.Bitmap
	ctx                  context.Context
	cancel               context.CancelFunc
	devices              []battery.Device
	updated              time.Time
	err                  error
	loading              bool
	quitting             bool
	visibilityGeneration uint64
	showAtStart          bool
}

func main() {
	showPanel := flag.Bool("show", false, "open the panel after launch; the default starts in the menu bar only")
	list := flag.Bool("list", false, "print accessory batteries as JSON without starting the GUI")
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
	mygo.App.SetName("VoltPuff")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &puff{ctx: ctx, cancel: cancel, showAtStart: *showPanel}
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { s.quitting = true; s.cancel() })
	mygo.App.OnSecondInstance(func([]string, string) {
		if s.win != nil {
			s.show()
		}
	})
	mygo.App.WhenReady(s.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (s *puff) ready() {
	var err error
	s.mascot, err = ui.DecodeBitmap(iconPNG)
	if err != nil {
		log.Fatal(err)
	}
	s.tray, err = mygo.NewTray(mygo.TrayOptions{
		ID:   "voltpuff.knothhe.com.battery",
		Icon: trayPNG, IconIsTemplate: true,
		ToolTip: "电团团 · 正在查看小伙伴的电量…",
	})
	if err != nil {
		log.Fatal("create tray: ", err)
	}
	s.win = mygo.NewWindow(mygo.WindowOptions{
		Title: "VoltPuff · 电团团", Width: 380, Height: 416,
		Hidden: true, Frameless: true, AlwaysOnTop: true,
		DisableResize: true, DisableMinimize: true, DisableMaximize: true,
		DisableFullScreen: true, SkipTaskbar: true,
		BackgroundColor: "light-dark(#faf8ff, #1e1c27)",
		Content:         ui.View(s.view),
	})
	s.win.OnBlur(s.dismissAfterBlur)
	s.win.OnClose(func(e *mygo.CloseEvent) {
		if !s.quitting {
			e.PreventDefault()
			s.dismiss()
		}
	})
	s.tray.OnClick(func() {
		if s.win.IsVisible() {
			s.dismiss()
		} else {
			s.show()
		}
	})
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开电团团", Click: func(*mygo.MenuItem, *mygo.Window) { s.show() }},
		{Label: "刷新电量", Click: func(*mygo.MenuItem, *mygo.Window) { s.refresh() }},
		mygo.Separator(),
		{Label: "VoltPuff v" + version, Disabled: true},
		{Label: "退出电团团", Role: mygo.RoleQuit},
	})
	s.tray.OnRightClick(func() {
		s.dismiss()
		s.tray.SetMenu(menu)
		defer s.tray.SetMenu(nil)
		s.tray.PopUpMenu()
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { s.tray.Destroy() })
	s.refresh()
	if s.showAtStart {
		s.show()
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

// refresh is entered on the main thread. One bounded process runs at a time;
// result publication and every UI mutation also stay on the main thread.
func (s *puff) refresh() {
	if s.loading || s.quitting {
		return
	}
	s.loading = true
	s.win.Invalidate()
	go func() {
		devices, err := battery.Read(s.ctx)
		s.win.Update(func() {
			if s.quitting {
				return
			}
			s.loading, s.err = false, err
			if err == nil {
				s.devices, s.updated = devices, time.Now()
			}
			s.updateTray()
		})
	}()
}

func (s *puff) updateTray() {
	if s.err != nil {
		s.tray.SetTitle("—")
		s.tray.SetToolTip("电团团 · 电量读取失败，点击重试")
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
		lines = append(lines, "还没有可读取电量的小伙伴")
	}
	s.tray.SetToolTip(strings.Join(lines, "\n"))
}

// dismiss only hides the panel. The window stays alive, preserving the tray
// and refresh loop; only the explicit Quit action ends the application.
func (s *puff) dismiss() {
	if s.win != nil && !s.quitting {
		s.visibilityGeneration++
		s.win.Hide()
	}
}

// A short focus transition can occur while macOS raises an accessory window.
// Recheck after that transition, and never dismiss a panel opened since then.
func (s *puff) dismissAfterBlur() {
	generation := s.visibilityGeneration
	time.AfterFunc(150*time.Millisecond, func() {
		mygo.RunOnMain(func() {
			if !s.quitting && s.visibilityGeneration == generation && !s.win.IsFocused() {
				s.dismiss()
			}
		})
	})
}

func (s *puff) show() {
	s.visibilityGeneration++
	a := s.tray.Bounds()
	if a.Width <= 0 || a.Height <= 0 {
		// A just-created status item may not have a frame yet. Keep an
		// explicit show request near the menu bar until its anchor is ready.
		area := mygo.Screen.PrimaryDisplay().WorkArea
		b := s.win.Bounds()
		s.win.SetPosition(area.X+max(0, area.Width-b.Width-12), area.Y+8)
	} else {
		area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: a.X + a.Width/2, Y: a.Y + a.Height/2}).WorkArea
		b := s.win.Bounds()
		x := max(area.X, min(a.X+(a.Width-b.Width)/2, area.X+max(0, area.Width-b.Width)))
		y := max(area.Y, min(a.Y+a.Height+8, area.Y+max(0, area.Height-b.Height)))
		s.win.SetPosition(x, y)
	}
	s.win.Show()
	s.refresh()
}

func percent(d battery.Device) string {
	if d.Percent == nil {
		return "电量未知"
	}
	return fmt.Sprintf("%d%%", *d.Percent)
}

func (s *puff) view(c *ui.Context) {
	c.OnShortcut(0, ui.KeyEscape, s.dismiss)
	c.OnShortcut(ui.Cmd, ui.KeyW, s.dismiss)
	c.OnShortcut(ui.Cmd, ui.KeyQ, mygo.App.Quit)
	t := *c.Theme()
	if t.Dark {
		t.Background, t.Surface = ui.Hex("#1e1c27"), ui.Hex("#2b2836")
		t.Accent, t.AccentHover = ui.Hex("#bca7f7"), ui.Hex("#cbbbff")
		t.AccentText = ui.Hex("#211b33")
	} else {
		t.Background, t.Surface = ui.Hex("#faf8ff"), ui.Hex("#ffffff")
		t.Accent, t.AccentHover = ui.Hex("#8260cf"), ui.Hex("#7452c0")
	}
	t.Radius = 10
	c.SetTheme(&t)
	ui.Column(c).Fill().Background(t.Background).Padding(22).Gap(16).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Image(c, s.mascot).Size(48, 48)
			ui.Column(c).Grow(1).Gap(3).Children(func() {
				ui.Text(c, "VoltPuff").FontSize(22).Bold()
				ui.Text(c, "电团团 · 陪你安心用电").FontSize(11).TextColor(t.TextMuted)
			})
			close := ui.ButtonBase(c.Key("dismiss-panel")).Size(28, 28).Shrink(0).
				Radius(8).TextColor(t.TextMuted).Label("收起面板").
				Tooltip("收起面板，电团团继续在菜单栏运行").OnClick(s.dismiss)
			if close.Hovered() {
				close.Background(t.SurfaceHover).TextColor(t.Text)
			}
			if close.Pressed() {
				close.Background(t.SurfacePressed)
			}
			close.Children(func() { ui.Icon(c, dismissIcon).Size(12, 12) })
		})
		ui.Column(c).Gap(4).Children(func() {
			ui.Textf(c, "%d 位小伙伴", len(s.devices)).FontSize(14).Bold()
			message := "电量的小事，交给团团。"
			if low, ok := battery.Lowest(s.devices); ok && low <= 20 {
				message = "有小伙伴饿啦，记得补充一点电。"
			}
			ui.Text(c, message).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(10).Children(func() {
				if s.err != nil {
					ui.Column(c).Padding(12).Gap(4).Background(t.Surface).Radius(10).Children(func() {
						ui.Text(c, "暂时没读到电量").FontSize(13).TextColor(t.Danger).Bold()
						message := "点击刷新重试。"
						if len(s.devices) > 0 {
							message = "下方为上次成功读取的结果。"
						}
						ui.Text(c, message).FontSize(11).TextColor(t.TextMuted)
					})
				}
				if len(s.devices) == 0 && s.err == nil {
					message := "正在寻找小伙伴…"
					if !s.loading {
						message = "还没有可读取电量的设备"
					}
					ui.Column(c).Padding(18).Gap(8).Background(t.Surface).Radius(12).Children(func() {
						ui.Text(c, message).FontSize(13).Bold()
						ui.Text(c, "连接键盘、鼠标等外设后，点击刷新。\n部分设备不会向 macOS 报告电量。").FontSize(11).TextColor(t.TextMuted)
					})
				}
				for _, d := range s.devices {
					s.deviceCard(c, &t, d)
				}
			})
		})
		ui.Divider(c)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			status := "每分钟自动刷新"
			if s.loading {
				status = "正在刷新…"
			} else if s.err != nil {
				status = "读取失败 · 请重试"
			} else if !s.updated.IsZero() {
				status = "更新于 " + s.updated.Format("15:04:05")
			}
			ui.Text(c, status).Grow(1).FontSize(10).TextColor(t.TextMuted)
			label := "刷新"
			if s.loading {
				label = "读取中"
			}
			ui.PrimaryButton(c, label).Disabled(s.loading).Label("刷新电量").OnClick(func() { s.win.Focus(); s.refresh() })
		})
		ui.Text(c, "v"+version+" · 每分钟照看一次").FontSize(10).TextColor(t.TextMuted)
	})
}

func (s *puff) deviceCard(c *ui.Context, t *ui.Theme, d battery.Device) {
	ink, status := t.Accent, "安心使用中"
	if d.Percent == nil {
		status = "设备尚未报告电量"
	} else if *d.Percent <= 20 {
		ink, status = t.Warning, "该补充能量啦"
	} else if *d.Percent >= 80 {
		status = "电量满满"
	}
	if d.Charging {
		ink, status = t.Success, "正在充电"
	}
	ui.Column(c).Padding(14).Gap(10).Background(t.Surface).Radius(12).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Gap(4).Children(func() {
				ui.Text(c, d.Name).FontSize(13).Bold().Tooltip(d.Name)
				ui.Text(c, status).FontSize(10).TextColor(t.TextMuted)
			})
			ui.Text(c, percent(d)).FontSize(24).Bold().TextColor(ink).Shrink(0)
		})
		if d.Percent != nil {
			ui.Row(c).Height(5).Background(t.Border).Radius(3).Clip().Children(func() {
				ui.Box(c).Height(5).WidthPercent(float32(*d.Percent)).Background(ink).Radius(3)
			})
		}
	})
}
