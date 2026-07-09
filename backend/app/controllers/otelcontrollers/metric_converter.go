package otelcontrollers

import (
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	"github.com/google/uuid"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

type convertedMetrics struct {
	Points  []models.MetricPoint
	Entries []transactional.MetricRegistrationEntry
}

// processResourceAttrAllowlist names the OTel Resource attributes we lift
// onto each metric point's tags. Necessary because the hostmetrics process
// scraper distinguishes per-process metrics via Resource attributes (one
// ResourceMetrics per process), not data-point attributes — without this,
// every process.* point looks identical save for the value. The list is an
// allowlist rather than a passthrough so other receivers can't blow up
// metric_points cardinality with arbitrary resource attrs.
var processResourceAttrAllowlist = []string{
	"process.pid",
	"process.executable.name",
	"process.command_line",
	"process.owner",
}

func convertMetricPoints(projectId uuid.UUID, req *colmetricspb.ExportMetricsServiceRequest) convertedMetrics {
	var points []models.MetricPoint
	seenEntries := make(map[string]transactional.MetricRegistrationEntry)

	for _, rm := range req.ResourceMetrics {
		resAttrs := rm.GetResource().GetAttributes()
		sn := getStringAttribute(resAttrs, "service.name")
		resTags := extractProcessResourceTags(resAttrs)

		for _, sm := range rm.ScopeMetrics {
			for _, metric := range sm.Metrics {
				name := metric.Name
				unit := metric.Unit

				switch data := metric.Data.(type) {
				case *metricspb.Metric_Gauge:
					points = appendNumberDataPoints(points, projectId, name, sn, resTags, data.Gauge.GetDataPoints())
					if _, ok := seenEntries[name]; !ok {
						seenEntries[name] = transactional.MetricRegistrationEntry{
							Name:       name,
							Unit:       unit,
							MetricType: "gauge",
						}
					}
				case *metricspb.Metric_Sum:
					points = appendNumberDataPoints(points, projectId, name, sn, resTags, data.Sum.GetDataPoints())
					if _, ok := seenEntries[name]; !ok {
						mt := "gauge"
						if data.Sum.IsMonotonic {
							mt = "counter"
						}
						seenEntries[name] = transactional.MetricRegistrationEntry{
							Name:       name,
							Unit:       unit,
							MetricType: mt,
						}
					}
				case *metricspb.Metric_Histogram:
					for _, dp := range data.Histogram.GetDataPoints() {
						ts := nanoToTime(dp.TimeUnixNano)
						tags := buildTags(sn, resTags, dp.Attributes)
						if dp.Count > 0 && dp.Sum != nil {
							points = append(points, models.MetricPoint{
								ProjectId:  projectId,
								Name:       name + ".avg",
								Value:      *dp.Sum / float64(dp.Count),
								Tags:       tags,
								RecordedAt: ts,
							})
						}
						points = append(points, models.MetricPoint{
							ProjectId:  projectId,
							Name:       name + ".count",
							Value:      float64(dp.Count),
							Tags:       tags,
							RecordedAt: ts,
						})
					}
					avgName := name + ".avg"
					countName := name + ".count"
					if _, ok := seenEntries[avgName]; !ok {
						seenEntries[avgName] = transactional.MetricRegistrationEntry{
							Name:       avgName,
							Unit:       unit,
							MetricType: "gauge",
						}
					}
					if _, ok := seenEntries[countName]; !ok {
						seenEntries[countName] = transactional.MetricRegistrationEntry{
							Name:       countName,
							Unit:       "count",
							MetricType: "counter",
						}
					}
				}
			}
		}
	}

	entries := make([]transactional.MetricRegistrationEntry, 0, len(seenEntries))
	for _, e := range seenEntries {
		entries = append(entries, e)
	}

	return convertedMetrics{Points: points, Entries: entries}
}

func appendNumberDataPoints(points []models.MetricPoint, projectId uuid.UUID, name, serverName string, resTags map[string]string, dps []*metricspb.NumberDataPoint) []models.MetricPoint {
	for _, dp := range dps {
		var value float64
		switch v := dp.Value.(type) {
		case *metricspb.NumberDataPoint_AsDouble:
			value = v.AsDouble
		case *metricspb.NumberDataPoint_AsInt:
			value = float64(v.AsInt)
		}
		tags := buildTags(serverName, resTags, dp.Attributes)
		points = append(points, models.MetricPoint{
			ProjectId:  projectId,
			Name:       name,
			Value:      value,
			Tags:       tags,
			RecordedAt: nanoToTime(dp.TimeUnixNano),
		})
	}
	return points
}

func buildTags(serverName string, resTags map[string]string, attrs []*commonpb.KeyValue) map[string]string {
	tags := extractAttributes(attrs)
	if tags == nil {
		tags = make(map[string]string, len(resTags)+1)
	}
	for k, v := range resTags {
		if _, exists := tags[k]; !exists {
			tags[k] = v
		}
	}
	if serverName != "" {
		tags["server_name"] = serverName
	}
	return tags
}

func extractProcessResourceTags(resAttrs []*commonpb.KeyValue) map[string]string {
	if len(resAttrs) == 0 {
		return nil
	}
	all := extractAttributes(resAttrs)
	if len(all) == 0 {
		return nil
	}
	out := make(map[string]string, len(processResourceAttrAllowlist))
	for _, k := range processResourceAttrAllowlist {
		if v, ok := all[k]; ok && v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
