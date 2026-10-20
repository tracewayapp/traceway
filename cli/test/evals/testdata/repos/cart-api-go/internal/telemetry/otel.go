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
		return otelhttp.NewHandler(routeAttr(h), "cart-api", otelhttp.WithSpanNameFormatter(spanName))
	}
	Transport = func(rt http.RoundTripper) http.RoundTripper {
		return otelhttp.NewTransport(rt, otelhttp.WithSpanNameFormatter(clientSpanName))
	}
}
