package config

import (
	"context"
	"sync"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/traceway"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

type recordingExporter struct {
	mu     sync.Mutex
	bodies []string
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range records {
		e.bodies = append(e.bodies, r.Body().AsString())
	}
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error   { return nil }
func (e *recordingExporter) ForceFlush(context.Context) error { return nil }

func TestDisabledLoggingStillReachesSelfMonitoring(t *testing.T) {
	exporter := &recordingExporter{}
	t.Cleanup(traceway.Install(traceway.Providers{
		LoggerProvider: sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter))),
	}))
	previous := LoggingEnabled
	LoggingEnabled = false
	t.Cleanup(func() { LoggingEnabled = previous })

	Logf("ERROR: panic recovered: %s", "boom")
	Logln("WARNING: slow tick")

	if len(exporter.bodies) != 2 || exporter.bodies[0] != "ERROR: panic recovered: boom" || exporter.bodies[1] != "WARNING: slow tick" {
		t.Fatalf("quiet stdout must not silence self-monitoring logs, got %q", exporter.bodies)
	}
}
