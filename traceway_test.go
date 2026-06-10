package traceway

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestSymbolicatorProcessor(t *testing.T) {
	factory := SymbolicatorProcessor()
	if factory.Type().String() != "source_map_symbolicator" {
		t.Fatalf("unexpected component type %q", factory.Type())
	}
	if NewFactory().Type() != factory.Type() {
		t.Fatal("NewFactory must return the same component")
	}

	set := processortest.NewNopSettings(factory.Type())
	traces, err := factory.CreateTraces(context.Background(), set, factory.CreateDefaultConfig(), consumertest.NewNop())
	if err != nil {
		t.Fatalf("CreateTraces: %v", err)
	}
	if err := traces.Start(context.Background(), componenttest.NewNopHost()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := traces.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}
