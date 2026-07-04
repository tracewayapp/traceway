package timerange

import (
	"errors"
	"testing"
	"time"
)

func TestParseRelativeDuration_validInputs(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"1h", time.Hour},
		{"24h", 24 * time.Hour},
		{"7d", 168 * time.Hour},
		{"30d", 720 * time.Hour},
		{"1d", 24 * time.Hour},
		{"30m", 30 * time.Minute},
		{"0s", 0},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseRelativeDuration(tc.in)
			if err != nil {
				t.Fatalf("ParseRelativeDuration(%q) error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseRelativeDuration(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseRelativeDuration_invalidInputs(t *testing.T) {
	cases := []string{
		"0d",
		"-1d",
		"7days",
		"7D",
		"7d2h",
		"d",
		"notaduration",
		"",
		"-1h",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseRelativeDuration(in); err == nil {
				t.Errorf("ParseRelativeDuration(%q) expected error, got nil", in)
			}
		})
	}
}

func TestResolve_defaultIsLastHour(t *testing.T) {
	tr, err := Resolve("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	delta := tr.To.Sub(tr.From)
	if delta < 59*time.Minute || delta > 61*time.Minute {
		t.Errorf("default range should be ~1h, got %v", delta)
	}
}

func TestResolve_explicitFromTo(t *testing.T) {
	tr, err := Resolve("", "2026-05-13T00:00:00Z", "2026-05-13T23:59:59Z")
	if err != nil {
		t.Fatal(err)
	}
	wantFrom, _ := time.Parse(time.RFC3339, "2026-05-13T00:00:00Z")
	wantTo, _ := time.Parse(time.RFC3339, "2026-05-13T23:59:59Z")
	if !tr.From.Equal(wantFrom) {
		t.Errorf("From = %v, want %v", tr.From, wantFrom)
	}
	if !tr.To.Equal(wantTo) {
		t.Errorf("To = %v, want %v", tr.To, wantTo)
	}
}

func TestResolve_fromEqualToToIsAccepted(t *testing.T) {
	tr, err := Resolve("", "2026-05-13T00:00:00Z", "2026-05-13T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if !tr.From.Equal(tr.To) {
		t.Errorf("From = %v, To = %v, want equal", tr.From, tr.To)
	}
}

func TestResolve_invalidCombinationsWrapErrInvalid(t *testing.T) {
	cases := []struct {
		name            string
		since, from, to string
	}{
		{"since with from", "1h", "2026-05-13T00:00:00Z", ""},
		{"from without to", "", "2026-05-13T00:00:00Z", ""},
		{"to without from", "", "", "2026-05-13T23:59:59Z"},
		{"bad from", "", "not-a-date", "2026-05-13T23:59:59Z"},
		{"bad to", "", "2026-05-13T00:00:00Z", "not-a-date"},
		{"bad since", "notaduration", "", ""},
		{"negative since", "-1h", "", ""},
		{"from after to", "", "2026-05-13T23:59:59Z", "2026-05-13T00:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Resolve(tc.since, tc.from, tc.to)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("error %v should wrap ErrInvalid", err)
			}
		})
	}
}
