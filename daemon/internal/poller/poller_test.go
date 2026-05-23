package poller

import (
	"net/http"
	"testing"
	"time"
)

// Unit tests target pure functions: no I/O, no goroutines, no time.Sleep.
// Table-driven style is idiomatic Go — one struct slice, one t.Run per case.

func TestParsePollInterval(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"empty header", "", 0},
		{"valid 60", "60", 60 * time.Second},
		{"valid 120", "120", 120 * time.Second},
		{"zero is treated as unset", "0", 0},
		{"negative is rejected", "-5", 0},
		{"garbage is rejected", "soon", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePollInterval(tc.in)
			if got != tc.want {
				t.Errorf("parsePollInterval(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Run("seconds form", func(t *testing.T) {
		if got := parseRetryAfter("30"); got != 30*time.Second {
			t.Errorf("got %v, want 30s", got)
		}
	})

	t.Run("http-date form in the future", func(t *testing.T) {
		future := time.Now().Add(45 * time.Second).UTC().Format(http.TimeFormat)
		got := parseRetryAfter(future)
		// allow a couple seconds of slack — wall clock can drift during the test
		if got < 40*time.Second || got > 50*time.Second {
			t.Errorf("got %v, want ~45s", got)
		}
	})

	t.Run("http-date in the past returns zero", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour).UTC().Format(http.TimeFormat)
		if got := parseRetryAfter(past); got != 0 {
			t.Errorf("got %v, want 0", got)
		}
	})

	t.Run("garbage returns zero", func(t *testing.T) {
		if got := parseRetryAfter("not a date"); got != 0 {
			t.Errorf("got %v, want 0", got)
		}
	})
}
