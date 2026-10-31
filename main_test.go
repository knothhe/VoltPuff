package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// sceneState puts a panel in the state a mock scene ends up in.
func sceneState(t *testing.T, name string) *puff {
	t.Helper()
	m := mockScenes[name]
	var err error
	s := &puff{}
	s.mascot, err = ui.DecodeBitmap(iconPNG)
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case m.stall:
		s.loading = true
	case m.err != nil:
		s.err = m.err
		if m.devices != nil {
			s.devices, s.updated = m.devices, time.Now().Add(-11*time.Minute)
		}
	default:
		s.devices, s.updated = m.devices, time.Now()
	}
	return s
}

func TestPanelScenes(t *testing.T) {
	want := map[string][]string{
		"normal":      {"电团团", "2 台设备 · 一切正常", "SKN-QingLong 3.0", "84%", "刚刚更新"},
		"low":         {"MX Master 3 需要充电", "5%", "电量低，请及时充电"},
		"charging":    {"1 台充电中", "充电中"},
		"many":        {"5 台设备 ·", "已充满", "未报告电量", "—"},
		"empty":       {"没有发现设备", "还没有可读取电量的设备", "重新检测"},
		"error":       {"读取失败", "读取失败，以下为上次结果", "上次成功"},
		"error-empty": {"暂时读不到电量", "重试"},
		"loading":     {"正在读取电量…"},
	}
	out := os.Getenv("VOLTPUFF_SNAPSHOTS") // a directory to save the frames to
	for _, name := range mockNames() {
		for _, dark := range []bool{false, true} {
			s := sceneState(t, name)
			tt := ui.NewTester(s.view, panelWidth, s.panelHeight())
			tt.SetDark(dark)
			tt.SetScale(2)
			tt.Frame()
			for _, text := range want[name] {
				if !tt.HasText(text) {
					t.Errorf("%s: missing %q in %q", name, text, tt.Texts())
				}
			}
			if out == "" {
				continue
			}
			file := name + ".png"
			if dark {
				file = name + "-dark.png"
			}
			f, err := os.Create(filepath.Join(out, file))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}

func TestPanelHeightFollowsDevices(t *testing.T) {
	s := sceneState(t, "many")
	if got, want := s.panelHeight(), headHeight+footHeight+5*rowHeight+listInset; got != want {
		t.Errorf("height %d, want %d", got, want)
	}
	s.devices = append(s.devices, s.devices...)
	if got, want := s.panelHeight(), headHeight+footHeight+maxRows*rowHeight+listInset; got != want {
		t.Errorf("height with %d devices %d, want %d (capped)", len(s.devices), got, want)
	}
}

func TestDeviceIcon(t *testing.T) {
	for name, want := range map[string]*ui.SVG{
		"SKN-QingLong 3.0":         keyboardIcon,
		"Magic Keyboard":           keyboardIcon,
		"MX Master 3":              mouseIcon,
		"Magic Trackpad":           trackpadIcon,
		"AirPods Pro":              headphonesIcon,
		"Xbox Wireless Controller": gamepadIcon,
		"Unknown Thing":            genericIcon,
	} {
		if deviceIcon(name) != want {
			t.Errorf("%s: wrong icon", name)
		}
	}
}
