// Package battery reads accessory power sources already known to macOS.
package battery

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Device struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Percent  *int   `json:"battery_percent"`
	Charging bool   `json:"charging"`
}

var (
	deviceLine = regexp.MustCompile(`^\s*-(.*?)\s+\(id=(\d+)\)\s*(.*)$`)
	percentage = regexp.MustCompile(`(?:^|\s)(\d+)\s*%`)
	charging   = regexp.MustCompile(`(?i)\bcharging\b`)
)

// Read is bounded, read-only and never scans, pairs or reconnects devices.
func Read(parent context.Context) ([]Device, error) {
	if runtime.GOOS != "darwin" {
		return nil, errors.New("accessory batteries require macOS")
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/pmset", "-g", "accps")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("read accessory batteries: %w", ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("pmset: %w", err)
	}
	return Parse(string(out))
}

// Parse preserves names, source order, unknown levels and genuine 0% readings.
// It rejects unexpected output rather than presenting it as no devices.
func Parse(output string) ([]Device, error) {
	devices := make([]Device, 0)
	heading := false
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "Now drawing from ") {
			heading = true
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		match := deviceLine.FindStringSubmatch(line)
		if match == nil || strings.TrimSpace(match[1]) == "" {
			return nil, errors.New("unrecognized accessory power output")
		}
		d := Device{ID: match[2], Name: strings.TrimSpace(match[1])}
		if p := percentage.FindStringSubmatch(match[3]); p != nil {
			n, err := strconv.Atoi(p[1])
			if err != nil || n > 100 {
				return nil, fmt.Errorf("invalid battery percentage for %q", d.Name)
			}
			d.Percent = &n
		}
		status := strings.ToLower(match[3])
		d.Charging = charging.MatchString(status) && !strings.Contains(status, "not charging")
		devices = append(devices, d)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read power output: %w", err)
	}
	if !heading {
		return nil, errors.New("missing accessory power header")
	}
	return devices, nil
}

// Lowest ignores unknown readings instead of treating them as empty batteries.
func Lowest(devices []Device) (int, bool) {
	lowest, found := 100, false
	for _, d := range devices {
		if d.Percent != nil {
			lowest, found = min(lowest, *d.Percent), true
		}
	}
	return lowest, found
}
