package main

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/knothhe/VoltPuff/internal/battery"
)

// mockScene stands in for pmset with -mock, to preview every state of the
// panel without the matching hardware.
type mockScene struct {
	devices []battery.Device
	err     error // returned by every read; devices show as the last result
	stall   bool  // reads never finish: the first-load state
}

func level(n int) *int { return &n }

var mockScenes = map[string]mockScene{
	"normal": {devices: []battery.Device{
		{ID: "1", Name: "SKN-QingLong 3.0", Percent: level(84)},
		{ID: "2", Name: "MX Master 3", Percent: level(62)},
	}},
	"low": {devices: []battery.Device{
		{ID: "1", Name: "SKN-QingLong 3.0", Percent: level(84)},
		{ID: "2", Name: "MX Master 3", Percent: level(5)},
	}},
	"charging": {devices: []battery.Device{
		{ID: "1", Name: "SKN-QingLong 3.0", Percent: level(84)},
		{ID: "2", Name: "MX Master 3", Percent: level(37), Charging: true},
	}},
	"many": {devices: []battery.Device{
		{ID: "1", Name: "Magic Keyboard", Percent: level(92)},
		{ID: "2", Name: "MX Master 3", Percent: level(18)},
		{ID: "3", Name: "Magic Trackpad", Percent: level(100), Charging: true},
		{ID: "4", Name: "AirPods Pro", Percent: level(54)},
		{ID: "5", Name: "Xbox Wireless Controller"},
	}},
	"empty": {devices: []battery.Device{}},
	"error": {
		devices: []battery.Device{
			{ID: "1", Name: "SKN-QingLong 3.0", Percent: level(84)},
			{ID: "2", Name: "MX Master 3", Percent: level(62)},
		},
		err: errors.New("pmset: exit status 1"),
	},
	"error-empty": {err: errors.New("pmset: exit status 1")},
	"loading":     {stall: true},
}

func mockNames() []string {
	names := make([]string, 0, len(mockScenes))
	for name := range mockScenes {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (m mockScene) read(ctx context.Context) ([]battery.Device, error) {
	delay := 600 * time.Millisecond
	if m.stall {
		delay = time.Duration(1<<63 - 1)
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(delay):
	}
	if m.err != nil {
		return nil, m.err
	}
	return slices.Clone(m.devices), nil
}
