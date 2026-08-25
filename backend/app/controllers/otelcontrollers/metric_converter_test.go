package otelcontrollers

import (
	"testing"

	"github.com/google/uuid"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func TestConvertMetricPointsPreservesInfrastructureResourceAttributes(t *testing.T) {
	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				strKV("service.name", "payments-node-1"),
				strKV("host.name", "ip-10-0-1-24"),
				strKV("host.id", "i-018b5d2"),
				strKV("host.arch", "amd64"),
				strKV("os.type", "linux"),
				strKV("cloud.region", "eu-central-1"),
				strKV("k8s.cluster.name", "production"),
				strKV("deployment.environment.name", "production"),
			}},
			ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{{
				Name: "system.cpu.utilization",
				Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{DataPoints: []*metricspb.NumberDataPoint{{
					Value:      &metricspb.NumberDataPoint_AsDouble{AsDouble: 0.25},
					Attributes: []*commonpb.KeyValue{strKV("state", "idle")},
				}}}},
			}}}},
		}},
	}

	converted := convertMetricPoints(uuid.New(), req)
	if len(converted.Points) != 1 {
		t.Fatalf("converted %d points, want 1", len(converted.Points))
	}
	tags := converted.Points[0].Tags
	for key, want := range map[string]string{
		"server_name":      "payments-node-1",
		"host.name":        "ip-10-0-1-24",
		"host.id":          "i-018b5d2",
		"host.arch":        "amd64",
		"os.type":          "linux",
		"cloud.region":     "eu-central-1",
		"k8s.cluster.name": "production",
		"state":            "idle",
	} {
		if got := tags[key]; got != want {
			t.Errorf("tag %q = %q, want %q", key, got, want)
		}
	}
	if _, ok := tags["deployment.environment.name"]; ok {
		t.Error("unexpected non-allowlisted deployment.environment.name tag")
	}
}
