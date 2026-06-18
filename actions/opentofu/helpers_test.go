package opentofu

import (
	"testing"
	"time"

	core "flomation.app/automate/executor"
)

func TestTimeout(t *testing.T) {
	cases := []struct {
		raw     string
		want    time.Duration
		wantErr bool
	}{
		{"", 600 * time.Second, false}, // default
		{"120", 120 * time.Second, false},
		{"3600", 3600 * time.Second, false}, // exactly max
		{"7200", 0, true},                   // over max -> error, not silent clamp
		{"0", 0, true},
		{"-5", 0, true},
		{"abc", 0, true},
	}
	for _, tc := range cases {
		got, err := Timeout(tc.raw, 600, 3600)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Timeout(%q): expected error, got %v", tc.raw, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Timeout(%q): unexpected error %v", tc.raw, err)
		}
		if got != tc.want {
			t.Errorf("Timeout(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestOptBool(t *testing.T) {
	str := func(v string) []*core.Connection {
		return []*core.Connection{{Name: "flag", Type: core.ConnectionTypeBoolean, Value: v}}
	}
	if !OptBool("flag", str("true"), false) {
		t.Error("true should be true")
	}
	if OptBool("flag", str("false"), true) {
		t.Error("false should be false")
	}
	// Unset -> default applies (covers the default-on require_approval idiom).
	if !OptBool("missing", nil, true) {
		t.Error("unset should fall back to default true")
	}
	if OptBool("missing", nil, false) {
		t.Error("unset should fall back to default false")
	}
}
