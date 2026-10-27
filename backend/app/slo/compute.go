package slo

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func ComputeBuckets(ctx context.Context, slo *models.SLO, from, to time.Time) ([]shared.SLOBucketPoint, error) {
	cfg, err := models.ParseSLIConfig(slo.SliKind, slo.SliConfig)
	if err != nil {
		return nil, err
	}
	bucketMinutes := slo.BucketMinutes
	if bucketMinutes < 1 {
		bucketMinutes = 1
	}
	switch slo.SliKind {
	case models.SLIKindEndpoint:
		spec := shared.EndpointSLISpec{
			Endpoint:         cfg.Endpoint.Endpoint,
			Latency:          cfg.Endpoint.Good == models.SLOGoodLatency,
			LatencyThreshold: millis(cfg.Endpoint.LatencyThresholdMs),
		}
		return telemetry.EndpointRepository.SLICounts(ctx, slo.ProjectId, spec, from, to, bucketMinutes)
	case models.SLIKindSpan:
		spec := shared.SpanSLISpec{
			Filter:           spanFilter(slo.ProjectId, cfg.Span),
			Latency:          cfg.Span.Good == models.SLOGoodLatency,
			LatencyThreshold: millis(cfg.Span.LatencyThresholdMs),
		}
		return telemetry.OtelSpanRepository.SLICounts(ctx, spec, from, to, bucketMinutes)
	case models.SLIKindTask:
		return telemetry.TaskRepository.SLICounts(ctx, slo.ProjectId, shared.TaskSLISpec{TaskName: cfg.Task.TaskName}, from, to, bucketMinutes)
	case models.SLIKindMonitor:
		return monitorBuckets(ctx, slo, cfg.Monitor, from, to, bucketMinutes)
	case models.SLIKindMetric:
		return metricBuckets(ctx, slo, cfg.Metric, from, to, bucketMinutes)
	case models.SLIKindSession:
		spec := shared.SessionSLISpec{AppVersion: cfg.Session.AppVersion, IncludeMessages: cfg.Session.IncludeMessages}
		return telemetry.SessionRepository.SLICounts(ctx, slo.ProjectId, spec, from, to, bucketMinutes)
	}
	return nil, fmt.Errorf("unknown SLI kind %q", slo.SliKind)
}

func millis(ms float64) time.Duration {
	return time.Duration(ms * float64(time.Millisecond))
}

func spanFilter(projectId uuid.UUID, cfg *models.SpanSLIConfig) shared.OtelSpanSearch {
	filter := shared.OtelSpanSearch{ProjectId: projectId, Service: cfg.ServiceName, Name: cfg.Name, Kind: cfg.Kind}
	for _, attribute := range cfg.Attributes {
		filter.Attributes = append(filter.Attributes, shared.OtelAttributeFilter{Key: attribute.Key, Value: attribute.Value})
	}
	return filter
}

func monitorBuckets(ctx context.Context, slo *models.SLO, cfg *models.MonitorSLIConfig, from, to time.Time, bucketMinutes int) ([]shared.SLOBucketPoint, error) {
	sums := map[int64]*shared.SLOBucketPoint{}
	for _, checkId := range cfg.CheckIds {
		points, err := telemetry.CheckResultRepository.SeriesByCheck(ctx, slo.ProjectId, checkId, from, to, bucketMinutes)
		if err != nil {
			return nil, err
		}
		for _, point := range points {
			if point.Bucket.Before(from) || !point.Bucket.Before(to) {
				continue
			}
			key := point.Bucket.Unix()
			sum, ok := sums[key]
			if !ok {
				sum = &shared.SLOBucketPoint{Bucket: point.Bucket.UTC()}
				sums[key] = sum
			}
			sum.Good += point.Up
			sum.Total += point.Total - point.Missed
		}
	}
	result := make([]shared.SLOBucketPoint, 0, len(sums))
	for _, sum := range sums {
		result = append(result, *sum)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Bucket.Before(result[j].Bucket) })
	return result, nil
}

func metricBuckets(ctx context.Context, slo *models.SLO, cfg *models.MetricSLIConfig, from, to time.Time, bucketMinutes int) ([]shared.SLOBucketPoint, error) {
	series, err := telemetry.MetricPointRepository.QueryTimeSeries(ctx, slo.ProjectId, cfg.MetricName, from, to, bucketMinutes, cfg.Aggregation, cfg.Tags, "", 1)
	if err != nil {
		return nil, err
	}
	values := map[int64]float64{}
	for _, points := range series {
		for _, point := range points {
			values[point.Timestamp.Unix()] = point.Value
		}
	}
	bucket := time.Duration(bucketMinutes) * time.Minute
	missingFrom := slo.MissingCountsFrom()
	result := make([]shared.SLOBucketPoint, 0, int(to.Sub(from)/bucket))
	for at := from; at.Before(to); at = at.Add(bucket) {
		point := shared.SLOBucketPoint{Bucket: at.UTC()}
		if value, ok := values[at.Unix()]; ok {
			point.Total = 1
			if metricSliceGood(value, cfg.Operator, cfg.Threshold) {
				point.Good = 1
			}
		} else if cfg.MissingIsBad && !at.Before(missingFrom) {
			point.Total = 1
		}
		result = append(result, point)
	}
	return result, nil
}

func metricSliceGood(value float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	}
	return false
}
