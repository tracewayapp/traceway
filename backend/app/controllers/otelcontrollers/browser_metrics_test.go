package otelcontrollers

import (
	"context"
	"testing"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

func doubleKV(key string, val float64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: val}}}
}

func TestConvertLogs_BrowserPerformanceEventsBecomeMetricPoints(t *testing.T) {
	setFakeStore(t, nil)
	project := tokenProject(testProjectId)
	project.Framework = "react"
	at := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

	req := eventLogs(browserResource(strKV("os.name", "Linux"), strKV("deployment.environment.name", "production")),
		eventRecord(eventWebVital, at, strKV("browser.web_vital.name", "LCP"), doubleKV("browser.web_vital.value", 2140.5),
			strKV("browser.web_vital.rating", "good"), strKV("browser.web_vital.navigation_type", "navigate")),
		eventRecord(eventWebVital, at, strKV("browser.web_vital.name", "cls"), doubleKV("browser.web_vital.value", 0.08)),
		eventRecord(eventWebVital, at, strKV("browser.web_vital.name", "made-up"), doubleKV("browser.web_vital.value", 1)),
		eventRecord(eventLongTask, at, intKV("browser.long_task.duration", 420), strKV("browser.long_task.type", "longtask"),
			strKV("url.full", "https://shop.example.com/checkout?step=2#pay")),
		eventRecord(eventRouteTiming, at, intKV("browser.route_timing.duration", 1310), strKV("browser.route_timing.outcome", "settled"),
			strKV("browser.route_timing.type", "push"), strKV("url.full", "https://shop.example.com/cart")),
		eventRecord(eventNavigationTiming, at, doubleKV("browser.navigation_timing.load_event_end", 912.4),
			doubleKV("browser.navigation_timing.dom_content_loaded_event_end", 301)),
		eventRecord(eventLongTask, at, strKV("browser.long_task.duration", "not a number")),
		eventRecord("browser.console", at),
	)
	result := convertLogs(context.Background(), project, testProjectId, "", req)

	if len(result.Records) != 8 {
		t.Fatalf("every record is still stored as a log, got %d", len(result.Records))
	}
	got := map[string]float64{}
	for _, point := range result.Metrics {
		got[point.Name] = point.Value
		if point.ProjectId != testProjectId || !point.RecordedAt.Equal(at) {
			t.Errorf("%s: project/time not carried: %+v", point.Name, point)
		}
		if point.Tags["browser.name"] != "Chrome" || point.Tags["os.name"] != "Linux" || point.Tags["service.version"] != "2.3.1" ||
			point.Tags["deployment.environment.name"] != "production" || point.Tags["server_name"] != "shop-web" {
			t.Errorf("%s: resource tags missing: %v", point.Name, point.Tags)
		}
		if _, ok := point.Tags["host.name"]; ok {
			t.Errorf("%s: only the allowed resource attributes become tags", point.Name)
		}
	}
	want := map[string]float64{
		"browser.web_vital.lcp": 2140.5, "browser.web_vital.cls": 0.08,
		MetricBrowserLongTask: 420, MetricBrowserRouteTiming: 1310, MetricBrowserPageLoad: 912.4, MetricBrowserDomReady: 301,
	}
	if len(got) != len(want) {
		t.Fatalf("got metrics %v, want %v", got, want)
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s = %v, want %v", name, got[name], value)
		}
	}
	for _, point := range result.Metrics {
		switch point.Name {
		case "browser.web_vital.lcp":
			if point.Tags["rating"] != "good" || point.Tags["navigation_type"] != "navigate" {
				t.Errorf("web vital tags: %v", point.Tags)
			}
		case MetricBrowserLongTask:
			if point.Tags["url.path"] != "/checkout" || point.Tags["type"] != "longtask" {
				t.Errorf("long task tags must name the path without its query: %v", point.Tags)
			}
		case MetricBrowserRouteTiming:
			if point.Tags["url.path"] != "/cart" || point.Tags["outcome"] != "settled" || point.Tags["type"] != "push" {
				t.Errorf("route timing tags: %v", point.Tags)
			}
		}
	}
	units := map[string]string{}
	for _, entry := range result.MetricEntries {
		units[entry.Name] = entry.Unit
	}
	if len(units) != len(want) || units["browser.web_vital.lcp"] != "ms" || units["browser.web_vital.cls"] != "" {
		t.Errorf("registry entries: %v", units)
	}

	project.Framework = "gin"
	if backend := convertLogs(context.Background(), project, testProjectId, "", req); len(backend.Metrics) != 0 {
		t.Errorf("a backend project's logs never become metric points, got %d", len(backend.Metrics))
	}
}
