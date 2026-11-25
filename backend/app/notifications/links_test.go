package notifications

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWithProjectIdPinsDashboardPaths(t *testing.T) {
	id := uuid.MustParse("6c26de62-38ad-41bb-8012-74f212d2b7c1")
	cases := map[string]string{
		"/issues/abc":                   "/issues/abc?projectId=" + id.String(),
		"/dashboards?preset=60m":        "/dashboards?preset=60m&projectId=" + id.String(),
		"/dashboards?projectId=other":   "/dashboards?projectId=other",
		"https://example.com/elsewhere": "https://example.com/elsewhere",
		"":                              "",
	}
	for in, want := range cases {
		if got := withProjectId(in, id); got != want {
			t.Errorf("withProjectId(%q) = %q, want %q", in, got, want)
		}
	}
	if got := withProjectId("/issues/abc", uuid.Nil); got != "/issues/abc" {
		t.Errorf("a rule without a project changed the path: %q", got)
	}
}

// The latency email said "last 10 minutes" but linked a three minute range.
func TestEndpointLinkCoversTheRuleWindow(t *testing.T) {
	now := time.Date(2026, 11, 25, 5, 7, 47, 0, time.UTC)
	if got, want := endpointTimeRangeURL(now, 10), "/endpoints?from=2026-11-25T04:57:47Z&to=2026-11-25T05:08:47Z"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if got := endpointTimeRangeURL(now, 0); !strings.Contains(got, "from=2026-11-25T04:52:47Z") {
		t.Errorf("a rule without a window should link the last 15 minutes: %s", got)
	}
}

// Every preset a notification links with must be one the dashboard knows
// (frontend/src/lib/utils/time-presets.ts): an unknown one is ignored there,
// and "1h" was used for every metric and issue link.
func TestNotificationLinksUseKnownPresets(t *testing.T) {
	known := map[string]bool{"5m": true, "30m": true, "60m": true, "3h": true, "6h": true, "12h": true, "24h": true, "3d": true, "7d": true, "1M": true, "3M": true}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	preset := regexp.MustCompile(`[?&]preset=([0-9A-Za-z]+)`)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range preset.FindAllSubmatch(source, -1) {
			if !known[string(m[1])] {
				t.Errorf("%s links the unknown preset %q", file, m[1])
			}
		}
	}
}
