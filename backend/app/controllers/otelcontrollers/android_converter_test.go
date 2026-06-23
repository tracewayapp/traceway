package otelcontrollers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/services"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/google/uuid"
)

func androidConvFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{"..", "..", "symbolicator", "android", "fixtures"}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func androidExceptionRequest(stacktrace, proguardUuid string, extraEvent []*commonpb.KeyValue) *coltracepb.ExportTraceServiceRequest {
	traceId := []byte{0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf}
	spanId := []byte{0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7}
	now := uint64(1_700_000_000_000_000_000)

	eventAttrs := []*commonpb.KeyValue{
		strKV("exception.type", "a.b"),
		strKV("exception.message", "card declined for $30.59"),
	}
	if stacktrace != "" {
		eventAttrs = append(eventAttrs, strKV("exception.stacktrace", stacktrace))
	}
	eventAttrs = append(eventAttrs, extraEvent...)

	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				strKV("service.name", "androdemo"),
				strKV("telemetry.sdk.language", "android"),
				strKV("app.debug.proguard_uuid", proguardUuid),
			}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Scope: &commonpb.InstrumentationScope{Name: "io.honeycomb.opentelemetry.android"},
				Spans: []*tracepb.Span{{
					TraceId: traceId, SpanId: spanId,
					Name: "UncaughtException", Kind: tracepb.Span_SPAN_KIND_INTERNAL,
					StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000,
					Events: []*tracepb.Span_Event{{
						Name:         "exception",
						TimeUnixNano: now + 500_000,
						Attributes:   eventAttrs,
					}},
				}},
			}},
		}},
	}
}

func TestConvertTraces_AndroidR8Retraces(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	const proguardUuid = "6A8CB813-45F6-3652-AD33-778FD1EAB196"

	setFakeStore(t, map[string][]byte{
		services.AndroidMappingKey(projectId, proguardUuid): androidConvFixture(t, "r8", "mapping.txt"),
	})

	req := androidExceptionRequest(string(androidConvFixture(t, "r8", "obfuscated.txt")), proguardUuid, nil)
	_, _, _, exceptions, _, _ := convertTraces(context.Background(), tokenProject(projectId), projectId, req)
	if len(exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(exceptions))
	}
	got := exceptions[0].StackTrace
	if strings.Contains(got, "a.a.a(") {
		t.Errorf("trace still obfuscated:\n%s", got)
	}
	for _, want := range []string{"com.example.demo.Checkout.chargeCard", "Checkout.java", "PaymentDeclinedException"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in retraced OTLP exception:\n%s", want, got)
		}
	}
	if exceptions[0].Attributes["telemetry.sdk.language"] != "android" {
		t.Errorf("expected stamped telemetry.sdk.language=android, got %q", exceptions[0].Attributes["telemetry.sdk.language"])
	}
}

func TestConvertTraces_AndroidStructuredArrays(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000a2")
	const proguardUuid = "AAA0CB813-45F6-3652-AD33-778FD1EAB196"

	setFakeStore(t, map[string][]byte{
		services.AndroidMappingKey(projectId, proguardUuid): androidConvFixture(t, "r8", "mapping.txt"),
	})

	extra := []*commonpb.KeyValue{
		strArrayKV("exception.structured_stacktrace.classes", "a.a"),
		strArrayKV("exception.structured_stacktrace.methods", "a"),
		strArrayKV("exception.structured_stacktrace.source_files", "SourceFile"),
		intArrayKV("exception.structured_stacktrace.lines", 2),
	}
	req := androidExceptionRequest("", proguardUuid, extra)
	_, _, _, exceptions, _, _ := convertTraces(context.Background(), tokenProject(projectId), projectId, req)
	if len(exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(exceptions))
	}
	got := exceptions[0].StackTrace
	if !strings.Contains(got, "com.example.demo.Checkout.chargeCard") {
		t.Errorf("structured-array android trace not retraced:\n%s", got)
	}
}
