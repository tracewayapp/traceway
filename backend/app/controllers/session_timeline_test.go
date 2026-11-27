package controllers

import (
	"fmt"
	"maps"
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

func TestTimelineKeepsWhatATapShows(t *testing.T) {
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	click := func(attrs map[string]string) models.LogRecord {
		attrs["event.name"] = "browser.user_action.click"
		attrs["session.id"] = "s"
		return models.LogRecord{Timestamp: at, LogAttributes: attrs}
	}
	timeline := BuildSessionTimeline([]models.LogRecord{
		click(map[string]string{"browser.css_selector": "#checkout .pay", "browser.tag_name": "button", "url.full": "https://shop.test/cart"}),
		click(map[string]string{
			"browser.css_selector": "MaterialButton#pay", "browser.tag_name": "MaterialButton",
			"app.widget.name": "MaterialButton", "app.widget.id": "pay", "app.widget.text": "Pay now",
			"app.widget.label": "Pay", "app.screen.name": "/CheckoutActivity",
			"app.screen.coordinate.x": "120", "app.screen.coordinate.y": "640",
		}),
		click(map[string]string{"browser.css_selector": "UIButton[label=\"Delete\"]", "browser.tag_name": "UIButton", "app.widget.label": "Delete", "app.widget.text": ""}),
	}, nil)

	want := []map[string]any{
		{"selector": "#checkout .pay", "tag": "button"},
		{"selector": "MaterialButton#pay", "tag": "MaterialButton", "id": "pay", "text": "Pay now", "label": "Pay", "screen": "/CheckoutActivity"},
		{"selector": `UIButton[label="Delete"]`, "tag": "UIButton", "label": "Delete"},
	}
	if len(timeline.Actions) != len(want) {
		t.Fatalf("actions = %+v", timeline.Actions)
	}
	for i, action := range timeline.Actions {
		if action.Type != "custom" || action.Category != "ui" || action.Name != "click" || !maps.Equal(action.Data, want[i]) {
			t.Errorf("action %d = %+v, want data %v", i, action, want[i])
		}
	}
}

// TW-8: the SDKs send traceway.collection.health with their other control events; it describes the SDK, not the
// user, and is left out of the timeline and of the SQL read that counts against the breadcrumb limit.
func TestTimelineSkipsCollectionHealth(t *testing.T) {
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	timeline := BuildSessionTimeline([]models.LogRecord{{Timestamp: at, LogAttributes: map[string]string{
		"event.name": "traceway.collection.health", "traceway.collection.dropped": "12",
	}}}, nil)
	if len(timeline.Actions) != 0 || len(timeline.Logs) != 0 {
		t.Fatalf("health report shown on the timeline: %+v", timeline)
	}
	if !slices.Contains(shared.SessionTimelineSkippedEvents, "traceway.collection.health") {
		t.Fatal("traceway.collection.health is not filtered in SQL")
	}
}

// TW-9: an app event row carries the event's own attributes, without the capture ids the SDKs stamp on every
// record during a dashboard capture, and the same ones on every read when it has more than fit.
func TestTimelineEventDataIsTheEventsOwnAndStable(t *testing.T) {
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	checkout := models.LogRecord{Timestamp: at, LogAttributes: map[string]string{
		"event.name": "checkout", "session.id": "s", "traceway.capture.ids": `["0195"]`, "cart.total": "30",
	}}
	timeline := BuildSessionTimeline([]models.LogRecord{checkout}, nil)
	if len(timeline.Actions) != 1 || !maps.Equal(timeline.Actions[0].Data, map[string]any{"cart.total": "30"}) {
		t.Fatalf("checkout row = %+v", timeline.Actions)
	}

	wide := map[string]string{"event.name": "wide", "traceway.capture.ids": `["0195"]`}
	for i := range 30 {
		wide[fmt.Sprintf("a%02d", i)] = "v"
	}
	var want []string
	for i := range 20 {
		want = append(want, fmt.Sprintf("a%02d", i))
	}
	for range 20 {
		row := BuildSessionTimeline([]models.LogRecord{{Timestamp: at, LogAttributes: wide}}, nil).Actions[0]
		keys := slices.Sorted(maps.Keys(row.Data))
		if !slices.Equal(keys, want) {
			t.Fatalf("data keys = %v, want %v", keys, want)
		}
	}
}

// Crash safety: records from any SDK build map onto timeline rows without panicking.
func TestTimelineSurvivesMalformedRecords(t *testing.T) {
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	records := []models.LogRecord{
		{Timestamp: at},
		{Timestamp: at, LogAttributes: map[string]string{"event.name": "traceway.action", "traceway.action.data": `[1,2]`}},
		{Timestamp: at, LogAttributes: map[string]string{"event.name": "traceway.action", "traceway.action.data": `{"a":`}},
		{Timestamp: at, LogAttributes: map[string]string{"event.name": "browser.navigation"}},
		{Timestamp: at, LogAttributes: map[string]string{"event.name": "exception"}},
		{Timestamp: at, LogAttributes: map[string]string{"event.name": "browser.user_action.click"}},
	}
	spans := []shared.SessionSpan{{SpanId: "a", StartTime: at, HTTPStatus: "abc"}, {SpanId: "b", StartTime: at, HTTPStatus: "99999999999999999999"}}
	timeline := BuildSessionTimeline(records, spans)
	if len(timeline.Logs)+len(timeline.Actions) != len(records)+len(spans) {
		t.Errorf("timeline = %+v", timeline)
	}
}

// Navigations carry the route template, on the browser as in the apps, and the first navigation of
// a window says where it came from. The screen context every record carries is not an event's own data.
func TestTimelineShowsTheScreenContext(t *testing.T) {
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	nav := func(seconds int, attrs map[string]string) models.LogRecord {
		attrs["event.name"] = "browser.navigation"
		return models.LogRecord{Timestamp: at.Add(time.Duration(seconds) * time.Second), LogAttributes: attrs}
	}
	timeline := BuildSessionTimeline([]models.LogRecord{
		nav(0, map[string]string{"url.full": "/orders/42", "browser.navigation.type": "push", "navigation.from": "/orders",
			"app.screen.name": "/orders/:id", "app.screen.route": "/orders/:id"}),
		nav(1, map[string]string{"url.full": "/orders/42/pay", "browser.navigation.type": "push", "navigation.from": "/somewhere-else"}),
		{Timestamp: at.Add(2 * time.Second), LogAttributes: map[string]string{"event.name": "cart.updated", "cart.items": "3",
			"app.screen.name": "/orders/:id", "app.screen.route": "/orders/:id", "app.screen.previous": "/orders",
			"app.screen.duration_ms": "1520", "viewport": "1440x900"}},
	}, nil)
	if len(timeline.Actions) != 3 {
		t.Fatalf("actions = %+v", timeline.Actions)
	}
	first, second, event := timeline.Actions[0], timeline.Actions[1], timeline.Actions[2]
	if first.From != "/orders" || first.To != "/orders/42" || first.Route != "/orders/:id" {
		t.Errorf("first navigation = %+v", first)
	}
	if second.From != "/orders/42" || second.Route != "" {
		t.Errorf("second navigation = %+v", second)
	}
	if !maps.Equal(event.Data, map[string]any{"cart.items": "3"}) {
		t.Errorf("event data = %v", event.Data)
	}
}

// The new issue types read as one line each, and a hang the app did not survive says so.
func TestTimelineNamesAppIssues(t *testing.T) {
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	exception := func(attrs map[string]string) models.LogRecord {
		attrs["event.name"] = "exception"
		return models.LogRecord{Timestamp: at, LogAttributes: attrs}
	}
	timeline := BuildSessionTimeline([]models.LogRecord{
		exception(map[string]string{"exception.type": "AppHang", "exception.message": "Main thread blocked for 3.2 s"}),
		exception(map[string]string{"exception.type": "AppHang", "exception.message": "Main thread blocked for 7.8 s", "app.hang.fatal": "true"}),
		exception(map[string]string{"exception.type": "NativeCrash", "exception.message": "SIGSEGV (SEGV_MAPERR), fault addr 0x0"}),
		exception(map[string]string{"exception.type": "WatchdogTermination"}),
		exception(map[string]string{"exception.type": "ApplicationNotResponding", "exception.message": " Input dispatching timed out "}),
		exception(map[string]string{}),
	}, nil)
	want := []string{
		"AppHang: Main thread blocked for 3.2 s",
		"AppHang: Main thread blocked for 7.8 s (fatal, the app was terminated)",
		"NativeCrash: SIGSEGV (SEGV_MAPERR), fault addr 0x0",
		"WatchdogTermination",
		"ApplicationNotResponding: Input dispatching timed out",
		"",
	}
	if len(timeline.Logs) != len(want) {
		t.Fatalf("logs = %+v", timeline.Logs)
	}
	for i, log := range timeline.Logs {
		if log.Message != want[i] || log.Level != "error" {
			t.Errorf("log %d = %+v, want %q", i, log, want[i])
		}
	}
}
