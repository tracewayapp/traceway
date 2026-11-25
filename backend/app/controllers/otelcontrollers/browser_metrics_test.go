package otelcontrollers

import (
	"context"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/clientmetrics"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

func doubleKV(key string, val float64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: val}}}
}

func TestConvertLogs_BrowserPerformanceEventsBecomeObservations(t *testing.T) {
	setFakeStore(t, nil)
	project := tokenProject(testProjectId)
	project.Framework = "react"
	at := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

	build := func(resource ...*commonpb.KeyValue) convertedLogs {
		req := eventLogs(browserResource(resource...),
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
		return convertLogs(context.Background(), project, testProjectId, "", req)
	}
	result := build(strKV("os.name", "Linux"), strKV("deployment.environment.name", "production"))

	if len(result.Records) != 8 {
		t.Fatalf("every record is still stored as a log, got %d", len(result.Records))
	}
	got := map[string]clientObservation{}
	for _, observation := range result.Observations {
		got[observation.Name] = observation
		if !observation.At.Equal(at) {
			t.Errorf("%s: time not carried: %v", observation.Name, observation.At)
		}
		if observation.Resource["browser.name"] != "Chrome" || observation.Resource["service.version"] != "2.3.1" {
			t.Errorf("%s: resource attributes missing: %v", observation.Name, observation.Resource)
		}
	}
	want := map[string]float64{
		"browser.web_vital.lcp": 2140.5, "browser.web_vital.cls": 0.08,
		MetricBrowserLongTask: 420, MetricBrowserRouteTiming: 1310, MetricBrowserPageLoad: 912.4, MetricBrowserDomReady: 301,
	}
	if len(got) != len(want) {
		t.Fatalf("got observations %v, want %v", got, want)
	}
	for name, value := range want {
		if got[name].Value != value {
			t.Errorf("%s = %v, want %v", name, got[name].Value, value)
		}
	}
	if attrs := got["browser.web_vital.lcp"].Attrs; attrs["rating"] != "good" || attrs["navigation_type"] != "navigate" {
		t.Errorf("web vital dimensions: %v", attrs)
	}
	if attrs := got[MetricBrowserLongTask].Attrs; attrs["url.path"] != "/checkout" || attrs["type"] != "longtask" {
		t.Errorf("long task dimensions must name the path without its query: %v", attrs)
	}
	if attrs := got[MetricBrowserRouteTiming].Attrs; attrs["url.path"] != "/cart" || attrs["outcome"] != "settled" || attrs["type"] != "push" {
		t.Errorf("route timing dimensions: %v", attrs)
	}

	// An SDK that aggregates on the device sends the numbers as metrics; counting its events too would double them.
	if aggregated := build(strKV(clientmetrics.AttrAggregated, clientmetrics.AggregatedValue)); len(aggregated.Observations) != 0 || len(aggregated.Records) != 8 {
		t.Errorf("an aggregating SDK's events were counted: %d observations, %d records", len(aggregated.Observations), len(aggregated.Records))
	}

	project.Framework = "gin"
	if backend := build(); len(backend.Observations) != 0 {
		t.Errorf("a backend project's logs never become client metrics, got %d", len(backend.Observations))
	}
}

func TestConvertMetricPoints_HistogramsKeepTheirBuckets(t *testing.T) {
	sum := 3000.0
	req := &colmetricspb.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "api")}},
		ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{
			{Name: "http.server.request.duration", Unit: "ms", Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{
				AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints: []*metricspb.HistogramDataPoint{{
					Attributes:     []*commonpb.KeyValue{strKV("http.route", "/users")},
					TimeUnixNano:   uint64(time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC).UnixNano()),
					Count:          100,
					Sum:            &sum,
					ExplicitBounds: []float64{10, 25, 50, 100},
					BucketCounts:   []uint64{10, 30, 40, 15, 5},
				}},
			}}},
			{Name: "db.query.duration", Unit: "ms", Data: &metricspb.Metric_ExponentialHistogram{ExponentialHistogram: &metricspb.ExponentialHistogram{
				AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA,
				DataPoints: []*metricspb.ExponentialHistogramDataPoint{{
					Scale: 3, Count: 4, Positive: &metricspb.ExponentialHistogramDataPoint_Buckets{Offset: 26, BucketCounts: []uint64{3, 1}},
				}},
			}}},
		}}},
	}}}
	result := convertMetricPoints(testProjectId, req)

	if len(result.Histograms) != 2 {
		t.Fatalf("got %d histograms", len(result.Histograms))
	}
	explicit, exponential := result.Histograms[0], result.Histograms[1]
	if !explicit.Cumulative || explicit.Hist.Count != 100 || explicit.Tags["http.route"] != "/users" || explicit.Tags["server_name"] != "api" {
		t.Errorf("explicit histogram: %+v", explicit)
	}
	if p50 := explicit.Hist.Quantile(0.5); p50 < 25 || p50 > 50 {
		t.Errorf("p50 = %v, want inside the 25..50 bucket", p50)
	}
	if exponential.Cumulative || exponential.Hist.Count != 4 || exponential.At.IsZero() {
		t.Errorf("exponential histogram: %+v", exponential)
	}
	// Installed widgets read .avg and .count of an explicit histogram; they keep arriving.
	names := map[string]float64{}
	for _, point := range result.Points {
		names[point.Name] = point.Value
	}
	if names["http.server.request.duration.avg"] != 30 || names["http.server.request.duration.count"] != 100 {
		t.Errorf("points %v", names)
	}
	types := map[string]string{}
	for _, entry := range result.Entries {
		types[entry.Name] = entry.MetricType
	}
	if types["http.server.request.duration"] != "histogram" || types["db.query.duration"] != "histogram" || types["http.server.request.duration.avg"] != "gauge" {
		t.Errorf("registry types %v", types)
	}
}
