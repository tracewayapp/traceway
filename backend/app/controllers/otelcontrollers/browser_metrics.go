package otelcontrollers

import (
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/clientmetrics"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
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
)

type browserMetric struct {
	name, valueAttr string
	tagAttrs        map[string]string
}

var browserEventMetrics = map[string][]browserMetric{
	eventLongTask: {{name: MetricBrowserLongTask, valueAttr: "browser.long_task.duration",
		tagAttrs: map[string]string{"type": "browser.long_task.type"}}},
	eventRouteTiming: {{name: MetricBrowserRouteTiming, valueAttr: "browser.route_timing.duration",
		tagAttrs: map[string]string{"outcome": "browser.route_timing.outcome", "type": "browser.route_timing.type"}}},
	eventNavigationTiming: {
		{name: MetricBrowserPageLoad, valueAttr: "browser.navigation_timing.load_event_end"},
		{name: MetricBrowserDomReady, valueAttr: "browser.navigation_timing.dom_content_loaded_event_end"},
	},
}

// clientObservation is one value of a catalog distribution, on its way into the
// client metrics aggregator.
type clientObservation struct {
	Name     string
	Value    float64
	At       time.Time
	Resource map[string]string
	Attrs    map[string]string
}

// browserObservations turns a performance event of a browser SDK into observations
// of the client metrics catalog, so dashboards and alerts read page performance from
// the same histograms whichever way the numbers arrive. This is the path of SDKs that
// only send events; an SDK that aggregates on the device marks its resource and is
// skipped by the caller. The log record is stored as well: it keeps the detail (the
// blocking script, the previous route) an aggregate cannot.
//
// The catalog, the dimension caps and the clamps are applied by the aggregator, not
// here: a vital outside the catalog is refused there.
func browserObservations(eventName string, record models.LogRecord) []clientObservation {
	attrs := record.LogAttributes
	var metrics []browserMetric
	if eventName == eventWebVital {
		vital := strings.ToLower(attrs["browser.web_vital.name"])
		if _, ok := clientmetrics.Lookup(metricWebVitalPrefix + vital); !ok {
			return nil
		}
		metrics = []browserMetric{{name: metricWebVitalPrefix + vital, valueAttr: "browser.web_vital.value",
			tagAttrs: map[string]string{"rating": "browser.web_vital.rating", "navigation_type": "browser.web_vital.navigation_type"}}}
	} else {
		metrics = browserEventMetrics[eventName]
	}

	var observations []clientObservation
	for _, metric := range metrics {
		value, err := strconv.ParseFloat(attrs[metric.valueAttr], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			continue
		}
		dims := map[string]string{}
		for tag, attr := range metric.tagAttrs {
			if attrs[attr] != "" {
				dims[tag] = attrs[attr]
			}
		}
		if path := urlPath(attrs["url.full"]); path != "" {
			dims["url.path"] = path
		}
		observations = append(observations, clientObservation{Name: metric.name, Value: value, At: record.Timestamp, Resource: record.ResourceAttributes, Attrs: dims})
	}
	return observations
}

// urlPath keeps the path of a page URL: the query and fragment would make every visit its own series. The path
// stays percent-encoded, as the browser reports location.pathname and as the SDKs that aggregate on the device
// send it, so both name the same series.
func urlPath(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.EscapedPath()
}
