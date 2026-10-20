//go:build otel

package telemetry

import (
	"context"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	RecordError = func(ctx context.Context, err error) {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err, trace.WithStackTrace(true))
		span.SetStatus(codes.Error, err.Error())
	}
	Handler = func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(routeAttr(h), "shipping-api", otelhttp.WithSpanNameFormatter(spanName))
	}
}
