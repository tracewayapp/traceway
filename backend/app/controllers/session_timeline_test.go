package controllers

import (
	"slices"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

func TestTrimToCoveredWindow(t *testing.T) {
	base := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	at := func(seconds int) time.Time { return base.Add(time.Duration(seconds) * time.Second) }
	logsAt := func(seconds ...int) []models.LogRecord {
		var out []models.LogRecord
		for _, s := range seconds {
			out = append(out, models.LogRecord{Timestamp: at(s)})
		}
		return out
	}
	spansAt := func(seconds ...int) []shared.SessionSpan {
		var out []shared.SessionSpan
		for _, s := range seconds {
			out = append(out, shared.SessionSpan{StartTime: at(s)})
		}
		return out
	}
	logSeconds := func(records []models.LogRecord) []int {
		out := []int{}
		for _, r := range records {
			out = append(out, int(r.Timestamp.Sub(base)/time.Second))
		}
		return out
	}
	spanSeconds := func(spans []shared.SessionSpan) []int {
		out := []int{}
		for _, s := range spans {
			out = append(out, int(s.StartTime.Sub(base)/time.Second))
		}
		return out
	}

	cases := []struct {
		name          string
		newest        bool
		logs          []models.LogRecord
		spans         []shared.SessionSpan
		wantLogs      []int
		wantSpans     []int
		wantTruncated bool
	}{
		{"under the limit", true, logsAt(1, 5), spansAt(2), []int{1, 5}, []int{2}, false},
		{"newest, logs full", true, logsAt(6, 7, 8), spansAt(1, 6), []int{6, 7, 8}, []int{6}, true},
		{"newest, both full", true, logsAt(4, 5, 6), spansAt(5, 7, 9), []int{5, 6}, []int{5, 7, 9}, true},
		{"oldest, spans full", false, logsAt(1, 8), spansAt(0, 2, 4), []int{1}, []int{0, 2, 4}, true},
		{"oldest, both full", false, logsAt(1, 2, 3), spansAt(0, 1, 5), []int{1, 2, 3}, []int{0, 1}, true},
	}
	for _, tc := range cases {
		q := shared.SessionBreadcrumbQuery{Limit: 3, Newest: tc.newest}
		records, spans, truncated := trimToCoveredWindow(q, tc.logs, tc.spans)
		if truncated != tc.wantTruncated || !slices.Equal(logSeconds(records), tc.wantLogs) || !slices.Equal(spanSeconds(spans), tc.wantSpans) {
			t.Errorf("%s: logs %v spans %v truncated %v, want %v %v %v", tc.name,
				logSeconds(records), spanSeconds(spans), truncated, tc.wantLogs, tc.wantSpans, tc.wantTruncated)
		}
	}
}
