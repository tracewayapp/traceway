package otelprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestFactoryCreatesComponents(t *testing.T) {
	factory := NewFactory()
	if factory.Type() != componentType {
		t.Fatalf("unexpected component type %q", factory.Type())
	}

	cfg := factory.CreateDefaultConfig()
	set := processortest.NewNopSettings(componentType)

	traces, err := factory.CreateTraces(context.Background(), set, cfg, consumertest.NewNop())
	if err != nil {
		t.Fatalf("CreateTraces: %v", err)
	}
	if err := traces.Start(context.Background(), componenttest.NewNopHost()); err != nil {
		t.Fatalf("traces Start: %v", err)
	}
	if err := traces.Shutdown(context.Background()); err != nil {
		t.Fatalf("traces Shutdown: %v", err)
	}

	logs, err := factory.CreateLogs(context.Background(), set, cfg, consumertest.NewNop())
	if err != nil {
		t.Fatalf("CreateLogs: %v", err)
	}
	if err := logs.Start(context.Background(), componenttest.NewNopHost()); err != nil {
		t.Fatalf("logs Start: %v", err)
	}
	if err := logs.Shutdown(context.Background()); err != nil {
		t.Fatalf("logs Shutdown: %v", err)
	}
}
