package clientcontrollers

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestReportBodyReadIsTimedApartFromDecoding(t *testing.T) {
	// The cap is read once per process, so every test of this package has to
	// ask for the same one.
	config.Init(&config.Cfg{ReportMaxBodyMB: "1"})
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer provider.Shutdown(context.Background())
	defer traceway.Install(traceway.Providers{TracerProvider: provider})()

	payload := []byte(`{"nope"`)
	var wire bytes.Buffer
	zw := gzip.NewWriter(&wire)
	if _, err := zw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	wireBytes := wire.Len()

	ctx, parent := provider.Tracer("report-test").Start(context.Background(), "POST /api/report")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/report",
		func(c *gin.Context) { c.Set(middleware.ProjectIdContextKey, uuid.New()) },
		middleware.UseGzip,
		ClientController.Report,
	)
	req := httptest.NewRequest(http.MethodPost, "/api/report", &wire).WithContext(ctx)
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	parent.End()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want the 400 of a malformed body", rec.Code)
	}

	spans := map[string]sdktrace.ReadOnlySpan{}
	for _, span := range recorder.Ended() {
		spans[span.Name()] = span
	}
	parse := spans["report.parse_body"]
	if parse == nil {
		t.Fatal("missing report.parse_body")
	}
	for _, name := range []string{"report.read_body", "report.decode_json"} {
		span := spans[name]
		if span == nil {
			t.Fatalf("missing span %s", name)
		}
		if span.Parent().SpanID() != parse.SpanContext().SpanID() {
			t.Fatalf("%s is not a child of report.parse_body", name)
		}
	}

	attrs := map[string]string{}
	for _, kv := range spans["POST /api/report"].Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	want := map[string]string{
		"ingest.body.wire_bytes":    strconv.Itoa(wireBytes),
		"ingest.body.decoded_bytes": strconv.Itoa(len(payload)),
	}
	for key, value := range want {
		if attrs[key] != value {
			t.Fatalf("%s=%q, want %q (all: %v)", key, attrs[key], value, attrs)
		}
	}
	for _, key := range []string{"ingest.body.network_ms", "ingest.body.gunzip_ms"} {
		if _, ok := attrs[key]; !ok {
			t.Fatalf("missing %s (all: %v)", key, attrs)
		}
	}
}
