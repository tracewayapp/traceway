package otelcontrollers

import (
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

const (
	eventWebVital         = "browser.web_vital"
	eventLongTask         = "browser.long_task"
	eventRouteTiming      = "browser.route_timing"
	eventNavigationTiming = "browser.navigation_timing"

	MetricBrowserLongTask    = "browser.long_task.duration"
	MetricBrowserRouteTiming = "browser.route_timing.duration"
	MetricBrowserPageLoad    = "browser.page_load.duration"
	MetricBrowserDomReady    = "browser.page_load.dom_content_loaded"
	metricWebVitalPrefix     = "browser.web_vital."

	maxBrowserMetricTagLen = 200
)

// webVitalUnits lists the vitals that become metrics. A name outside it is dropped: metric names are part of every
// dashboard and alert, so a client cannot mint new ones.
// CLS is a score with no unit; the OTel unit "1" would chart it as a percentage.
var webVitalUnits = map[string]string{"lcp": "ms", "inp": "ms", "fcp": "ms", "ttfb": "ms", "cls": ""}

// browserMetricResourceTags are the resource attributes a browser metric can be filtered and grouped by.
var browserMetricResourceTags = []string{
	"browser.name", "browser.mobile", "os.name", "service.version", "deployment.environment.name",
}

type browserMetric struct {
	name, unit, valueAttr string
	tagAttrs              map[string]string
}

var browserEventMetrics = map[string][]browserMetric{
	eventLongTask: {{name: MetricBrowserLongTask, unit: "ms", valueAttr: "browser.long_task.duration",
		tagAttrs: map[string]string{"type": "browser.long_task.type"}}},
	eventRouteTiming: {{name: MetricBrowserRouteTiming, unit: "ms", valueAttr: "browser.route_timing.duration",
		tagAttrs: map[string]string{"outcome": "browser.route_timing.outcome", "type": "browser.route_timing.type"}}},
	eventNavigationTiming: {
		{name: MetricBrowserPageLoad, unit: "ms", valueAttr: "browser.navigation_timing.load_event_end"},
		{name: MetricBrowserDomReady, unit: "ms", valueAttr: "browser.navigation_timing.dom_content_loaded_event_end"},
	},
}

// browserMetricPoints turns a performance event of the browser SDK into metric points, so dashboards and metric
// alerts read page performance the way they read any other metric. The log record is stored as well: it keeps the
// detail (the blocking script, the previous route) a point cannot.
func browserMetricPoints(projectId uuid.UUID, eventName string, record models.LogRecord) ([]models.MetricPoint, []transactional.MetricRegistrationEntry) {
	attrs := record.LogAttributes
	var metrics []browserMetric
	if eventName == eventWebVital {
		vital := strings.ToLower(attrs["browser.web_vital.name"])
		unit, ok := webVitalUnits[vital]
		if !ok {
			return nil, nil
		}
		metrics = []browserMetric{{name: metricWebVitalPrefix + vital, unit: unit, valueAttr: "browser.web_vital.value",
			tagAttrs: map[string]string{"rating": "browser.web_vital.rating", "navigation_type": "browser.web_vital.navigation_type"}}}
	} else {
		metrics = browserEventMetrics[eventName]
	}

	var points []models.MetricPoint
	var entries []transactional.MetricRegistrationEntry
	for _, metric := range metrics {
		value, err := strconv.ParseFloat(attrs[metric.valueAttr], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			continue
		}
		tags := map[string]string{}
		if record.ServiceName != "" {
			tags["server_name"] = record.ServiceName
		}
		for _, key := range browserMetricResourceTags {
			setBrowserMetricTag(tags, key, record.ResourceAttributes[key])
		}
		for tag, attr := range metric.tagAttrs {
			setBrowserMetricTag(tags, tag, attrs[attr])
		}
		setBrowserMetricTag(tags, "url.path", urlPath(attrs["url.full"]))
		points = append(points, models.MetricPoint{ProjectId: projectId, Name: metric.name, Value: value, Tags: tags, RecordedAt: record.Timestamp})
		entries = append(entries, transactional.MetricRegistrationEntry{Name: metric.name, Unit: metric.unit, MetricType: "gauge"})
	}
	return points, entries
}

func setBrowserMetricTag(tags map[string]string, key, value string) {
	if value == "" {
		return
	}
	if len(value) > maxBrowserMetricTagLen {
		value = value[:maxBrowserMetricTagLen]
	}
	tags[key] = value
}

// urlPath keeps the path of a page URL: the query and fragment would make every visit its own series.
func urlPath(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Path
}
