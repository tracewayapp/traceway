//go:build telemetry_ch

package chdb

import (
	"context"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/db/dbtrace"
	"github.com/tracewayapp/traceway/backend/app/traceway"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracedConnSpansFollowTheCaller(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	t.Cleanup(traceway.Install(traceway.Providers{
		TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
	}))
	conn := tracedConn{inner: &fakeConn{}, opts: dbtrace.Options{System: "clickhouse", Namespace: "traceway"}}

	if err := conn.Exec(context.Background(), "OPTIMIZE TABLE x"); err != nil {
		t.Fatal(err)
	}
	if n := len(recorder.Ended()); n != 0 {
		t.Fatalf("background statements must not start root spans, got %d", n)
	}

	ctx, request := traceway.Tracer().Start(context.Background(), "POST /api/otel/v1/traces")
	batch, err := conn.PrepareBatch(BatchCtx(ctx), "INSERT INTO spans_v2")
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.Send(); err != nil {
		t.Fatal(err)
	}
	request.End()

	var insert sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.Name() == "INSERT spans_v2" {
			insert = span
		}
	}
	if insert == nil || insert.Parent().SpanID() != request.SpanContext().SpanID() {
		t.Fatal("the batch insert must be a child of the ingest request")
	}
}
