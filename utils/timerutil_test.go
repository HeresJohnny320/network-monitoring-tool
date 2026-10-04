package utils

import (
	"testing"
	"time"
)

func TestParseInterval(t *testing.T) {
	cases := map[string]time.Duration{
		"30s":   30 * time.Second,
		"5m":    5 * time.Minute,
		"1h30m": 90 * time.Minute,
		"1d":    24 * time.Hour,
		"1d12h": 36 * time.Hour,
	}
	for in, want := range cases {
		if got, err := ParseInterval(in); err != nil || got != want {
			t.Errorf("ParseInterval(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "abc", "5x", "1h junk"} {
		if _, err := ParseInterval(bad); err == nil {
			t.Errorf("ParseInterval(%q) should fail", bad)
		}
	}
}

func TestNextRunTime(t *testing.T) {
	now := time.Date(2026, 5, 1, 10, 17, 3, 0, time.Local)
	if got := NextRunTime(now, time.Hour, true); !got.Equal(time.Date(2026, 5, 1, 11, 0, 0, 0, time.Local)) {
		t.Errorf("aligned 1h: %v", got)
	}
	if got := NextRunTime(now, 15*time.Minute, true); !got.Equal(time.Date(2026, 5, 1, 10, 30, 0, 0, time.Local)) {
		t.Errorf("aligned 15m: %v", got)
	}
	if got := NextRunTime(now, 7*time.Hour, true); !got.Equal(time.Date(2026, 5, 1, 14, 0, 0, 0, time.Local)) {
		t.Errorf("aligned 7h: %v", got)
	}
	late := time.Date(2026, 5, 1, 22, 0, 0, 0, time.Local)
	if got := NextRunTime(late, 7*time.Hour, true); !got.Equal(time.Date(2026, 5, 2, 0, 0, 0, 0, time.Local)) {
		t.Errorf("aligned 7h near midnight: %v", got)
	}
	if got := NextRunTime(now, time.Hour, false); !got.Equal(now.Add(time.Hour)) {
		t.Errorf("unaligned: %v", got)
	}
}

func TestValidateRejectsFlagLikeHosts(t *testing.T) {
	c := DefaultConfig()
	c.PingHost = []string{"-f"}
	if c.Validate() == nil {
		t.Fatal("host starting with - must be rejected")
	}
	c.PingHost = []string{"google.com", "1.1.1.1", "2606:4700:4700::1111", "router.lan"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
