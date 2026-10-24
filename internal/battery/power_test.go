package battery

import (
	"reflect"
	"testing"
)

func TestParseRealAccessoryOutput(t *testing.T) {
	out := "Now drawing from 'AC Power'\n -SKN-QingLong 3.0 (id=53420486)\t84%; \n -MX Master 3 (id=53420487)\t20%; \n"
	got, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	p84, p20 := 84, 20
	want := []Device{{ID: "53420486", Name: "SKN-QingLong 3.0", Percent: &p84}, {ID: "53420487", Name: "MX Master 3", Percent: &p20}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if low, ok := Lowest(got); !ok || low != 20 {
		t.Fatalf("lowest = %d, %v", low, ok)
	}
}

func TestParseBatteryStates(t *testing.T) {
	for _, tt := range []struct {
		name, status string
		percent      int
		unknown      bool
		charging     bool
	}{
		{"empty battery", "0%; discharging", 0, false, false},
		{"full battery", "100%; charged", 100, false, false},
		{"charging", "42%; charging", 42, false, true},
		{"not charging", "42%; not charging", 42, false, false},
		{"unknown", "(no estimate)", 0, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse("Now drawing from 'AC Power'\n -团团's Keyboard (Office) (id=123)\t" + tt.status + "\n")
			if err != nil || len(got) != 1 {
				t.Fatalf("devices=%v, err=%v", got, err)
			}
			d := got[0]
			if d.Name != "团团's Keyboard (Office)" || d.Charging != tt.charging || (d.Percent == nil) != tt.unknown {
				t.Fatalf("unexpected device: %#v", d)
			}
			if !tt.unknown && *d.Percent != tt.percent {
				t.Fatalf("percent = %d, want %d", *d.Percent, tt.percent)
			}
		})
	}
}

func TestParseNoAccessories(t *testing.T) {
	got, err := Parse("Now drawing from 'AC Power'\n")
	if err != nil || len(got) != 0 {
		t.Fatalf("devices=%v err=%v", got, err)
	}
	if _, ok := Lowest(got); ok {
		t.Fatal("empty list should not have a lowest percentage")
	}
	if _, ok := Lowest([]Device{{Name: "Unknown"}}); ok {
		t.Fatal("unknown battery should not have a lowest percentage")
	}
}

func TestParseRejectsInvalidOutput(t *testing.T) {
	for _, out := range []string{
		"", "permission denied", "Now drawing from 'AC Power'\n -Keyboard (id=4) 101%;",
		"Now drawing from 'AC Power'\n -Keyboard invalid", "Now drawing from 'AC Power'\n - (id=4) 50%;",
	} {
		if _, err := Parse(out); err == nil {
			t.Errorf("accepted invalid output %q", out)
		}
	}
}
