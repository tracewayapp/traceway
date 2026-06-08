package main

import (
	"crypto/rand"
	mathrand "math/rand"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// endpointPaths is a (method, path) pool. Keeping them paired so the synthetic
// traffic mix maps cleanly to real-world HTTP shapes (POST /users + GET /users
// etc.). The span Name is set to "{method} {path}" — what dashboards display —
// while http.method and http.route are set to the matched method and path
// individually. Earlier the loadgen hardcoded http.method="GET" and emitted
// Names like "GET /api/users", causing the /endpoints page to render
// "GET GET /api/users" (and "GET POST /api/users" for non-GET picks). Fixed
// 2026-06-08; see POSTS.md decision log.
var endpointPaths = []struct {
	method string
	path   string
}{
	{"GET", "/api/users"}, {"POST", "/api/users"},
	{"GET", "/api/orders"}, {"POST", "/api/orders"},
	{"GET", "/api/products/:id"}, {"PUT", "/api/products/:id"},
	{"GET", "/api/search"}, {"POST", "/api/checkout"},
	{"GET", "/api/cart"}, {"DELETE", "/api/cart/:id"},
	{"GET", "/api/auth/me"}, {"POST", "/api/auth/login"},
	{"GET", "/api/health"},
}

type spansSender struct{}

func (spansSender) Name() string { return "spans" }
func (spansSender) Path() string { return "/api/otel/v1/traces" }

func (spansSender) BuildBody(rng *mathrand.Rand, batchSize int) ([]byte, error) {
	now := time.Now().UTC()
	resourceAttrs := []*commonpb.KeyValue{
		strAttr("service.name", "bench-loadgen"),
		strAttr("service.version", "1.0.0"),
		strAttr("deployment.environment", "bench"),
	}

	spans := make([]*tracepb.Span, batchSize)
	for i := range spans {
		traceId := make([]byte, 16)
		spanId := make([]byte, 8)
		_, _ = rand.Read(traceId)
		_, _ = rand.Read(spanId)

		startNs := now.Add(-time.Duration(rng.Intn(500)) * time.Millisecond).UnixNano()
		duration := time.Duration(10+rng.Intn(990)) * time.Millisecond
		endNs := startNs + int64(duration)

		ep := endpointPaths[rng.Intn(len(endpointPaths))]
		spans[i] = &tracepb.Span{
			TraceId:           traceId,
			SpanId:            spanId,
			Name:              ep.method + " " + ep.path,
			Kind:              tracepb.Span_SPAN_KIND_SERVER,
			StartTimeUnixNano: uint64(startNs),
			EndTimeUnixNano:   uint64(endNs),
			Attributes: []*commonpb.KeyValue{
				strAttr("http.method", ep.method),
				intAttr("http.status_code", int64(pickStatus(rng))),
				strAttr("http.route", ep.path),
			},
			Status: &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
		}
	}

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: resourceAttrs},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Scope: &commonpb.InstrumentationScope{Name: "bench-loadgen", Version: "1.0.0"},
				Spans: spans,
			}},
		}},
	}
	return proto.Marshal(req)
}

func (spansSender) ParseRejected(respBody []byte) int {
	if len(respBody) == 0 {
		return 0
	}
	var resp coltracepb.ExportTraceServiceResponse
	if err := proto.Unmarshal(respBody, &resp); err != nil {
		return 0
	}
	if resp.PartialSuccess == nil {
		return 0
	}
	return int(resp.PartialSuccess.RejectedSpans)
}

func pickStatus(rng *mathrand.Rand) int {
	r := rng.Float64()
	switch {
	case r < 0.92:
		return 200
	case r < 0.95:
		return 201
	case r < 0.98:
		return 404
	default:
		return 500
	}
}
