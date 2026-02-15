package otelcontrollers

import (
	"backend/app/models"

	"github.com/google/uuid"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func convertMetrics(projectId uuid.UUID, req *colmetricspb.ExportMetricsServiceRequest, serverName string) []models.MetricRecord {
	var records []models.MetricRecord

	for _, rm := range req.ResourceMetrics {
		resAttrs := rm.GetResource().GetAttributes()
		sn := getStringAttribute(resAttrs, "service.name")
		if sn == "" {
			sn = serverName
		}

		for _, sm := range rm.ScopeMetrics {
			for _, metric := range sm.Metrics {
				name := metric.Name

				switch data := metric.Data.(type) {
				case *metricspb.Metric_Gauge:
					records = appendNumberDataPoints(records, projectId, name, sn, data.Gauge.GetDataPoints())
				case *metricspb.Metric_Sum:
					records = appendNumberDataPoints(records, projectId, name, sn, data.Sum.GetDataPoints())
				case *metricspb.Metric_Histogram:
					for _, dp := range data.Histogram.GetDataPoints() {
						ts := nanoToTime(dp.TimeUnixNano)
						if dp.Count > 0 && dp.Sum != nil {
							records = append(records, models.MetricRecord{
								ProjectId:  projectId,
								Name:       name + ".avg",
								Value:      *dp.Sum / float64(dp.Count),
								RecordedAt: ts,
								ServerName: sn,
							})
						}
						records = append(records, models.MetricRecord{
							ProjectId:  projectId,
							Name:       name + ".count",
							Value:      float64(dp.Count),
							RecordedAt: ts,
							ServerName: sn,
						})
					}
				}
			}
		}
	}
	return records
}

func appendNumberDataPoints(records []models.MetricRecord, projectId uuid.UUID, name, serverName string, dps []*metricspb.NumberDataPoint) []models.MetricRecord {
	for _, dp := range dps {
		var value float64
		switch v := dp.Value.(type) {
		case *metricspb.NumberDataPoint_AsDouble:
			value = v.AsDouble
		case *metricspb.NumberDataPoint_AsInt:
			value = float64(v.AsInt)
		}
		records = append(records, models.MetricRecord{
			ProjectId:  projectId,
			Name:       name,
			Value:      value,
			RecordedAt: nanoToTime(dp.TimeUnixNano),
			ServerName: serverName,
		})
	}
	return records
}
