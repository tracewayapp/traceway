package main

import (
	"crypto/rand"
	mathrand "math/rand"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

var severityChoices = []struct {
	number logspb.SeverityNumber
	text   string
}{
	{logspb.SeverityNumber_SEVERITY_NUMBER_INFO, "INFO"},
	{logspb.SeverityNumber_SEVERITY_NUMBER_INFO, "INFO"},
	{logspb.SeverityNumber_SEVERITY_NUMBER_INFO, "INFO"},
	{logspb.SeverityNumber_SEVERITY_NUMBER_WARN, "WARN"},
	{logspb.SeverityNumber_SEVERITY_NUMBER_ERROR, "ERROR"},
}

// logBodies is a deterministic pool of realistic-looking log lines so the
// /logs dashboard page renders publishable screenshots instead of 120 chars
// of random gibberish. Workload shape (uniform random selection across a
// fixed pool) is unchanged from the prior randomString path — only the
// per-record byte payload differs. See POSTS.md decision log 2026-06-08.
var logBodies = []string{
	"user logged in",
	"user logged out",
	"checkout completed for order 4821",
	"payment authorized via stripe (amount=42.99 usd)",
	"product 9123 added to cart",
	"product 9123 removed from cart",
	"search returned 18 results for query=\"running shoes\"",
	"cache miss for key=user:8821 — refilled from db (3.2ms)",
	"db pool reached 80% capacity (24/30 conns)",
	"slow query detected: SELECT * FROM orders WHERE ... (842ms)",
	"rate limit hit for ip=10.0.4.21 endpoint=/api/search",
	"request retried after 502 from upstream service=inventory",
	"connection timeout reaching downstream service=billing",
	"failed to deserialize webhook payload: unexpected token",
	"session expired for user_id=8821 — redirecting to /login",
	"feature flag \"new-checkout-flow\" evaluated to true",
	"background job \"daily-report\" finished in 12.4s",
	"OTLP exporter flushed batch=128 size=42.1KB",
}

type logsSender struct{}

func (logsSender) Name() string { return "logs" }
func (logsSender) Path() string { return "/api/otel/v1/logs" }

func (logsSender) BuildBody(rng *mathrand.Rand, batchSize int) ([]byte, error) {
	now := time.Now().UTC()
	resourceAttrs := []*commonpb.KeyValue{
		strAttr("service.name", "bench-loadgen"),
		strAttr("service.version", "1.0.0"),
		strAttr("deployment.environment", "bench"),
	}

	records := make([]*logspb.LogRecord, batchSize)
	for i := range records {
		ts := now.Add(-time.Duration(rng.Intn(5000)) * time.Millisecond).UnixNano()
		sev := severityChoices[rng.Intn(len(severityChoices))]
		traceId := make([]byte, 16)
		spanId := make([]byte, 8)
		_, _ = rand.Read(traceId)
		_, _ = rand.Read(spanId)

		records[i] = &logspb.LogRecord{
			TimeUnixNano:         uint64(ts),
			ObservedTimeUnixNano: uint64(ts),
			SeverityNumber:       sev.number,
			SeverityText:         sev.text,
			Body:                 &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: logBodies[rng.Intn(len(logBodies))]}},
			Attributes: []*commonpb.KeyValue{
				strAttr("logger.name", "bench-loadgen"),
				strAttr("thread.name", "worker-1"),
				strAttr("code.namespace", "bench.handler"),
				intAttr("retry.count", int64(rng.Intn(3))),
				strAttr("user.id", randomString(rng, 16)),
			},
			TraceId: traceId,
			SpanId:  spanId,
		}
	}

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: &resourcepb.Resource{Attributes: resourceAttrs},
			ScopeLogs: []*logspb.ScopeLogs{{
				Scope:      &commonpb.InstrumentationScope{Name: "bench-loadgen", Version: "1.0.0"},
				LogRecords: records,
			}},
		}},
	}
	return proto.Marshal(req)
}

func (logsSender) ParseRejected(respBody []byte) int {
	if len(respBody) == 0 {
		return 0
	}
	var resp collogspb.ExportLogsServiceResponse
	if err := proto.Unmarshal(respBody, &resp); err != nil {
		return 0
	}
	if resp.PartialSuccess == nil {
		return 0
	}
	return int(resp.PartialSuccess.RejectedLogRecords)
}
