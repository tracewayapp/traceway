// Package traceway reports the backend to itself over OpenTelemetry, behind
// the call-site API of the retired native Go SDK.
package traceway

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log"
	lognoop "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

const (
	scopeName         = "github.com/tracewayapp/traceway/backend"
	backendModulePath = "github.com/tracewayapp/traceway/backend"

	// Every self-monitoring reporter ticks at 10s or slower. Collecting more
	// often than that means a delta gauge never holds two recordings of the
	// same series, so each recorded value is exported exactly once.
	exportInterval = 5 * time.Second
)

type Options struct {
	ConnectionString string
	ServiceName      string
	Version          string
}

type Providers struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	LoggerProvider otellog.LoggerProvider
	ServiceName    string
}

type state struct {
	enabled        bool
	serviceName    string
	tracerProvider trace.TracerProvider
	tracer         trace.Tracer
	meter          metric.Meter
	logger         otellog.Logger
	gauges         sync.Map
	counters       sync.Map
}

var current atomic.Pointer[state]

var propagator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

func init() {
	current.Store(newState(Providers{
		TracerProvider: tracenoop.NewTracerProvider(),
		MeterProvider:  metricnoop.NewMeterProvider(),
		LoggerProvider: lognoop.NewLoggerProvider(),
	}, false))
}

func newState(p Providers, enabled bool) *state {
	return &state{
		enabled:        enabled,
		serviceName:    p.ServiceName,
		tracerProvider: p.TracerProvider,
		tracer:         p.TracerProvider.Tracer(scopeName),
		meter:          p.MeterProvider.Meter(scopeName),
		logger:         p.LoggerProvider.Logger(scopeName),
	}
}

func Install(p Providers) (restore func()) {
	if p.TracerProvider == nil {
		p.TracerProvider = tracenoop.NewTracerProvider()
	}
	if p.MeterProvider == nil {
		p.MeterProvider = metricnoop.NewMeterProvider()
	}
	if p.LoggerProvider == nil {
		p.LoggerProvider = lognoop.NewLoggerProvider()
	}
	previous := current.Swap(newState(p, true))
	return func() { current.Store(previous) }
}

func Enabled() bool {
	return current.Load().enabled
}

func TracerProvider() trace.TracerProvider {
	return current.Load().tracerProvider
}

func Tracer() trace.Tracer {
	return current.Load().tracer
}

func ServiceName() string {
	return current.Load().serviceName
}

func Propagator() propagation.TextMapPropagator {
	return propagator
}

// The providers are deliberately not the OTel globals: a library tracing
// through otel.Tracer (the GCS client does) would start root spans of its own.
func Setup(ctx context.Context, opts Options) (shutdown func(context.Context) error, err error) {
	token, base, err := ParseConnectionString(opts.ConnectionString)
	if err != nil {
		return nil, err
	}

	serviceName := opts.ServiceName
	if serviceName == "" {
		serviceName = shortHostname()
	}
	attrs := []attribute.KeyValue{attribute.String("service.name", serviceName)}
	if opts.Version != "" {
		attrs = append(attrs, attribute.String("service.version", opts.Version))
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("self-monitoring resource: %w", err)
	}
	if v, ok := res.Set().Value("service.name"); ok {
		serviceName = v.AsString()
	}

	endpoint := base + "/api/otel"
	headers := map[string]string{"Authorization": "Bearer " + token}

	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint+"/v1/traces"),
		otlptracehttp.WithHeaders(headers),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
	)
	if err != nil {
		return nil, fmt.Errorf("self-monitoring trace exporter: %w", err)
	}
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(endpoint+"/v1/metrics"),
		otlpmetrichttp.WithHeaders(headers),
		otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression),
		otlpmetrichttp.WithTemporalitySelector(temporality),
	)
	if err != nil {
		return nil, fmt.Errorf("self-monitoring metric exporter: %w", err)
	}
	logExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpointURL(endpoint+"/v1/logs"),
		otlploghttp.WithHeaders(headers),
		otlploghttp.WithCompression(otlploghttp.GzipCompression),
	)
	if err != nil {
		return nil, fmt.Errorf("self-monitoring log exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(exportInterval)),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(exportInterval))),
	)
	lp := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter, sdklog.WithExportInterval(exportInterval))),
	)

	if err := runtime.Start(runtime.WithMeterProvider(mp)); err != nil {
		return nil, fmt.Errorf("self-monitoring runtime metrics: %w", err)
	}

	Install(Providers{TracerProvider: tp, MeterProvider: mp, LoggerProvider: lp, ServiceName: serviceName})

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
	}, nil
}

// Traceway stores every metric data point as it arrives, so counters and
// gauges must be deltas: a cumulative counter would chart its running total.
// Values that are absolute by nature (up-down counters such as goroutine
// count, observed gauges) stay cumulative.
func temporality(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	switch kind {
	case sdkmetric.InstrumentKindUpDownCounter,
		sdkmetric.InstrumentKindObservableUpDownCounter,
		sdkmetric.InstrumentKindObservableGauge:
		return metricdata.CumulativeTemporality
	}
	return metricdata.DeltaTemporality
}

func ParseConnectionString(s string) (token, base string, err error) {
	s = strings.TrimSpace(s)
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return "", "", errors.New("self-monitoring connection string must look like <project token>@<instance URL>")
	}
	token, base = s[:at], strings.TrimRight(s[at+1:], "/")
	for _, suffix := range []string{"/api/report", "/api/otel"} {
		base = strings.TrimSuffix(base, suffix)
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "", fmt.Errorf("self-monitoring connection string has an invalid instance URL %q", base)
	}
	return token, base, nil
}

func BuildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	version := ""
	if info.Main.Path == backendModulePath {
		version = info.Main.Version
	} else {
		for _, dep := range info.Deps {
			if dep.Path == backendModulePath {
				version = dep.Version
				if dep.Replace != nil {
					version = dep.Replace.Version
				}
				break
			}
		}
	}
	if version == "(devel)" {
		return ""
	}
	return version
}

func shortHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	if i := strings.IndexByte(hostname, '.'); i > 0 {
		return hostname[:i]
	}
	return hostname
}
