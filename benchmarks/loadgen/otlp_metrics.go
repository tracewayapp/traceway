package main

import (
	mathrand "math/rand"
	"time"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

// metricNames is intentionally low cardinality. The backend auto-registers
// metrics in metric_registry on first sight; bench traffic with 10 stable
// names exercises that path once per name per project and otherwise lets the
// hot path stay hot.
//
// First 7 names match canonical constants in backend/app/models/metric_record.model.go
// (`MetricNameCpuUsage`, `MetricNameMemoryUsage`, etc.) so the dashboard cards
// at /metrics render real-looking values during ingest. Remaining 3 are
// application-style names that exist in the metric_points table for the
// read-probe's generic time-range queries but don't bind to a specific card.
// Workload shape (uniform random, 10 stable names) is unchanged from prior
// posts — see POSTS.md decision log 2026-06-08 for rationale.
var metricNames = []string{
	"cpu.used_pcnt", "mem.used", "mem.total",
	"go.go_routines", "go.heap_objects", "go.num_gc", "go.gc_pause",
	"disk.used_pcnt", "net.bytes_tx", "app.requests_total",
}

type metricsSender struct{}

func (metricsSender) Name() string { return "metrics" }
func (metricsSender) Path() string { return "/api/otel/v1/metrics" }

func (metricsSender) BuildBody(rng *mathrand.Rand, batchSize int) ([]byte, error) {
	now := time.Now().UTC()
	resourceAttrs := []*commonpb.KeyValue{
		strAttr("service.name", "bench-loadgen"),
		strAttr("service.version", "1.0.0"),
		strAttr("deployment.environment", "bench"),
	}

	// Distribute batchSize data points across the 10 unique metric names so
	// each batch exercises all of them. Trailing points go on the last metric.
	perMetric := batchSize / len(metricNames)
	remainder := batchSize % len(metricNames)

	metrics := make([]*metricspb.Metric, 0, len(metricNames))
	for i, name := range metricNames {
		n := perMetric
		if i == len(metricNames)-1 {
			n += remainder
		}
		if n <= 0 {
			continue
		}
		points := make([]*metricspb.NumberDataPoint, n)
		for p := 0; p < n; p++ {
			pointTime := now.Add(-time.Duration(rng.Intn(1000)) * time.Millisecond).UnixNano()
			points[p] = &metricspb.NumberDataPoint{
				TimeUnixNano: uint64(pointTime),
				Value:        &metricspb.NumberDataPoint_AsDouble{AsDouble: rng.Float64() * 100},
				Attributes: []*commonpb.KeyValue{
					strAttr("host", "bench-host-1"),
				},
			}
		}
		metrics = append(metrics, &metricspb.Metric{
			Name: name,
			Unit: "1",
			Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{DataPoints: points}},
		})
	}

	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{{
			Resource: &resourcepb.Resource{Attributes: resourceAttrs},
			ScopeMetrics: []*metricspb.ScopeMetrics{{
				Scope:   &commonpb.InstrumentationScope{Name: "bench-loadgen", Version: "1.0.0"},
				Metrics: metrics,
			}},
		}},
	}
	return proto.Marshal(req)
}

func (metricsSender) ParseRejected(respBody []byte) int {
	if len(respBody) == 0 {
		return 0
	}
	var resp colmetricspb.ExportMetricsServiceResponse
	if err := proto.Unmarshal(respBody, &resp); err != nil {
		return 0
	}
	if resp.PartialSuccess == nil {
		return 0
	}
	return int(resp.PartialSuccess.RejectedDataPoints)
}
