package main

import (
	"context"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// setupTelemetry exports traces to the "Shop API" Traceway project over
// OTLP/HTTP. The instance URL and the ingest token come from the environment
// (api/.env locally, the container's env file in production).
func setupTelemetry(ctx context.Context) (func(context.Context) error, error) {
	base := strings.TrimRight(os.Getenv("TRACEWAY_URL"), "/")
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(base+"/api/otel/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{"Authorization": "Bearer " + os.Getenv("TRACEWAY_API_TOKEN")}),
	)
	if err != nil {
		return nil, err
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", envOr("OTEL_SERVICE_NAME", "shop-api")),
			attribute.String("service.version", envOr("APP_VERSION", "dev")),
		),
		resource.WithHost(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp, sdktrace.WithBatchTimeout(2*time.Second)), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp.Shutdown, nil
}
