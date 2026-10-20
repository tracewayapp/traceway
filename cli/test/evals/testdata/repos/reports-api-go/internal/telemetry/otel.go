//go:build otel

package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	RecordError = func(ctx context.Context, err error) {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err, trace.WithStackTrace(true))
		span.SetStatus(codes.Error, err.Error())
	}
	SetAttributes = func(ctx context.Context, kv ...any) {
		span := trace.SpanFromContext(ctx)
		for i := 0; i+1 < len(kv); i += 2 {
			span.SetAttributes(attribute.String(fmt.Sprint(kv[i]), fmt.Sprint(kv[i+1])))
		}
	}
	Handler = func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(routeAttr(h), "reports-api", otelhttp.WithSpanNameFormatter(spanName))
	}
	HTTPClient = func() *http.Client {
		return &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithSpanNameFormatter(clientSpanName))}
	}
	slog.SetDefault(otelslog.NewLogger("example.com/reports-api"))
}
